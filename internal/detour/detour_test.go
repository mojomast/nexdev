package detour

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/devplan"
	"github.com/mojomast/nexdev/internal/state"
)

func TestRequestDetour(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	detour, err := manager.RequestDetour("project-1", "phase-1", "task-1", "Add new feature", "User requested")
	if err != nil {
		t.Fatalf("failed to request detour: %v", err)
	}

	if detour.ProjectID != "project-1" {
		t.Errorf("expected project ID 'project-1', got '%s'", detour.ProjectID)
	}

	if detour.PhaseID != "phase-1" {
		t.Errorf("expected phase ID 'phase-1', got '%s'", detour.PhaseID)
	}

	if detour.TaskID != "task-1" {
		t.Errorf("expected task ID 'task-1', got '%s'", detour.TaskID)
	}

	if detour.Description != "Add new feature" {
		t.Errorf("expected description 'Add new feature', got '%s'", detour.Description)
	}

	if detour.Reason != "User requested" {
		t.Errorf("expected reason 'User requested', got '%s'", detour.Reason)
	}

	if detour.Status != DetourPending {
		t.Errorf("expected status 'pending', got '%s'", detour.Status)
	}

	if detour.CreatedAt.IsZero() {
		t.Error("expected created at to be set")
	}
}

func TestGatherDetourInformation(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	detour := &Detour{
		ID:          "detour-1",
		ProjectID:   "project-1",
		PhaseID:     "phase-1",
		TaskID:      "task-1",
		Description: "Add new feature",
		Reason:      "User requested",
		Status:      DetourPending,
		CreatedAt:   time.Now(),
	}

	err = manager.GatherDetourInformation(detour)
	if err != nil {
		t.Fatalf("failed to gather detour information: %v", err)
	}

	if detour.Status != DetourPlanned {
		t.Errorf("expected status 'planned', got '%s'", detour.Status)
	}
}

func TestGatherDetourInformation_InvalidStatus(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	detour := &Detour{
		ID:          "detour-1",
		ProjectID:   "project-1",
		PhaseID:     "phase-1",
		TaskID:      "task-1",
		Description: "Add new feature",
		Reason:      "User requested",
		Status:      DetourCompleted, // Invalid status
		CreatedAt:   time.Now(),
	}

	err = manager.GatherDetourInformation(detour)
	if err == nil {
		t.Error("expected error when gathering information for non-pending detour")
	}
}

func TestGenerateDetourTasks(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	detour := &Detour{
		ID:          "detour-1",
		ProjectID:   "project-1",
		PhaseID:     "phase-1",
		TaskID:      "task-1",
		Description: "Add new feature",
		Reason:      "User requested",
		Status:      DetourPlanned,
		CreatedAt:   time.Now(),
	}

	tasks := manager.generateDetourTasks(detour)

	if len(tasks) == 0 {
		t.Error("expected at least one task to be generated")
	}

	if tasks[0].Description == "" {
		t.Error("expected task description to be set")
	}

	if tasks[0].Status != devplan.TaskNotStarted {
		t.Errorf("expected task status 'not_started', got '%s'", tasks[0].Status)
	}
}

func TestUpdateDevPlan_InvalidStatus(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	detour := &Detour{
		ID:          "detour-1",
		ProjectID:   "project-1",
		PhaseID:     "phase-1",
		TaskID:      "task-1",
		Description: "Add new feature",
		Reason:      "User requested",
		Status:      DetourPending, // Invalid status for update
		CreatedAt:   time.Now(),
	}

	err = manager.UpdateDevPlan(detour, "task-1")
	if err == nil {
		t.Error("expected error when updating devplan for non-planned detour")
	}
}

