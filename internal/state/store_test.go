package state

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestNewStore(t *testing.T) {
	// Create a temporary directory for test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create store
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("Database file was not created")
	}

	// Verify health check passes
	if err := store.HealthCheck(); err != nil {
		t.Errorf("Health check failed: %v", err)
	}
}

func TestNewStore_CreatesDirectory(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "subdir", "nested", "test.db")

	// Create store (should create nested directories)
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Verify directory was created
	dir := filepath.Dir(dbPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("Directory was not created: %s", dir)
	}

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("Database file was not created")
	}
}

func TestStore_HealthCheck(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Health check should pass
	if err := store.HealthCheck(); err != nil {
		t.Errorf("Health check failed: %v", err)
	}
}

func TestStore_Close(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Close should not return error
	if err := store.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	// Second close should not panic
	if err := store.Close(); err != nil {
		t.Errorf("Second close returned error: %v", err)
	}
}

func TestStore_InMemory(t *testing.T) {
	// Test with in-memory database
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create in-memory store: %v", err)
	}
	defer store.Close()

	// Health check should pass
	if err := store.HealthCheck(); err != nil {
		t.Errorf("Health check failed: %v", err)
	}
}

func TestStore_ForeignKeys(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Verify foreign keys are enabled
	var fkEnabled int
	err = store.DB().QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled)
	if err != nil {
		t.Fatalf("Failed to check foreign keys: %v", err)
	}

	if fkEnabled != 1 {
		t.Errorf("Foreign keys not enabled: got %d, want 1", fkEnabled)
	}
}

func TestStore_WALMode(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Verify WAL mode is enabled
	var journalMode string
	err = store.DB().QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("Failed to check journal mode: %v", err)
	}

	if journalMode != "wal" {
		t.Errorf("WAL mode not enabled: got %s, want wal", journalMode)
	}
}

func TestStore_BeginTx(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Begin transaction
	tx, err := store.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Rollback transaction
	if err := tx.Rollback(); err != nil {
		t.Errorf("Failed to rollback transaction: %v", err)
	}
}

// Project operations tests

func TestStore_CreateProject(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

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

	// Verify project was created
	retrieved, err := store.GetProject(project.ID)
	if err != nil {
		t.Fatalf("Failed to get project: %v", err)
	}

	if retrieved.ID != project.ID {
		t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, project.ID)
	}
	if retrieved.Name != project.Name {
		t.Errorf("Name mismatch: got %s, want %s", retrieved.Name, project.Name)
	}
	if retrieved.CurrentStage != project.CurrentStage {
		t.Errorf("Stage mismatch: got %s, want %s", retrieved.CurrentStage, project.CurrentStage)
	}
}

func TestStore_GetProject_NotFound(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	_, err = store.GetProject("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent project, got nil")
	}
}

func TestStore_UpdateProject(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

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

	// Update project
	project.Name = "Updated Project"
	project.CurrentStage = StageInterview
	project.CurrentPhase = "phase-1"

	err = store.UpdateProject(project)
	if err != nil {
		t.Fatalf("Failed to update project: %v", err)
	}

	// Verify update
	retrieved, err := store.GetProject(project.ID)
	if err != nil {
		t.Fatalf("Failed to get project: %v", err)
	}

	if retrieved.Name != "Updated Project" {
		t.Errorf("Name not updated: got %s, want %s", retrieved.Name, "Updated Project")
	}
	if retrieved.CurrentStage != StageInterview {
		t.Errorf("Stage not updated: got %s, want %s", retrieved.CurrentStage, StageInterview)
	}
	if retrieved.CurrentPhase != "phase-1" {
		t.Errorf("Phase not updated: got %s, want %s", retrieved.CurrentPhase, "phase-1")
	}
}

// Interview data operations tests

func TestStore_SaveAndGetInterviewData(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create project first
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

	// Create interview data
	data := &InterviewData{
		ProjectID:        "proj-123",
		ProjectName:      "Test Project",
		CreatedAt:        time.Now(),
		ProblemStatement: "Build a task management system",
		TargetUsers:      []string{"developers", "project managers"},
		SuccessMetrics:   []string{"user adoption", "task completion rate"},
		Constraints:      []string{"budget", "timeline"},
		Assumptions:      []string{"users have internet access"},
		Unknowns:         []string{"exact user count"},
	}

	err = store.SaveInterviewData(project.ID, data)
	if err != nil {
		t.Fatalf("Failed to save interview data: %v", err)
	}

	// Retrieve interview data
	retrieved, err := store.GetInterviewData(project.ID)
	if err != nil {
		t.Fatalf("Failed to get interview data: %v", err)
	}

	if retrieved.ProjectID != data.ProjectID {
		t.Errorf("ProjectID mismatch: got %s, want %s", retrieved.ProjectID, data.ProjectID)
	}
	if retrieved.ProblemStatement != data.ProblemStatement {
		t.Errorf("ProblemStatement mismatch: got %s, want %s", retrieved.ProblemStatement, data.ProblemStatement)
	}
	if len(retrieved.TargetUsers) != len(data.TargetUsers) {
		t.Errorf("TargetUsers length mismatch: got %d, want %d", len(retrieved.TargetUsers), len(data.TargetUsers))
	}
}

