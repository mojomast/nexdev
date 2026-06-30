package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mojomast/nexdev/internal/safety"
)

const ProjectLockRelativePath = ".nexdev/run/project.lock"

var ErrProjectLockHeld = errors.New("project lock is already held")

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

func (l *ProjectLock) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Release closes and removes the lock file. Stale lock detection is intentionally
// deferred; an existing lock file is treated as held until a later M15 hardening task.
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