func TestValidateDetourDependencies(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	detour := &Detour{
		ID:          "detour-1",
		ProjectID:   "project-1",
		PhaseID:     "phase-1",
		TaskID:      "task-1",
		Description: "Add new feature",
		Reason:      "User requested",
		Status:      DetourPlanned,
		CreatedAt:   time.Now(),
	}

	phase := &devplan.Phase{
		ID:     "phase-1",
		Number: 1,
		Title:  "Test Phase",
		Tasks: []devplan.Task{
			{
				ID:          "task-1",
				Number:      "1.1",
				Description: "Existing task",
				Status:      devplan.TaskNotStarted,
			},
		},
	}

	valid, conflicts := manager.ValidateDetourDependencies(detour, phase)

	if !valid {
		t.Errorf("expected detour to be valid, got conflicts: %v", conflicts)
	}

	if len(conflicts) > 0 {
		t.Errorf("expected no conflicts, got %d", len(conflicts))
	}
}

func TestDetourStatusTransitions(t *testing.T) {
	tests := []struct {
		name           string
		initialStatus  DetourStatus
		expectedStatus DetourStatus
		operation      string
	}{
		{
			name:           "Pending to Gathering",
			initialStatus:  DetourPending,
			expectedStatus: DetourPlanned,
			operation:      "gather",
		},
		{
			name:           "Planned to Active",
			initialStatus:  DetourPlanned,
			expectedStatus: DetourActive,
			operation:      "update",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := state.NewStore(":memory:")
			if err != nil {
				t.Fatalf("failed to create store: %v", err)
			}
			defer store.Close()

			manager := NewManager(store, nil, nil)

			detour := &Detour{
				ID:          "detour-1",
				ProjectID:   "project-1",
				PhaseID:     "phase-1",
				TaskID:      "task-1",
				Description: "Add new feature",
				Reason:      "User requested",
				Status:      tt.initialStatus,
				CreatedAt:   time.Now(),
			}

			switch tt.operation {
			case "gather":
				err = manager.GatherDetourInformation(detour)
			case "update":
				// Create a test phase first
				project := &state.Project{
					ID:        "project-1",
					Name:      "Test Project",
					CreatedAt: time.Now(),
				}
				if err := store.CreateProject(project); err != nil {
					t.Fatalf("failed to create project: %v", err)
				}

				phase := &state.Phase{
					ID:        "phase-1",
					ProjectID: "project-1",
					Number:    1,
					Title:     "Test Phase",
					Status:    "not_started",
					CreatedAt: time.Now(),
				}
				if err := store.SavePhase(phase); err != nil {
					t.Fatalf("failed to save phase: %v", err)
				}

				err = manager.UpdateDevPlan(detour, "task-1")
			}

			if err != nil {
				t.Fatalf("operation failed: %v", err)
			}

			if detour.Status != tt.expectedStatus {
				t.Errorf("expected status '%s', got '%s'", tt.expectedStatus, detour.Status)
			}
		})
	}
}

func TestSaveDetour(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	detour := &Detour{
		ID:          "detour-1",
		ProjectID:   "project-1",
		PhaseID:     "phase-1",
		TaskID:      "task-1",
		Description: "Add new feature",
		Reason:      "User requested",
		Status:      DetourPlanned,
		CreatedAt:   time.Now(),
	}

	err = manager.SaveDetour(detour)
	if err != nil {
		t.Fatalf("failed to save detour: %v", err)
	}
}

func TestTrackDetourInDirectory(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	detour := &Detour{
		ID:          "detour-1",
		ProjectID:   "project-1",
		PhaseID:     "phase-1",
		TaskID:      "task-1",
		Description: "Add new feature",
		Reason:      "User requested",
		Status:      DetourPlanned,
		CreatedAt:   time.Now(),
	}

	err = manager.TrackDetourInDirectory(detour, "./detours")
	if err != nil {
		t.Fatalf("failed to track detour in directory: %v", err)
	}
}

func TestGetDetourDependencies(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	detour := &Detour{
		ID:          "detour-1",
		ProjectID:   "project-1",
		PhaseID:     "phase-1",
		TaskID:      "task-1",
		Description: "Add new feature",
		Reason:      "User requested",
		Status:      DetourPlanned,
		CreatedAt:   time.Now(),
		NewTasks: []devplan.Task{
			{
				ID:          "detour-task-1",
				Number:      "detour-1",
				Description: "Implement detour feature",
				Status:      devplan.TaskNotStarted,
			},
		},
	}

	deps, err := manager.GetDetourDependencies(detour)
	if err != nil {
		t.Fatalf("failed to get detour dependencies: %v", err)
	}

	if deps == nil {
		t.Error("expected dependencies to be non-nil")
	}
}

