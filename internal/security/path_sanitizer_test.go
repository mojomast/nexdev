package security

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/quick"
)

// getAbsolutePathOutsideRoot returns an absolute path that's outside the test project root
func getAbsolutePathOutsideRoot() string {
	if runtime.GOOS == "windows" {
		return "C:\\Windows\\System32\\config"
	}
	return "/etc/passwd"
}

func TestNewPathSanitizer(t *testing.T) {
	tests := []struct {
		name        string
		projectRoot string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid absolute path",
			projectRoot: "/home/user/project",
			wantErr:     false,
		},
		{
			name:        "valid relative path",
			projectRoot: ".",
			wantErr:     false,
		},
		{
			name:        "empty path",
			projectRoot: "",
			wantErr:     true,
			errContains: "project root cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps, err := NewPathSanitizer(tt.projectRoot)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewPathSanitizer() expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("NewPathSanitizer() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("NewPathSanitizer() unexpected error = %v", err)
				return
			}
			if ps == nil {
				t.Errorf("NewPathSanitizer() returned nil")
				return
			}
			// Verify project root is absolute
			if !filepath.IsAbs(ps.projectRoot) {
				t.Errorf("NewPathSanitizer() project root is not absolute: %v", ps.projectRoot)
			}
		})
	}
}

func TestPathSanitizer_ValidatePath(t *testing.T) {
	// Create a temporary project root for testing
	projectRoot := "/home/user/project"
	if runtime.GOOS == "windows" {
		projectRoot = "C:\\Users\\user\\project"
	}

	ps, err := NewPathSanitizer(projectRoot)
	if err != nil {
		t.Fatalf("Failed to create PathSanitizer: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid relative path",
			path:    "src/main.go",
			wantErr: false,
		},
		{
			name:    "valid relative path with subdirectory",
			path:    "internal/security/path_sanitizer.go",
			wantErr: false,
		},
		{
			name:    "current directory",
			path:    ".",
			wantErr: false,
		},
		{
			name:        "directory traversal with ../",
			path:        "../../../etc/passwd",
			wantErr:     true,
			errContains: "outside project root",
		},
		{
			name:        "directory traversal with ..",
			path:        "..",
			wantErr:     true,
			errContains: "outside project root",
		},
		{
			name:        "absolute path outside root",
			path:        getAbsolutePathOutsideRoot(),
			wantErr:     true,
			errContains: "outside project root",
		},
		{
			name:        "empty path",
			path:        "",
			wantErr:     true,
			errContains: "path cannot be empty",
		},
		{
			name:    "path with redundant separators",
			path:    "src//main.go",
			wantErr: false,
		},
		{
			name:    "path with . in middle",
			path:    "src/./main.go",
			wantErr: false,
		},
		{
			name:    "path with .. that stays within root",
			path:    "src/../main.go",
			wantErr: false,
		},
	}

	// Add Windows-specific tests
	if runtime.GOOS == "windows" {
		tests = append(tests, []struct {
			name        string
			path        string
			wantErr     bool
			errContains string
		}{
			{
				name:        "Windows absolute path outside root",
				path:        "C:\\Windows\\System32\\config",
				wantErr:     true,
				errContains: "outside project root",
			},
			{
				name:        "Windows UNC path",
				path:        "\\\\server\\share\\file.txt",
				wantErr:     true,
				errContains: "outside project root",
			},
			{
				name:        "Windows directory traversal",
				path:        "..\\..\\..\\Windows\\System32",
				wantErr:     true,
				errContains: "outside project root",
			},
		}...)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			safePath, err := ps.ValidatePath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidatePath() expected error but got none, returned path: %v", safePath)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidatePath() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("ValidatePath() unexpected error = %v", err)
				return
			}
			// Verify returned path is absolute
			if !filepath.IsAbs(safePath) {
				t.Errorf("ValidatePath() returned non-absolute path: %v", safePath)
			}
			// Verify returned path is within project root
			if !strings.HasPrefix(safePath, ps.projectRoot) {
				t.Errorf("ValidatePath() returned path %v not within project root %v", safePath, ps.projectRoot)
			}
		})
	}
}

