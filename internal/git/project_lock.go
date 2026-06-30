package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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

var projectLockPIDLive = defaultProjectLockPIDLive

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

func AcquireProjectLockWithTimeout(ctx context.Context, projectRoot string, pollInterval time.Duration) (*ProjectLock, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if pollInterval <= 0 {
		pollInterval = 100 * time.Millisecond
	}

	for {
		lock, err := AcquireProjectLock(projectRoot)
		if err == nil {
			return lock, nil
		}
		if !errors.Is(err, ErrProjectLockHeld) {
			return nil, err
		}

		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func AcquireProjectLockWithPolicy(projectRoot string, policy StaleLockPolicy) (*ProjectLock, error) {
	_ = policy
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

	for {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			if errors.Is(err, os.ErrExist) {
				if err := recoverExistingProjectLock(path); err != nil {
					return nil, err
				}
				continue
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

func recoverExistingProjectLock(path string) error {
	pid, err := readProjectLockPID(path)
	if err != nil {
		return ErrProjectLockStale
	}
	live, err := projectLockPIDLive(pid)
	if err != nil || live {
		return ErrProjectLockHeld
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale project lock: %w", err)
	}
	return nil
}

func readProjectLockPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || key != "pid" {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || pid <= 0 {
			return 0, ErrProjectLockStale
		}
		return pid, nil
	}
	return 0, ErrProjectLockStale
}

func defaultProjectLockPIDLive(pid int) (bool, error) {
	if pid <= 0 {
		return false, ErrProjectLockStale
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, nil
	}
	defer process.Release()

	if runtime.GOOS == "windows" {
		return true, nil
	}
	err = process.Signal(syscall.Signal(0))
	if err == nil || errors.Is(err, syscall.EPERM) {
		return true, nil
	}
	if errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
		return false, nil
	}
	return true, err
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

// Release closes and removes the lock file.
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