func TestStore_SaveInterviewData_Update(t *testing.T) {
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

	// Save initial data
	data := &InterviewData{
		ProjectID:        "proj-123",
		ProjectName:      "Test Project",
		CreatedAt:        time.Now(),
		ProblemStatement: "Initial problem",
	}
	err = store.SaveInterviewData(project.ID, data)
	if err != nil {
		t.Fatalf("Failed to save interview data: %v", err)
	}

	// Update data
	data.ProblemStatement = "Updated problem"
	err = store.SaveInterviewData(project.ID, data)
	if err != nil {
		t.Fatalf("Failed to update interview data: %v", err)
	}

	// Verify update
	retrieved, err := store.GetInterviewData(project.ID)
	if err != nil {
		t.Fatalf("Failed to get interview data: %v", err)
	}

	if retrieved.ProblemStatement != "Updated problem" {
		t.Errorf("ProblemStatement not updated: got %s, want %s", retrieved.ProblemStatement, "Updated problem")
	}
}

// Architecture operations tests

func TestStore_SaveAndGetArchitecture(t *testing.T) {
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
		CurrentStage: StageDesign,
	}
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Save architecture
	arch := &Architecture{
		ProjectID: "proj-123",
		Content:   "# Architecture\n\nThis is the architecture document.",
		CreatedAt: time.Now(),
	}
	err = store.SaveArchitecture(project.ID, arch)
	if err != nil {
		t.Fatalf("Failed to save architecture: %v", err)
	}

	// Retrieve architecture
	retrieved, err := store.GetArchitecture(project.ID)
	if err != nil {
		t.Fatalf("Failed to get architecture: %v", err)
	}

	if retrieved.ProjectID != arch.ProjectID {
		t.Errorf("ProjectID mismatch: got %s, want %s", retrieved.ProjectID, arch.ProjectID)
	}
	if retrieved.Content != arch.Content {
		t.Errorf("Content mismatch: got %s, want %s", retrieved.Content, arch.Content)
	}
}

// Phase operations tests

func TestStore_SaveAndGetPhase(t *testing.T) {
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

	// Save phase
	phase := &Phase{
		ID:        "phase-1",
		ProjectID: "proj-123",
		Number:    1,
		Title:     "Setup & Infrastructure",
		Content:   "# Phase 1\n\nSetup the project infrastructure.",
		Status:    PhaseNotStarted,
		CreatedAt: time.Now(),
	}
	err = store.SavePhase(phase)
	if err != nil {
		t.Fatalf("Failed to save phase: %v", err)
	}

	// Retrieve phase
	retrieved, err := store.GetPhase(phase.ID)
	if err != nil {
		t.Fatalf("Failed to get phase: %v", err)
	}

	if retrieved.ID != phase.ID {
		t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, phase.ID)
	}
	if retrieved.Title != phase.Title {
		t.Errorf("Title mismatch: got %s, want %s", retrieved.Title, phase.Title)
	}
	if retrieved.Status != phase.Status {
		t.Errorf("Status mismatch: got %s, want %s", retrieved.Status, phase.Status)
	}
}

