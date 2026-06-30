package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mojomast/nexdev/internal/controlplane"
	"github.com/mojomast/nexdev/internal/git"
	"github.com/mojomast/nexdev/internal/state"
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

func TestRunFakeProviderCompletesAndWritesArtifacts(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Fixture\nUse local-first defaults and redact sk-testsecret1234567890.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("API_KEY=sk-envsecret1234567890\n"), 0600); err != nil {
		t.Fatal(err)
	}
	rt, err := OpenRuntime(context.Background(), Options{ProjectDir: root}, true)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	result, err := rt.RunFakeProvider(context.Background(), RunRequest{Prompt: "build safely with token=sk-promptsecret1234567890", Yes: true, FakeProvider: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" || result.RunID == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	for _, rel := range []string{
		".nexdev/artifacts/repo_analysis.json",
		".nexdev/artifacts/devplan.md",
		".nexdev/artifacts/verify_report.json",
		".nexdev/artifacts/changed_files.json",
		".nexdev/artifacts/run_summary.json",
		".nexdev/artifacts/handoff.md",
		"generated/fake_e2e.txt",
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("expected %s: %v", rel, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, ".nexdev/artifacts/changed_files.json"))
	if err != nil {
		t.Fatal(err)
	}
	var changed struct {
		ChangedFiles []struct {
			Path string `json:"path"`
		} `json:"changed_files"`
	}
	if err := json.Unmarshal(data, &changed); err != nil {
		t.Fatal(err)
	}
	if len(changed.ChangedFiles) != 1 || changed.ChangedFiles[0].Path != "generated/fake_e2e.txt" {
		t.Fatalf("unexpected changed files: %s", data)
	}
	events, err := rt.Store.ListEvents(context.Background(), state.EventListOptions{RunID: result.RunID})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 || events[len(events)-1].Type != "done" {
		t.Fatalf("expected done event, got %d events", len(events))
	}
	for _, rel := range []string{".nexdev/artifacts/repo_analysis.json", ".nexdev/artifacts/handoff.md"} {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "sk-promptsecret") || strings.Contains(string(data), "sk-envsecret") || strings.Contains(string(data), "sk-testsecret") {
			t.Fatalf("secret leaked in %s: %s", rel, data)
		}
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
