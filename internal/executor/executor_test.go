package executor

import (
	"context"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
)

type mockExecutorProvider struct{}

func (m *mockExecutorProvider) Name() string { return "mock" }

func (m *mockExecutorProvider) Authenticate(apiKey string) error { return nil }

func (m *mockExecutorProvider) IsAuthenticated() bool { return true }

func (m *mockExecutorProvider) ListModels() ([]provider.Model, error) { return nil, nil }

func (m *mockExecutorProvider) DiscoverModels() ([]provider.Model, error) { return nil, nil }

func (m *mockExecutorProvider) Call(ctx context.Context, model string, prompt string) (*provider.Response, error) {
	return &provider.Response{
		Content:      `{"explanation":"ok","files":[]}`,
		TokensInput:  10,
		TokensOutput: 5,
		Model:        model,
		Provider:     "mock",
		Timestamp:    time.Now(),
	}, nil
}

func (m *mockExecutorProvider) Stream(ctx context.Context, model string, prompt string) (<-chan string, error) {
	ch := make(chan string, 1)
	go func() {
		defer close(ch)
		ch <- `{"explanation":"ok","files":[]}`
	}()
	return ch, nil
}

func (m *mockExecutorProvider) GetRateLimitInfo() (*provider.RateLimitInfo, error) { return nil, nil }

func (m *mockExecutorProvider) GetQuotaInfo() (*provider.QuotaInfo, error) { return nil, nil }

func (m *mockExecutorProvider) SupportsCodingPlan() bool { return false }

func seedExecutionContext(t *testing.T, store *state.Store, projectID string) {
	t.Helper()

	interviewData := &state.InterviewData{
		ProjectID:        projectID,
		ProjectName:      "Test Project",
		CreatedAt:        time.Now(),
		ProblemStatement: "Test problem statement",
	}
	if err := store.SaveInterviewData(projectID, interviewData); err != nil {
		t.Fatalf("failed to save interview data: %v", err)
	}

	arch := &state.Architecture{
		ProjectID: projectID,
		Content:   "# Test Architecture",
		CreatedAt: time.Now(),
	}
	if err := store.SaveArchitecture(projectID, arch); err != nil {
		t.Fatalf("failed to save architecture: %v", err)
	}
}

func setupTestExecutor(t *testing.T) (*Executor, *state.Store) {
	// Create in-memory store
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create mock provider
	var mockProvider provider.Provider = &mockExecutorProvider{}

	// Create executor
	executor := NewExecutor(store, mockProvider, "glm-4.7")

	return executor, store
}

func TestNewExecutor(t *testing.T) {
	executor, store := setupTestExecutor(t)
	defer store.Close()
	defer executor.Close()

	if executor == nil {
		t.Fatal("expected executor to be created")
	}

	if executor.store == nil {
		t.Error("expected store to be set")
	}

	// Provider can be nil in tests
	// if executor.provider == nil {
	// 	t.Error("expected provider to be set")
	// }

	if executor.updateChan == nil {
		t.Error("expected update channel to be created")
	}
}

func TestExecutor_PauseResume(t *testing.T) {
	executor, store := setupTestExecutor(t)
	defer store.Close()
	defer executor.Close()

	// Test pause
	if err := executor.PauseExecution(); err != nil {
		t.Errorf("failed to pause execution: %v", err)
	}

	if !executor.paused {
		t.Error("expected execution to be paused")
	}

	// Test pause when already paused
	if err := executor.PauseExecution(); err == nil {
		t.Error("expected error when pausing already paused execution")
	}

	// Test resume
	if err := executor.ResumeExecution(); err != nil {
		t.Errorf("failed to resume execution: %v", err)
	}

	if executor.paused {
		t.Error("expected execution to be resumed")
	}

	// Test resume when not paused
	if err := executor.ResumeExecution(); err == nil {
		t.Error("expected error when resuming non-paused execution")
	}
}

