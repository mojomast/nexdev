package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateProgress_SkippedTasks(t *testing.T) {
	// Create in-memory store
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	// Create project
	project := &Project{
		ID:           "test-project",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: StageDevelop,
	}
	err = store.CreateProject(project)
	require.NoError(t, err)

	// Create phase
	phase := &Phase{
		ID:        "phase-1",
		ProjectID: project.ID,
		Number:    1,
		Title:     "Phase 1",
		Status:    PhaseInProgress,
		CreatedAt: time.Now(),
	}
	err = store.SavePhase(phase)
	require.NoError(t, err)

	// Create 1 completed task
	task1 := &Task{
		ID:          "task-1",
		PhaseID:     phase.ID,
		Number:      "1.1",
		Description: "Task 1",
		Status:      TaskCompleted,
	}
	err = store.SaveTask(task1)
	require.NoError(t, err)

	// Create 1 skipped task
	task2 := &Task{
		ID:          "task-2",
		PhaseID:     phase.ID,
		Number:      "1.2",
		Description: "Task 2",
		Status:      TaskSkipped,
	}
	err = store.SaveTask(task2)
	require.NoError(t, err)

	// Calculate progress
	stats, err := store.CalculateProgress(project.ID)
	require.NoError(t, err)

	// Assertions
	assert.Equal(t, 2, stats.TotalTasks, "Total tasks should include skipped tasks")
	assert.Equal(t, 1, stats.CompletedTasks, "Completed tasks count")
	assert.Equal(t, 1, stats.SkippedTasks, "Skipped tasks count")

	// Percentage calculation: Currently it's (1 / 2) * 100 = 50%
	// Expected (after fix): (1 / (2 - 1)) * 100 = 100%
	// If this assertion fails with 50.0, then the fix is needed.
	assert.Equal(t, 100.0, stats.CompletionPercentage, "Completion percentage should exclude skipped tasks from denominator")

	// Verify phase progress as well
	phaseProgressList, err := store.ListAllPhaseProgress(project.ID)
	require.NoError(t, err)
	require.Len(t, phaseProgressList, 1)

	pp := phaseProgressList[0]
	assert.Equal(t, 2, pp.TotalTasks)
	assert.Equal(t, 1, pp.CompletedTasks)
	// Currently skipped tasks are not counted in phase progress struct explicitly but implicitly in total
	// assert.Equal(t, 1, pp.SkippedTasks) // PhaseProgress doesn't have SkippedTasks field yet? I need to check.

	assert.Equal(t, 100.0, pp.Percentage, "Phase progress percentage should exclude skipped tasks")
}
