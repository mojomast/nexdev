package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mojomast/nexdev/internal/controlplane"
	"github.com/mojomast/nexdev/internal/git"
)

func TestOpenRuntimeDefaultsCreateProjectAndReleaseLock(t *testing.T) {
	root := t.TempDir()
	rt, err := OpenRuntime(context.Background(), Options{ProjectDir: root}, true)
	if err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(root, git.ProjectLockRelativePath)
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock not acquired: %v", err)
	}
	if rt.ProjectID == "" {
		t.Fatal("project id was not initialized")
	}
	if _, err := rt.Store.GetProjectWithContext(context.Background(), rt.ProjectID); err != nil {
		t.Fatalf("project not persisted: %v", err)
	}
	if err := rt.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lockPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock not released, stat err=%v", err)
	}
}

func TestServerConfigRejectsRemoteBindWithoutAuthBeforeListen(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "nexdev.yaml"), []byte("controlplane:\n  bind: 0.0.0.0\n  auth_required: 'false'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := OpenRuntime(context.Background(), Options{ProjectDir: root}, false)
	if err == nil {
		t.Fatal("expected remote bind without auth to fail")
	}
}

func TestCreateAuthTokenStoresHashOnly(t *testing.T) {
	root := t.TempDir()
	rt, err := OpenRuntime(context.Background(), Options{ProjectDir: root}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	plain, record, err := rt.CreateAuthToken(context.Background(), controlplane.RoleOperator, "test", 0)
	if err != nil {
		t.Fatal(err)
	}
	if plain == "" || record.TokenHash == "" || plain == record.TokenHash {
		t.Fatalf("unexpected token/hash plain=%q hash=%q", plain, record.TokenHash)
	}
	stored, err := rt.Store.GetAuthToken(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.TokenHash == plain {
		t.Fatal("stored plaintext token")
	}
}