func TestUpdateTaskDependencies(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	detour := &Detour{
		ID:          "detour-1",
		ProjectID:   "project-1",
		PhaseID:     "phase-1",
		TaskID:      "task-1",
		Description: "Add new feature",
		Reason:      "User requested",
		Status:      DetourPlanned,
		CreatedAt:   time.Now(),
	}

	affectedTaskIDs := []string{"task-2", "task-3"}

	err = manager.UpdateTaskDependencies(detour, affectedTaskIDs)
	if err != nil {
		t.Fatalf("failed to update task dependencies: %v", err)
	}
}

func TestExportDetourMarkdown(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	completedAt := time.Now()
	detour := &Detour{
		ID:          "detour-1",
		ProjectID:   "project-1",
		PhaseID:     "phase-1",
		TaskID:      "task-1",
		Description: "Add new feature",
		Reason:      "User requested",
		Status:      DetourCompleted,
		CreatedAt:   time.Now(),
		CompletedAt: &completedAt,
		NewTasks: []devplan.Task{
			{
				ID:                  "detour-task-1",
				Number:              "detour-1",
				Description:         "Implement detour feature",
				AcceptanceCriteria:  []string{"Feature works", "Tests pass"},
				ImplementationNotes: []string{"Use existing API"},
				Status:              devplan.TaskCompleted,
			},
		},
	}

	md, err := manager.ExportDetourMarkdown(detour)
	if err != nil {
		t.Fatalf("failed to export detour markdown: %v", err)
	}

	if md == "" {
		t.Error("expected markdown to be non-empty")
	}

	// Check that markdown contains key information
	if !contains(md, "detour-1") {
		t.Error("expected markdown to contain detour ID")
	}

	if !contains(md, "Add new feature") {
		t.Error("expected markdown to contain description")
	}

	if !contains(md, "User requested") {
		t.Error("expected markdown to contain reason")
	}

	if !contains(md, "Implement detour feature") {
		t.Error("expected markdown to contain task description")
	}
}

func TestExportDetourMarkdown_NoTasks(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	detour := &Detour{
		ID:          "detour-1",
		ProjectID:   "project-1",
		PhaseID:     "phase-1",
		TaskID:      "task-1",
		Description: "Add new feature",
		Reason:      "User requested",
		Status:      DetourPending,
		CreatedAt:   time.Now(),
		NewTasks:    []devplan.Task{},
	}

	md, err := manager.ExportDetourMarkdown(detour)
	if err != nil {
		t.Fatalf("failed to export detour markdown: %v", err)
	}

	if md == "" {
		t.Error("expected markdown to be non-empty")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || contains(s[1:], substr)))
}

func TestSaveAndGetDetour(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	detour := &Detour{
		ID:          "detour-save-1",
		ProjectID:   "project-1",
		PhaseID:     "phase-1",
		TaskID:      "task-1",
		Description: "Add caching layer",
		Reason:      "Performance optimization needed",
		Status:      DetourPlanned,
		CreatedAt:   time.Now(),
		NewTasks: []devplan.Task{
			{
				ID:          "dt-task-1",
				Number:      "d1",
				Description: "Implement cache",
				Status:      devplan.TaskNotStarted,
			},
		},
	}

	// Save
	err = manager.SaveDetour(detour)
	if err != nil {
		t.Fatalf("failed to save detour: %v", err)
	}

	// Get by ID
	retrieved, err := manager.GetDetour("detour-save-1")
	if err != nil {
		t.Fatalf("failed to get detour: %v", err)
	}

	if retrieved.ID != detour.ID {
		t.Errorf("expected ID '%s', got '%s'", detour.ID, retrieved.ID)
	}
	if retrieved.Description != detour.Description {
		t.Errorf("expected description '%s', got '%s'", detour.Description, retrieved.Description)
	}
	if retrieved.Status != DetourPlanned {
		t.Errorf("expected status 'planned', got '%s'", retrieved.Status)
	}
	if len(retrieved.NewTasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(retrieved.NewTasks))
	}
}

