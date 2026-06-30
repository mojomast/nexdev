package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

type ProjectFixture struct {
	Root         string
	NexdevDir    string
	ArtifactsDir string
	StateDir     string
	ConfigPath   string
	ReadmePath   string
}

// TempProject creates a minimal project tree with no secret-bearing files.
func TempProject(t testing.TB) ProjectFixture {
	t.Helper()

	root := t.TempDir()
	fixture := ProjectFixture{
		Root:         root,
		NexdevDir:    filepath.Join(root, ".nexdev"),
		ArtifactsDir: filepath.Join(root, ".nexdev", "artifacts"),
		StateDir:     filepath.Join(root, ".nexdev", "state"),
		ConfigPath:   filepath.Join(root, "nexdev.yaml"),
		ReadmePath:   filepath.Join(root, "README.md"),
	}

	mustMkdirAll(t, fixture.ArtifactsDir)
	mustMkdirAll(t, fixture.StateDir)
	mustWriteFile(t, fixture.ConfigPath, []byte("project:\n  name: temp-project\ncontrolplane:\n  bind: 127.0.0.1\n"))
	mustWriteFile(t, fixture.ReadmePath, []byte("# Temp Project\n\nDeterministic test project fixture.\n"))

	return fixture
}

func mustMkdirAll(t testing.TB, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("create directory %s: %v", path, err)
	}
}

func mustWriteFile(t testing.TB, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
