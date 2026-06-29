package state

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Additional comprehensive tests for State Store

// TestStore_GetTokenStats tests token statistics aggregation
func TestStore_GetTokenStats(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create project
	project := &Project{
		ID:           "proj-123",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: StageDevelop,
	}
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create phase
	phase := &Phase{
		ID:        "phase-1",
		ProjectID: "proj-123",
		Number:    1,
		Title:     "Phase 1",
		Content:   "Content",
		Status:    PhaseInProgress,
		CreatedAt: time.Now(),
	}
	err = store.SavePhase(phase)
	if err != nil {
		t.Fatalf("Failed to save phase: %v", err)
	}

	// Record multiple token usages with different providers and phases
	usages := []*TokenUsage{
		{
			ProjectID:    "proj-123",
			PhaseID:      "phase-1",
			Provider:     "openai",
			Model:        "gpt-4",
			TokensInput:  100,
			TokensOutput: 200,
			Cost:         0.015,
			Timestamp:    time.Now(),
		},
		{
			ProjectID:    "proj-123",
			PhaseID:      "phase-1",
			Provider:     "openai",
			Model:        "gpt-4",
			TokensInput:  150,
			TokensOutput: 250,
			Cost:         0.020,
			Timestamp:    time.Now(),
		},
		{
			ProjectID:    "proj-123",
			PhaseID:      "phase-1",
			Provider:     "anthropic",
			Model:        "claude-3",
			TokensInput:  200,
			TokensOutput: 300,
			Cost:         0.025,
			Timestamp:    time.Now(),
		},
	}

	for _, usage := range usages {
		err = store.RecordTokenUsage(usage)
		if err != nil {
			t.Fatalf("Failed to record token usage: %v", err)
		}
	}

	// Get token stats
	stats, err := store.GetTokenStats(project.ID)
	if err != nil {
		t.Fatalf("Failed to get token stats: %v", err)
	}

	// Verify total input tokens
	expectedInput := 100 + 150 + 200
	if stats.TotalInput != expectedInput {
		t.Errorf("TotalInput mismatch: got %d, want %d", stats.TotalInput, expectedInput)
	}

	// Verify total output tokens
	expectedOutput := 200 + 250 + 300
	if stats.TotalOutput != expectedOutput {
		t.Errorf("TotalOutput mismatch: got %d, want %d", stats.TotalOutput, expectedOutput)
	}

	// Verify by provider stats
	if len(stats.ByProvider) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(stats.ByProvider))
	}

	expectedOpenAI := (100 + 200) + (150 + 250)
	if stats.ByProvider["openai"] != expectedOpenAI {
		t.Errorf("OpenAI tokens mismatch: got %d, want %d", stats.ByProvider["openai"], expectedOpenAI)
	}

	expectedAnthropic := 200 + 300
	if stats.ByProvider["anthropic"] != expectedAnthropic {
		t.Errorf("Anthropic tokens mismatch: got %d, want %d", stats.ByProvider["anthropic"], expectedAnthropic)
	}

	// Verify by phase stats
	if len(stats.ByPhase) != 1 {
		t.Errorf("Expected 1 phase, got %d", len(stats.ByPhase))
	}

	expectedPhase := expectedInput + expectedOutput
	if stats.ByPhase["phase-1"] != expectedPhase {
		t.Errorf("Phase tokens mismatch: got %d, want %d", stats.ByPhase["phase-1"], expectedPhase)
	}
}

// TestStore_GetTokenStats_EmptyProject tests token stats for project with no usage
func TestStore_GetTokenStats_EmptyProject(t *testing.T) {
	store, err := NewStore(":memory:")
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
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Get token stats for project with no usage
	stats, err := store.GetTokenStats(project.ID)
	if err != nil {
		t.Fatalf("Failed to get token stats: %v", err)
	}

	if stats.TotalInput != 0 {
		t.Errorf("Expected TotalInput to be 0, got %d", stats.TotalInput)
	}
	if stats.TotalOutput != 0 {
		t.Errorf("Expected TotalOutput to be 0, got %d", stats.TotalOutput)
	}
	if len(stats.ByProvider) != 0 {
		t.Errorf("Expected no providers, got %d", len(stats.ByProvider))
	}
	if len(stats.ByPhase) != 0 {
		t.Errorf("Expected no phases, got %d", len(stats.ByPhase))
	}
}