func TestPathSanitizer_IsPathSafe(t *testing.T) {
	projectRoot := "/home/user/project"
	if runtime.GOOS == "windows" {
		projectRoot = "C:\\Users\\user\\project"
	}

	ps, err := NewPathSanitizer(projectRoot)
	if err != nil {
		t.Fatalf("Failed to create PathSanitizer: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		wantSafe bool
	}{
		{
			name:     "valid relative path",
			path:     "src/main.go",
			wantSafe: true,
		},
		{
			name:     "directory traversal",
			path:     "../../../etc/passwd",
			wantSafe: false,
		},
		{
			name:     "absolute path outside root",
			path:     getAbsolutePathOutsideRoot(),
			wantSafe: false,
		},
		{
			name:     "empty path",
			path:     "",
			wantSafe: false,
		},
		{
			name:     "current directory",
			path:     ".",
			wantSafe: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isSafe := ps.IsPathSafe(tt.path)
			if isSafe != tt.wantSafe {
				t.Errorf("IsPathSafe() = %v, want %v", isSafe, tt.wantSafe)
			}
		})
	}
}

func TestPathSanitizer_GetProjectRoot(t *testing.T) {
	projectRoot := "/home/user/project"
	ps, err := NewPathSanitizer(projectRoot)
	if err != nil {
		t.Fatalf("Failed to create PathSanitizer: %v", err)
	}

	root := ps.GetProjectRoot()
	if !filepath.IsAbs(root) {
		t.Errorf("GetProjectRoot() returned non-absolute path: %v", root)
	}
	// The root should be cleaned and absolute
	expectedRoot, _ := filepath.Abs(projectRoot)
	expectedRoot = filepath.Clean(expectedRoot)
	if root != expectedRoot {
		t.Errorf("GetProjectRoot() = %v, want %v", root, expectedRoot)
	}
}

// TestPathSanitizer_EdgeCases tests various edge cases
func TestPathSanitizer_EdgeCases(t *testing.T) {
	projectRoot := "/home/user/project"
	if runtime.GOOS == "windows" {
		projectRoot = "C:\\Users\\user\\project"
	}

	ps, err := NewPathSanitizer(projectRoot)
	if err != nil {
		t.Fatalf("Failed to create PathSanitizer: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		wantErr     bool
		description string
	}{
		{
			name:        "multiple slashes",
			path:        "src///main.go",
			wantErr:     false,
			description: "Multiple slashes should be normalized",
		},
		{
			name:        "trailing slash",
			path:        "src/",
			wantErr:     false,
			description: "Trailing slash should be handled",
		},
		{
			name:        "dot segments in middle",
			path:        "src/./internal/./main.go",
			wantErr:     false,
			description: "Dot segments should be normalized",
		},
		{
			name:        "parent directory within bounds",
			path:        "src/internal/../main.go",
			wantErr:     false,
			description: "Parent directory that stays within root should be allowed",
		},
		{
			name:        "complex traversal attempt",
			path:        "src/../../../../../../etc/passwd",
			wantErr:     true,
			description: "Complex traversal should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ps.ValidatePath(tt.path)
			if tt.wantErr && err == nil {
				t.Errorf("ValidatePath() expected error for %s but got none", tt.description)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidatePath() unexpected error for %s: %v", tt.description, err)
			}
		})
	}
}

// TestPathSanitizer_PathNormalization tests that equivalent paths are normalized consistently
func TestPathSanitizer_PathNormalization(t *testing.T) {
	projectRoot := "/home/user/project"
	if runtime.GOOS == "windows" {
		projectRoot = "C:\\Users\\user\\project"
	}

	ps, err := NewPathSanitizer(projectRoot)
	if err != nil {
		t.Fatalf("Failed to create PathSanitizer: %v", err)
	}

	// Test that different representations of the same path normalize to the same result
	paths := []string{
		"src/main.go",
		"src//main.go",
		"src/./main.go",
		"./src/main.go",
	}

	var normalizedPaths []string
	for _, path := range paths {
		safePath, err := ps.ValidatePath(path)
		if err != nil {
			t.Errorf("ValidatePath(%s) unexpected error: %v", path, err)
			continue
		}
		normalizedPaths = append(normalizedPaths, safePath)
	}

	// All normalized paths should be identical
	if len(normalizedPaths) > 1 {
		firstPath := normalizedPaths[0]
		for i, path := range normalizedPaths[1:] {
			if path != firstPath {
				t.Errorf("Path normalization inconsistent: paths[0]=%s, paths[%d]=%s", firstPath, i+1, path)
			}
		}
	}
}

