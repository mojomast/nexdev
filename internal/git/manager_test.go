package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitManager(t *testing.T) {
	// Create temporary directory for testing
	tmpDir := t.TempDir()

	manager := NewManager(tmpDir)

	t.Run("IsRepository_NotInitialized", func(t *testing.T) {
		isRepo, err := manager.IsRepository()
		if err != nil {
			t.Fatalf("Failed to check repository: %v", err)
		}
		if isRepo {
			t.Error("Expected directory to not be a repository")
		}
	})

	t.Run("Initialize", func(t *testing.T) {
		err := manager.Initialize()
		if err != nil {
			t.Fatalf("Failed to initialize repository: %v", err)
		}

		// Verify .git directory exists
		gitDir := filepath.Join(tmpDir, ".git")
		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			t.Error(".git directory was not created")
		}
	})

	t.Run("IsRepository_AfterInit", func(t *testing.T) {
		isRepo, err := manager.IsRepository()
		if err != nil {
			t.Fatalf("Failed to check repository: %v", err)
		}
		if !isRepo {
			t.Error("Expected directory to be a repository")
		}
	})

	t.Run("GetStatus_Empty", func(t *testing.T) {
		status, err := manager.GetStatus()
		if err != nil {
			t.Fatalf("Failed to get status: %v", err)
		}
		if status != "" {
			t.Errorf("Expected empty status, got: %s", status)
		}
	})

	t.Run("CommitFile", func(t *testing.T) {
		// Create a test file
		testFile := filepath.Join(tmpDir, "test.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Commit the file
		metadata := map[string]string{
			"stage": "test",
			"task":  "test-task",
		}
		err = manager.CommitFile("test.txt", "Add test file", metadata)
		if err != nil {
			t.Fatalf("Failed to commit file: %v", err)
		}

		// Verify status is clean
		status, err := manager.GetStatus()
		if err != nil {
			t.Fatalf("Failed to get status: %v", err)
		}
		if status != "" {
			t.Errorf("Expected clean status after commit, got: %s", status)
		}
	})

	t.Run("CommitFiles_Multiple", func(t *testing.T) {
		// Create multiple test files
		file1 := filepath.Join(tmpDir, "file1.txt")
		file2 := filepath.Join(tmpDir, "file2.txt")

		os.WriteFile(file1, []byte("content 1"), 0644)
		os.WriteFile(file2, []byte("content 2"), 0644)

		// Commit multiple files
		err := manager.CommitFiles(
			[]string{"file1.txt", "file2.txt"},
			"Add multiple files",
			nil,
		)
		if err != nil {
			t.Fatalf("Failed to commit files: %v", err)
		}
	})

	t.Run("CreateTag", func(t *testing.T) {
		err := manager.CreateTag("v1.0.0", "Version 1.0.0")
		if err != nil {
			t.Fatalf("Failed to create tag: %v", err)
		}

		// Verify tag exists
		tags, err := manager.ListTags()
		if err != nil {
			t.Fatalf("Failed to list tags: %v", err)
		}

		found := false
		for _, tag := range tags {
			if tag == "v1.0.0" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Tag v1.0.0 not found")
		}
	})

	t.Run("GetChangedFiles", func(t *testing.T) {
		// Create a new file
		newFile := filepath.Join(tmpDir, "new.txt")
		os.WriteFile(newFile, []byte("new content"), 0644)

		files, err := manager.GetChangedFiles()
		if err != nil {
			t.Fatalf("Failed to get changed files: %v", err)
		}

		if len(files) == 0 {
			t.Error("Expected changed files, got none")
		}

		// Clean up
		manager.Stage([]string{"new.txt"})
		manager.Commit("Add new file", nil)
	})

	t.Run("HasUncommittedChanges", func(t *testing.T) {
		// Should be clean after previous commit
		hasChanges, err := manager.HasUncommittedChanges()
		if err != nil {
			t.Fatalf("Failed to check uncommitted changes: %v", err)
		}
		if hasChanges {
			t.Error("Expected no uncommitted changes")
		}

		// Create a change
		changeFile := filepath.Join(tmpDir, "change.txt")
		os.WriteFile(changeFile, []byte("change"), 0644)

		hasChanges, err = manager.HasUncommittedChanges()
		if err != nil {
			t.Fatalf("Failed to check uncommitted changes: %v", err)
		}
		if !hasChanges {
			t.Error("Expected uncommitted changes")
		}

		// Clean up
		manager.CommitFile("change.txt", "Add change file", nil)
	})

	t.Run("GetCurrentBranch", func(t *testing.T) {
		branch, err := manager.GetCurrentBranch()
		if err != nil {
			t.Fatalf("Failed to get current branch: %v", err)
		}
		// Default branch is usually "master" or "main"
		if branch != "master" && branch != "main" {
			t.Logf("Current branch: %s (expected master or main)", branch)
		}
	})

	t.Run("ResetToTag", func(t *testing.T) {
		// Create another file and commit
		resetFile := filepath.Join(tmpDir, "reset.txt")
		os.WriteFile(resetFile, []byte("reset content"), 0644)
		manager.CommitFile("reset.txt", "Add reset file", nil)

		// Create a tag
		manager.CreateTag("checkpoint-1", "Checkpoint 1")

		// Create another file
		afterFile := filepath.Join(tmpDir, "after.txt")
		os.WriteFile(afterFile, []byte("after content"), 0644)
		manager.CommitFile("after.txt", "Add after file", nil)

		// Reset to tag
		err := manager.ResetToTag("checkpoint-1")
		if err != nil {
			t.Fatalf("Failed to reset to tag: %v", err)
		}

		// Verify after.txt no longer exists
		if _, err := os.Stat(afterFile); !os.IsNotExist(err) {
			t.Error("Expected after.txt to be removed after reset")
		}
	})
}

