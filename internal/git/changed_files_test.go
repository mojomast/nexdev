package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseChangedFilesParsesStatusesAndRename(t *testing.T) {
	root := t.TempDir()
	got, err := ParseChangedFiles(root, "A\tadded.txt\nM\tmodified.txt\nD\tdeleted.txt\nR100\told/name.txt\tnew/name.txt\n")
	if err != nil {
		t.Fatalf("ParseChangedFiles failed: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 changed files, got %#v", got)
	}
	byPath := map[string]ChangedFile{}
	for _, file := range got {
		byPath[file.Path] = file
	}
	for path, status := range map[string]string{"added.txt": "A", "modified.txt": "M", "deleted.txt": "D", "new/name.txt": "R"} {
		if byPath[path].Status != status {
			t.Fatalf("path %s status = %q, want %q in %#v", path, byPath[path].Status, status, got)
		}
	}
	if byPath["new/name.txt"].OldPath != "old/name.txt" {
		t.Fatalf("rename old path not preserved: %#v", byPath["new/name.txt"])
	}
}

func TestParseChangedFilesRejectsPathOutsideProjectRoot(t *testing.T) {
	_, err := ParseChangedFiles(t.TempDir(), "M\t../outside.txt\n")
	if err == nil {
		t.Fatal("expected sanitizer rejection")
	}
}

func TestChangedFilesEmptyBaseRefFallsBackSignal(t *testing.T) {
	_, err := ChangedFiles(context.Background(), t.TempDir(), "", "HEAD")
	if !errors.Is(err, ErrEmptyBaseRef) {
		t.Fatalf("ChangedFiles error = %v, want ErrEmptyBaseRef", err)
	}
}

func TestChangedFilesFromTempGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "nexdev@example.invalid")
	runGit(t, root, "config", "user.name", "Nexdev Test")
	writeTestFile(t, root, "modified.txt", "before")
	writeTestFile(t, root, "deleted.txt", "delete me")
	writeTestFile(t, root, "old.txt", "rename me")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "base")
	writeTestFile(t, root, "added.txt", "new")
	writeTestFile(t, root, "modified.txt", "after")
	if err := os.Remove(filepath.Join(root, "deleted.txt")); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "mv", "old.txt", "renamed.txt")
	runGit(t, root, "add", "added.txt")

	got, err := ChangedFiles(ctx, root, "HEAD", "")
	if err != nil {
		t.Fatalf("ChangedFiles failed: %v", err)
	}
	byPath := map[string]ChangedFile{}
	for _, file := range got {
		byPath[file.Path] = file
	}
	for path, status := range map[string]string{"added.txt": "A", "modified.txt": "M", "deleted.txt": "D", "renamed.txt": "R"} {
		if byPath[path].Status != status {
			t.Fatalf("path %s status = %q, want %q in %#v", path, byPath[path].Status, status, got)
		}
	}
	if byPath["renamed.txt"].OldPath != "old.txt" {
		t.Fatalf("rename old path not preserved: %#v", byPath["renamed.txt"])
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
