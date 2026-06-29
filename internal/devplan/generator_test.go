package devplan

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/design"
	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
)

// MockProvider for testing
type MockProvider struct {
	response string
}

func (m *MockProvider) Name() string {
	return "mock"
}

func (m *MockProvider) Authenticate(apiKey string) error {
	return nil
}

func (m *MockProvider) IsAuthenticated() bool {
	return true
}

func (m *MockProvider) ListModels() ([]provider.Model, error) {
	return []provider.Model{}, nil
}

func (m *MockProvider) DiscoverModels() ([]provider.Model, error) {
	return []provider.Model{}, nil
}

func (m *MockProvider) Call(ctx context.Context, model string, prompt string) (*provider.Response, error) {
	return &provider.Response{
		Content:      m.response,
		TokensInput:  100,
		TokensOutput: 200,
		Model:        model,
		Provider:     "mock",
	}, nil
}

func (m *MockProvider) Stream(ctx context.Context, model string, prompt string) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- m.response
	close(ch)
	return ch, nil
}

func (m *MockProvider) GetRateLimitInfo() (*provider.RateLimitInfo, error) {
	return nil, nil
}

func (m *MockProvider) GetQuotaInfo() (*provider.QuotaInfo, error) {
	return nil, nil
}

func (m *MockProvider) SupportsCodingPlan() bool {
	return false
}