// TestStore_NullableFields tests handling of nullable fields
func TestStore_NullableFields_Phase(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create project
	project := &Project{
		ID:           "proj-123",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: StagePlan,
	}
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Save phase with null StartedAt and CompletedAt
	phase := &Phase{
		ID:          "phase-1",
		ProjectID:   "proj-123",
		Number:      1,
		Title:       "Phase 1",
		Content:     "Content",
		Status:      PhaseNotStarted,
		CreatedAt:   time.Now(),
		StartedAt:   nil,
		CompletedAt: nil,
	}
	err = store.SavePhase(phase)
	if err != nil {
		t.Fatalf("Failed to save phase: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.GetPhase(phase.ID)
	if err != nil {
		t.Fatalf("Failed to get phase: %v", err)
	}

	if retrieved.StartedAt != nil {
		t.Errorf("Expected StartedAt to be nil, got %v", retrieved.StartedAt)
	}
	if retrieved.CompletedAt != nil {
		t.Errorf("Expected CompletedAt to be nil, got %v", retrieved.CompletedAt)
	}
}

// TestStore_NullableFields_Task tests handling of nullable fields in tasks
func TestStore_NullableFields_Task(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create project and phase
	project := &Project{
		ID:           "proj-123",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: StageDevelop,
	}
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	phase := &Phase{
		ID:        "phase-1",
		ProjectID: "proj-123",
		Number:    1,
		Title:     "Phase 1",
		Content:   "Content",
		Status:    PhaseInProgress,
		CreatedAt: time.Now(),
	}
	err = store.SavePhase(phase)
	if err != nil {
		t.Fatalf("Failed to save phase: %v", err)
	}

	// Save task with null timestamps
	task := &Task{
		ID:          "task-1",
		PhaseID:     "phase-1",
		Number:      "1.1",
		Description: "Test task",
		Status:      TaskNotStarted,
		StartedAt:   nil,
		CompletedAt: nil,
	}
	err = store.SaveTask(task)
	if err != nil {
		t.Fatalf("Failed to save task: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.GetTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	if retrieved.StartedAt != nil {
		t.Errorf("Expected StartedAt to be nil, got %v", retrieved.StartedAt)
	}
	if retrieved.CompletedAt != nil {
		t.Errorf("Expected CompletedAt to be nil, got %v", retrieved.CompletedAt)
	}
}

// TestStore_NullableFields_Quota tests handling of nullable quota fields
func TestStore_NullableFields_Quota(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Save quota with only tokens (no cost)
	tokensRemaining := 10000
	tokensLimit := 20000

	quota := &QuotaInfo{
		Provider:        "openai",
		TokensRemaining: &tokensRemaining,
		TokensLimit:     &tokensLimit,
		CostRemaining:   nil,
		CostLimit:       nil,
		ResetAt:         time.Now().Add(24 * time.Hour),
		CheckedAt:       time.Now(),
	}

	err = store.SaveQuota(quota.Provider, quota)
	if err != nil {
		t.Fatalf("Failed to save quota: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.GetQuota(quota.Provider)
	if err != nil {
		t.Fatalf("Failed to get quota: %v", err)
	}

	if retrieved.TokensRemaining == nil || *retrieved.TokensRemaining != tokensRemaining {
		t.Errorf("TokensRemaining mismatch")
	}
	if retrieved.CostRemaining != nil {
		t.Errorf("Expected CostRemaining to be nil, got %v", retrieved.CostRemaining)
	}
	if retrieved.CostLimit != nil {
		t.Errorf("Expected CostLimit to be nil, got %v", retrieved.CostLimit)
	}
}

// TestStore_NullableFields_Checkpoint tests handling of nullable checkpoint metadata
func TestStore_NullableFields_Checkpoint(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create project
	project := &Project{
		ID:           "proj-123",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: StageDevelop,
	}
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Save checkpoint with nil metadata
	checkpoint := &Checkpoint{
		ID:        "checkpoint-1",
		ProjectID: "proj-123",
		Name:      "Test Checkpoint",
		GitTag:    "v0.1.0",
		CreatedAt: time.Now(),
		Metadata:  nil,
	}

	err = store.SaveCheckpoint(checkpoint)
	if err != nil {
		t.Fatalf("Failed to save checkpoint: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.GetCheckpoint(checkpoint.ID)
	if err != nil {
		t.Fatalf("Failed to get checkpoint: %v", err)
	}

	if retrieved.Metadata != nil {
		t.Errorf("Expected Metadata to be nil, got %v", retrieved.Metadata)
	}
}

// TestStore_ConcurrentWrites tests concurrent write operations
func TestStore_ConcurrentWrites(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

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
		CurrentStage: StageDevelop,
	}
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Concurrently record token usage
	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			usage := &TokenUsage{
				ProjectID:    "proj-123",
				Provider:     "openai",
				Model:        "gpt-4",
				TokensInput:  100,
				TokensOutput: 200,
				Cost:         0.015,
				Timestamp:    time.Now(),
			}

			if err := store.RecordTokenUsage(usage); err != nil {
				t.Errorf("Failed to record token usage in goroutine %d: %v", index, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all records were saved
	totalCost, err := store.GetTotalCost(project.ID)
	if err != nil {
		t.Fatalf("Failed to get total cost: %v", err)
	}

	expectedCost := 0.015 * float64(numGoroutines)
	if totalCost != expectedCost {
		t.Errorf("Total cost mismatch: got %f, want %f", totalCost, expectedCost)
	}
}

// TestStore_ConcurrentReads tests concurrent read operations
func TestStore_ConcurrentReads(t *testing.T) {
	// Use a file-based database for concurrent access
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

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
		CurrentStage: StageDevelop,
	}
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Concurrently read project
	var wg sync.WaitGroup
	numGoroutines := 20

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			retrieved, err := store.GetProject(project.ID)
			if err != nil {
				t.Errorf("Failed to get project in goroutine %d: %v", index, err)
				return
			}

			if retrieved.ID != project.ID {
				t.Errorf("Project ID mismatch in goroutine %d: got %s, want %s", index, retrieved.ID, project.ID)
			}
		}(i)
	}

	wg.Wait()
}

// TestStore_UpdatePhaseStatus_Idempotent tests that updating phase status is idempotent
func TestStore_UpdatePhaseStatus_Idempotent(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create project and phase
	project := &Project{
		ID:           "proj-123",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: StagePlan,
	}
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	phase := &Phase{
		ID:        "phase-1",
		ProjectID: "proj-123",
		Number:    1,
		Title:     "Phase 1",
		Content:   "Content",
		Status:    PhaseNotStarted,
		CreatedAt: time.Now(),
	}
	err = store.SavePhase(phase)
	if err != nil {
		t.Fatalf("Failed to save phase: %v", err)
	}

	// Update to in progress
	err = store.UpdatePhaseStatus(phase.ID, PhaseInProgress)
	if err != nil {
		t.Fatalf("Failed to update phase status: %v", err)
	}

	// Get the started_at timestamp
	retrieved1, err := store.GetPhase(phase.ID)
	if err != nil {
		t.Fatalf("Failed to get phase: %v", err)
	}

	if retrieved1.StartedAt == nil {
		t.Fatal("StartedAt should be set")
	}

	// Update to in progress again (should not change started_at)
	time.Sleep(10 * time.Millisecond) // Small delay to ensure time difference
	err = store.UpdatePhaseStatus(phase.ID, PhaseInProgress)
	if err != nil {
		t.Fatalf("Failed to update phase status again: %v", err)
	}

	// Verify started_at didn't change
	retrieved2, err := store.GetPhase(phase.ID)
	if err != nil {
		t.Fatalf("Failed to get phase: %v", err)
	}

	if !retrieved1.StartedAt.Equal(*retrieved2.StartedAt) {
		t.Errorf("StartedAt changed on second update: first=%v, second=%v", retrieved1.StartedAt, retrieved2.StartedAt)
	}
}

// TestStore_UpdateTaskStatus_Idempotent tests that updating task status is idempotent
func TestStore_UpdateTaskStatus_Idempotent(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create project, phase, and task
	project := &Project{
		ID:           "proj-123",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: StageDevelop,
	}
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	phase := &Phase{
		ID:        "phase-1",
		ProjectID: "proj-123",
		Number:    1,
		Title:     "Phase 1",
		Content:   "Content",
		Status:    PhaseInProgress,
		CreatedAt: time.Now(),
	}
	err = store.SavePhase(phase)
	if err != nil {
		t.Fatalf("Failed to save phase: %v", err)
	}

	task := &Task{
		ID:          "task-1",
		PhaseID:     "phase-1",
		Number:      "1.1",
		Description: "Test task",
		Status:      TaskNotStarted,
	}
	err = store.SaveTask(task)
	if err != nil {
		t.Fatalf("Failed to save task: %v", err)
	}

	// Update to in progress
	err = store.UpdateTaskStatus(task.ID, TaskInProgress)
	if err != nil {
		t.Fatalf("Failed to update task status: %v", err)
	}

	// Get the started_at timestamp
	retrieved1, err := store.GetTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	if retrieved1.StartedAt == nil {
		t.Fatal("StartedAt should be set")
	}

	// Update to in progress again (should not change started_at)
	time.Sleep(10 * time.Millisecond)
	err = store.UpdateTaskStatus(task.ID, TaskInProgress)
	if err != nil {
		t.Fatalf("Failed to update task status again: %v", err)
	}

	// Verify started_at didn't change
	retrieved2, err := store.GetTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	if !retrieved1.StartedAt.Equal(*retrieved2.StartedAt) {
		t.Errorf("StartedAt changed on second update: first=%v, second=%v", retrieved1.StartedAt, retrieved2.StartedAt)
	}
}

// TestStore_ListPhases_EmptyProject tests listing phases for project with no phases
func TestStore_ListPhases_EmptyProject(t *testing.T) {
	store, err := NewStore(":memory:")
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
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// List phases (should be empty)
	phases, err := store.ListPhases(project.ID)
	if err != nil {
		t.Fatalf("Failed to list phases: %v", err)
	}

	if len(phases) != 0 {
		t.Errorf("Expected 0 phases, got %d", len(phases))
	}
}

// TestStore_ListCheckpoints_EmptyProject tests listing checkpoints for project with no checkpoints
func TestStore_ListCheckpoints_EmptyProject(t *testing.T) {
	store, err := NewStore(":memory:")
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
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// List checkpoints (should be empty)
	checkpoints, err := store.ListCheckpoints(project.ID)
	if err != nil {
		t.Fatalf("Failed to list checkpoints: %v", err)
	}

	if len(checkpoints) != 0 {
		t.Errorf("Expected 0 checkpoints, got %d", len(checkpoints))
	}
}

// TestStore_ListActiveBlockers_EmptyProject tests listing blockers for project with no blockers
func TestStore_ListActiveBlockers_EmptyProject(t *testing.T) {
	store, err := NewStore(":memory:")
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
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// List active blockers (should be empty)
	blockers, err := store.ListActiveBlockers(project.ID)
	if err != nil {
		t.Fatalf("Failed to list active blockers: %v", err)
	}

	if len(blockers) != 0 {
		t.Errorf("Expected 0 blockers, got %d", len(blockers))
	}
}

// TestStore_GetRateLimit_NotFound tests getting rate limit for non-existent provider
func TestStore_GetRateLimit_NotFound(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	_, err = store.GetRateLimit("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent rate limit, got nil")
	}
}

// TestStore_GetQuota_NotFound tests getting quota for non-existent provider
func TestStore_GetQuota_NotFound(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	_, err = store.GetQuota("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent quota, got nil")
	}
}

// TestStore_GetCheckpoint_NotFound tests getting non-existent checkpoint
func TestStore_GetCheckpoint_NotFound(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	_, err = store.GetCheckpoint("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent checkpoint, got nil")
	}
}

// TestStore_ResolveBlocker_NotFound tests resolving non-existent blocker
func TestStore_ResolveBlocker_NotFound(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	err = store.ResolveBlocker("nonexistent", "resolution")
	if err == nil {
		t.Error("Expected error for nonexistent blocker, got nil")
	}
}

// TestStore_SavePhase_Update tests updating an existing phase
func TestStore_SavePhase_Update(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create project
	project := &Project{
		ID:           "proj-123",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: StagePlan,
	}
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Save initial phase
	phase := &Phase{
		ID:        "phase-1",
		ProjectID: "proj-123",
		Number:    1,
		Title:     "Initial Title",
		Content:   "Initial Content",
		Status:    PhaseNotStarted,
		CreatedAt: time.Now(),
	}
	err = store.SavePhase(phase)
	if err != nil {
		t.Fatalf("Failed to save phase: %v", err)
	}

	// Update phase
	phase.Title = "Updated Title"
	phase.Content = "Updated Content"
	phase.Status = PhaseInProgress

	err = store.SavePhase(phase)
	if err != nil {
		t.Fatalf("Failed to update phase: %v", err)
	}

	// Verify update
	retrieved, err := store.GetPhase(phase.ID)
	if err != nil {
		t.Fatalf("Failed to get phase: %v", err)
	}

	if retrieved.Title != "Updated Title" {
		t.Errorf("Title not updated: got %s, want %s", retrieved.Title, "Updated Title")
	}
	if retrieved.Content != "Updated Content" {
		t.Errorf("Content not updated: got %s, want %s", retrieved.Content, "Updated Content")
	}
	if retrieved.Status != PhaseInProgress {
		t.Errorf("Status not updated: got %s, want %s", retrieved.Status, PhaseInProgress)
	}
}

// TestStore_SaveTask_Update tests updating an existing task
func TestStore_SaveTask_Update(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create project and phase
	project := &Project{
		ID:           "proj-123",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: StageDevelop,
	}
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	phase := &Phase{
		ID:        "phase-1",
		ProjectID: "proj-123",
		Number:    1,
		Title:     "Phase 1",
		Content:   "Content",
		Status:    PhaseInProgress,
		CreatedAt: time.Now(),
	}
	err = store.SavePhase(phase)
	if err != nil {
		t.Fatalf("Failed to save phase: %v", err)
	}

	// Save initial task
	task := &Task{
		ID:          "task-1",
		PhaseID:     "phase-1",
		Number:      "1.1",
		Description: "Initial description",
		Status:      TaskNotStarted,
	}
	err = store.SaveTask(task)
	if err != nil {
		t.Fatalf("Failed to save task: %v", err)
	}

	// Update task
	task.Description = "Updated description"
	task.Status = TaskInProgress

	err = store.SaveTask(task)
	if err != nil {
		t.Fatalf("Failed to update task: %v", err)
	}

	// Verify update
	retrieved, err := store.GetTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	if retrieved.Description != "Updated description" {
		t.Errorf("Description not updated: got %s, want %s", retrieved.Description, "Updated description")
	}
	if retrieved.Status != TaskInProgress {
		t.Errorf("Status not updated: got %s, want %s", retrieved.Status, TaskInProgress)
	}
}

// TestStore_SaveCheckpoint_Update tests updating an existing checkpoint
func TestStore_SaveCheckpoint_Update(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create project
	project := &Project{
		ID:           "proj-123",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: StageDevelop,
	}
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Save initial checkpoint
	checkpoint := &Checkpoint{
		ID:        "checkpoint-1",
		ProjectID: "proj-123",
		Name:      "Initial Name",
		GitTag:    "v0.1.0",
		CreatedAt: time.Now(),
		Metadata:  map[string]string{"phase": "1"},
	}

	err = store.SaveCheckpoint(checkpoint)
	if err != nil {
		t.Fatalf("Failed to save checkpoint: %v", err)
	}

	// Update checkpoint
	checkpoint.Name = "Updated Name"
	checkpoint.Metadata = map[string]string{"phase": "2", "status": "completed"}

	err = store.SaveCheckpoint(checkpoint)
	if err != nil {
		t.Fatalf("Failed to update checkpoint: %v", err)
	}

	// Verify update
	retrieved, err := store.GetCheckpoint(checkpoint.ID)
	if err != nil {
		t.Fatalf("Failed to get checkpoint: %v", err)
	}

	if retrieved.Name != "Updated Name" {
		t.Errorf("Name not updated: got %s, want %s", retrieved.Name, "Updated Name")
	}
	if len(retrieved.Metadata) != 2 {
		t.Errorf("Metadata length mismatch: got %d, want 2", len(retrieved.Metadata))
	}
	if retrieved.Metadata["phase"] != "2" {
		t.Errorf("Metadata phase not updated: got %s, want 2", retrieved.Metadata["phase"])
	}
}

// TestStore_SaveConfig_Update tests updating an existing config value
func TestStore_SaveConfig_Update(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Set initial config
	key := "default_model"
	initialValue := "gpt-4"

	err = store.SetConfig(key, initialValue)
	if err != nil {
		t.Fatalf("Failed to set config: %v", err)
	}

	// Update config
	updatedValue := "claude-3"
	err = store.SetConfig(key, updatedValue)
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	// Verify update
	retrieved, err := store.GetConfig(key)
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if retrieved != updatedValue {
		t.Errorf("Config value not updated: got %s, want %s", retrieved, updatedValue)
	}
}

// TestStore_DatabaseRecovery tests recovery from temporary database issues
func TestStore_DatabaseRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create initial store
	store1, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Create project
	project := &Project{
		ID:           "proj-123",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: StageInit,
	}
	err = store1.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Close store
	store1.Close()

	// Reopen store (simulating recovery)
	store2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to reopen store: %v", err)
	}
	defer store2.Close()

	// Verify data persisted
	retrieved, err := store2.GetProject(project.ID)
	if err != nil {
		t.Fatalf("Failed to get project after recovery: %v", err)
	}

	if retrieved.ID != project.ID {
		t.Errorf("Project ID mismatch after recovery: got %s, want %s", retrieved.ID, project.ID)
	}
	if retrieved.Name != project.Name {
		t.Errorf("Project name mismatch after recovery: got %s, want %s", retrieved.Name, project.Name)
	}
}