func TestGitManager_EnsureRepository(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	t.Run("EnsureRepository_NotInitialized", func(t *testing.T) {
		err := manager.EnsureRepository()
		if err != nil {
			t.Fatalf("Failed to ensure repository: %v", err)
		}

		isRepo, err := manager.IsRepository()
		if err != nil {
			t.Fatalf("Failed to check repository: %v", err)
		}
		if !isRepo {
			t.Error("Expected directory to be a repository after EnsureRepository")
		}
	})

	t.Run("EnsureRepository_AlreadyInitialized", func(t *testing.T) {
		// Should not error if already initialized
		err := manager.EnsureRepository()
		if err != nil {
			t.Fatalf("Failed to ensure repository: %v", err)
		}
	})
}

func TestGitManager_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	t.Run("Stage_EmptyFiles", func(t *testing.T) {
		err := manager.Stage([]string{})
		if err == nil {
			t.Error("Expected error when staging empty file list")
		}
	})

	t.Run("Commit_EmptyMessage", func(t *testing.T) {
		err := manager.Commit("", nil)
		if err == nil {
			t.Error("Expected error when committing with empty message")
		}
	})

	t.Run("CreateTag_EmptyName", func(t *testing.T) {
		err := manager.CreateTag("", "message")
		if err == nil {
			t.Error("Expected error when creating tag with empty name")
		}
	})

	t.Run("ResetToTag_EmptyName", func(t *testing.T) {
		err := manager.ResetToTag("")
		if err == nil {
			t.Error("Expected error when resetting to empty tag name")
		}
	})
}

func TestGitManager_SetRepoPath(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager("")

	t.Run("SetRepoPath_Valid", func(t *testing.T) {
		err := manager.SetRepoPath(tmpDir)
		if err != nil {
			t.Fatalf("Failed to set repo path: %v", err)
		}

		if manager.GetRepoPath() != tmpDir {
			t.Errorf("Expected repo path %s, got %s", tmpDir, manager.GetRepoPath())
		}
	})

	t.Run("SetRepoPath_NonExistent", func(t *testing.T) {
		err := manager.SetRepoPath("/nonexistent/path")
		if err == nil {
			t.Error("Expected error when setting non-existent path")
		}
	})
}

func TestGitManager_DetectConflicts(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	// Initialize repository
	manager.Initialize()

	t.Run("DetectConflicts_NoConflicts", func(t *testing.T) {
		hasConflicts, conflicts, err := manager.DetectConflicts()
		if err != nil {
			t.Fatalf("Failed to detect conflicts: %v", err)
		}
		if hasConflicts {
			t.Error("Expected no conflicts")
		}
		if len(conflicts) > 0 {
			t.Errorf("Expected no conflict files, got %d", len(conflicts))
		}
	})
}