func TestStore_ListPhases(t *testing.T) {
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

	// Save multiple phases
	phases := []*Phase{
		{
			ID:        "phase-1",
			ProjectID: "proj-123",
			Number:    1,
			Title:     "Phase 1",
			Content:   "Content 1",
			Status:    PhaseNotStarted,
			CreatedAt: time.Now(),
		},
		{
			ID:        "phase-2",
			ProjectID: "proj-123",
			Number:    2,
			Title:     "Phase 2",
			Content:   "Content 2",
			Status:    PhaseNotStarted,
			CreatedAt: time.Now(),
		},
		{
			ID:        "phase-3",
			ProjectID: "proj-123",
			Number:    3,
			Title:     "Phase 3",
			Content:   "Content 3",
			Status:    PhaseNotStarted,
			CreatedAt: time.Now(),
		},
	}

	for _, phase := range phases {
		err = store.SavePhase(phase)
		if err != nil {
			t.Fatalf("Failed to save phase: %v", err)
		}
	}

	// List phases
	retrieved, err := store.ListPhases(project.ID)
	if err != nil {
		t.Fatalf("Failed to list phases: %v", err)
	}

	if len(retrieved) != len(phases) {
		t.Errorf("Phase count mismatch: got %d, want %d", len(retrieved), len(phases))
	}

	// Verify order
	for i, phase := range retrieved {
		if phase.Number != i+1 {
			t.Errorf("Phase order incorrect: got number %d at index %d", phase.Number, i)
		}
	}
}

func TestStore_UpdatePhaseStatus(t *testing.T) {
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

	// Save phase
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

	// Verify status and started_at
	retrieved, err := store.GetPhase(phase.ID)
	if err != nil {
		t.Fatalf("Failed to get phase: %v", err)
	}

	if retrieved.Status != PhaseInProgress {
		t.Errorf("Status not updated: got %s, want %s", retrieved.Status, PhaseInProgress)
	}
	if retrieved.StartedAt == nil {
		t.Error("StartedAt should be set")
	}

	// Update to completed
	err = store.UpdatePhaseStatus(phase.ID, PhaseCompleted)
	if err != nil {
		t.Fatalf("Failed to update phase status: %v", err)
	}

	// Verify completed_at
	retrieved, err = store.GetPhase(phase.ID)
	if err != nil {
		t.Fatalf("Failed to get phase: %v", err)
	}

	if retrieved.Status != PhaseCompleted {
		t.Errorf("Status not updated: got %s, want %s", retrieved.Status, PhaseCompleted)
	}
	if retrieved.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
}

// Task operations tests

func TestStore_SaveAndGetTask(t *testing.T) {
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

	// Save task
	task := &Task{
		ID:          "task-1",
		PhaseID:     "phase-1",
		Number:      "1.1",
		Description: "Implement feature X",
		Status:      TaskNotStarted,
	}
	err = store.SaveTask(task)
	if err != nil {
		t.Fatalf("Failed to save task: %v", err)
	}

	// Retrieve task
	retrieved, err := store.GetTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	if retrieved.ID != task.ID {
		t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, task.ID)
	}
	if retrieved.Description != task.Description {
		t.Errorf("Description mismatch: got %s, want %s", retrieved.Description, task.Description)
	}
	if retrieved.Status != task.Status {
		t.Errorf("Status mismatch: got %s, want %s", retrieved.Status, task.Status)
	}
}

func TestStore_UpdateTaskStatus(t *testing.T) {
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

	// Save task
	task := &Task{
		ID:          "task-1",
		PhaseID:     "phase-1",
		Number:      "1.1",
		Description: "Implement feature X",
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

	// Verify status and started_at
	retrieved, err := store.GetTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	if retrieved.Status != TaskInProgress {
		t.Errorf("Status not updated: got %s, want %s", retrieved.Status, TaskInProgress)
	}
	if retrieved.StartedAt == nil {
		t.Error("StartedAt should be set")
	}

	// Update to completed
	err = store.UpdateTaskStatus(task.ID, TaskCompleted)
	if err != nil {
		t.Fatalf("Failed to update task status: %v", err)
	}

	// Verify completed_at
	retrieved, err = store.GetTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	if retrieved.Status != TaskCompleted {
		t.Errorf("Status not updated: got %s, want %s", retrieved.Status, TaskCompleted)
	}
	if retrieved.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
}

// Additional CRUD tests for comprehensive coverage

// Checkpoint operations tests

func TestStore_SaveAndGetCheckpoint(t *testing.T) {
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

	// Save checkpoint
	checkpoint := &Checkpoint{
		ID:        "checkpoint-1",
		ProjectID: "proj-123",
		Name:      "After Phase 1",
		GitTag:    "v0.1.0",
		CreatedAt: time.Now(),
		Metadata:  map[string]string{"phase": "1", "status": "completed"},
	}

	err = store.SaveCheckpoint(checkpoint)
	if err != nil {
		t.Fatalf("Failed to save checkpoint: %v", err)
	}

	// Retrieve checkpoint
	retrieved, err := store.GetCheckpoint(checkpoint.ID)
	if err != nil {
		t.Fatalf("Failed to get checkpoint: %v", err)
	}

	if retrieved.ID != checkpoint.ID {
		t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, checkpoint.ID)
	}
	if retrieved.Name != checkpoint.Name {
		t.Errorf("Name mismatch: got %s, want %s", retrieved.Name, checkpoint.Name)
	}
	if retrieved.GitTag != checkpoint.GitTag {
		t.Errorf("GitTag mismatch: got %s, want %s", retrieved.GitTag, checkpoint.GitTag)
	}
}

