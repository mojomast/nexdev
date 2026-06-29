package checkpoint

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mojomast/nexdev/internal/git"
	"github.com/mojomast/nexdev/internal/state"
)

// Manager handles checkpoint creation and rollback
type Manager struct {
	store      *state.Store
	gitManager *git.Manager
	dataDir    string
}

// NewManager creates a new checkpoint manager
func NewManager(store *state.Store, gitManager *git.Manager, dataDir string) *Manager {
	return &Manager{
		store:      store,
		gitManager: gitManager,
		dataDir:    dataDir,
	}
}

// CreateCheckpoint creates a new checkpoint with the current state
func (m *Manager) CreateCheckpoint(projectID, name string, metadata map[string]string) (*state.Checkpoint, error) {
	// Generate checkpoint ID with nanosecond precision
	checkpointID := fmt.Sprintf("checkpoint-%s-%d", projectID, time.Now().UnixNano())

	// Sanitize name for git tag (replace spaces and special characters)
	sanitizedName := sanitizeTagName(name)

	// Create Git tag with unique suffix to avoid conflicts
	tagName := fmt.Sprintf("checkpoint-%s-%d", sanitizedName, time.Now().UnixNano())
	tagMessage := fmt.Sprintf("Checkpoint: %s", name)

	if err := m.gitManager.CreateTag(tagName, tagMessage); err != nil {
		return nil, fmt.Errorf("failed to create git tag: %w", err)
	}

	// Create checkpoint record
	checkpoint := &state.Checkpoint{
		ID:        checkpointID,
		ProjectID: projectID,
		Name:      name,
		GitTag:    tagName,
		CreatedAt: time.Now(),
		Metadata:  metadata,
	}

	// Save checkpoint to store
	if err := m.store.SaveCheckpoint(checkpoint); err != nil {
		return nil, fmt.Errorf("failed to save checkpoint: %w", err)
	}

	// Backup database state
	backupPath := filepath.Join(m.dataDir, "checkpoints", checkpointID+".db")
	if err := m.store.Backup(backupPath); err != nil {
		// Log warning but don't fail, as checkpoint record and git tag are created
		fmt.Printf("Warning: Failed to backup state database: %v\n", err)
	}

	return checkpoint, nil
}

// sanitizeTagName sanitizes a name for use as a git tag
func sanitizeTagName(name string) string {
	// Replace spaces with dashes
	result := ""
	for _, ch := range name {
		if ch == ' ' {
			result += "-"
		} else if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
			result += string(ch)
		}
	}
	return result
}

// CreateAutoCheckpoint creates an automatic checkpoint after phase completion
func (m *Manager) CreateAutoCheckpoint(projectID, phaseID string) (*state.Checkpoint, error) {
	// Get phase info
	phase, err := m.store.GetPhase(phaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get phase: %w", err)
	}

	// Create checkpoint name (replace spaces with dashes for git tag)
	name := fmt.Sprintf("phase-%d-%s", phase.Number, phase.Title)

	// Create metadata
	metadata := map[string]string{
		"type":     "auto",
		"phase_id": phaseID,
		"phase":    fmt.Sprintf("%d", phase.Number),
		"title":    phase.Title,
	}

	return m.CreateCheckpoint(projectID, name, metadata)
}

// ListCheckpoints lists all checkpoints for a project
func (m *Manager) ListCheckpoints(projectID string) ([]*state.Checkpoint, error) {
	checkpoints, err := m.store.ListCheckpoints(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list checkpoints: %w", err)
	}

	return checkpoints, nil
}

// GetCheckpoint retrieves a specific checkpoint
func (m *Manager) GetCheckpoint(checkpointID string) (*state.Checkpoint, error) {
	checkpoint, err := m.store.GetCheckpoint(checkpointID)
	if err != nil {
		return nil, fmt.Errorf("failed to get checkpoint: %w", err)
	}

	return checkpoint, nil
}

// Rollback rolls back to a specific checkpoint
func (m *Manager) Rollback(checkpointID string) error {
	// Get checkpoint
	checkpoint, err := m.store.GetCheckpoint(checkpointID)
	if err != nil {
		return fmt.Errorf("failed to get checkpoint: %w", err)
	}

	// Restore database state
	backupPath := filepath.Join(m.dataDir, "checkpoints", checkpointID+".db")
	if _, err := os.Stat(backupPath); err == nil {
		// Restore state
		if err := m.store.Restore(backupPath); err != nil {
			return fmt.Errorf("failed to restore state: %w", err)
		}
	} else {
		fmt.Printf("Warning: State backup not found for checkpoint %s. Proceeding with partial rollback (git only).\n", checkpointID)
	}

	// Reset Git repository to the checkpoint tag
	if err := m.gitManager.ResetToTag(checkpoint.GitTag); err != nil {
		return fmt.Errorf("failed to reset git to tag %s: %w", checkpoint.GitTag, err)
	}

	return nil
}

// RollbackToLatest rolls back to the most recent checkpoint
func (m *Manager) RollbackToLatest(projectID string) error {
	// Get all checkpoints
	checkpoints, err := m.ListCheckpoints(projectID)
	if err != nil {
		return fmt.Errorf("failed to list checkpoints: %w", err)
	}

	if len(checkpoints) == 0 {
		return fmt.Errorf("no checkpoints found for project %s", projectID)
	}

	// Find the most recent checkpoint
	var latest *state.Checkpoint
	for _, cp := range checkpoints {
		if latest == nil || cp.CreatedAt.After(latest.CreatedAt) {
			latest = cp
		}
	}

	// Rollback to the latest checkpoint
	return m.Rollback(latest.ID)
}

// DeleteCheckpoint deletes a checkpoint (soft delete - keeps in history)
func (m *Manager) DeleteCheckpoint(checkpointID string) error {
	// Note: We don't actually delete checkpoints from the database
	// to preserve history. We could add a "deleted" flag if needed.
	// For now, this is a no-op that returns success.
	return nil
}

// GetCheckpointHistory returns the complete checkpoint history
func (m *Manager) GetCheckpointHistory(projectID string) ([]*state.Checkpoint, error) {
	// Get all checkpoints (including those that might have been "deleted")
	checkpoints, err := m.store.ListCheckpoints(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get checkpoint history: %w", err)
	}

	return checkpoints, nil
}

// ValidateCheckpoint validates that a checkpoint is valid and can be rolled back to
func (m *Manager) ValidateCheckpoint(checkpointID string) error {
	// Get checkpoint
	checkpoint, err := m.store.GetCheckpoint(checkpointID)
	if err != nil {
		return fmt.Errorf("checkpoint not found: %w", err)
	}

	// Check if Git tag exists
	// Note: This would require adding a method to git.Manager to check if a tag exists
	// For now, we'll assume the tag exists if the checkpoint exists

	if checkpoint.GitTag == "" {
		return fmt.Errorf("checkpoint has no git tag")
	}

	return nil
}
