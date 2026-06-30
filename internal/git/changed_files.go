package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mojomast/nexdev/internal/safety"
)

var ErrEmptyBaseRef = errors.New("git diff base ref is empty")

type ChangedFile struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	OldPath string `json:"old_path,omitempty"`
}

func ChangedFiles(ctx context.Context, projectRoot, baseRef, headRef string) ([]ChangedFile, error) {
	if strings.TrimSpace(baseRef) == "" {
		return nil, ErrEmptyBaseRef
	}
	args := []string{"diff", "--name-status", baseRef}
	if strings.TrimSpace(headRef) != "" {
		args = append(args, headRef)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = projectRoot
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git diff changed files: %w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("git diff changed files: %w", err)
	}
	return ParseChangedFiles(projectRoot, string(output))
}

func ParseChangedFiles(projectRoot, output string) ([]ChangedFile, error) {
	sanitizer, err := safety.NewPathSanitizer(projectRoot)
	if err != nil {
		return nil, err
	}
	var changed []ChangedFile
	for lineNo, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		status, err := normalizeNameStatus(fields[0])
		if err != nil {
			return nil, fmt.Errorf("parse git diff line %d: %w", lineNo+1, err)
		}
		if status == "R" {
			if len(fields) != 3 {
				return nil, fmt.Errorf("parse git diff line %d: rename requires old and new path", lineNo+1)
			}
			oldPath, err := sanitizeGitDiffPath(sanitizer, fields[1])
			if err != nil {
				return nil, fmt.Errorf("parse git diff line %d old path: %w", lineNo+1, err)
			}
			newPath, err := sanitizeGitDiffPath(sanitizer, fields[2])
			if err != nil {
				return nil, fmt.Errorf("parse git diff line %d path: %w", lineNo+1, err)
			}
			changed = append(changed, ChangedFile{Path: newPath, Status: status, OldPath: oldPath})
			continue
		}
		if len(fields) != 2 {
			return nil, fmt.Errorf("parse git diff line %d: status %q requires one path", lineNo+1, fields[0])
		}
		path, err := sanitizeGitDiffPath(sanitizer, fields[1])
		if err != nil {
			return nil, fmt.Errorf("parse git diff line %d path: %w", lineNo+1, err)
		}
		changed = append(changed, ChangedFile{Path: path, Status: status})
	}
	sort.Slice(changed, func(i, j int) bool {
		if changed[i].Path == changed[j].Path {
			return changed[i].Status < changed[j].Status
		}
		return changed[i].Path < changed[j].Path
	})
	return changed, nil
}

func normalizeNameStatus(status string) (string, error) {
	status = strings.TrimSpace(status)
	switch {
	case status == "A" || status == "M" || status == "D":
		return status, nil
	case strings.HasPrefix(status, "R"):
		return "R", nil
	default:
		return "", fmt.Errorf("unsupported status %q", status)
	}
}

func sanitizeGitDiffPath(sanitizer *safety.PathSanitizer, path string) (string, error) {
	abs, err := sanitizer.ValidateWrite(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(sanitizer.Root(), abs)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}