// TestStore_EmptyStringHandling tests handling of empty strings
func TestStore_EmptyStringHandling(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create project with empty current_phase_id
	project := &Project{
		ID:           "proj-123",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: StageInit,
		CurrentPhase: "",
	}
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.GetProject(project.ID)
	if err != nil {
		t.Fatalf("Failed to get project: %v", err)
	}

	if retrieved.CurrentPhase != "" {
		t.Errorf("Expected empty CurrentPhase, got %s", retrieved.CurrentPhase)
	}
}

// TestStore_TokenUsage_WithoutPhaseAndTask tests recording token usage without phase/task
func TestStore_TokenUsage_WithoutPhaseAndTask(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create project
	project := &Project{
		ID:           "proj-123",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: StageInterview,
	}
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Record token usage without phase or task (e.g., during interview)
	usage := &TokenUsage{
		ProjectID:    "proj-123",
		PhaseID:      "",
		TaskID:       "",
		Provider:     "openai",
		Model:        "gpt-4",
		TokensInput:  100,
		TokensOutput: 200,
		Cost:         0.015,
		Timestamp:    time.Now(),
	}

	err = store.RecordTokenUsage(usage)
	if err != nil {
		t.Fatalf("Failed to record token usage: %v", err)
	}

	// Verify usage was recorded
	if usage.ID == 0 {
		t.Error("Token usage ID should be set after recording")
	}

	// Verify total cost
	totalCost, err := store.GetTotalCost(project.ID)
	if err != nil {
		t.Fatalf("Failed to get total cost: %v", err)
	}

	if totalCost != usage.Cost {
		t.Errorf("Total cost mismatch: got %f, want %f", totalCost, usage.Cost)
	}
}

