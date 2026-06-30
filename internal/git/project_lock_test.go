package git

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectLockPath(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, ".nexdev", "run", "project.lock")

	got, err := ProjectLockPath(root)
	if err != nil {
		t.Fatalf("ProjectLockPath returned error: %v", err)
	}
	if got != want {
		t.Fatalf("ProjectLockPath = %q, want %q", got, want)
	}
}

func TestAcquireProjectLock(t *testing.T) {
	root := t.TempDir()

	lock, err := AcquireProjectLock(root)
	if err != nil {
		t.Fatalf("AcquireProjectLock returned error: %v", err)
	}
	defer lock.Release()

	if lock.Path() != filepath.Join(root, ".nexdev", "run", "project.lock") {
		t.Fatalf("lock path = %q", lock.Path())
	}
	data, err := os.ReadFile(lock.Path())
	if err != nil {
		t.Fatalf("read lock metadata: %v", err)
	}
	metadata := string(data)
	if !strings.Contains(metadata, "pid=") || !strings.Contains(metadata, "acquired_at=") {
		t.Fatalf("lock metadata missing pid/timestamp: %q", metadata)
	}

	second, err := AcquireProjectLock(root)
	if !errors.Is(err, ErrProjectLockHeld) {
		if second != nil {
			_ = second.Release()
		}
		t.Fatalf("second acquire error = %v, want ErrProjectLockHeld", err)
	}
}

func TestProjectLockReleaseAllowsReacquire(t *testing.T) {
	root := t.TempDir()

	lock, err := AcquireProjectLock(root)
	if err != nil {
		t.Fatalf("AcquireProjectLock returned error: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("Release returned error: %v", err)
	}

	reacquired, err := AcquireProjectLock(root)
	if err != nil {
		t.Fatalf("reacquire returned error: %v", err)
	}
	defer reacquired.Release()
}

func TestProjectLockRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".nexdev")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	if _, err := ProjectLockPath(root); err == nil {
		t.Fatal("ProjectLockPath succeeded for symlink escape")
	}
	if lock, err := AcquireProjectLock(root); err == nil {
		_ = lock.Release()
		t.Fatal("AcquireProjectLock succeeded for symlink escape")
	}
}