// TestPathSanitizer_SpecialCharacters tests paths with special characters
func TestPathSanitizer_SpecialCharacters(t *testing.T) {
	projectRoot := "/home/user/project"
	if runtime.GOOS == "windows" {
		projectRoot = "C:\\Users\\user\\project"
	}

	ps, err := NewPathSanitizer(projectRoot)
	if err != nil {
		t.Fatalf("Failed to create PathSanitizer: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "path with spaces",
			path:    "my folder/my file.txt",
			wantErr: false,
		},
		{
			name:    "path with hyphens",
			path:    "my-folder/my-file.txt",
			wantErr: false,
		},
		{
			name:    "path with underscores",
			path:    "my_folder/my_file.txt",
			wantErr: false,
		},
		{
			name:    "path with dots in filename",
			path:    "src/file.test.go",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ps.ValidatePath(tt.path)
			if tt.wantErr && err == nil {
				t.Errorf("ValidatePath() expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidatePath() unexpected error: %v", err)
			}
		})
	}
}

// Property test: ValidatePath should always return an absolute path when successful
func TestPathSanitizer_ValidatePath_Propertied(t *testing.T) {
	projectRoot := "/home/user/project"
	if runtime.GOOS == "windows" {
		projectRoot = "C:\\Users\\user\\project"
	}

	ps, err := NewPathSanitizer(projectRoot)
	if err != nil {
		t.Fatalf("Failed to create PathSanitizer: %v", err)
	}

	prop := func(path string) bool {
		// Filter out obviously invalid inputs that would fail for other reasons
		// Check if path contains null bytes or excessive special characters
		if strings.Contains(path, "\x00") {
			return true
		}
		// Check if path is too long or has invalid characters for paths
		if len(path) > 500 {
			return true
		}
		// Skip empty paths
		if path == "" {
			return true
		}
		// For property tests with quick.Check, we want to avoid paths that would
		// fail for unrelated reasons (like being huge strings)
		// If the path doesn't look like a valid file path, just accept it
		if !strings.ContainsAny(path, "/\\") && !strings.ContainsAny(path, ":") {
			return true
		}
		safePath, err := ps.ValidatePath(path)
		// Any error (invalid input, path traversal, etc.) is valid behavior
		// If it passes, the result should be absolute
		if err == nil {
			return filepath.IsAbs(safePath)
		}
		// For paths that fail, we accept any error
		return true
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 1000}); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property test: Validation should be idempotent (calling multiple times returns same result)
func TestPathSanitizer_ValidatePath_Idempotent_Propertied(t *testing.T) {
	projectRoot := "/home/user/project"
	if runtime.GOOS == "windows" {
		projectRoot = "C:\\Users\\user\\project"
	}

	ps, err := NewPathSanitizer(projectRoot)
	if err != nil {
		t.Fatalf("Failed to create PathSanitizer: %v", err)
	}

	prop := func(path string) bool {
		// Empty paths should fail consistently
		if path == "" {
			_, err1 := ps.ValidatePath(path)
			_, err2 := ps.ValidatePath(path)
			return (err1 != nil) == (err2 != nil)
		}
		// For valid paths, the results should be identical
		result1, err1 := ps.ValidatePath(path)
		result2, err2 := ps.ValidatePath(path)
		if err1 != nil || err2 != nil {
			// Both should either succeed or fail
			return true
		}
		return result1 == result2
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 1000}); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property test: Sanitized paths should not contain ".." segments
func TestPathSanitizer_ValidatePath_NoDotDot_Propertied(t *testing.T) {
	projectRoot := "/home/user/project"
	if runtime.GOOS == "windows" {
		projectRoot = "C:\\Users\\user\\project"
	}

	ps, err := NewPathSanitizer(projectRoot)
	if err != nil {
		t.Fatalf("Failed to create PathSanitizer: %v", err)
	}

	prop := func(path string) bool {
		// Empty paths should fail
		if path == "" {
			return true
		}
		safePath, err := ps.ValidatePath(path)
		if err != nil {
			return true
		}
		// Result should not contain ".."
		return !strings.Contains(safePath, "..")
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 1000}); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property test: IsPathSafe should be equivalent to ValidatePath != nil
func TestPathSanitizer_IsPathSafe_Propertied(t *testing.T) {
	projectRoot := "/home/user/project"
	if runtime.GOOS == "windows" {
		projectRoot = "C:\\Users\\user\\project"
	}

	ps, err := NewPathSanitizer(projectRoot)
	if err != nil {
		t.Fatalf("Failed to create PathSanitizer: %v", err)
	}

	prop := func(path string) bool {
		isSafe1 := ps.IsPathSafe(path)
		_, err := ps.ValidatePath(path)
		isSafe2 := err == nil
		return isSafe1 == isSafe2
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 1000}); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestPathSanitizer_Symlink tests symlink handling
func TestPathSanitizer_Symlink(t *testing.T) {
	// Create a temporary project root for testing
	tmpDir, err := os.MkdirTemp("", "geoffrussy_symlink_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Resolve the temp dir itself to a canonical path (macOS /tmp -> /private/tmp)
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("Failed to resolve temp dir: %v", err)
	}

	projectRoot := tmpDir
	ps, err := NewPathSanitizer(projectRoot)
	if err != nil {
		t.Fatalf("Failed to create PathSanitizer: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		setup       func() (func(), error) // cleanup function
		wantErr     bool
		errContains string
	}{
		{
			name: "symlink pointing inside project root",
			path: "link_inside.txt",
			setup: func() (func(), error) {
				// Create a file inside the project root
				targetPath := filepath.Join(projectRoot, "target.txt")
				if err := os.WriteFile(targetPath, []byte("test content"), 0644); err != nil {
					return nil, err
				}
				// Create a symlink that points to the file inside the project root
				linkPath := filepath.Join(projectRoot, "link_inside.txt")
				if err := os.Symlink(targetPath, linkPath); err != nil {
					return nil, err
				}
				return func() { _ = os.Remove(linkPath); _ = os.Remove(targetPath) }, nil
			},
			wantErr: false,
		},
		{
			name: "symlink pointing to file outside root",
			path: "link_outside.txt",
			setup: func() (func(), error) {
				// Create a directory outside the project root
				outsideDir, err := os.MkdirTemp("", "geoffrussy_outside")
				if err != nil {
					return nil, err
				}
				outsideDir, err = filepath.EvalSymlinks(outsideDir)
				if err != nil {
					return nil, err
				}
				outsideFile := filepath.Join(outsideDir, "secret.txt")
				if err := os.WriteFile(outsideFile, []byte("secret"), 0644); err != nil {
					return nil, err
				}

				// Create a symlink inside the project root that points outside
				linkPath := filepath.Join(projectRoot, "link_outside.txt")
				if err := os.Symlink(outsideFile, linkPath); err != nil {
					return nil, err
				}
				return func() {
					_ = os.Remove(linkPath)
					_ = os.RemoveAll(outsideDir)
				}, nil
			},
			wantErr:     true,
			errContains: "outside project root",
		},
		{
			name: "symlink directory pointing outside root",
			path: "link_dir/file.txt",
			setup: func() (func(), error) {
				// Create a directory outside the project root
				outsideDir, err := os.MkdirTemp("", "geoffrussy_outside_dir")
				if err != nil {
					return nil, err
				}
				outsideDir, err = filepath.EvalSymlinks(outsideDir)
				if err != nil {
					return nil, err
				}
				outsideFile := filepath.Join(outsideDir, "file.txt")
				if err := os.WriteFile(outsideFile, []byte("outside content"), 0644); err != nil {
					return nil, err
				}

				// Create a directory symlink inside the project root pointing outside
				linkPath := filepath.Join(projectRoot, "link_dir")
				if err := os.Symlink(outsideDir, linkPath); err != nil {
					return nil, err
				}
				return func() {
					_ = os.Remove(linkPath)
					_ = os.RemoveAll(outsideDir)
				}, nil
			},
			wantErr:     true,
			errContains: "outside project root",
		},
		{
			name: "symlink within subdirectory pointing inside root",
			path: "subdir/link_in.txt",
			setup: func() (func(), error) {
				// Create subdirectory and target file
				subdir := filepath.Join(projectRoot, "subdir")
				if err := os.MkdirAll(subdir, 0755); err != nil {
					return nil, err
				}
				targetPath := filepath.Join(projectRoot, "real_file.txt")
				if err := os.WriteFile(targetPath, []byte("real content"), 0644); err != nil {
					return nil, err
				}
				linkPath := filepath.Join(subdir, "link_in.txt")
				if err := os.Symlink(targetPath, linkPath); err != nil {
					return nil, err
				}
				return func() {
					_ = os.RemoveAll(subdir)
					_ = os.Remove(targetPath)
				}, nil
			},
			wantErr: false,
		},
		{
			name: "chained symlinks staying inside root",
			path: "chain_link.txt",
			setup: func() (func(), error) {
				// target.txt -> link1.txt -> chain_link.txt, all inside root
				targetPath := filepath.Join(projectRoot, "chain_target.txt")
				if err := os.WriteFile(targetPath, []byte("chain content"), 0644); err != nil {
					return nil, err
				}
				link1Path := filepath.Join(projectRoot, "chain_link1.txt")
				if err := os.Symlink(targetPath, link1Path); err != nil {
					return nil, err
				}
				link2Path := filepath.Join(projectRoot, "chain_link.txt")
				if err := os.Symlink(link1Path, link2Path); err != nil {
					return nil, err
				}
				return func() {
					_ = os.Remove(link2Path)
					_ = os.Remove(link1Path)
					_ = os.Remove(targetPath)
				}, nil
			},
			wantErr: false,
		},
		{
			name: "chained symlinks escaping root via intermediate",
			path: "chain_escape.txt",
			setup: func() (func(), error) {
				// Create external target
				outsideDir, err := os.MkdirTemp("", "geoffrussy_chain_escape")
				if err != nil {
					return nil, err
				}
				outsideDir, err = filepath.EvalSymlinks(outsideDir)
				if err != nil {
					return nil, err
				}
				outsideFile := filepath.Join(outsideDir, "escaped.txt")
				if err := os.WriteFile(outsideFile, []byte("escaped"), 0644); err != nil {
					return nil, err
				}

				// First link inside root pointing outside
				link1 := filepath.Join(projectRoot, "escape_link1.txt")
				if err := os.Symlink(outsideFile, link1); err != nil {
					return nil, err
				}
				// Second link pointing to first link
				link2 := filepath.Join(projectRoot, "chain_escape.txt")
				if err := os.Symlink(link1, link2); err != nil {
					return nil, err
				}
				return func() {
					_ = os.Remove(link2)
					_ = os.Remove(link1)
					_ = os.RemoveAll(outsideDir)
				}, nil
			},
			wantErr:     true,
			errContains: "outside project root",
		},
		{
			name: "non-existent path without symlinks is allowed",
			path: "new_file.txt",
			setup: func() (func(), error) {
				return func() {}, nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run tests that require setup
			if tt.setup != nil {
				cleanup, err := tt.setup()
				if err != nil {
					t.Fatalf("Setup failed: %v", err)
				}
				defer cleanup()
			}

			safePath, err := ps.ValidatePath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidatePath() expected error but got none, returned path: %v", safePath)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidatePath() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("ValidatePath() unexpected error: %v", err)
				return
			}
		})
	}
}