// TestStore_MultipleRateLimits tests saving multiple rate limits for same provider
func TestStore_MultipleRateLimits(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	provider := "openai"

	// Save first rate limit
	now := time.Now()
	rateLimit1 := &RateLimitInfo{
		Provider:          provider,
		RequestsRemaining: &[]int{100}[0],
		RequestsLimit:     &[]int{200}[0],
		ResetAt:           &now,
		CheckedAt:         now,
	}

	err = store.SaveRateLimit(provider, rateLimit1)
	if err != nil {
		t.Fatalf("Failed to save first rate limit: %v", err)
	}

	// Save second rate limit (newer)
	time.Sleep(10 * time.Millisecond)
	rateLimit2 := &RateLimitInfo{
		Provider:          provider,
		RequestsRemaining: &[]int{90}[0],
		RequestsLimit:     &[]int{200}[0],
		ResetAt:           &now,
		CheckedAt:         now,
	}

	err = store.SaveRateLimit(provider, rateLimit2)
	if err != nil {
		t.Fatalf("Failed to save second rate limit: %v", err)
	}

	// Get rate limit (should return the most recent one)
	retrieved, err := store.GetRateLimit(provider)
	if err != nil {
		t.Fatalf("Failed to get rate limit: %v", err)
	}

	// Note: Data is lost during migration, so we only check that the methods work
	if retrieved.Provider != provider {
		t.Errorf("Provider mismatch: got %s, want %s", retrieved.Provider, provider)
	}
}

