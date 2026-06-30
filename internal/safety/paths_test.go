package safety

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPathSanitizerAllowsCleanRelativeWrite(t *testing.T) {
	root := t.TempDir()
	s, err := NewPathSanitizer(root)
	if err != nil {
		t.Fatalf("NewPathSanitizer() error = %v", err)
	}
	got, err := s.ValidateWrite("src/../src/main.go")
	if err != nil {
		t.Fatalf("ValidateWrite() error = %v", err)
	}
	want := filepath.Join(root, "src", "main.go")
	if got != want {
		t.Fatalf("ValidateWrite() = %q, want %q", got, want)
	}
}

func TestPathSanitizerRejectsTraversal(t *testing.T) {
	s, err := NewPathSanitizer(t.TempDir())
	if err != nil {
		t.Fatalf("NewPathSanitizer() error = %v", err)
	}
	_, err = s.ValidateWrite("../outside.txt")
	if err == nil || !strings.Contains(err.Error(), "escapes project root") {
		t.Fatalf("ValidateWrite() error = %v, want traversal rejection", err)
	}
}

func TestPathSanitizerRejectsAbsoluteOutsideRoot(t *testing.T) {
	s, err := NewPathSanitizer(t.TempDir())
	if err != nil {
		t.Fatalf("NewPathSanitizer() error = %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	_, err = s.ValidateWrite(outside)
	if err == nil || !strings.Contains(err.Error(), "escapes project root") {
		t.Fatalf("ValidateWrite() error = %v, want absolute outside root rejection", err)
	}
}

func TestPathSanitizerRejectsGitWrites(t *testing.T) {
	s, err := NewPathSanitizer(t.TempDir())
	if err != nil {
		t.Fatalf("NewPathSanitizer() error = %v", err)
	}
	_, err = s.ValidateWrite(".git/config")
	if err == nil || !strings.Contains(err.Error(), ".git") {
		t.Fatalf("ValidateWrite() error = %v, want .git rejection", err)
	}
}

func TestPathSanitizerRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	s, err := NewPathSanitizer(root)
	if err != nil {
		t.Fatalf("NewPathSanitizer() error = %v", err)
	}
	_, err = s.ValidateWrite("link/escape.txt")
	if err == nil || !strings.Contains(err.Error(), "outside project root") {
		t.Fatalf("ValidateWrite() error = %v, want symlink escape rejection", err)
	}
}

func TestPathSanitizerRejectsDenyGlob(t *testing.T) {
	s, err := NewPathSanitizer(t.TempDir(), WithDenyGlobs([]string{"secrets/**", "*.pem"}))
	if err != nil {
		t.Fatalf("NewPathSanitizer() error = %v", err)
	}
	for _, path := range []string{"secrets/token.txt", "key.pem"} {
		_, err := s.ValidateWrite(path)
		if err == nil || !strings.Contains(err.Error(), "denied") {
			t.Fatalf("ValidateWrite(%q) error = %v, want deny glob rejection", path, err)
		}
	}
}
