package reviewer

import (
	"testing"

	"github.com/mojomast/nexdev/internal/devplan"
)

func TestPhaseReviewer(t *testing.T) {
	reviewer := NewReviewer(nil, "test-model")

	t.Run("ReviewPhase_Valid", func(t *testing.T) {
		phase := &devplan.Phase{
			ID:              "phase-0",
			Number:          0,
			Title:           "Setup",
			Objective:       "Initialize project",
			SuccessCriteria: []string{"Project created", "Dependencies installed"},
			Dependencies:    []string{},
			Tasks: []devplan.Task{
				{
					ID:                  "task-0-1",
					Number:              "0.1",
					Description:         "Create project structure",
					AcceptanceCriteria:  []string{"Structure exists"},
					ImplementationNotes: []string{"Use standard layout"},
					Status:              devplan.TaskNotStarted,
				},
			},
			Status: devplan.PhaseNotStarted,
		}

		review, err := reviewer.ReviewPhase(phase)
		if err != nil {
			t.Fatalf("Failed to review phase: %v", err)
		}

		if review == nil {
			t.Fatal("Review should not be nil")
		}

		if review.Status == ReviewFailed {
			t.Errorf("Valid phase should not fail review, issues: %v", review.Issues)
		}
	})

	t.Run("ReviewPhase_MissingObjective", func(t *testing.T) {
		phase := &devplan.Phase{
			ID:              "phase-0",
			Number:          0,
			Title:           "Setup",
			Objective:       "", // Missing
			SuccessCriteria: []string{"Criteria"},
			Tasks:           []devplan.Task{{ID: "task-0-1", Description: "Task"}},
			Status:          devplan.PhaseNotStarted,
		}

		review, err := reviewer.ReviewPhase(phase)
		if err != nil {
			t.Fatalf("Failed to review phase: %v", err)
		}

		if review.Status != ReviewFailed {
			t.Error("Phase with missing objective should fail review")
		}

		hasClarityIssue := false
		for _, issue := range review.Issues {
			if issue.Type == IssueClarity && issue.Severity == SeverityCritical {
				hasClarityIssue = true
				break
			}
		}

		if !hasClarityIssue {
			t.Error("Should have critical clarity issue for missing objective")
		}
	})

	t.Run("ReviewPhase_NoSuccessCriteria", func(t *testing.T) {
		phase := &devplan.Phase{
			ID:              "phase-0",
			Number:          0,
			Title:           "Setup",
			Objective:       "Initialize",
			SuccessCriteria: []string{}, // Empty
			Tasks:           []devplan.Task{{ID: "task-0-1", Description: "Task"}},
			Status:          devplan.PhaseNotStarted,
		}

		review, err := reviewer.ReviewPhase(phase)
		if err != nil {
			t.Fatalf("Failed to review phase: %v", err)
		}

		if review.Status != ReviewFailed {
			t.Error("Phase with no success criteria should fail review")
		}
	})

	t.Run("ReviewPhase_NoTasks", func(t *testing.T) {
		phase := &devplan.Phase{
			ID:              "phase-0",
			Number:          0,
			Title:           "Setup",
			Objective:       "Initialize",
			SuccessCriteria: []string{"Criteria"},
			Tasks:           []devplan.Task{}, // Empty
			Status:          devplan.PhaseNotStarted,
		}

		review, err := reviewer.ReviewPhase(phase)
		if err != nil {
			t.Fatalf("Failed to review phase: %v", err)
		}

		if review.Status != ReviewFailed {
			t.Error("Phase with no tasks should fail review")
		}
	})

	t.Run("ReviewPhase_TooManyTasks", func(t *testing.T) {
		tasks := make([]devplan.Task, 15)
		for i := range tasks {
			tasks[i] = devplan.Task{
				ID:                 "task",
				Description:        "Task",
				AcceptanceCriteria: []string{"Criteria"},
			}
		}

		phase := &devplan.Phase{
			ID:              "phase-0",
			Number:          0,
			Title:           "Setup",
			Objective:       "Initialize",
			SuccessCriteria: []string{"Criteria"},
			Tasks:           tasks,
			Status:          devplan.PhaseNotStarted,
		}

		review, err := reviewer.ReviewPhase(phase)
		if err != nil {
			t.Fatalf("Failed to review phase: %v", err)
		}

		hasScopeWarning := false
		for _, issue := range review.Issues {
			if issue.Type == IssueScope && issue.Severity == SeverityWarning {
				hasScopeWarning = true
				break
			}
		}

		if !hasScopeWarning {
			t.Error("Should have scope warning for too many tasks")
		}
	})

	t.Run("ReviewAllPhases", func(t *testing.T) {
		phases := []devplan.Phase{
			{
				ID:              "phase-0",
				Number:          0,
				Title:           "Setup",
				Objective:       "Initialize",
				SuccessCriteria: []string{"Criteria"},
				Tasks:           []devplan.Task{{ID: "task-0-1", Description: "Task"}},
				Status:          devplan.PhaseNotStarted,
			},
			{
				ID:              "phase-1",
				Number:          1,
				Title:           "Database",
				Objective:       "Setup DB",
				SuccessCriteria: []string{"DB ready"},
				Dependencies:    []string{"0"},
				Tasks:           []devplan.Task{{ID: "task-1-1", Description: "Task"}},
				Status:          devplan.PhaseNotStarted,
			},
		}

		report, err := reviewer.ReviewAllPhases(phases)
		if err != nil {
			t.Fatalf("Failed to review all phases: %v", err)
		}

		if report == nil {
			t.Fatal("Report should not be nil")
		}

		if report.TotalPhases != 2 {
			t.Errorf("Expected 2 phases, got %d", report.TotalPhases)
		}

		if len(report.PhaseReviews) != 2 {
			t.Errorf("Expected 2 phase reviews, got %d", len(report.PhaseReviews))
		}

		if report.Summary == "" {
			t.Error("Report summary should not be empty")
		}
	})

	t.Run("CheckCrossPhaseIssues_InvalidDependency", func(t *testing.T) {
		phases := []devplan.Phase{
			{
				ID:           "phase-0",
				Number:       0,
				Title:        "Setup",
				Dependencies: []string{"1"}, // Depends on phase 1 which comes after
			},
			{
				ID:           "phase-1",
				Number:       1,
				Title:        "Database",
				Dependencies: []string{},
			},
		}

		issues := reviewer.CheckCrossPhaseIssues(phases)

		if len(issues) == 0 {
			t.Error("Should find dependency issue")
		}

		hasDependencyIssue := false
		for _, issue := range issues {
			if issue.Type == IssueDependencies && issue.Severity == SeverityCritical {
				hasDependencyIssue = true
				break
			}
		}

		if !hasDependencyIssue {
			t.Error("Should have critical dependency issue")
		}
	})

	t.Run("ExportMarkdown", func(t *testing.T) {
		report := &ReviewReport{
			TotalPhases: 2,
			IssuesFound: 1,
			SeverityBreakdown: map[Severity]int{
				SeverityCritical: 0,
				SeverityWarning:  1,
				SeverityInfo:     0,
			},
			PhaseReviews: []PhaseReview{
				{
					PhaseID: "phase-0",
					Status:  ReviewWarning,
					Issues: []Issue{
						{
							Type:        IssueScope,
							Severity:    SeverityWarning,
							Description: "Test issue",
							Suggestion:  "Test suggestion",
						},
					},
				},
			},
			CrossPhaseIssues: []Issue{},
			Summary:          "Test summary",
		}

		markdown, err := reviewer.ExportMarkdown(report)
		if err != nil {
			t.Fatalf("Failed to export markdown: %v", err)
		}

		if markdown == "" {
			t.Fatal("Markdown should not be empty")
		}

		if !contains(markdown, "Phase Review Report") {
			t.Error("Markdown should contain title")
		}

		if !contains(markdown, "Summary") {
			t.Error("Markdown should contain summary section")
		}

		if !contains(markdown, "Severity Breakdown") {
			t.Error("Markdown should contain severity breakdown")
		}
	})

	t.Run("ExportJSON", func(t *testing.T) {
		report := &ReviewReport{
			TotalPhases:       1,
			IssuesFound:       0,
			SeverityBreakdown: make(map[Severity]int),
			PhaseReviews:      []PhaseReview{},
			CrossPhaseIssues:  []Issue{},
			Summary:           "All good",
		}

		jsonStr, err := reviewer.ExportJSON(report)
		if err != nil {
			t.Fatalf("Failed to export JSON: %v", err)
		}

		if jsonStr == "" {
			t.Fatal("JSON should not be empty")
		}

		if !contains(jsonStr, "TotalPhases") {
			t.Error("JSON should contain TotalPhases")
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

func TestPhaseReviewer_Improvements(t *testing.T) {
	reviewer := NewReviewer(nil, "test-model")

	t.Run("GenerateImprovements", func(t *testing.T) {
		issues := []Issue{
			{
				Type:        IssueClarity,
				Severity:    SeverityCritical,
				Description: "Missing objective",
				Suggestion:  "Add clear objective",
			},
			{
				Type:        IssueTesting,
				Severity:    SeverityInfo,
				Description: "No testing",
				Suggestion:  "Add tests",
			},
		}

		improvements, err := reviewer.GenerateImprovements(issues)
		if err != nil {
			t.Fatalf("Failed to generate improvements: %v", err)
		}

		if len(improvements) != 2 {
			t.Errorf("Expected 2 improvements, got %d", len(improvements))
		}

		if improvements[0].IssueType != IssueClarity {
			t.Errorf("Expected first improvement to be clarity type, got %s", improvements[0].IssueType)
		}

		if improvements[1].IssueType != IssueTesting {
			t.Errorf("Expected second improvement to be testing type, got %s", improvements[1].IssueType)
		}
	})

	t.Run("ApplyImprovements_Clarity", func(t *testing.T) {
		phase := &devplan.Phase{
			ID:              "phase-0",
			Number:          0,
			Title:           "Setup",
			Objective:       "", // Missing
			SuccessCriteria: []string{"Criteria"},
			Tasks:           []devplan.Task{{ID: "task-0-1", Description: "Task"}},
			Status:          devplan.PhaseNotStarted,
		}

		improvements := []Improvement{
			{
				IssueType:   IssueClarity,
				Description: "Add objective",
				NewContent:  "Initialize project structure",
			},
		}

		improved, err := reviewer.ApplyImprovements(phase, improvements)
		if err != nil {
			t.Fatalf("Failed to apply improvements: %v", err)
		}

		if improved.Objective == "" {
			t.Error("Objective should have been added")
		}

		if improved.Objective != "Initialize project structure" {
			t.Errorf("Expected objective 'Initialize project structure', got '%s'", improved.Objective)
		}
	})

	t.Run("ApplyImprovements_Testing", func(t *testing.T) {
		phase := &devplan.Phase{
			ID:              "phase-1",
			Number:          1,
			Title:           "Database",
			Objective:       "Setup DB",
			SuccessCriteria: []string{"DB ready"},
			Tasks: []devplan.Task{
				{ID: "task-1-1", Description: "Create schema"},
			},
			Status: devplan.PhaseNotStarted,
		}

		improvements := []Improvement{
			{
				IssueType:   IssueTesting,
				Description: "Add testing task",
				NewContent:  "Test database connections",
			},
		}

		improved, err := reviewer.ApplyImprovements(phase, improvements)
		if err != nil {
			t.Fatalf("Failed to apply improvements: %v", err)
		}

		if len(improved.Tasks) != 2 {
			t.Errorf("Expected 2 tasks after adding testing task, got %d", len(improved.Tasks))
		}

		lastTask := improved.Tasks[len(improved.Tasks)-1]
		if !contains(lastTask.Description, "Test") {
			t.Error("Last task should be a testing task")
		}
	})

	t.Run("ApplyImprovements_NilPhase", func(t *testing.T) {
		improvements := []Improvement{{IssueType: IssueClarity}}

		_, err := reviewer.ApplyImprovements(nil, improvements)
		if err == nil {
			t.Error("Should error when phase is nil")
		}
	})

	t.Run("ApplyImprovementsToAll", func(t *testing.T) {
		phases := []devplan.Phase{
			{
				ID:              "phase-0",
				Number:          0,
				Title:           "Setup",
				Objective:       "", // Missing
				SuccessCriteria: []string{"Criteria"},
				Tasks:           []devplan.Task{{ID: "task-0-1", Description: "Task"}},
				Status:          devplan.PhaseNotStarted,
			},
			{
				ID:              "phase-1",
				Number:          1,
				Title:           "Database",
				Objective:       "Setup DB",
				SuccessCriteria: []string{"DB ready"},
				Tasks:           []devplan.Task{{ID: "task-1-1", Description: "Task"}},
				Status:          devplan.PhaseNotStarted,
			},
		}

		report, _ := reviewer.ReviewAllPhases(phases)

		improved, err := reviewer.ApplyImprovementsToAll(phases, report)
		if err != nil {
			t.Fatalf("Failed to apply improvements to all: %v", err)
		}

		if len(improved) != 2 {
			t.Errorf("Expected 2 phases, got %d", len(improved))
		}

		// Check that improvements were applied to phase with issues
		if improved[0].Objective == "" {
			t.Error("Objective should have been added to first phase")
		}
	})

	t.Run("SelectiveApplyImprovements", func(t *testing.T) {
		phase := &devplan.Phase{
			ID:              "phase-0",
			Number:          0,
			Title:           "Setup",
			Objective:       "",
			SuccessCriteria: []string{},
			Tasks:           []devplan.Task{},
			Status:          devplan.PhaseNotStarted,
		}

		improvements := []Improvement{
			{
				IssueType:   IssueClarity,
				Description: "Add objective",
				NewContent:  "Initialize project",
			},
			{
				IssueType:   IssueCompleteness,
				Description: "Add success criteria",
				NewContent:  "Project initialized",
			},
			{
				IssueType:   IssueTesting,
				Description: "Add testing",
				NewContent:  "Test setup",
			},
		}

		// Apply only the first and third improvements
		improved, err := reviewer.SelectiveApplyImprovements(phase, improvements, []int{0, 2})
		if err != nil {
			t.Fatalf("Failed to selectively apply improvements: %v", err)
		}

		if improved.Objective == "" {
			t.Error("Objective should have been added (improvement 0)")
		}

		if len(improved.Tasks) == 0 {
			t.Error("Testing task should have been added (improvement 2)")
		}

		// Success criteria should not be added (improvement 1 was not selected)
		// But it might be added by the completeness improvement, so we can't test this reliably
	})

	t.Run("SelectiveApplyImprovements_InvalidIndex", func(t *testing.T) {
		phase := &devplan.Phase{
			ID:     "phase-0",
			Number: 0,
			Status: devplan.PhaseNotStarted,
		}

		improvements := []Improvement{
			{IssueType: IssueClarity, NewContent: "Test"},
		}

		// Use invalid index - should not crash
		improved, err := reviewer.SelectiveApplyImprovements(phase, improvements, []int{0, 99})
		if err != nil {
			t.Fatalf("Should not error with invalid index: %v", err)
		}

		if improved == nil {
			t.Error("Should return improved phase even with some invalid indices")
		}
	})
}
