package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_BackupAndRestore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	backupPath := filepath.Join(tmpDir, "backup.db")

	// 1. Create store and initial data
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create project
	project := &Project{
		ID:           "proj-123",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: StageInit,
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create a checkpoint (CP1)
	cp1 := &Checkpoint{
		ID:        "cp-1",
		ProjectID: "proj-123",
		Name:      "Checkpoint 1",
		GitTag:    "v1",
		CreatedAt: time.Now(),
	}
	if err := store.SaveCheckpoint(cp1); err != nil {
		t.Fatalf("Failed to save checkpoint 1: %v", err)
	}

	// 2. Backup
	if err := store.Backup(backupPath); err != nil {
		t.Fatalf("Failed to backup: %v", err)
	}

	// Verify backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Fatalf("Backup file not created")
	}

	// 3. Make changes (Create CP2, Change Stage)
	cp2 := &Checkpoint{
		ID:        "cp-2",
		ProjectID: "proj-123",
		Name:      "Checkpoint 2",
		GitTag:    "v2",
		CreatedAt: time.Now().Add(time.Hour),
	}
	if err := store.SaveCheckpoint(cp2); err != nil {
		t.Fatalf("Failed to save checkpoint 2: %v", err)
	}

	if err := store.UpdateProjectStage("proj-123", StageDevelop); err != nil {
		t.Fatalf("Failed to update project stage: %v", err)
	}

	// Verify current state
	p, _ := store.GetProject("proj-123")
	if p.CurrentStage != StageDevelop {
		t.Fatalf("Project stage should be Develop")
	}

	// 4. Restore
	if err := store.Restore(backupPath); err != nil {
		t.Fatalf("Failed to restore: %v", err)
	}

	// 5. Verify restored state
	// Project stage should be back to Init
	p, err = store.GetProject("proj-123")
	if err != nil {
		t.Fatalf("Failed to get project after restore: %v", err)
	}
	if p.CurrentStage != StageInit {
		t.Errorf("Restored stage mismatch: got %s, want %s", p.CurrentStage, StageInit)
	}

	// 6. Verify history preservation
	// Both CP1 and CP2 should exist
	checkpoints, err := store.ListCheckpoints("proj-123")
	if err != nil {
		t.Fatalf("Failed to list checkpoints: %v", err)
	}

	foundCP1 := false
	foundCP2 := false
	for _, cp := range checkpoints {
		if cp.ID == "cp-1" {
			foundCP1 = true
		}
		if cp.ID == "cp-2" {
			foundCP2 = true
		}
	}

	if !foundCP1 {
		t.Error("Checkpoint 1 lost after restore")
	}
	if !foundCP2 {
		t.Error("Checkpoint 2 (future history) lost after restore")
	}
}

func TestStore_Backup_InvalidPath(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Invalid path (directory exists)
	invalidPath := tmpDir
	if err := store.Backup(invalidPath); err == nil {
		t.Error("Expected error backing up to directory, got nil")
	}
}

func TestStore_Restore_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	if err := store.Restore("nonexistent_file"); err == nil {
		t.Error("Expected error restoring nonexistent file, got nil")
	}
}

func TestStore_Backup_SpecialChars(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	// Path with single quote and space
	backupPath := filepath.Join(tmpDir, "back up's", "backup.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	if err := store.Backup(backupPath); err != nil {
		t.Fatalf("Failed to backup to path with special chars: %v", err)
	}

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Fatalf("Backup file not created")
	}
}