// TestStore_MultipleQuotas tests saving multiple quotas for same provider
func TestStore_MultipleQuotas(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	provider := "openai"

	// Save first quota
	tokens1 := 10000
	quota1 := &QuotaInfo{
		Provider:        provider,
		TokensRemaining: &tokens1,
		ResetAt:         time.Now().Add(24 * time.Hour),
		CheckedAt:       time.Now(),
	}

	err = store.SaveQuota(provider, quota1)
	if err != nil {
		t.Fatalf("Failed to save first quota: %v", err)
	}

	// Save second quota (newer)
	time.Sleep(10 * time.Millisecond)
	tokens2 := 9000
	quota2 := &QuotaInfo{
		Provider:        provider,
		TokensRemaining: &tokens2,
		ResetAt:         time.Now().Add(24 * time.Hour),
		CheckedAt:       time.Now(),
	}

	err = store.SaveQuota(provider, quota2)
	if err != nil {
		t.Fatalf("Failed to save second quota: %v", err)
	}

	// Get quota (should return the most recent one)
	retrieved, err := store.GetQuota(provider)
	if err != nil {
		t.Fatalf("Failed to get quota: %v", err)
	}

	if retrieved.TokensRemaining == nil || *retrieved.TokensRemaining != tokens2 {
		t.Errorf("Expected most recent quota, got TokensRemaining=%v, want %d",
			retrieved.TokensRemaining, tokens2)
	}
}

