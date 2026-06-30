package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/safety"
)

const ProjectLockRelativePath = ".nexdev/run/project.lock"

var ErrProjectLockHeld = errors.New("project lock is already held")
var ErrProjectLockStale = errors.New("project lock is stale and requires manual recovery")

type StaleLockPolicy struct {
	MaxAge time.Duration
	Now    func() time.Time
}

type ProjectLockMetadata struct {
	PID        int
	AcquiredAt time.Time
	Raw        string
}

type ProjectLock struct {
	path string
	file *os.File
}

func ProjectLockPath(projectRoot string) (string, error) {
	sanitizer, err := safety.NewPathSanitizer(projectRoot)
	if err != nil {
		return "", err
	}
	return sanitizer.ValidateWrite(ProjectLockRelativePath)
}

func AcquireProjectLock(projectRoot string) (*ProjectLock, error) {
	return AcquireProjectLockWithPolicy(projectRoot, StaleLockPolicy{})
}

func AcquireProjectLockWithPolicy(projectRoot string, policy StaleLockPolicy) (*ProjectLock, error) {
	path, err := ProjectLockPath(projectRoot)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create project lock directory: %w", err)
	}
	path, err = ProjectLockPath(projectRoot)
	if err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			if isProjectLockStale(path, policy) {
				return nil, ErrProjectLockStale
			}
			return nil, ErrProjectLockHeld
		}
		return nil, fmt.Errorf("create project lock: %w", err)
	}

	lock := &ProjectLock{path: path, file: file}
	if _, err := fmt.Fprintf(file, "pid=%d\nacquired_at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		_ = lock.Release()
		return nil, fmt.Errorf("write project lock metadata: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = lock.Release()
		return nil, fmt.Errorf("sync project lock metadata: %w", err)
	}

	return lock, nil
}

func ReadProjectLockMetadata(projectRoot string) (ProjectLockMetadata, error) {
	path, err := ProjectLockPath(projectRoot)
	if err != nil {
		return ProjectLockMetadata{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ProjectLockMetadata{}, err
	}
	return parseProjectLockMetadata(string(data)), nil
}

func isProjectLockStale(path string, policy StaleLockPolicy) bool {
	if policy.MaxAge <= 0 {
		return false
	}
	now := policy.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	metadata := parseProjectLockMetadata(string(data))
	if metadata.AcquiredAt.IsZero() {
		return false
	}
	return now().UTC().Sub(metadata.AcquiredAt) > policy.MaxAge
}

func parseProjectLockMetadata(raw string) ProjectLockMetadata {
	metadata := ProjectLockMetadata{Raw: raw}
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "pid":
			if pid, err := strconv.Atoi(value); err == nil {
				metadata.PID = pid
			}
		case "acquired_at":
			if at, err := time.Parse(time.RFC3339Nano, value); err == nil {
				metadata.AcquiredAt = at.UTC()
			}
		}
	}
	return metadata
}

func (l *ProjectLock) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Release closes and removes the lock file. Stale lock policy is deterministic:
// callers may detect old metadata, but Nexdev does not probe processes or delete
// stale locks automatically because doing so can race another local owner.
func (l *ProjectLock) Release() error {
	if l == nil {
		return nil
	}
	var closeErr error
	if l.file != nil {
		closeErr = l.file.Close()
		l.file = nil
	}
	removeErr := os.Remove(l.path)
	if removeErr != nil && errors.Is(removeErr, os.ErrNotExist) {
		removeErr = nil
	}
	if closeErr != nil {
		return fmt.Errorf("close project lock: %w", closeErr)
	}
	if removeErr != nil {
		return fmt.Errorf("remove project lock: %w", removeErr)
	}
	return nil
}
