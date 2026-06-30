package safety

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PathSanitizer struct {
	root      string
	denyGlobs []string
}

type PathOption func(*PathSanitizer)

func WithDenyGlobs(globs []string) PathOption {
	return func(s *PathSanitizer) {
		s.denyGlobs = append([]string{}, globs...)
	}
}

func NewPathSanitizer(projectRoot string, opts ...PathOption) (*PathSanitizer, error) {
	if projectRoot == "" {
		return nil, errors.New("project root cannot be empty")
	}
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}
	root = filepath.Clean(root)
	s := &PathSanitizer{root: root, denyGlobs: []string{".git/**", ".git"}}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

func (s *PathSanitizer) ValidateWrite(path string) (string, error) {
	if path == "" {
		return "", errors.New("path cannot be empty")
	}
	clean := filepath.Clean(path)
	var abs string
	if filepath.IsAbs(clean) {
		abs = clean
	} else {
		abs = filepath.Join(s.root, clean)
	}
	abs = filepath.Clean(abs)
	if !withinRoot(s.root, abs) {
		return "", fmt.Errorf("path %q escapes project root", path)
	}
	rel, err := filepath.Rel(s.root, abs)
	if err != nil {
		return "", fmt.Errorf("resolve relative path: %w", err)
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return abs, nil
	}
	if isGitPath(rel) {
		return "", fmt.Errorf("writes to .git are denied: %q", path)
	}
	if matchesAnyDenyGlob(rel, s.denyGlobs) {
		return "", fmt.Errorf("path %q is denied by policy", path)
	}
	resolved, err := resolvePathForWrite(abs)
	if err != nil {
		return "", err
	}
	root := s.root
	if realRoot, err := filepath.EvalSymlinks(s.root); err == nil {
		root = filepath.Clean(realRoot)
	}
	if !withinRoot(root, resolved) {
		return "", fmt.Errorf("path %q resolves outside project root", path)
	}
	return abs, nil
}

func (s *PathSanitizer) Root() string {
	return s.root
}

func ValidateWritePath(projectRoot, path string, denyGlobs []string) (string, error) {
	s, err := NewPathSanitizer(projectRoot, WithDenyGlobs(denyGlobs))
	if err != nil {
		return "", err
	}
	return s.ValidateWrite(path)
}

func withinRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, "../"))
}

func isGitPath(rel string) bool {
	return rel == ".git" || strings.HasPrefix(rel, ".git/")
}

func matchesAnyDenyGlob(rel string, globs []string) bool {
	for _, glob := range globs {
		glob = filepath.ToSlash(filepath.Clean(glob))
		if glob == "." || glob == "" {
			continue
		}
		if glob == rel {
			return true
		}
		if strings.HasSuffix(glob, "/**") {
			prefix := strings.TrimSuffix(glob, "/**")
			if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
				return true
			}
			continue
		}
		if ok, _ := filepath.Match(glob, rel); ok {
			return true
		}
	}
	return false
}

func resolvePathForWrite(abs string) (string, error) {
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(resolved), nil
	}
	current := abs
	tail := []string{}
	for {
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				resolved, err := filepath.EvalSymlinks(current)
				if err != nil {
					return "", fmt.Errorf("resolve symlink ancestor: %w", err)
				}
				parts := append([]string{resolved}, tail...)
				return filepath.Clean(filepath.Join(parts...)), nil
			}
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", fmt.Errorf("resolve existing ancestor: %w", err)
			}
			parts := append([]string{resolved}, tail...)
			return filepath.Clean(filepath.Join(parts...)), nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(abs), nil
		}
		tail = append([]string{filepath.Base(current)}, tail...)
		current = parent
	}
}
