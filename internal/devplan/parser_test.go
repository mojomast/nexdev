package devplan

import (
	"testing"
)

func TestParsePhaseMarkdown(t *testing.T) {
	markdown := `# Phase 1: Database & Models

**Status:** not_started

## Objective

Implement data models and database schema

## Success Criteria

- Database schema created
- Models implemented
- Migrations working

## Dependencies

Depends on phases: 0

## Tasks

### 1.1: Design database schema

**Status:** not_started

**Acceptance Criteria:**
- Schema documented
- Relationships defined

**Implementation Notes:**
- Consider normalization

### 1.2: Implement data models

**Status:** in_progress

**Acceptance Criteria:**
- Models created
- Validation added

## Estimates

- **Tokens:** 2000
- **Cost:** $0.02
`

	phase, err := ParsePhaseMarkdown(markdown)
	if err != nil {
		t.Fatalf("Failed to parse markdown: %v", err)
	}

	if phase.Number != 1 {
		t.Errorf("Expected Number 1, got %d", phase.Number)
	}
	if phase.Title != "Database & Models" {
		t.Errorf("Expected Title 'Database & Models', got '%s'", phase.Title)
	}
	if phase.Status != PhaseNotStarted {
		t.Errorf("Expected Status 'not_started', got '%s'", phase.Status)
	}
	if phase.Objective != "Implement data models and database schema" {
		t.Errorf("Expected Objective matches, got '%s'", phase.Objective)
	}

	if len(phase.SuccessCriteria) != 3 {
		t.Errorf("Expected 3 success criteria, got %d", len(phase.SuccessCriteria))
	} else {
		if phase.SuccessCriteria[0] != "Database schema created" {
			t.Errorf("Unexpected first criterion: %s", phase.SuccessCriteria[0])
		}
	}

	if len(phase.Dependencies) != 1 || phase.Dependencies[0] != "0" {
		t.Errorf("Expected dependency '0', got %v", phase.Dependencies)
	}

	if len(phase.Tasks) != 2 {
		t.Fatalf("Expected 2 tasks, got %d", len(phase.Tasks))
	}

	task1 := phase.Tasks[0]
	if task1.Number != "1.1" {
		t.Errorf("Expected task 1 number '1.1', got '%s'", task1.Number)
	}
	if task1.Description != "Design database schema" {
		t.Errorf("Expected task 1 description 'Design database schema', got '%s'", task1.Description)
	}
	if task1.Status != TaskNotStarted {
		t.Errorf("Expected task 1 status 'not_started', got '%s'", task1.Status)
	}
	if len(task1.AcceptanceCriteria) != 2 {
		t.Errorf("Expected 2 acceptance criteria for task 1, got %d", len(task1.AcceptanceCriteria))
	}
	if len(task1.ImplementationNotes) != 1 {
		t.Errorf("Expected 1 implementation note for task 1, got %d", len(task1.ImplementationNotes))
	}

	task2 := phase.Tasks[1]
	if task2.Status != TaskInProgress {
		t.Errorf("Expected task 2 status 'in_progress', got '%s'", task2.Status)
	}

	if phase.EstimatedTokens != 2000 {
		t.Errorf("Expected 2000 tokens, got %d", phase.EstimatedTokens)
	}
	if phase.EstimatedCost != 0.02 {
		t.Errorf("Expected cost 0.02, got %f", phase.EstimatedCost)
	}
}