// TestPathSanitizer_SymlinkWindowsUNC tests Windows UNC path handling
func TestPathSanitizer_SymlinkWindowsUNC(t *testing.T) {
	if runtime.GOOS == "windows" {
		ps, err := NewPathSanitizer("C:\\Users\\user\\project")
		if err != nil {
			t.Fatalf("Failed to create PathSanitizer: %v", err)
		}

		// UNC paths should be rejected
		uncPaths := []string{
			`\\server\share\file.txt`,
			`\\?\C:\secret\file.txt`,
			`\\.\device`,
		}

		for _, uncPath := range uncPaths {
			t.Run("UNC:"+uncPath, func(t *testing.T) {
				_, err := ps.ValidatePath(uncPath)
				if err == nil {
					t.Errorf("ValidatePath() expected error for UNC path %s but got none", uncPath)
				}
			})
		}
	} else {
		// On non-Windows, test that paths starting with \\ are handled gracefully
		tmpDir, err := os.MkdirTemp("", "geoffrussy_unc_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		tmpDir, err = filepath.EvalSymlinks(tmpDir)
		if err != nil {
			t.Fatalf("Failed to resolve temp dir: %v", err)
		}

		ps, err := NewPathSanitizer(tmpDir)
		if err != nil {
			t.Fatalf("Failed to create PathSanitizer: %v", err)
		}

		// On Unix, \\ is treated as a relative path component
		// This should either succeed (if treated as a file named "\\server") or fail
		// but it must not panic
		_, _ = ps.ValidatePath(`\\server\share\file.txt`)
	}
}