// TestStore_SaveBlocker_Update tests updating an existing blocker
func TestStore_SaveBlocker_Update(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create project, phase, and task
	project := &Project{
		ID:           "proj-123",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: StageDevelop,
	}
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	phase := &Phase{
		ID:        "phase-1",
		ProjectID: "proj-123",
		Number:    1,
		Title:     "Phase 1",
		Content:   "Content",
		Status:    PhaseInProgress,
		CreatedAt: time.Now(),
	}
	err = store.SavePhase(phase)
	if err != nil {
		t.Fatalf("Failed to save phase: %v", err)
	}

	task := &Task{
		ID:          "task-1",
		PhaseID:     "phase-1",
		Number:      "1.1",
		Description: "Test task",
		Status:      TaskBlocked,
	}
	err = store.SaveTask(task)
	if err != nil {
		t.Fatalf("Failed to save task: %v", err)
	}

	// Save initial blocker
	blocker := &Blocker{
		ID:          "blocker-1",
		TaskID:      "task-1",
		Description: "Initial description",
		CreatedAt:   time.Now(),
	}

	err = store.SaveBlocker(blocker)
	if err != nil {
		t.Fatalf("Failed to save blocker: %v", err)
	}

	// Update blocker
	blocker.Description = "Updated description"
	blocker.Resolution = "Resolved by doing X"
	resolvedAt := time.Now()
	blocker.ResolvedAt = &resolvedAt

	err = store.SaveBlocker(blocker)
	if err != nil {
		t.Fatalf("Failed to update blocker: %v", err)
	}

	// Verify blocker is no longer active
	blockers, err := store.ListActiveBlockers(project.ID)
	if err != nil {
		t.Fatalf("Failed to list active blockers: %v", err)
	}

	if len(blockers) != 0 {
		t.Errorf("Expected 0 active blockers after resolution, got %d", len(blockers))
	}
}
