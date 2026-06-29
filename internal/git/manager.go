package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Manager handles Git operations for the project
type Manager struct {
	repoPath string
}

// NewManager creates a new Git manager
func NewManager(repoPath string) *Manager {
	return &Manager{
		repoPath: repoPath,
	}
}

// IsRepository checks if the directory is a Git repository
func (m *Manager) IsRepository() (bool, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = m.repoPath
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 128 {
				// Not a git repository
				return false, nil
			}
		}
		return false, fmt.Errorf("failed to check if directory is a repository: %w", err)
	}
	return true, nil
}

// Initialize initializes a new Git repository
func (m *Manager) Initialize() error {
	cmd := exec.Command("git", "init")
	cmd.Dir = m.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// Stage stages files for commit
func (m *Manager) Stage(files []string) error {
	if len(files) == 0 {
		return fmt.Errorf("no files to stage")
	}

	args := append([]string{"add"}, files...)
	cmd := exec.Command("git", args...)
	cmd.Dir = m.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stage files: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// Commit creates a commit with the given message and metadata
func (m *Manager) Commit(message string, metadata map[string]string) error {
	if message == "" {
		return fmt.Errorf("commit message cannot be empty")
	}

	// Build commit message with metadata
	fullMessage := message
	if len(metadata) > 0 {
		fullMessage += "\n\n"
		for key, value := range metadata {
			fullMessage += fmt.Sprintf("%s: %s\n", key, value)
		}
	}

	cmd := exec.Command("git", "commit", "-m", fullMessage)
	cmd.Dir = m.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to commit: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// CommitFile stages and commits a single file
func (m *Manager) CommitFile(file string, message string, metadata map[string]string) error {
	if err := m.Stage([]string{file}); err != nil {
		return err
	}
	return m.Commit(message, metadata)
}

// CommitFiles stages and commits multiple files
func (m *Manager) CommitFiles(files []string, message string, metadata map[string]string) error {
	if err := m.Stage(files); err != nil {
		return err
	}
	return m.Commit(message, metadata)
}

// CreateTag creates a Git tag for checkpoints
func (m *Manager) CreateTag(tagName string, message string) error {
	if tagName == "" {
		return fmt.Errorf("tag name cannot be empty")
	}

	args := []string{"tag", "-a", tagName}
	if message != "" {
		args = append(args, "-m", message)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = m.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create tag: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// ResetToTag resets the repository to a specific tag
func (m *Manager) ResetToTag(tagName string) error {
	if tagName == "" {
		return fmt.Errorf("tag name cannot be empty")
	}

	cmd := exec.Command("git", "reset", "--hard", tagName)
	cmd.Dir = m.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to reset to tag: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// GetStatus returns the current Git status
func (m *Manager) GetStatus() (string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = m.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get status: %w\nOutput: %s", err, string(output))
	}
	return string(output), nil
}

// HasUncommittedChanges checks if there are uncommitted changes
func (m *Manager) HasUncommittedChanges() (bool, error) {
	status, err := m.GetStatus()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(status) != "", nil
}

// DetectConflicts checks for merge conflicts
func (m *Manager) DetectConflicts() (bool, []string, error) {
	status, err := m.GetStatus()
	if err != nil {
		return false, nil, err
	}

	lines := strings.Split(status, "\n")
	var conflicts []string
	hasConflicts := false

	for _, line := range lines {
		if strings.HasPrefix(line, "UU ") || strings.HasPrefix(line, "AA ") ||
			strings.HasPrefix(line, "DD ") || strings.HasPrefix(line, "AU ") ||
			strings.HasPrefix(line, "UA ") || strings.HasPrefix(line, "DU ") ||
			strings.HasPrefix(line, "UD ") {
			hasConflicts = true
			// Extract filename (skip status prefix)
			if len(line) > 3 {
				conflicts = append(conflicts, strings.TrimSpace(line[3:]))
			}
		}
	}

	return hasConflicts, conflicts, nil
}

// GetChangedFiles returns a list of changed files
func (m *Manager) GetChangedFiles() ([]string, error) {
	status, err := m.GetStatus()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(status, "\n")
	var files []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip status prefix (first 3 characters)
		if len(line) > 3 {
			files = append(files, strings.TrimSpace(line[3:]))
		}
	}

	return files, nil
}

// ListTags returns all tags in the repository
func (m *Manager) ListTags() ([]string, error) {
	cmd := exec.Command("git", "tag", "-l")
	cmd.Dir = m.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w\nOutput: %s", err, string(output))
	}

	tags := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(tags) == 1 && tags[0] == "" {
		return []string{}, nil
	}
	return tags, nil
}

// GetCurrentBranch returns the current branch name
func (m *Manager) GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = m.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w\nOutput: %s", err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// EnsureRepository ensures the directory is a Git repository, initializing if needed
func (m *Manager) EnsureRepository() error {
	isRepo, err := m.IsRepository()
	if err != nil {
		return err
	}

	if !isRepo {
		return m.Initialize()
	}

	return nil
}

// GetRepoPath returns the repository path
func (m *Manager) GetRepoPath() string {
	return m.repoPath
}

// SetRepoPath sets the repository path
func (m *Manager) SetRepoPath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if directory exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", absPath)
	}

	m.repoPath = absPath
	return nil
}

// CommitAll stages all changes and commits them
func (m *Manager) CommitAll(message string, metadata map[string]string) error {
	// Stage all changes
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = m.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stage all changes: %w\nOutput: %s", err, string(output))
	}

	// Check if there's anything to commit
	hasChanges, err := m.HasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("failed to check for uncommitted changes: %w", err)
	}

	if !hasChanges {
		// Nothing to commit, return success
		return nil
	}

	// Commit the changes
	return m.Commit(message, metadata)
}