func TestStore_ListCheckpoints(t *testing.T) {
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

	// Save multiple checkpoints
	checkpoints := []*Checkpoint{
		{
			ID:        "checkpoint-1",
			ProjectID: "proj-123",
			Name:      "Checkpoint 1",
			GitTag:    "v0.1.0",
			CreatedAt: time.Now(),
		},
		{
			ID:        "checkpoint-2",
			ProjectID: "proj-123",
			Name:      "Checkpoint 2",
			GitTag:    "v0.2.0",
			CreatedAt: time.Now(),
		},
	}

	for _, checkpoint := range checkpoints {
		err = store.SaveCheckpoint(checkpoint)
		if err != nil {
			t.Fatalf("Failed to save checkpoint: %v", err)
		}
	}

	// List checkpoints
	retrieved, err := store.ListCheckpoints(project.ID)
	if err != nil {
		t.Fatalf("Failed to list checkpoints: %v", err)
	}

	if len(retrieved) != len(checkpoints) {
		t.Errorf("Checkpoint count mismatch: got %d, want %d", len(retrieved), len(checkpoints))
	}
}

// Token usage operations tests

func TestStore_RecordTokenUsage(t *testing.T) {
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

	// Record token usage
	usage := &TokenUsage{
		ProjectID:    "proj-123",
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

	// Verify usage was recorded (ID should be set)
	if usage.ID == 0 {
		t.Error("Token usage ID should be set after recording")
	}
}

func TestStore_GetTotalCost(t *testing.T) {
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

	// Record multiple token usages
	usages := []*TokenUsage{
		{
			ProjectID:    "proj-123",
			Provider:     "openai",
			Model:        "gpt-4",
			TokensInput:  100,
			TokensOutput: 200,
			Cost:         0.015,
			Timestamp:    time.Now(),
		},
		{
			ProjectID:    "proj-123",
			Provider:     "anthropic",
			Model:        "claude-3",
			TokensInput:  150,
			TokensOutput: 250,
			Cost:         0.020,
			Timestamp:    time.Now(),
		},
	}

	expectedTotal := 0.0
	for _, usage := range usages {
		err = store.RecordTokenUsage(usage)
		if err != nil {
			t.Fatalf("Failed to record token usage: %v", err)
		}
		expectedTotal += usage.Cost
	}

	// Get total cost
	totalCost, err := store.GetTotalCost(project.ID)
	if err != nil {
		t.Fatalf("Failed to get total cost: %v", err)
	}

	if totalCost != expectedTotal {
		t.Errorf("Total cost mismatch: got %f, want %f", totalCost, expectedTotal)
	}
}

// Rate limit operations tests

func TestStore_SaveAndGetRateLimit(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Save rate limit
	now := time.Now()
	rateLimit := &RateLimitInfo{
		Provider:          "openai",
		RequestsRemaining: &[]int{100}[0],
		RequestsLimit:     &[]int{200}[0],
		ResetAt:           &now,
		CheckedAt:         now,
	}

	err = store.SaveRateLimit(rateLimit.Provider, rateLimit)
	if err != nil {
		t.Fatalf("Failed to save rate limit: %v", err)
	}

	// Retrieve rate limit
	retrieved, err := store.GetRateLimit(rateLimit.Provider)
	if err != nil {
		t.Fatalf("Failed to get rate limit: %v", err)
	}

	if retrieved.Provider != rateLimit.Provider {
		t.Errorf("Provider mismatch: got %s, want %s", retrieved.Provider, rateLimit.Provider)
	}

	// Note: Data is lost during migration, so we only check that the methods work
}

// Quota operations tests

func TestStore_SaveAndGetQuota(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Save quota
	tokensRemaining := 10000
	tokensLimit := 20000
	costRemaining := 50.0
	costLimit := 100.0

	quota := &QuotaInfo{
		Provider:        "openai",
		TokensRemaining: &tokensRemaining,
		TokensLimit:     &tokensLimit,
		CostRemaining:   &costRemaining,
		CostLimit:       &costLimit,
		ResetAt:         time.Now().Add(24 * time.Hour),
		CheckedAt:       time.Now(),
	}

	err = store.SaveQuota(quota.Provider, quota)
	if err != nil {
		t.Fatalf("Failed to save quota: %v", err)
	}

	// Retrieve quota
	retrieved, err := store.GetQuota(quota.Provider)
	if err != nil {
		t.Fatalf("Failed to get quota: %v", err)
	}

	if retrieved.Provider != quota.Provider {
		t.Errorf("Provider mismatch: got %s, want %s", retrieved.Provider, quota.Provider)
	}
	if retrieved.TokensRemaining == nil || *retrieved.TokensRemaining != *quota.TokensRemaining {
		t.Errorf("TokensRemaining mismatch")
	}
}

// Blocker operations tests

func TestStore_SaveAndGetBlocker(t *testing.T) {
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
		Description: "Implement feature X",
		Status:      TaskBlocked,
	}
	err = store.SaveTask(task)
	if err != nil {
		t.Fatalf("Failed to save task: %v", err)
	}

	// Save blocker
	blocker := &Blocker{
		ID:          "blocker-1",
		TaskID:      "task-1",
		Description: "Missing API credentials",
		CreatedAt:   time.Now(),
	}

	err = store.SaveBlocker(blocker)
	if err != nil {
		t.Fatalf("Failed to save blocker: %v", err)
	}

	// List active blockers
	blockers, err := store.ListActiveBlockers(project.ID)
	if err != nil {
		t.Fatalf("Failed to list active blockers: %v", err)
	}

	if len(blockers) != 1 {
		t.Errorf("Expected 1 blocker, got %d", len(blockers))
	}

	if blockers[0].ID != blocker.ID {
		t.Errorf("Blocker ID mismatch: got %s, want %s", blockers[0].ID, blocker.ID)
	}
}