func TestDevPlanGenerator(t *testing.T) {
	mockResponse := `
[
  {
    "number": 0,
    "title": "Setup & Infrastructure",
    "objective": "Initialize project",
    "success_criteria": ["Project created", "Dependencies installed"],
    "dependencies": [],
    "tasks": [
      {
        "number": "0.1",
        "description": "Create project structure",
        "acceptance_criteria": ["Directory structure exists"],
        "implementation_notes": ["Use standard layout"]
      }
    ]
  }
]
`

	mockProvider := &MockProvider{response: mockResponse}
	generator := NewGenerator(mockProvider, "test-model", nil)

	architecture := &design.Architecture{
		ProjectID:      "test-project",
		SystemOverview: "Test system",
		CreatedAt:      time.Now(),
	}

	interviewData := &state.InterviewData{
		ProjectID:        "test-project",
		ProjectName:      "Test Project",
		ProblemStatement: "Need a task manager",
		CreatedAt:        time.Now(),
	}

	t.Run("GeneratePhases", func(t *testing.T) {
		phases, err := generator.GeneratePhases(architecture, interviewData)
		if err != nil {
			t.Fatalf("Failed to generate phases: %v", err)
		}

		if len(phases) == 0 {
			t.Fatal("Should generate at least one phase")
		}

		// Check first phase
		phase := phases[0]
		if phase.Number != 0 {
			t.Errorf("Expected phase number 0, got %d", phase.Number)
		}

		if phase.Title == "" {
			t.Error("Phase title should not be empty")
		}

		if phase.Objective == "" {
			t.Error("Phase objective should not be empty")
		}

		if len(phase.Tasks) == 0 {
			t.Error("Phase should have tasks")
		}

		if phase.EstimatedTokens == 0 {
			t.Error("Phase should have estimated tokens")
		}

		if phase.EstimatedCost == 0 {
			t.Error("Phase should have estimated cost")
		}
	})

	t.Run("GeneratePhases_WithCoT", func(t *testing.T) {
		mockResponseCoT := `
<scratchpad>
Thinking about the architecture...
1. Database layer needs to be first.
2. Then API.
</scratchpad>

[
  {
    "number": 0,
    "title": "Setup",
    "objective": "Init",
    "success_criteria": ["Done"],
    "dependencies": [],
    "tasks": [{"number": "0.1", "description": "Task", "acceptance_criteria": [], "implementation_notes": []}]
  }
]
`
		mockProviderCoT := &MockProvider{response: mockResponseCoT}
		generatorCoT := NewGenerator(mockProviderCoT, "test-model", nil)

		phases, err := generatorCoT.GeneratePhases(architecture, interviewData)
		if err != nil {
			t.Fatalf("Failed to generate phases with CoT: %v", err)
		}

		if len(phases) != 1 {
			t.Errorf("Expected 1 phase, got %d", len(phases))
		}
	})

	t.Run("GeneratePhases_InvalidJSON", func(t *testing.T) {
		// Create a generator with a provider that returns invalid JSON
		invalidJSONProvider := &MockProvider{response: "This is not JSON"}
		invalidGenerator := NewGenerator(invalidJSONProvider, "test-model", nil)

		_, err := invalidGenerator.GeneratePhases(architecture, interviewData)
		if err == nil {
			t.Fatal("Should return error on invalid JSON")
		}
	})

	t.Run("ExportPhaseMarkdown", func(t *testing.T) {
		phase := Phase{
			ID:              "phase-0",
			Number:          0,
			Title:           "Setup",
			Objective:       "Initialize project",
			SuccessCriteria: []string{"Project created"},
			Dependencies:    []string{},
			Tasks: []Task{
				{
					ID:                  "task-0-1",
					Number:              "0.1",
					Description:         "Create structure",
					AcceptanceCriteria:  []string{"Structure exists"},
					ImplementationNotes: []string{"Use standard layout"},
					Status:              TaskNotStarted,
				},
			},
			EstimatedTokens: 1000,
			EstimatedCost:   0.01,
			Status:          PhaseNotStarted,
		}

		markdown, err := generator.ExportPhaseMarkdown(&phase)
		if err != nil {
			t.Fatalf("Failed to export phase markdown: %v", err)
		}

		if markdown == "" {
			t.Fatal("Markdown should not be empty")
		}

		if !contains(markdown, "Phase 0: Setup") {
			t.Error("Markdown should contain phase title")
		}

		if !contains(markdown, "Initialize project") {
			t.Error("Markdown should contain objective")
		}

		if !contains(markdown, "Success Criteria") {
			t.Error("Markdown should contain success criteria section")
		}

		if !contains(markdown, "Tasks") {
			t.Error("Markdown should contain tasks section")
		}
	})

	t.Run("ExportMasterPlan", func(t *testing.T) {
		devplan := &DevPlan{
			ProjectID: "test-project",
			Phases: []Phase{
				{
					Number:          0,
					Title:           "Setup",
					Objective:       "Initialize",
					Tasks:           []Task{{ID: "task-0-1"}},
					EstimatedTokens: 1000,
					EstimatedCost:   0.01,
					Status:          PhaseNotStarted,
				},
				{
					Number:          1,
					Title:           "Database",
					Objective:       "Setup DB",
					Tasks:           []Task{{ID: "task-1-1"}},
					EstimatedTokens: 2000,
					EstimatedCost:   0.02,
					Status:          PhaseNotStarted,
				},
			},
			TotalTokens: 3000,
			TotalCost:   0.03,
			CreatedAt:   time.Now(),
		}

		markdown, err := generator.ExportMasterPlan(devplan)
		if err != nil {
			t.Fatalf("Failed to export master plan: %v", err)
		}

		if markdown == "" {
			t.Fatal("Markdown should not be empty")
		}

		if !contains(markdown, "Development Plan") {
			t.Error("Markdown should contain title")
		}

		if !contains(markdown, "Phase 0: Setup") {
			t.Error("Markdown should contain first phase")
		}

		if !contains(markdown, "Phase 1: Database") {
			t.Error("Markdown should contain second phase")
		}

		if !contains(markdown, "Total Estimates") {
			t.Error("Markdown should contain total estimates")
		}
	})

	t.Run("ExportJSON", func(t *testing.T) {
		devplan := &DevPlan{
			ProjectID:   "test-project",
			Phases:      []Phase{},
			TotalTokens: 1000,
			TotalCost:   0.01,
			CreatedAt:   time.Now(),
		}

		jsonStr, err := generator.ExportJSON(devplan)
		if err != nil {
			t.Fatalf("Failed to export JSON: %v", err)
		}

		if jsonStr == "" {
			t.Fatal("JSON should not be empty")
		}

		if !contains(jsonStr, "test-project") {
			t.Error("JSON should contain project ID")
		}
	})

	t.Run("GeneratePhases_NoProvider", func(t *testing.T) {
		generator := NewGenerator(nil, "test-model", nil)

		_, err := generator.GeneratePhases(architecture, interviewData)
		if err == nil {
			t.Error("Should error when provider is nil")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsHelper(s, substr)
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDevPlanGenerator_PhaseManipulation(t *testing.T) {
	generator := NewGenerator(nil, "test-model", nil)

	phase1 := &Phase{
		ID:              "phase-0",
		Number:          0,
		Title:           "Setup",
		Objective:       "Initialize project",
		SuccessCriteria: []string{"Project created"},
		Dependencies:    []string{},
		Tasks: []Task{
			{ID: "task-0-1", Number: "0.1", Description: "Task 1"},
			{ID: "task-0-2", Number: "0.2", Description: "Task 2"},
		},
		EstimatedTokens: 2000,
		EstimatedCost:   0.02,
		Status:          PhaseNotStarted,
	}

	phase2 := &Phase{
		ID:              "phase-1",
		Number:          1,
		Title:           "Database",
		Objective:       "Setup database",
		SuccessCriteria: []string{"Database ready"},
		Dependencies:    []string{"0"},
		Tasks: []Task{
			{ID: "task-1-1", Number: "1.1", Description: "Task 3"},
		},
		EstimatedTokens: 1000,
		EstimatedCost:   0.01,
		Status:          PhaseNotStarted,
	}

	t.Run("MergePhases", func(t *testing.T) {
		merged, err := generator.MergePhases(phase1, phase2)
		if err != nil {
			t.Fatalf("Failed to merge phases: %v", err)
		}

		if merged == nil {
			t.Fatal("Merged phase should not be nil")
		}

		if len(merged.Tasks) != 3 {
			t.Errorf("Expected 3 tasks, got %d", len(merged.Tasks))
		}

		if merged.EstimatedTokens != 3000 {
			t.Errorf("Expected 3000 tokens, got %d", merged.EstimatedTokens)
		}

		if merged.EstimatedCost != 0.03 {
			t.Errorf("Expected cost 0.03, got %.2f", merged.EstimatedCost)
		}

		// Check that tasks were renumbered
		if merged.Tasks[0].Number != "0.1" {
			t.Errorf("Expected task number 0.1, got %s", merged.Tasks[0].Number)
		}
	})

	t.Run("MergePhases_NilPhase", func(t *testing.T) {
		_, err := generator.MergePhases(nil, phase2)
		if err == nil {
			t.Error("Should error when phase1 is nil")
		}

		_, err = generator.MergePhases(phase1, nil)
		if err == nil {
			t.Error("Should error when phase2 is nil")
		}
	})

	t.Run("SplitPhase", func(t *testing.T) {
		split, err := generator.SplitPhase(phase1, 1)
		if err != nil {
			t.Fatalf("Failed to split phase: %v", err)
		}

		if len(split) != 2 {
			t.Fatalf("Expected 2 phases, got %d", len(split))
		}

		if len(split[0].Tasks) != 1 {
			t.Errorf("Expected 1 task in first phase, got %d", len(split[0].Tasks))
		}

		if len(split[1].Tasks) != 1 {
			t.Errorf("Expected 1 task in second phase, got %d", len(split[1].Tasks))
		}

		// Check that second phase depends on first
		if len(split[1].Dependencies) == 0 {
			t.Error("Second phase should depend on first phase")
		}
	})

	t.Run("SplitPhase_InvalidSplitPoint", func(t *testing.T) {
		_, err := generator.SplitPhase(phase1, 0)
		if err == nil {
			t.Error("Should error with split point 0")
		}

		_, err = generator.SplitPhase(phase1, len(phase1.Tasks))
		if err == nil {
			t.Error("Should error with split point at end")
		}
	})

	t.Run("SplitPhase_NilPhase", func(t *testing.T) {
		_, err := generator.SplitPhase(nil, 1)
		if err == nil {
			t.Error("Should error when phase is nil")
		}
	})

	t.Run("ReorderPhases", func(t *testing.T) {
		phases := []Phase{*phase1, *phase2}

		// Swap the order
		reordered, err := generator.ReorderPhases(phases, []int{1, 0})
		if err != nil {
			t.Fatalf("Failed to reorder phases: %v", err)
		}

		if len(reordered) != 2 {
			t.Fatalf("Expected 2 phases, got %d", len(reordered))
		}

		// Check that phases were swapped
		if reordered[0].Title != "Database" {
			t.Errorf("Expected first phase to be Database, got %s", reordered[0].Title)
		}

		if reordered[1].Title != "Setup" {
			t.Errorf("Expected second phase to be Setup, got %s", reordered[1].Title)
		}

		// Check that numbers were updated
		if reordered[0].Number != 0 {
			t.Errorf("Expected first phase number 0, got %d", reordered[0].Number)
		}

		if reordered[1].Number != 1 {
			t.Errorf("Expected second phase number 1, got %d", reordered[1].Number)
		}
	})

	t.Run("ReorderPhases_InvalidLength", func(t *testing.T) {
		phases := []Phase{*phase1, *phase2}

		_, err := generator.ReorderPhases(phases, []int{0})
		if err == nil {
			t.Error("Should error when new order has different length")
		}
	})

	t.Run("ReorderPhases_InvalidIndex", func(t *testing.T) {
		phases := []Phase{*phase1, *phase2}

		_, err := generator.ReorderPhases(phases, []int{0, 5})
		if err == nil {
			t.Error("Should error with invalid index")
		}
	})

	t.Run("ReorderPhases_DuplicateIndex", func(t *testing.T) {
		phases := []Phase{*phase1, *phase2}

		_, err := generator.ReorderPhases(phases, []int{0, 0})
		if err == nil {
			t.Error("Should error with duplicate index")
		}
	})

	t.Run("ValidatePhaseOrder_Valid", func(t *testing.T) {
		phases := []Phase{*phase1, *phase2}

		isValid, issues := generator.ValidatePhaseOrder(phases)
		if !isValid {
			t.Errorf("Phase order should be valid, issues: %v", issues)
		}

		if len(issues) != 0 {
			t.Errorf("Should have no issues, got: %v", issues)
		}
	})

	t.Run("ValidatePhaseOrder_Invalid", func(t *testing.T) {
		// Create phases with invalid dependencies
		invalidPhase1 := *phase1
		invalidPhase1.Dependencies = []string{"1"} // Depends on phase 1 which comes after

		phases := []Phase{invalidPhase1, *phase2}

		isValid, issues := generator.ValidatePhaseOrder(phases)
		if isValid {
			t.Error("Phase order should be invalid")
		}

		if len(issues) == 0 {
			t.Error("Should have validation issues")
		}
	})
}

func TestWriteFile(t *testing.T) {
	t.Run("WriteToExistingDirectory", func(t *testing.T) {
		tmpDir := t.TempDir()
		fp := filepath.Join(tmpDir, "test.md")

		err := writeFile(fp, "# Test Content")
		if err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}

		content, err := os.ReadFile(fp)
		if err != nil {
			t.Fatalf("Failed to read written file: %v", err)
		}

		if string(content) != "# Test Content" {
			t.Errorf("Expected '# Test Content', got %q", string(content))
		}
	})

	t.Run("CreateNestedDirectories", func(t *testing.T) {
		tmpDir := t.TempDir()
		fp := filepath.Join(tmpDir, "a", "b", "c", "test.md")

		err := writeFile(fp, "Nested content")
		if err != nil {
			t.Fatalf("Failed to write file with nested dirs: %v", err)
		}

		content, err := os.ReadFile(fp)
		if err != nil {
			t.Fatalf("Failed to read written file: %v", err)
		}

		if string(content) != "Nested content" {
			t.Errorf("Expected 'Nested content', got %q", string(content))
		}
	})

	t.Run("OverwriteExistingFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		fp := filepath.Join(tmpDir, "test.md")

		err := writeFile(fp, "Original")
		if err != nil {
			t.Fatalf("Failed first write: %v", err)
		}

		err = writeFile(fp, "Updated")
		if err != nil {
			t.Fatalf("Failed second write: %v", err)
		}

		content, err := os.ReadFile(fp)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		if string(content) != "Updated" {
			t.Errorf("Expected 'Updated', got %q", string(content))
		}
	})

	t.Run("WriteEmptyContent", func(t *testing.T) {
		tmpDir := t.TempDir()
		fp := filepath.Join(tmpDir, "empty.md")

		err := writeFile(fp, "")
		if err != nil {
			t.Fatalf("Failed to write empty file: %v", err)
		}

		info, err := os.Stat(fp)
		if err != nil {
			t.Fatalf("Failed to stat file: %v", err)
		}

		if info.Size() != 0 {
			t.Errorf("Expected empty file, got size %d", info.Size())
		}
	})
}