func TestGetDetour_NotFound(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	_, err = manager.GetDetour("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent detour")
	}
}

func TestListDetours(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	// Save two detours for project-1 and one for project-2
	for i, pid := range []string{"project-1", "project-1", "project-2"} {
		d := &Detour{
			ID:          fmt.Sprintf("detour-%d", i+1),
			ProjectID:   pid,
			PhaseID:     "phase-1",
			TaskID:      "task-1",
			Description: fmt.Sprintf("Detour %d", i+1),
			Reason:      "Testing",
			Status:      DetourPlanned,
			CreatedAt:   time.Now(),
		}
		if err := manager.SaveDetour(d); err != nil {
			t.Fatalf("failed to save detour %d: %v", i+1, err)
		}
	}

	// List for project-1
	detours, err := manager.ListDetours("project-1")
	if err != nil {
		t.Fatalf("failed to list detours: %v", err)
	}
	if len(detours) != 2 {
		t.Errorf("expected 2 detours for project-1, got %d", len(detours))
	}

	// List for project-2
	detours, err = manager.ListDetours("project-2")
	if err != nil {
		t.Fatalf("failed to list detours: %v", err)
	}
	if len(detours) != 1 {
		t.Errorf("expected 1 detour for project-2, got %d", len(detours))
	}

	// List for non-existent project
	detours, err = manager.ListDetours("project-3")
	if err != nil {
		t.Fatalf("failed to list detours: %v", err)
	}
	if len(detours) != 0 {
		t.Errorf("expected 0 detours for project-3, got %d", len(detours))
	}
}

func TestCompleteDetour(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	detour := &Detour{
		ID:        "detour-complete-1",
		ProjectID: "project-1",
		PhaseID:   "phase-1",
		TaskID:    "task-1",
		Status:    DetourActive,
		CreatedAt: time.Now(),
		NewTasks: []devplan.Task{
			{ID: "dt-1", Status: devplan.TaskCompleted},
			{ID: "dt-2", Status: devplan.TaskSkipped},
		},
	}

	// Save it first
	if err := manager.SaveDetour(detour); err != nil {
		t.Fatalf("failed to save detour: %v", err)
	}

	// Complete it
	err = manager.CompleteDetour("detour-complete-1")
	if err != nil {
		t.Fatalf("failed to complete detour: %v", err)
	}

	// Verify status changed
	updated, err := manager.GetDetour("detour-complete-1")
	if err != nil {
		t.Fatalf("failed to get completed detour: %v", err)
	}
	if updated.Status != DetourCompleted {
		t.Errorf("expected status 'completed', got '%s'", updated.Status)
	}
	if updated.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestCompleteDetour_IncompleteTasksFails(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	detour := &Detour{
		ID:        "detour-incomplete-1",
		ProjectID: "project-1",
		PhaseID:   "phase-1",
		TaskID:    "task-1",
		Status:    DetourActive,
		CreatedAt: time.Now(),
		NewTasks: []devplan.Task{
			{ID: "dt-1", Status: devplan.TaskCompleted},
			{ID: "dt-2", Status: devplan.TaskInProgress}, // Not done
		},
	}

	if err := manager.SaveDetour(detour); err != nil {
		t.Fatalf("failed to save detour: %v", err)
	}

	err = manager.CompleteDetour("detour-incomplete-1")
	if err == nil {
		t.Error("expected error when completing detour with incomplete tasks")
	}
}

func TestCompleteDetour_WrongStatus(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	detour := &Detour{
		ID:        "detour-wrong-status",
		ProjectID: "project-1",
		PhaseID:   "phase-1",
		TaskID:    "task-1",
		Status:    DetourPending, // Wrong status
		CreatedAt: time.Now(),
	}

	if err := manager.SaveDetour(detour); err != nil {
		t.Fatalf("failed to save detour: %v", err)
	}

	err = manager.CompleteDetour("detour-wrong-status")
	if err == nil {
		t.Error("expected error when completing non-active detour")
	}
}

func TestTrackDetourInDirectory_CreatesFile(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	detour := &Detour{
		ID:          "detour-track-1",
		ProjectID:   "project-1",
		PhaseID:     "phase-1",
		TaskID:      "task-1",
		Description: "Add tracking",
		Reason:      "Testing file write",
		Status:      DetourPlanned,
		CreatedAt:   time.Now(),
	}

	tmpDir := t.TempDir()
	detourDir := tmpDir + "/detours"

	err = manager.TrackDetourInDirectory(detour, detourDir)
	if err != nil {
		t.Fatalf("failed to track detour: %v", err)
	}

	// Verify file was created
	expectedFile := detourDir + "/detour-track-1.md"
	data, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("failed to read detour file: %v", err)
	}

	if !contains(string(data), "detour-track-1") {
		t.Error("expected file to contain detour ID")
	}
	if !contains(string(data), "Add tracking") {
		t.Error("expected file to contain detour description")
	}
}

