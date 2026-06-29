package token

import (
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/state"
)

func TestNewCounter(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	counter := NewCounter(store)
	if counter == nil {
		t.Fatal("NewCounter returned nil")
	}
	if counter.store == nil {
		t.Fatal("Counter store is nil")
	}
}

func TestCounter_CountTokens(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	counter := NewCounter(store)

	tests := []struct {
		name    string
		text    string
		model   string
		wantMin int
		wantMax int
	}{
		{
			name:    "empty text",
			text:    "",
			model:   "gpt-4",
			wantMin: 0,
			wantMax: 0,
		},
		{
			name:    "single word",
			text:    "hello",
			model:   "gpt-4",
			wantMin: 1,
			wantMax: 3,
		},
		{
			name:    "short sentence",
			text:    "Hello, how are you?",
			model:   "gpt-4",
			wantMin: 3,
			wantMax: 8,
		},
		{
			name:    "longer text",
			text:    "This is a longer piece of text that should result in more tokens being counted.",
			model:   "gpt-4",
			wantMin: 10,
			wantMax: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := counter.CountTokens(tt.text, tt.model)
			if err != nil {
				t.Fatalf("CountTokens failed: %v", err)
			}
			if tokens < tt.wantMin || tokens > tt.wantMax {
				t.Errorf("CountTokens() = %d, want between %d and %d", tokens, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCounter_EstimateTokens(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	counter := NewCounter(store)

	tokens, err := counter.EstimateTokens("This is a test")
	if err != nil {
		t.Fatalf("EstimateTokens failed: %v", err)
	}
	if tokens <= 0 {
		t.Error("Expected positive token count")
	}
}

func TestCounter_RecordUsage(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a project first
	project := &state.Project{
		ID:           "test-project",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInterview,
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create a phase
	phase := &state.Phase{
		ID:        "phase-1",
		ProjectID: "test-project",
		Number:    1,
		Title:     "Test Phase",
		Content:   "Test phase content",
		Status:    state.PhaseNotStarted,
		CreatedAt: time.Now(),
	}
	if err := store.SavePhase(phase); err != nil {
		t.Fatalf("Failed to create phase: %v", err)
	}

	// Create a task
	task := &state.Task{
		ID:          "task-1",
		PhaseID:     "phase-1",
		Number:      "1.1",
		Description: "Test Task",
		Status:      state.TaskNotStarted,
	}
	if err := store.SaveTask(task); err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	counter := NewCounter(store)

	err = counter.RecordUsage(
		"test-project",
		"phase-1",
		"task-1",
		"openai",
		"gpt-4",
		100,
		200,
		0.05,
	)
	if err != nil {
		t.Fatalf("RecordUsage failed: %v", err)
	}

	// Verify the usage was recorded
	cost, err := store.GetTotalCost("test-project")
	if err != nil {
		t.Fatalf("GetTotalCost failed: %v", err)
	}
	if cost != 0.05 {
		t.Errorf("Expected cost 0.05, got %f", cost)
	}
}

func TestCounter_GetTotalTokens(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a project
	project := &state.Project{
		ID:           "test-project",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInterview,
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create a phase
	phase := &state.Phase{
		ID:        "phase-1",
		ProjectID: "test-project",
		Number:    1,
		Title:     "Test Phase",
		Content:   "Test phase content",
		Status:    state.PhaseNotStarted,
		CreatedAt: time.Now(),
	}
	if err := store.SavePhase(phase); err != nil {
		t.Fatalf("Failed to create phase: %v", err)
	}

	// Create tasks
	task1 := &state.Task{
		ID:          "task-1",
		PhaseID:     "phase-1",
		Number:      "1.1",
		Description: "Test Task 1",
		Status:      state.TaskNotStarted,
	}
	if err := store.SaveTask(task1); err != nil {
		t.Fatalf("Failed to create task 1: %v", err)
	}

	task2 := &state.Task{
		ID:          "task-2",
		PhaseID:     "phase-1",
		Number:      "1.2",
		Description: "Test Task 2",
		Status:      state.TaskNotStarted,
	}
	if err := store.SaveTask(task2); err != nil {
		t.Fatalf("Failed to create task 2: %v", err)
	}

	counter := NewCounter(store)

	// Record some usage
	counter.RecordUsage("test-project", "phase-1", "task-1", "openai", "gpt-4", 100, 200, 0.05)
	counter.RecordUsage("test-project", "phase-1", "task-2", "openai", "gpt-4", 150, 250, 0.07)

	stats, err := counter.GetTotalTokens("test-project")
	if err != nil {
		t.Fatalf("GetTotalTokens failed: %v", err)
	}

	if stats == nil {
		t.Fatal("Expected non-nil stats")
	}

	// Stats should have been calculated
	if stats.TotalInput != 250 {
		t.Errorf("Expected total input 250, got %d", stats.TotalInput)
	}
	if stats.TotalOutput != 450 {
		t.Errorf("Expected total output 450, got %d", stats.TotalOutput)
	}
}

func TestCounter_GetTotalTokens_EmptyProject(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	counter := NewCounter(store)

	_, err = counter.GetTotalTokens("")
	if err == nil {
		t.Error("Expected error for empty project ID")
	}
}

func TestCounter_GetTokensByProvider(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a project
	project := &state.Project{
		ID:           "test-project",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInterview,
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	counter := NewCounter(store)

	// Record usage for different providers
	counter.RecordUsage("test-project", "phase-1", "task-1", "openai", "openai-gpt-4", 100, 200, 0.05)
	counter.RecordUsage("test-project", "phase-1", "task-2", "anthropic", "anthropic-claude", 150, 250, 0.07)

	stats, err := counter.GetTokensByProvider("test-project", "openai")
	if err != nil {
		t.Fatalf("GetTokensByProvider failed: %v", err)
	}

	if stats == nil {
		t.Fatal("Expected non-nil stats")
	}
}

func TestCounter_GetTokensByProvider_EmptyParams(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	counter := NewCounter(store)

	_, err = counter.GetTokensByProvider("", "openai")
	if err == nil {
		t.Error("Expected error for empty project ID")
	}

	_, err = counter.GetTokensByProvider("test-project", "")
	if err == nil {
		t.Error("Expected error for empty provider")
	}
}

func TestCounter_GetTokensByPhase(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a project
	project := &state.Project{
		ID:           "test-project",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInterview,
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	counter := NewCounter(store)

	// Record usage for different phases
	counter.RecordUsage("test-project", "phase-1", "task-1", "openai", "gpt-4", 100, 200, 0.05)
	counter.RecordUsage("test-project", "phase-2", "task-2", "openai", "gpt-4", 150, 250, 0.07)

	stats, err := counter.GetTokensByPhase("test-project", "phase-1")
	if err != nil {
		t.Fatalf("GetTokensByPhase failed: %v", err)
	}

	if stats == nil {
		t.Fatal("Expected non-nil stats")
	}
}

func TestCounter_GetTokensByPhase_EmptyParams(t *testing.T) {
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	counter := NewCounter(store)

	_, err = counter.GetTokensByPhase("", "phase-1")
	if err == nil {
		t.Error("Expected error for empty project ID")
	}

	_, err = counter.GetTokensByPhase("test-project", "")
	if err == nil {
		t.Error("Expected error for empty phase ID")
	}
}