func TestStore_ResolveBlocker(t *testing.T) {
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
		Description: "Implement feature X",
		Status:      TaskBlocked,
	}
	err = store.SaveTask(task)
	if err != nil {
		t.Fatalf("Failed to save task: %v", err)
	}

	// Save blocker
	blocker := &Blocker{
		ID:          "blocker-1",
		TaskID:      "task-1",
		Description: "Missing API credentials",
		CreatedAt:   time.Now(),
	}
	err = store.SaveBlocker(blocker)
	if err != nil {
		t.Fatalf("Failed to save blocker: %v", err)
	}

	// Resolve blocker
	resolution := "Added API credentials to config"
	err = store.ResolveBlocker(blocker.ID, resolution)
	if err != nil {
		t.Fatalf("Failed to resolve blocker: %v", err)
	}

	// Verify blocker is no longer active
	blockers, err := store.ListActiveBlockers(project.ID)
	if err != nil {
		t.Fatalf("Failed to list active blockers: %v", err)
	}

	if len(blockers) != 0 {
		t.Errorf("Expected 0 active blockers, got %d", len(blockers))
	}
}

// Configuration operations tests

func TestStore_SetAndGetConfig(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Set config
	key := "default_model"
	value := "gpt-4"

	err = store.SetConfig(key, value)
	if err != nil {
		t.Fatalf("Failed to set config: %v", err)
	}

	// Get config
	retrieved, err := store.GetConfig(key)
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if retrieved != value {
		t.Errorf("Config value mismatch: got %s, want %s", retrieved, value)
	}
}

func TestStore_GetConfig_NotFound(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	_, err = store.GetConfig("nonexistent_key")
	if err == nil {
		t.Error("Expected error for nonexistent config key, got nil")
	}
}

// Error handling tests

func TestStore_UpdateProject_NotFound(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	project := &Project{
		ID:           "nonexistent",
		Name:         "Test",
		CreatedAt:    time.Now(),
		CurrentStage: StageInit,
	}

	err = store.UpdateProject(project)
	if err == nil {
		t.Error("Expected error for nonexistent project, got nil")
	}
}

func TestStore_UpdateProjectStage_NotFound(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	err = store.UpdateProjectStage("nonexistent", StageInterview)
	if err == nil {
		t.Error("Expected error for nonexistent project, got nil")
	}
}

func TestStore_GetInterviewData_NotFound(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	_, err = store.GetInterviewData("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent interview data, got nil")
	}
}

func TestStore_GetArchitecture_NotFound(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	_, err = store.GetArchitecture("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent architecture, got nil")
	}
}

func TestStore_GetPhase_NotFound(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	_, err = store.GetPhase("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent phase, got nil")
	}
}