func TestValidateDetourDependencies_Conflicts(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	detour := &Detour{
		ID:        "detour-conflict-1",
		ProjectID: "project-1",
		PhaseID:   "phase-1",
		TaskID:    "task-1",
		Status:    DetourPlanned,
		CreatedAt: time.Now(),
		NewTasks: []devplan.Task{
			{
				ID:     "task-1", // Conflicts with existing task
				Number: "1.1",    // Also conflicts
			},
		},
	}

	phase := &devplan.Phase{
		ID:     "phase-1",
		Number: 1,
		Title:  "Test Phase",
		Tasks: []devplan.Task{
			{ID: "task-1", Number: "1.1", Description: "Existing task"},
		},
	}

	valid, conflicts := manager.ValidateDetourDependencies(detour, phase)

	if valid {
		t.Error("expected detour to have conflicts")
	}

	if len(conflicts) < 2 {
		t.Errorf("expected at least 2 conflicts (ID and number), got %d", len(conflicts))
	}
}

func TestResolveDetourConflict_Rename(t *testing.T) {
	detour := &Detour{
		NewTasks: []devplan.Task{
			{ID: "task-1", Description: "Conflicting task"},
		},
	}

	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	err = manager.ResolveDetourConflict(detour, "task-1", "rename")
	if err != nil {
		t.Fatalf("failed to resolve conflict: %v", err)
	}

	if detour.NewTasks[0].ID == "task-1" {
		t.Error("expected task ID to be renamed")
	}
}

func TestResolveDetourConflict_Skip(t *testing.T) {
	detour := &Detour{
		NewTasks: []devplan.Task{
			{ID: "task-1", Description: "Conflicting task", Status: devplan.TaskNotStarted},
		},
	}

	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	err = manager.ResolveDetourConflict(detour, "task-1", "skip")
	if err != nil {
		t.Fatalf("failed to resolve conflict: %v", err)
	}

	if detour.NewTasks[0].Status != devplan.TaskSkipped {
		t.Errorf("expected task to be skipped, got %s", detour.NewTasks[0].Status)
	}
}

func TestResolveDetourConflict_UnknownStrategy(t *testing.T) {
	detour := &Detour{
		NewTasks: []devplan.Task{
			{ID: "task-1"},
		},
	}

	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	manager := NewManager(store, nil, nil)

	err = manager.ResolveDetourConflict(detour, "task-1", "invalid")
	if err == nil {
		t.Error("expected error for unknown resolution strategy")
	}
}

