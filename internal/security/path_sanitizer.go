package security

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// PathSanitizer validates and sanitizes file paths to prevent directory traversal attacks.
// It ensures all file operations stay within the designated project root directory.
type PathSanitizer struct {
	projectRoot string
}

// NewPathSanitizer creates a new PathSanitizer with the specified project root.
// The project root is converted to an absolute path and cleaned.
// Returns an error if the project root cannot be resolved to an absolute path.
func NewPathSanitizer(projectRoot string) (*PathSanitizer, error) {
	if projectRoot == "" {
		return nil, fmt.Errorf("project root cannot be empty")
	}

	// Convert to absolute path and clean it
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve project root to absolute path: %w", err)
	}

	// Clean the path to normalize it (removes .., ., redundant separators)
	absRoot = filepath.Clean(absRoot)

	return &PathSanitizer{
		projectRoot: absRoot,
	}, nil
}

// ValidatePath validates that the given path is safe and within the project root.
// It returns the absolute, cleaned path if validation succeeds, or an error if:
// - The path cannot be resolved to an absolute path
// - The resolved path is outside the project root
// - The path contains directory traversal sequences that escape the project root
// - The path is a symlink that resolves to a location outside the project root
//
// The returned path is always an absolute, cleaned path suitable for file operations.
// On Windows, UNC paths (\\server\share) are rejected unless they resolve within
// the project root.
func (ps *PathSanitizer) ValidatePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// On Windows, reject UNC paths that start with \\ early
	if runtime.GOOS == "windows" && strings.HasPrefix(path, `\\`) {
		return "", fmt.Errorf("path '%s' is outside project root '%s'", path, ps.projectRoot)
	}

	// Convert to absolute path
	var absPath string
	var err error

	if filepath.IsAbs(path) {
		// Already absolute, just clean it
		absPath = filepath.Clean(path)

		// If it's an absolute path, it must be checked immediately
		// to ensure it's within the project root
		if !ps.isWithinRoot(absPath) {
			return "", fmt.Errorf("path '%s' is outside project root '%s'", path, ps.projectRoot)
		}
	} else {
		// Relative path - resolve relative to project root
		absPath, err = filepath.Abs(filepath.Join(ps.projectRoot, path))
		if err != nil {
			return "", fmt.Errorf("failed to resolve path to absolute: %w", err)
		}
		absPath = filepath.Clean(absPath)

		// Check if the resolved path is within the project root
		if !ps.isWithinRoot(absPath) {
			return "", fmt.Errorf("path '%s' is outside project root '%s'", path, ps.projectRoot)
		}
	}

	// Additional check: ensure the path doesn't contain ".." after resolution
	// This catches edge cases where symbolic links or other mechanisms might be used
	if strings.Contains(absPath, "..") {
		return "", fmt.Errorf("path '%s' contains directory traversal sequences", path)
	}

	// Resolve symlinks to get the canonical path.
	// If the path exists on disk, EvalSymlinks will follow all symlinks
	// and return the real path. This prevents symlink escape attacks where
	// a symlink inside the project root points to a file outside it.
	canonicalPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If the path doesn't exist (e.g., creating a new file),
		// walk up to find the deepest existing ancestor and resolve that.
		canonicalPath, err = resolveExistingAncestor(absPath)
		if err != nil {
			// If we can't resolve any ancestor, the cleaned absPath is acceptable
			// as long as it passed the string-based checks above.
			return absPath, nil
		}
	}

	canonicalPath = filepath.Clean(canonicalPath)

	// Verify the canonical (symlink-resolved) path is still within project root.
	// We also need a canonical project root for a fair comparison.
	canonicalRoot := ps.projectRoot
	if resolved, err := filepath.EvalSymlinks(ps.projectRoot); err == nil {
		canonicalRoot = filepath.Clean(resolved)
	}

	if !isWithinRootCanonical(canonicalPath, canonicalRoot) {
		return "", fmt.Errorf("path '%s' resolves to '%s' which is outside project root '%s'",
			path, canonicalPath, ps.projectRoot)
	}

	return absPath, nil
}

// resolveExistingAncestor walks up from the given path and resolves symlinks
// for the deepest existing ancestor, then appends the remaining path components.
// This is used when the full path does not exist yet (e.g., creating a new file).
func resolveExistingAncestor(absPath string) (string, error) {
	current := absPath
	var tail []string

	for {
		if _, err := os.Lstat(current); err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			// Re-join the non-existent tail components
			parts := append([]string{resolved}, tail...)
			return filepath.Join(parts...), nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root without finding an existing path
			return "", fmt.Errorf("no existing ancestor found for path: %s", absPath)
		}
		tail = append([]string{filepath.Base(current)}, tail...)
		current = parent
	}
}

// isWithinRootCanonical checks if the given canonical path is within the canonical root.
func isWithinRootCanonical(absPath, root string) bool {
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return false
	}

	// If the relative path starts with "..", it's outside the root
	if strings.HasPrefix(rel, "..") {
		return false
	}

	// Handle Windows paths with forward slash conversion
	relForward := filepath.ToSlash(rel)
	if strings.HasPrefix(relForward, "../") || relForward == ".." {
		return false
	}

	return true
}

// IsPathSafe performs a boolean check to determine if a path is safe.
// It returns true if the path passes validation, false otherwise.
// This is a convenience method that wraps ValidatePath for cases where
// only a boolean result is needed without error details.
func (ps *PathSanitizer) IsPathSafe(path string) bool {
	_, err := ps.ValidatePath(path)
	return err == nil
}

// isWithinRoot checks if the given absolute path is within the project root.
// It uses filepath.Rel to determine the relationship between the paths.
// A path is considered within the root if:
// - The relative path doesn't start with ".."
// - The relative path is not "." (which would mean they're the same)
func (ps *PathSanitizer) isWithinRoot(absPath string) bool {
	// Use filepath.Rel to get the relative path from project root to the target
	rel, err := filepath.Rel(ps.projectRoot, absPath)
	if err != nil {
		return false
	}

	// If the relative path starts with "..", it's outside the root
	if strings.HasPrefix(rel, "..") {
		return false
	}

	// If the relative path starts with ".." after converting to forward slashes
	// (handles Windows paths)
	relForward := filepath.ToSlash(rel)
	if strings.HasPrefix(relForward, "../") || relForward == ".." {
		return false
	}

	return true
}

// GetProjectRoot returns the project root path used by this sanitizer.
func (ps *PathSanitizer) GetProjectRoot() string {
	return ps.projectRoot
}
