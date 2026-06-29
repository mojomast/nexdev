package cli

import (
	"testing"

	"github.com/mojomast/nexdev/internal/devplan"
)

func TestFormatPhaseContent(t *testing.T) {
	phase := &devplan.Phase{
		Number:    1,
		Title:     "Test Phase",
		Objective: "Test Objective",
		SuccessCriteria: []string{
			"Criterion 1",
			"Criterion 2",
		},
		Tasks: []devplan.Task{
			{
				Number:      "1.1",
				Description: "Task 1",
				AcceptanceCriteria: []string{
					"AC 1",
					"AC 2",
				},
			},
		},
	}

	expected := "# Phase 1: Test Phase\n\n" +
		"## Objective\n\nTest Objective\n\n" +
		"## Success Criteria\n\n" +
		"- Criterion 1\n" +
		"- Criterion 2\n" +
		"\n" +
		"## Tasks\n\n" +
		"### 1.1: Task 1\n\n" +
		"**Acceptance Criteria:**\n" +
		"- AC 1\n" +
		"- AC 2\n" +
		"\n"

	result := formatPhaseContent(phase)

	if result != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, result)
	}
}

func BenchmarkFormatPhaseContent(b *testing.B) {
	phase := &devplan.Phase{
		Number:    1,
		Title:     "Test Phase",
		Objective: "Test Objective",
		SuccessCriteria: []string{
			"Criterion 1",
			"Criterion 2",
			"Criterion 3",
		},
		Tasks: []devplan.Task{
			{
				Number:      "1.1",
				Description: "Task 1",
				AcceptanceCriteria: []string{
					"AC 1",
					"AC 2",
				},
			},
			{
				Number:      "1.2",
				Description: "Task 2",
				AcceptanceCriteria: []string{
					"AC 3",
					"AC 4",
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formatPhaseContent(phase)
	}
}