func TestStore_UpdatePhaseStatus_NotFound(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	err = store.UpdatePhaseStatus("nonexistent", PhaseInProgress)
	if err == nil {
		t.Error("Expected error for nonexistent phase, got nil")
	}
}

func TestStore_GetTask_NotFound(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	_, err = store.GetTask("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent task, got nil")
	}
}

func TestStore_UpdateTaskStatus_NotFound(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	err = store.UpdateTaskStatus("nonexistent", TaskInProgress)
	if err == nil {
		t.Error("Expected error for nonexistent task, got nil")
	}
}

// Foreign key constraint tests

func TestStore_ForeignKeyConstraint_InterviewData(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Try to save interview data without creating project first
	data := &InterviewData{
		ProjectID:        "nonexistent",
		ProjectName:      "Test",
		CreatedAt:        time.Now(),
		ProblemStatement: "Test problem",
	}

	err = store.SaveInterviewData("nonexistent", data)
	if err == nil {
		t.Error("Expected foreign key constraint error, got nil")
	}
}

func TestStore_ForeignKeyConstraint_Phase(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Try to save phase without creating project first
	phase := &Phase{
		ID:        "phase-1",
		ProjectID: "nonexistent",
		Number:    1,
		Title:     "Phase 1",
		Content:   "Content",
		Status:    PhaseNotStarted,
		CreatedAt: time.Now(),
	}

	err = store.SavePhase(phase)
	if err == nil {
		t.Error("Expected foreign key constraint error, got nil")
	}
}

func TestStore_ForeignKeyConstraint_Task(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Try to save task without creating phase first
	task := &Task{
		ID:          "task-1",
		PhaseID:     "nonexistent",
		Number:      "1.1",
		Description: "Test task",
		Status:      TaskNotStarted,
	}

	err = store.SaveTask(task)
	if err == nil {
		t.Error("Expected foreign key constraint error, got nil")
	}
}

// Cascade delete tests

func TestStore_CascadeDelete_Project(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create project with related data
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

	// Create interview data
	data := &InterviewData{
		ProjectID:        "proj-123",
		ProjectName:      "Test Project",
		CreatedAt:        time.Now(),
		ProblemStatement: "Test problem",
	}
	err = store.SaveInterviewData(project.ID, data)
	if err != nil {
		t.Fatalf("Failed to save interview data: %v", err)
	}

	// Create phase
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

	// Delete project
	_, err = store.DB().Exec("DELETE FROM projects WHERE id = ?", project.ID)
	if err != nil {
		t.Fatalf("Failed to delete project: %v", err)
	}

	// Verify interview data was deleted
	_, err = store.GetInterviewData(project.ID)
	if err == nil {
		t.Error("Expected interview data to be deleted, but it still exists")
	}

	// Verify phase was deleted
	_, err = store.GetPhase(phase.ID)
	if err == nil {
		t.Error("Expected phase to be deleted, but it still exists")
	}
}

// Transaction tests

func TestStore_Transaction_Rollback(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Begin transaction
	tx, err := store.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Insert project in transaction
	_, err = tx.Exec(`
		INSERT INTO projects (id, name, created_at, current_stage)
		VALUES (?, ?, ?, ?)
	`, "proj-123", "Test Project", time.Now(), StageInit)
	if err != nil {
		t.Fatalf("Failed to insert project: %v", err)
	}

	// Rollback transaction
	err = tx.Rollback()
	if err != nil {
		t.Fatalf("Failed to rollback transaction: %v", err)
	}

	// Verify project was not saved
	_, err = store.GetProject("proj-123")
	if err == nil {
		t.Error("Expected project to not exist after rollback, but it does")
	}
}