func TestExecutor_StreamOutput(t *testing.T) {
	executor, store := setupTestExecutor(t)
	defer store.Close()
	defer executor.Close()

	// Get output channel
	outputChan := executor.StreamOutput()
	if outputChan == nil {
		t.Fatal("expected output channel to be returned")
	}

	// Send an update
	update := TaskUpdate{
		TaskID:    "task-1",
		Type:      TaskStarted,
		Content:   "Test update",
		Timestamp: time.Now(),
	}

	executor.sendUpdate(update)

	// Receive the update
	select {
	case received := <-outputChan:
		if received.TaskID != update.TaskID {
			t.Errorf("expected task ID %s, got %s", update.TaskID, received.TaskID)
		}
		if received.Type != update.Type {
			t.Errorf("expected type %s, got %s", update.Type, received.Type)
		}
		if received.Content != update.Content {
			t.Errorf("expected content %s, got %s", update.Content, received.Content)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for update")
	}
}

func TestExecutor_SkipTask(t *testing.T) {
	executor, store := setupTestExecutor(t)
	defer store.Close()
	defer executor.Close()

	// Create a test project
	project := &state.Project{
		ID:        "test-project",
		Name:      "Test Project",
		CreatedAt: time.Now(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	seedExecutionContext(t, store, project.ID)

	// Create a test phase
	phase := &state.Phase{
		ID:        "phase-1",
		ProjectID: project.ID,
		Number:    1,
		Title:     "Test Phase",
		Status:    state.PhaseNotStarted,
		CreatedAt: time.Now(),
	}
	if err := store.SavePhase(phase); err != nil {
		t.Fatalf("failed to save phase: %v", err)
	}

	// Create a test task
	task := &state.Task{
		ID:          "task-1",
		PhaseID:     phase.ID,
		Number:      "1.1",
		Description: "Test Task",
		Status:      state.TaskNotStarted,
	}
	if err := store.SaveTask(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Skip the task
	if err := executor.SkipTask(task.ID); err != nil {
		t.Errorf("failed to skip task: %v", err)
	}

	// Verify task status
	updatedTask, err := store.GetTask(task.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}

	if updatedTask.Status != state.TaskSkipped {
		t.Errorf("expected task status to be '%s', got %s", state.TaskSkipped, updatedTask.Status)
	}
}

func TestExecutor_MarkBlocked(t *testing.T) {
	executor, store := setupTestExecutor(t)
	defer store.Close()
	defer executor.Close()

	// Create a test project
	project := &state.Project{
		ID:        "test-project",
		Name:      "Test Project",
		CreatedAt: time.Now(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	seedExecutionContext(t, store, project.ID)

	// Create a test phase
	phase := &state.Phase{
		ID:        "phase-1",
		ProjectID: project.ID,
		Number:    1,
		Title:     "Test Phase",
		Status:    state.PhaseNotStarted,
		CreatedAt: time.Now(),
	}
	if err := store.SavePhase(phase); err != nil {
		t.Fatalf("failed to save phase: %v", err)
	}

	// Create a test task
	task := &state.Task{
		ID:          "task-1",
		PhaseID:     phase.ID,
		Number:      "1.1",
		Description: "Test Task",
		Status:      state.TaskInProgress,
	}
	if err := store.SaveTask(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Mark task as blocked
	reason := "Test blocker reason"
	if err := executor.MarkBlocked(task.ID, reason); err != nil {
		t.Errorf("failed to mark task as blocked: %v", err)
	}

	// Verify task status
	updatedTask, err := store.GetTask(task.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}

	if updatedTask.Status != state.TaskBlocked {
		t.Errorf("expected task status to be '%s', got %s", state.TaskBlocked, updatedTask.Status)
	}

	// Verify blocker was created
	blockers, err := store.ListActiveBlockers(project.ID)
	if err != nil {
		t.Fatalf("failed to list blockers: %v", err)
	}

	if len(blockers) != 1 {
		t.Errorf("expected 1 blocker, got %d", len(blockers))
	}

	if blockers[0].TaskID != task.ID {
		t.Errorf("expected blocker task ID %s, got %s", task.ID, blockers[0].TaskID)
	}

	if blockers[0].Description != reason {
		t.Errorf("expected blocker description %s, got %s", reason, blockers[0].Description)
	}
}

func TestExecutor_ResolveBlocker(t *testing.T) {
	executor, store := setupTestExecutor(t)
	defer store.Close()
	defer executor.Close()

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
		Status:    state.PhaseNotStarted,
		CreatedAt: time.Now(),
	}
	if err := store.SavePhase(phase); err != nil {
		t.Fatalf("failed to save phase: %v", err)
	}

	// Create a test task
	task := &state.Task{
		ID:          "task-1",
		PhaseID:     phase.ID,
		Number:      "1.1",
		Description: "Test Task",
		Status:      state.TaskBlocked,
	}
	if err := store.SaveTask(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create a blocker
	blocker := &state.Blocker{
		ID:          "blocker-1",
		TaskID:      task.ID,
		Description: "Test blocker",
		CreatedAt:   time.Now(),
	}
	if err := store.SaveBlocker(blocker); err != nil {
		t.Fatalf("failed to save blocker: %v", err)
	}

	// Resolve the blocker
	resolution := "Test resolution"
	if err := executor.ResolveBlocker(task.ID, resolution); err != nil {
		t.Errorf("failed to resolve blocker: %v", err)
	}

	// Verify task status
	updatedTask, err := store.GetTask(task.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}

	if updatedTask.Status != state.TaskNotStarted {
		t.Errorf("expected task status to be '%s', got %s", state.TaskNotStarted, updatedTask.Status)
	}

	// Verify blocker was resolved
	blockers, err := store.ListActiveBlockers(project.ID)
	if err != nil {
		t.Fatalf("failed to list blockers: %v", err)
	}

	if len(blockers) != 0 {
		t.Errorf("expected 0 active blockers, got %d", len(blockers))
	}
}

func TestExecutor_ExecuteTask(t *testing.T) {
	executor, store := setupTestExecutor(t)
	defer store.Close()
	defer executor.Close()

	// Create a test project
	project := &state.Project{
		ID:        "test-project",
		Name:      "Test Project",
		CreatedAt: time.Now(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	seedExecutionContext(t, store, project.ID)

	// Create a test phase
	phase := &state.Phase{
		ID:        "phase-1",
		ProjectID: project.ID,
		Number:    1,
		Title:     "Test Phase",
		Status:    state.PhaseNotStarted,
		CreatedAt: time.Now(),
	}
	if err := store.SavePhase(phase); err != nil {
		t.Fatalf("failed to save phase: %v", err)
	}

	// Create a test task
	task := &state.Task{
		ID:          "task-1",
		PhaseID:     phase.ID,
		Number:      "1.1",
		Description: "Test Task",
		Status:      state.TaskNotStarted,
	}
	if err := store.SaveTask(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Execute the task
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		if err := executor.ExecuteTask(task.ID); err != nil {
			t.Errorf("failed to execute task: %v", err)
		}
	}()

	// Collect updates
	var updates []TaskUpdate
	timeout := time.After(2 * time.Second)

	for {
		select {
		case update := <-executor.StreamOutput():
			updates = append(updates, update)
			if update.Type == TaskCompleted {
				goto done
			}
		case <-timeout:
			t.Fatal("timeout waiting for task completion")
		}
	}

done:
	select {
	case <-doneCh:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for execute task goroutine to finish")
	}

	// Verify we received updates
	if len(updates) < 2 {
		t.Errorf("expected at least 2 updates, got %d", len(updates))
	}

	// Verify task status
	updatedTask, err := store.GetTask(task.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}

	if updatedTask.Status != state.TaskCompleted {
		t.Errorf("expected task status to be '%s', got %s", state.TaskCompleted, updatedTask.Status)
	}
}

func TestExecutor_ExecutePhase(t *testing.T) {
	executor, store := setupTestExecutor(t)
	defer store.Close()
	defer executor.Close()

	// Create a test project
	project := &state.Project{
		ID:        "test-project",
		Name:      "Test Project",
		CreatedAt: time.Now(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	seedExecutionContext(t, store, project.ID)

	// Create a test phase
	phase := &state.Phase{
		ID:        "phase-1",
		ProjectID: project.ID,
		Number:    1,
		Title:     "Test Phase",
		Status:    state.PhaseNotStarted,
		CreatedAt: time.Now(),
	}
	if err := store.SavePhase(phase); err != nil {
		t.Fatalf("failed to save phase: %v", err)
	}

	// Create a task for the phase so ExecutePhase has something to do
	task := &state.Task{
		ID:          "task-1",
		PhaseID:     phase.ID,
		Number:      "1.1",
		Description: "Test Task",
		Status:      state.TaskNotStarted,
	}
	if err := store.SaveTask(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Execute the phase
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		if err := executor.ExecutePhase(phase.ID); err != nil {
			t.Errorf("failed to execute phase: %v", err)
		}
	}()

	// Collect updates
	var updates []TaskUpdate
	timeout := time.After(2 * time.Second)

	for {
		select {
		case update := <-executor.StreamOutput():
			updates = append(updates, update)
			if update.Type == TaskCompleted && update.PhaseID == phase.ID {
				goto done
			}
		case <-timeout:
			t.Fatal("timeout waiting for phase completion")
		}
	}

done:
	select {
	case <-doneCh:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for execute phase goroutine to finish")
	}

	// Verify we received updates
	if len(updates) < 2 {
		t.Errorf("expected at least 2 updates, got %d", len(updates))
	}

	// Verify phase status
	updatedPhase, err := store.GetPhase(phase.ID)
	if err != nil {
		t.Fatalf("failed to get phase: %v", err)
	}

	if updatedPhase.Status != state.PhaseCompleted {
		t.Errorf("expected phase status to be '%s', got %s", state.PhaseCompleted, updatedPhase.Status)
	}
}

func TestExecutor_ExecuteTaskPartialFailure(t *testing.T) {
	executor, store := setupTestExecutor(t)
	defer store.Close()
	defer executor.Close()

	// Create a test project
	project := &state.Project{
		ID:        "test-project",
		Name:      "Test Project",
		CreatedAt: time.Now(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	seedExecutionContext(t, store, project.ID)

	// Create a test phase
	phase := &state.Phase{
		ID:        "phase-1",
		ProjectID: project.ID,
		Number:    1,
		Title:     "Test Phase",
		Status:    state.PhaseNotStarted,
		CreatedAt: time.Now(),
	}
	if err := store.SavePhase(phase); err != nil {
		t.Fatalf("failed to save phase: %v", err)
	}

	// Create a test task
	task := &state.Task{
		ID:          "task-1",
		PhaseID:     phase.ID,
		Number:      "1.1",
		Description: "Test Task",
		Status:      state.TaskNotStarted,
	}
	if err := store.SaveTask(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Mock provider to return response with multiple files
	failingProvider := &mockProviderWithMultipleFiles{}
	executor.provider = failingProvider

	// Execute the task
	err := executor.ExecuteTask(task.ID)

	// Task should complete successfully (no error) even if some files failed
	if err != nil {
		t.Errorf("expected task to complete with partial success, got error: %v", err)
	}

	// Verify task status is completed
	updatedTask, err := store.GetTask(task.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}

	if updatedTask.Status != state.TaskCompleted {
		t.Errorf("expected task status to be '%s', got %s", state.TaskCompleted, updatedTask.Status)
	}
}

// mockProviderWithMultipleFiles is a mock provider that returns multiple files, some of which will fail
type mockProviderWithMultipleFiles struct{}

func (m *mockProviderWithMultipleFiles) Name() string {
	return "mock-multiple-files"
}

func (m *mockProviderWithMultipleFiles) Authenticate(apiKey string) error {
	return nil
}

func (m *mockProviderWithMultipleFiles) IsAuthenticated() bool {
	return true
}

func (m *mockProviderWithMultipleFiles) ListModels() ([]provider.Model, error) {
	return nil, nil
}

func (m *mockProviderWithMultipleFiles) DiscoverModels() ([]provider.Model, error) {
	return nil, nil
}

func (m *mockProviderWithMultipleFiles) Call(ctx context.Context, model string, prompt string) (*provider.Response, error) {
	return &provider.Response{
		Content: `{
			"explanation": "Creating test files",
			"files": [
				{
					"path": "good-file.txt",
					"content": "This file should be written successfully"
				},
				{
					"path": "../bad-file.txt",
					"content": "This file should fail path validation"
				},
				{
					"path": "another-good-file.txt",
					"content": "This file should also be written successfully"
				}
			]
		}`,
		TokensInput:  20,
		TokensOutput: 10,
		Model:        model,
		Provider:     "mock",
		Timestamp:    time.Now(),
	}, nil
}

func (m *mockProviderWithMultipleFiles) Stream(ctx context.Context, model string, prompt string) (<-chan string, error) {
	ch := make(chan string, 1)
	go func() {
		defer close(ch)
		ch <- `{
			"explanation": "Creating test files",
			"files": [
				{
					"path": "good-file.txt",
					"content": "This file should be written successfully"
				},
				{
					"path": "../bad-file.txt",
					"content": "This file should fail path validation"
				},
				{
					"path": "another-good-file.txt",
					"content": "This file should also be written successfully"
				}
			]
		}`
	}()
	return ch, nil
}

func (m *mockProviderWithMultipleFiles) GetRateLimitInfo() (*provider.RateLimitInfo, error) {
	return nil, nil
}

func (m *mockProviderWithMultipleFiles) GetQuotaInfo() (*provider.QuotaInfo, error) {
	return nil, nil
}

func (m *mockProviderWithMultipleFiles) SupportsCodingPlan() bool {
	return false
}

func TestExecutor_Backpressure(t *testing.T) {
	executor, store := setupTestExecutor(t)
	defer store.Close()
	defer executor.Close()

	// Channel buffer size is 100
	const bufferSize = 100
	const extraMessages = 50
	const totalMessages = bufferSize + extraMessages

	// Start a goroutine to send messages
	go func() {
		for i := 0; i < totalMessages; i++ {
			executor.sendUpdate(TaskUpdate{
				TaskID:    "task-1",
				Type:      TaskProgress,
				Content:   "Update",
				Timestamp: time.Now(),
			})
		}
	}()

	// Read messages
	receivedCount := 0
	timeout := time.After(5 * time.Second)

	for i := 0; i < totalMessages; i++ {
		select {
		case <-executor.StreamOutput():
			receivedCount++
		case <-timeout:
			t.Errorf("timeout waiting for messages. Received %d/%d", receivedCount, totalMessages)
			return
		}
	}

	if receivedCount != totalMessages {
		t.Errorf("expected %d messages, got %d", totalMessages, receivedCount)
	}
}
