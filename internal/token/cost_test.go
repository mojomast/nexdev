package token

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/state"
)

func TestCostEstimator(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := state.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create test project
	project := &state.Project{
		ID:           "test-project",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInit,
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	estimator := NewCostEstimator(store)

	t.Run("CalculateCost", func(t *testing.T) {
		cost := estimator.CalculateCost(1000, 500, 0.01, 0.03)
		expected := (1000.0/1000.0)*0.01 + (500.0/1000.0)*0.03
		if cost != expected {
			t.Errorf("Expected cost %f, got %f", expected, cost)
		}
	})

	t.Run("GetTotalCost_Empty", func(t *testing.T) {
		total, err := estimator.GetTotalCost(project.ID)
		if err != nil {
			t.Fatalf("Failed to get total cost: %v", err)
		}
		if total != 0 {
			t.Errorf("Expected total cost 0, got %f", total)
		}
	})

	t.Run("RecordAndGetCost", func(t *testing.T) {
		// Record some usage
		usage1 := &state.TokenUsage{
			ProjectID:    project.ID,
			Provider:     "openai",
			Model:        "gpt-4",
			TokensInput:  1000,
			TokensOutput: 500,
			Cost:         0.025,
			Timestamp:    time.Now(),
		}
		if err := store.RecordTokenUsage(usage1); err != nil {
			t.Fatalf("Failed to record usage: %v", err)
		}

		usage2 := &state.TokenUsage{
			ProjectID:    project.ID,
			Provider:     "anthropic",
			Model:        "claude-3",
			TokensInput:  2000,
			TokensOutput: 1000,
			Cost:         0.050,
			Timestamp:    time.Now(),
		}
		if err := store.RecordTokenUsage(usage2); err != nil {
			t.Fatalf("Failed to record usage: %v", err)
		}

		// Get total cost
		total, err := estimator.GetTotalCost(project.ID)
		if err != nil {
			t.Fatalf("Failed to get total cost: %v", err)
		}
		expected := 0.075
		if total < expected-0.0001 || total > expected+0.0001 {
			t.Errorf("Expected total cost %f, got %f", expected, total)
		}
	})

	t.Run("GetCostByProvider", func(t *testing.T) {
		cost, err := estimator.GetCostByProvider(project.ID, "openai")
		if err != nil {
			t.Fatalf("Failed to get cost by provider: %v", err)
		}
		if cost != 0.025 {
			t.Errorf("Expected cost 0.025, got %f", cost)
		}
	})

	t.Run("GetCostStats", func(t *testing.T) {
		stats, err := estimator.GetCostStats(project.ID)
		if err != nil {
			t.Fatalf("Failed to get cost stats: %v", err)
		}

		expected := 0.075
		if stats.TotalCost < expected-0.0001 || stats.TotalCost > expected+0.0001 {
			t.Errorf("Expected total cost %f, got %f", expected, stats.TotalCost)
		}

		if len(stats.ByProvider) != 2 {
			t.Errorf("Expected 2 providers, got %d", len(stats.ByProvider))
		}

		if stats.ByProvider["openai"] != 0.025 {
			t.Errorf("Expected openai cost 0.025, got %f", stats.ByProvider["openai"])
		}
	})

	t.Run("BudgetLimit", func(t *testing.T) {
		estimator.SetBudgetLimit(0.10)
		estimator.SetWarningLevel(0.8)

		// Should get warning (0.075 is 75% of 0.10, which is >= 80% threshold)
		// Actually 0.075/0.10 = 0.75 which is < 0.8, so no warning
		// Let's set a lower budget to trigger warning
		estimator.SetBudgetLimit(0.09)

		// Now 0.075/0.09 = 0.833 which is > 0.8, should get warning
		warning, err := estimator.CheckBudget(project.ID)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if warning == "" {
			t.Error("Expected warning, got none")
		}

		// Set lower limit to trigger error
		estimator.SetBudgetLimit(0.05)
		_, err = estimator.CheckBudget(project.ID)
		if err == nil {
			t.Error("Expected error for exceeded budget, got none")
		}
	})

	t.Run("EstimateDevPlanCost", func(t *testing.T) {
		phases := []PhaseEstimate{
			{PhaseID: "phase1", EstimatedCost: 1.50},
			{PhaseID: "phase2", EstimatedCost: 2.25},
			{PhaseID: "phase3", EstimatedCost: 0.75},
		}

		total := estimator.EstimateDevPlanCost(phases)
		expected := 4.50
		if total != expected {
			t.Errorf("Expected total %f, got %f", expected, total)
		}
	})

	t.Run("GetMostExpensiveCalls", func(t *testing.T) {
		calls, err := estimator.GetMostExpensiveCalls(project.ID, 5)
		if err != nil {
			t.Fatalf("Failed to get most expensive calls: %v", err)
		}

		if len(calls) != 2 {
			t.Errorf("Expected 2 calls, got %d", len(calls))
		}

		// Should be sorted by cost descending
		if calls[0].Cost < calls[1].Cost {
			t.Error("Calls not sorted by cost descending")
		}
	})
}

func TestCostEstimator_EmptyProject(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := state.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	estimator := NewCostEstimator(store)

	t.Run("GetTotalCost_NonExistent", func(t *testing.T) {
		// Non-existent project should return 0 cost, not error
		// (SQLite COALESCE will return 0 for no rows)
		cost, err := estimator.GetTotalCost("nonexistent")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if cost != 0 {
			t.Errorf("Expected cost 0 for nonexistent project, got %f", cost)
		}
	})

	t.Run("GetCostByProvider_Empty", func(t *testing.T) {
		_, err := estimator.GetCostByProvider("", "openai")
		if err == nil {
			t.Error("Expected error for empty project ID")
		}
	})
}

func TestCostEstimator_BudgetWarningLevels(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := state.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	project := &state.Project{
		ID:           "test-project",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInit,
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Record usage
	usage := &state.TokenUsage{
		ProjectID:    project.ID,
		Provider:     "openai",
		Model:        "gpt-4",
		TokensInput:  1000,
		TokensOutput: 500,
		Cost:         0.50,
		Timestamp:    time.Now(),
	}
	if err := store.RecordTokenUsage(usage); err != nil {
		t.Fatalf("Failed to record usage: %v", err)
	}

	estimator := NewCostEstimator(store)

	t.Run("NoBudgetLimit", func(t *testing.T) {
		warning, err := estimator.CheckBudget(project.ID)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if warning != "" {
			t.Errorf("Expected no warning with no budget limit, got: %s", warning)
		}
	})

	t.Run("BelowWarningLevel", func(t *testing.T) {
		estimator.SetBudgetLimit(1.00)
		estimator.SetWarningLevel(0.8)

		warning, err := estimator.CheckBudget(project.ID)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if warning != "" {
			t.Errorf("Expected no warning below threshold, got: %s", warning)
		}
	})

	t.Run("AtWarningLevel", func(t *testing.T) {
		estimator.SetBudgetLimit(0.60)
		estimator.SetWarningLevel(0.8)

		warning, err := estimator.CheckBudget(project.ID)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if warning == "" {
			t.Error("Expected warning at threshold")
		}
	})

	t.Run("ExceededBudget", func(t *testing.T) {
		estimator.SetBudgetLimit(0.40)

		warning, err := estimator.CheckBudget(project.ID)
		if err == nil {
			t.Error("Expected error for exceeded budget")
		}
		if warning != "" {
			t.Error("Should not have warning when budget exceeded (should be error)")
		}
	})
}