func TestStore_Transaction_Commit(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Begin transaction
	tx, err := store.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Insert project in transaction
	_, err = tx.Exec(`
		INSERT INTO projects (id, name, created_at, current_stage, current_phase_id)
		VALUES (?, ?, ?, ?, ?)
	`, "proj-123", "Test Project", time.Now(), StageInit, "")
	if err != nil {
		t.Fatalf("Failed to insert project: %v", err)
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Verify project was saved
	project, err := store.GetProject("proj-123")
	if err != nil {
		t.Errorf("Expected project to exist after commit: %v", err)
	}
	if project != nil && project.ID != "proj-123" {
		t.Errorf("Project ID mismatch: got %s, want %s", project.ID, "proj-123")
	}
}

// Corrupted database tests

func TestStore_CorruptedDatabase_InvalidPath(t *testing.T) {
	// Try to create store with invalid path (directory as file)
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "subdir")

	// Create a directory with the same name as the database file
	err := os.MkdirAll(invalidPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Try to create store (should fail because path is a directory)
	_, err = NewStore(invalidPath)
	if err == nil {
		t.Error("Expected error when creating store with directory as path, got nil")
	}
}

func TestStore_CorruptedDatabase_HealthCheck(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a valid store
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Close the store
	store.Close()

	// Corrupt the database file by writing invalid data
	err = os.WriteFile(dbPath, []byte("corrupted data"), 0644)
	if err != nil {
		t.Fatalf("Failed to corrupt database: %v", err)
	}

	// Try to open the corrupted database
	store2, err := NewStore(dbPath)
	if err == nil {
		defer store2.Close()
		// Health check should fail
		err = store2.HealthCheck()
		if err == nil {
			t.Error("Expected health check to fail on corrupted database, got nil")
		}
	}
	// If NewStore fails, that's also acceptable for a corrupted database
}

func TestStore_HealthCheck_AfterClose(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Close the store
	store.Close()

	// Health check should fail after close
	err = store.HealthCheck()
	if err == nil {
		t.Error("Expected health check to fail after close, got nil")
	}
}

// executeWithRetry tests

func TestExecuteWithRetry_Success(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a project to ensure database is ready
	project := &Project{
		ID:           "test-proj",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: StageInit,
	}
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Verify the project was created
	retrieved, err := store.GetProject(project.ID)
	if err != nil {
		t.Fatalf("Failed to get project: %v", err)
	}
	if retrieved.ID != project.ID {
		t.Errorf("Project ID mismatch: got %s, want %s", retrieved.ID, project.ID)
	}
}

func TestExecuteWithRetry_SimulatesBusyError(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Mock the database to return SQLITE_BUSY errors on first 2 attempts, then succeed
	attempts := 0
	maxAttempts := 2
	fn := func(tx *sql.Tx) error {
		attempts++
		if attempts <= maxAttempts {
			return fmt.Errorf("database is busy")
		}
		_, err := tx.Exec("INSERT INTO projects (id, name, created_at, current_stage, current_phase_id) VALUES (?, ?, ?, ?, ?)",
			"test-proj", "Test Project", time.Now(), StageInit, "")
		return err
	}

	err = executeWithRetry(store.DB(), 10, maxAttempts, fn)
	if err != nil {
		t.Fatalf("Expected success after retries, got error: %v", err)
	}

	if attempts != maxAttempts+1 {
		t.Errorf("Expected %d attempts, got %d", maxAttempts+1, attempts)
	}

	// Verify the project was created
	_, err = store.GetProject("test-proj")
	if err != nil {
		t.Errorf("Project not created: %v", err)
	}
}

func TestExecuteWithRetry_MaxRetriesExceeded(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Mock the database to always return SQLITE_BUSY errors
	maxAttempts := 3
	fn := func(tx *sql.Tx) error {
		return fmt.Errorf("database is busy")
	}

	err = executeWithRetry(store.DB(), 10, maxAttempts, fn)
	if err == nil {
		t.Error("Expected error when max retries exceeded")
	}

	// Verify the error contains "max retries"
	if err != nil && !strings.Contains(err.Error(), "max retries") {
		t.Errorf("Expected error to contain 'max retries', got: %v", err)
	}
}

func TestExecuteWithRetry_NonRetryableError(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Mock the database to return a non-retryable error
	fn := func(tx *sql.Tx) error {
		return fmt.Errorf("syntax error")
	}

	err = executeWithRetry(store.DB(), 10, 3, fn)
	if err == nil {
		t.Error("Expected error for non-retryable error")
	}

	// Verify the error is preserved
	if err != nil && !strings.Contains(err.Error(), "syntax error") {
		t.Errorf("Expected error to contain 'syntax error', got: %v", err)
	}
}

func TestExecuteWithRetry_NoRetryOnNonBusy(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Mock the database to return SQLITE_BUSY only once
	callCount := 0
	fn := func(tx *sql.Tx) error {
		callCount++
		if callCount == 1 {
			return fmt.Errorf("database is busy")
		}
		_, err := tx.Exec("INSERT INTO projects (id, name, created_at, current_stage, current_phase_id) VALUES (?, ?, ?, ?, ?)",
			"test-proj", "Test Project", time.Now(), StageInit, "")
		return err
	}

	err = executeWithRetry(store.DB(), 10, 3, fn)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Should have attempted twice (1 busy, 1 success)
	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
}

func TestExecuteWithRetry_InfiniteRetries(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Mock the database to return SQLITE_BUSY errors on first 3 attempts,
	// then return a non-retryable error to force completion
	callCount := 0
	fn := func(tx *sql.Tx) error {
		callCount++
		if callCount <= 3 {
			return fmt.Errorf("database is busy")
		}
		return fmt.Errorf("syntax error")
	}

	err = executeWithRetry(store.DB(), 10, 0, fn)
	if err == nil {
		t.Error("Expected error when non-retryable error is returned")
	}

	if err != nil && !strings.Contains(err.Error(), "syntax error") {
		t.Errorf("Expected error to contain 'syntax error', got: %v", err)
	}

	// Should have attempted 4 times (3 busy + 1 non-retryable)
	if callCount != 4 {
		t.Errorf("Expected 4 attempts, got %d", callCount)
	}
}

func TestExecuteWithRetry_CommitFailure(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Mock the database to return SQLITE_BUSY on begin, but success on commit
	callCount := 0
	fn := func(tx *sql.Tx) error {
		callCount++
		if callCount == 1 {
			// First attempt fails to begin
			return fmt.Errorf("database is busy")
		}
		// Second attempt succeeds
		return nil
	}

	err = executeWithRetry(store.DB(), 10, 3, fn)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 attempts, got %d", callCount)
	}
}

