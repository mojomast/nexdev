package checkpoint

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/git"
	"github.com/mojomast/nexdev/internal/state"
)

func setupTestManager(t *testing.T) (*Manager, *state.Store, *git.Manager, string) {
	// Create temporary directory for git repo
	tempDir, err := os.MkdirTemp("", "checkpoint-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create data dir
	dataDir := filepath.Join(tempDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	// Create file-based store
	dbPath := filepath.Join(dataDir, "state.db")
	store, err := state.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create git manager
	gitManager := git.NewManager(tempDir)
	if err := gitManager.Initialize(); err != nil {
		t.Fatalf("failed to initialize git: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := gitManager.CommitFile("test.txt", "Initial commit", nil); err != nil {
		t.Fatalf("failed to create initial commit: %v", err)
	}

	// Create checkpoint manager
	manager := NewManager(store, gitManager, dataDir)

	return manager, store, gitManager, tempDir
}

func TestNewManager(t *testing.T) {
	manager, store, _, tempDir := setupTestManager(t)
	defer store.Close()
	defer os.RemoveAll(tempDir)

	if manager == nil {
		t.Fatal("expected manager to be created")
	}

	if manager.store == nil {
		t.Error("expected store to be set")
	}

	if manager.gitManager == nil {
		t.Error("expected git manager to be set")
	}

	if manager.dataDir == "" {
		t.Error("expected dataDir to be set")
	}
}

func TestManager_CreateCheckpoint(t *testing.T) {
	manager, store, _, tempDir := setupTestManager(t)
	defer store.Close()
	defer os.RemoveAll(tempDir)

	// Create a test project
	project := &state.Project{
		ID:        "test-project",
		Name:      "Test Project",
		CreatedAt: time.Now(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Create checkpoint
	metadata := map[string]string{
		"type": "manual",
		"note": "Test checkpoint",
	}

	checkpoint, err := manager.CreateCheckpoint(project.ID, "test-checkpoint", metadata)
	if err != nil {
		t.Fatalf("failed to create checkpoint: %v", err)
	}

	if checkpoint == nil {
		t.Fatal("expected checkpoint to be created")
	}

	if checkpoint.ProjectID != project.ID {
		t.Errorf("expected project ID %s, got %s", project.ID, checkpoint.ProjectID)
	}

	if checkpoint.Name != "test-checkpoint" {
		t.Errorf("expected name 'test-checkpoint', got %s", checkpoint.Name)
	}

	if checkpoint.GitTag == "" {
		t.Error("expected git tag to be set")
	}

	if checkpoint.Metadata["type"] != "manual" {
		t.Errorf("expected metadata type 'manual', got %s", checkpoint.Metadata["type"])
	}
}

func TestManager_CreateAutoCheckpoint(t *testing.T) {
	manager, store, _, tempDir := setupTestManager(t)
	defer store.Close()
	defer os.RemoveAll(tempDir)

	// Create a test project
	project := &state.Project{
		ID:        "test-project",
		Name:      "Test Project",
		CreatedAt: time.Now(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Create a test phase
	phase := &state.Phase{
		ID:        "phase-1",
		ProjectID: project.ID,
		Number:    1,
		Title:     "Test Phase",
		Status:    "completed",
		CreatedAt: time.Now(),
	}
	if err := store.SavePhase(phase); err != nil {
		t.Fatalf("failed to save phase: %v", err)
	}

	// Create auto checkpoint
	checkpoint, err := manager.CreateAutoCheckpoint(project.ID, phase.ID)
	if err != nil {
		t.Fatalf("failed to create auto checkpoint: %v", err)
	}

	if checkpoint == nil {
		t.Fatal("expected checkpoint to be created")
	}

	if checkpoint.Metadata["type"] != "auto" {
		t.Errorf("expected metadata type 'auto', got %s", checkpoint.Metadata["type"])
	}

	if checkpoint.Metadata["phase_id"] != phase.ID {
		t.Errorf("expected metadata phase_id %s, got %s", phase.ID, checkpoint.Metadata["phase_id"])
	}
}

func TestManager_ListCheckpoints(t *testing.T) {
	manager, store, _, tempDir := setupTestManager(t)
	defer store.Close()
	defer os.RemoveAll(tempDir)

	// Create a test project
	project := &state.Project{
		ID:        "test-project",
		Name:      "Test Project",
		CreatedAt: time.Now(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Create multiple checkpoints
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("checkpoint-%d", i)
		if _, err := manager.CreateCheckpoint(project.ID, name, nil); err != nil {
			t.Fatalf("failed to create checkpoint %d: %v", i, err)
		}
		time.Sleep(100 * time.Millisecond) // Ensure different timestamps
	}

	// List checkpoints
	checkpoints, err := manager.ListCheckpoints(project.ID)
	if err != nil {
		t.Fatalf("failed to list checkpoints: %v", err)
	}

	if len(checkpoints) != 3 {
		t.Errorf("expected 3 checkpoints, got %d", len(checkpoints))
	}
}

func TestManager_GetCheckpoint(t *testing.T) {
	manager, store, _, tempDir := setupTestManager(t)
	defer store.Close()
	defer os.RemoveAll(tempDir)

	// Create a test project
	project := &state.Project{
		ID:        "test-project",
		Name:      "Test Project",
		CreatedAt: time.Now(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Create checkpoint
	checkpoint, err := manager.CreateCheckpoint(project.ID, "test-checkpoint", nil)
	if err != nil {
		t.Fatalf("failed to create checkpoint: %v", err)
	}

	// Get checkpoint
	retrieved, err := manager.GetCheckpoint(checkpoint.ID)
	if err != nil {
		t.Fatalf("failed to get checkpoint: %v", err)
	}

	if retrieved.ID != checkpoint.ID {
		t.Errorf("expected checkpoint ID %s, got %s", checkpoint.ID, retrieved.ID)
	}

	if retrieved.Name != checkpoint.Name {
		t.Errorf("expected checkpoint name %s, got %s", checkpoint.Name, retrieved.Name)
	}
}

func TestManager_Rollback(t *testing.T) {
	manager, store, gitManager, tempDir := setupTestManager(t)
	defer store.Close()
	defer os.RemoveAll(tempDir)

	// Create a test project
	project := &state.Project{
		ID:        "test-project",
		Name:      "Test Project",
		CreatedAt: time.Now(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Create checkpoint
	checkpoint, err := manager.CreateCheckpoint(project.ID, "test-checkpoint", nil)
	if err != nil {
		t.Fatalf("failed to create checkpoint: %v", err)
	}

	// Make some changes after checkpoint
	testFile := filepath.Join(tempDir, "test2.txt")
	if err := os.WriteFile(testFile, []byte("test2"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := gitManager.CommitFile("test2.txt", "Second commit", nil); err != nil {
		t.Fatalf("failed to create second commit: %v", err)
	}

	// Rollback to checkpoint
	if err := manager.Rollback(checkpoint.ID); err != nil {
		t.Fatalf("failed to rollback: %v", err)
	}

	// Verify rollback (test2.txt should not exist)
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("expected test2.txt to not exist after rollback")
	}
}

func TestManager_RollbackToLatest(t *testing.T) {
	manager, store, _, tempDir := setupTestManager(t)
	defer store.Close()
	defer os.RemoveAll(tempDir)

	// Create a test project
	project := &state.Project{
		ID:        "test-project",
		Name:      "Test Project",
		CreatedAt: time.Now(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Create multiple checkpoints
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("checkpoint-%d", i)
		if _, err := manager.CreateCheckpoint(project.ID, name, nil); err != nil {
			t.Fatalf("failed to create checkpoint %d: %v", i, err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Rollback to latest
	if err := manager.RollbackToLatest(project.ID); err != nil {
		t.Fatalf("failed to rollback to latest: %v", err)
	}
}

func TestManager_ValidateCheckpoint(t *testing.T) {
	manager, store, _, tempDir := setupTestManager(t)
	defer store.Close()
	defer os.RemoveAll(tempDir)

	// Create a test project
	project := &state.Project{
		ID:        "test-project",
		Name:      "Test Project",
		CreatedAt: time.Now(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Create checkpoint
	checkpoint, err := manager.CreateCheckpoint(project.ID, "test-checkpoint", nil)
	if err != nil {
		t.Fatalf("failed to create checkpoint: %v", err)
	}

	// Validate checkpoint
	if err := manager.ValidateCheckpoint(checkpoint.ID); err != nil {
		t.Errorf("expected checkpoint to be valid: %v", err)
	}

	// Validate non-existent checkpoint
	if err := manager.ValidateCheckpoint("non-existent"); err == nil {
		t.Error("expected error for non-existent checkpoint")
	}
}

func TestManager_GetCheckpointHistory(t *testing.T) {
	manager, store, _, tempDir := setupTestManager(t)
	defer store.Close()
	defer os.RemoveAll(tempDir)

	// Create a test project
	project := &state.Project{
		ID:        "test-project",
		Name:      "Test Project",
		CreatedAt: time.Now(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Create multiple checkpoints
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("checkpoint-%d", i)
		if _, err := manager.CreateCheckpoint(project.ID, name, nil); err != nil {
			t.Fatalf("failed to create checkpoint %d: %v", i, err)
		}
		time.Sleep(100 * time.Millisecond) // Ensure different timestamps
	}

	// Get checkpoint history
	history, err := manager.GetCheckpointHistory(project.ID)
	if err != nil {
		t.Fatalf("failed to get checkpoint history: %v", err)
	}

	if len(history) != 3 {
		t.Errorf("expected 3 checkpoints in history, got %d", len(history))
	}
}