func TestGetDetourDependencies_WithTasks(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create project and phase
	project := &state.Project{
		ID:        "project-1",
		Name:      "Test",
		CreatedAt: time.Now(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	phase := &state.Phase{
		ID:        "phase-1",
		ProjectID: "project-1",
		Number:    1,
		Title:     "Phase 1",
		Status:    state.PhaseInProgress,
		CreatedAt: time.Now(),
	}
	if err := store.SavePhase(phase); err != nil {
		t.Fatalf("failed to save phase: %v", err)
	}

	// Create tasks in order
	for i := 1; i <= 4; i++ {
		task := &state.Task{
			ID:          fmt.Sprintf("task-%d", i),
			PhaseID:     "phase-1",
			Number:      fmt.Sprintf("1.%d", i),
			Description: fmt.Sprintf("Task %d", i),
			Status:      state.TaskNotStarted,
		}
		if err := store.SaveTask(task); err != nil {
			t.Fatalf("failed to save task %d: %v", i, err)
		}
	}

	manager := NewManager(store, nil, nil)

	detour := &Detour{
		ID:      "detour-deps-1",
		PhaseID: "phase-1",
		TaskID:  "task-2", // Detour at task-2; tasks 3 and 4 should be dependent
	}

	deps, err := manager.GetDetourDependencies(detour)
	if err != nil {
		t.Fatalf("failed to get dependencies: %v", err)
	}

	if len(deps) != 2 {
		t.Errorf("expected 2 dependent tasks, got %d: %v", len(deps), deps)
	}
}

func TestDetourFullWorkflow(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Set up project infrastructure
	project := &state.Project{
		ID:        "project-wf",
		Name:      "Workflow Test",
		CreatedAt: time.Now(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	phase := &state.Phase{
		ID:        "phase-wf-1",
		ProjectID: "project-wf",
		Number:    1,
		Title:     "Phase 1",
		Status:    state.PhaseInProgress,
		CreatedAt: time.Now(),
	}
	if err := store.SavePhase(phase); err != nil {
		t.Fatalf("failed to save phase: %v", err)
	}

	manager := NewManager(store, nil, nil)

	// Step 1: Request detour
	detour, err := manager.RequestDetour("project-wf", "phase-wf-1", "task-1", "Add error handling", "Missing error handling discovered")
	if err != nil {
		t.Fatalf("failed to request detour: %v", err)
	}
	if detour.Status != DetourPending {
		t.Errorf("expected pending, got %s", detour.Status)
	}

	// Step 2: Gather information
	err = manager.GatherDetourInformation(detour)
	if err != nil {
		t.Fatalf("failed to gather info: %v", err)
	}
	if detour.Status != DetourPlanned {
		t.Errorf("expected planned, got %s", detour.Status)
	}

	// Step 3: Update dev plan (generates and saves tasks)
	err = manager.UpdateDevPlan(detour, "task-1")
	if err != nil {
		t.Fatalf("failed to update dev plan: %v", err)
	}
	if detour.Status != DetourActive {
		t.Errorf("expected active, got %s", detour.Status)
	}
	if len(detour.NewTasks) == 0 {
		t.Fatal("expected tasks to be generated")
	}

	// Step 4: Simulate completing tasks
	for i := range detour.NewTasks {
		detour.NewTasks[i].Status = devplan.TaskCompleted
	}
	if err := manager.SaveDetour(detour); err != nil {
		t.Fatalf("failed to save updated detour: %v", err)
	}

	// Step 5: Complete detour
	err = manager.CompleteDetour(detour.ID)
	if err != nil {
		t.Fatalf("failed to complete detour: %v", err)
	}

	// Verify final state
	final, err := manager.GetDetour(detour.ID)
	if err != nil {
		t.Fatalf("failed to get final detour: %v", err)
	}
	if final.Status != DetourCompleted {
		t.Errorf("expected completed, got %s", final.Status)
	}
	if final.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}

	// Verify detour shows up in list
	detours, err := manager.ListDetours("project-wf")
	if err != nil {
		t.Fatalf("failed to list detours: %v", err)
	}
	if len(detours) != 1 {
		t.Errorf("expected 1 detour in list, got %d", len(detours))
	}
}
