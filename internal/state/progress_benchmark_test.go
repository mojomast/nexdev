package state

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func BenchmarkCalculateProgress(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")
	store, err := NewStore(dbPath)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create project
	project := &Project{
		ID:           "bench-project",
		Name:         "Benchmark Project",
		CreatedAt:    time.Now(),
		CurrentStage: StageDevelop,
	}
	if err := store.CreateProject(project); err != nil {
		b.Fatalf("Failed to create project: %v", err)
	}

	// Create phases and tasks
	numPhases := 100
	tasksPerPhase := 100

	for i := 0; i < numPhases; i++ {
		phaseID := fmt.Sprintf("phase-%d", i)
		phase := &Phase{
			ID:        phaseID,
			ProjectID: project.ID,
			Number:    i + 1,
			Title:     fmt.Sprintf("Phase %d", i),
			Status:    PhaseInProgress,
			CreatedAt: time.Now(),
		}
		if err := store.SavePhase(phase); err != nil {
			b.Fatalf("Failed to save phase: %v", err)
		}

		// Batch insert tasks if possible, but SaveTask is single.
		// Doing this in a transaction to speed up setup
		tx, err := store.BeginTx()
		if err != nil {
			b.Fatalf("Failed to begin tx: %v", err)
		}

		stmt, err := tx.Prepare(`
			INSERT INTO tasks (id, phase_id, number, description, status, started_at, completed_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			b.Fatalf("Failed to prepare stmt: %v", err)
		}

		for j := 0; j < tasksPerPhase; j++ {
			taskID := fmt.Sprintf("task-%d-%d", i, j)
			_, err := stmt.Exec(
				taskID,
				phaseID,
				fmt.Sprintf("%d.%d", i+1, j+1),
				fmt.Sprintf("Task %d.%d", i, j),
				TaskNotStarted,
				nil,
				nil,
			)
			if err != nil {
				b.Fatalf("Failed to save task: %v", err)
			}
		}
		stmt.Close()
		tx.Commit()
	}

	// Reset timer to ignore setup time
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := store.CalculateProgress(project.ID)
		if err != nil {
			b.Fatalf("CalculateProgress failed: %v", err)
		}
	}
}
