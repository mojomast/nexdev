package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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

func TestAcquireProjectLockLivePIDReturnsHeld(t *testing.T) {
	root := t.TempDir()
	lockPath := writeProjectLockForTest(t, root, os.Getpid())

	lock, err := AcquireProjectLock(root)
	if lock != nil {
		_ = lock.Release()
	}
	if !errors.Is(err, ErrProjectLockHeld) {
		t.Fatalf("AcquireProjectLock error = %v, want ErrProjectLockHeld", err)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("live lock should remain: %v", err)
	}
}

func TestAcquireProjectLockDeadPIDIsCleanedUp(t *testing.T) {
	root := t.TempDir()
	lockPath := writeProjectLockForTest(t, root, 12345)
	restore := stubProjectLockPIDLive(func(pid int) (bool, error) {
		if pid != 12345 {
			t.Fatalf("pid liveness check got %d, want 12345", pid)
		}
		return false, nil
	})
	defer restore()

	lock, err := AcquireProjectLock(root)
	if err != nil {
		t.Fatalf("AcquireProjectLock returned error: %v", err)
	}
	defer lock.Release()
	if lock.Path() != lockPath {
		t.Fatalf("lock path = %q, want %q", lock.Path(), lockPath)
	}
	metadata, err := ReadProjectLockMetadata(root)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.PID != os.Getpid() {
		t.Fatalf("metadata PID = %d, want current pid %d", metadata.PID, os.Getpid())
	}
}

func TestAcquireProjectLockMalformedPIDReturnsStale(t *testing.T) {
	root := t.TempDir()
	lockPath, err := ProjectLockPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, []byte("pid=not-a-pid\nacquired_at=2026-06-30T10:00:00Z\n"), 0600); err != nil {
		t.Fatal(err)
	}

	lock, err := AcquireProjectLock(root)
	if lock != nil {
		_ = lock.Release()
	}
	if !errors.Is(err, ErrProjectLockStale) {
		t.Fatalf("AcquireProjectLock error = %v, want ErrProjectLockStale", err)
	}
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("malformed lock should remain: %v", err)
	}
	if !strings.Contains(string(data), "pid=not-a-pid") {
		t.Fatalf("malformed lock was changed: %q", string(data))
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

func TestAcquireProjectLockWithTimeoutExitsAfterContextCancellation(t *testing.T) {
	root := t.TempDir()
	writeProjectLockForTest(t, root, os.Getpid())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	lock, err := AcquireProjectLockWithTimeout(ctx, root, time.Millisecond)
	if lock != nil {
		_ = lock.Release()
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("AcquireProjectLockWithTimeout error = %v, want context.Canceled", err)
	}
}

func writeProjectLockForTest(t *testing.T, root string, pid int) string {
	t.Helper()
	lockPath, err := ProjectLockPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0700); err != nil {
		t.Fatal(err)
	}
	acquiredAt := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	if err := os.WriteFile(lockPath, []byte("pid="+strconv.Itoa(pid)+"\nacquired_at="+acquiredAt.Format(time.RFC3339Nano)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return lockPath
}

func stubProjectLockPIDLive(fn func(int) (bool, error)) func() {
	original := projectLockPIDLive
	projectLockPIDLive = fn
	return func() { projectLockPIDLive = original }
}