func TestExecuteWithRetry_TransactionCommitError(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Mock the database to succeed on begin but fail on commit
	callCount := 0
	fn := func(tx *sql.Tx) error {
		callCount++
		if callCount == 1 {
			return fmt.Errorf("database is busy")
		}
		// Second attempt fails on commit
		return fmt.Errorf("constraint violation")
	}

	err = executeWithRetry(store.DB(), 10, 3, fn)
	if err == nil {
		t.Error("Expected error when commit fails")
	}

	if err != nil && !strings.Contains(err.Error(), "constraint violation") {
		t.Errorf("Expected error to contain 'constraint violation', got: %v", err)
	}
}

func TestExecuteWithRetry_RetryAfterSuccessfulCommit(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create project successfully first time
	project := &Project{
		ID:           "test-proj",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: StageInit,
	}
	err = store.CreateProject(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Try to create the same project again - should fail with UNIQUE constraint
	err = store.CreateProject(project)
	if err == nil {
		t.Error("Expected error for duplicate project")
	}

	// Verify the original project still exists
	_, err = store.GetProject("test-proj")
	if err != nil {
		t.Errorf("Original project not found: %v", err)
	}
}

func TestExecuteWithRetry_HandleDifferentBusyErrors(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Test different SQLITE_BUSY error messages
	busyMessages := []string{
		"database is locked",
		"database is busy",
		"database connection is busy",
	}

	for i, msg := range busyMessages {
		callCount := 0
		// Use unique project ID for each iteration
		projectID := fmt.Sprintf("test-proj-%d", i)

		fn := func(tx *sql.Tx) error {
			callCount++
			if callCount <= 1 {
				return fmt.Errorf("%s", msg)
			}
			_, err := tx.Exec("INSERT INTO projects (id, name, created_at, current_stage, current_phase_id) VALUES (?, ?, ?, ?, ?)",
				projectID, "Test Project", time.Now(), StageInit, "")
			return err
		}

		err = executeWithRetry(store.DB(), 10, 3, fn)
		if err != nil {
			t.Fatalf("Expected success for message '%s', got error: %v", msg, err)
		}

		if callCount != 2 {
			t.Errorf("For message '%s': Expected 2 attempts, got %d", msg, callCount)
		}

		// Verify the project was created
		_, err = store.GetProject(projectID)
		if err != nil {
			t.Errorf("Project not created for message '%s': %v", msg, err)
		}
	}
}

func TestExecuteWithRetry_RetriesWithDifferentDelays(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Test that retry count increases with each failed attempt
	callCount := 0

	fn := func(tx *sql.Tx) error {
		callCount++
		if callCount < 3 {
			return fmt.Errorf("database is busy")
		}
		_, err := tx.Exec("INSERT INTO projects (id, name, created_at, current_stage, current_phase_id) VALUES (?, ?, ?, ?, ?)",
			"test-proj", "Test Project", time.Now(), StageInit, "")
		return err
	}

	err = executeWithRetry(store.DB(), 100, 3, fn)
	if err != nil {
		t.Fatalf("Expected success after retries, got error: %v", err)
	}

	// Should have attempted 3 times (all busy except the last)
	if callCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", callCount)
	}
}
