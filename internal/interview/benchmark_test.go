package interview

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/state"
)

func BenchmarkGenerateSummary(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "benchmark.db")

	store, err := state.NewStore(dbPath)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	project := &state.Project{
		ID:           "bench-project",
		Name:         "Benchmark Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInterview,
	}
	store.CreateProject(project)

	engine := NewEngine(store, nil, "")
	session, _ := engine.StartInterview(project.ID)

	// Populate session with significant data
	phases := engine.GetAllPhases()
	for _, phase := range phases {
		questions := engine.GetPhaseQuestions(phase)
		for _, q := range questions {
			// Record main answer
			engine.RecordAnswer(session, q.ID, fmt.Sprintf("Answer for %s. This is a somewhat long answer to simulate real content. It contains enough text to make the allocation matter.", q.Text))

			// Record multiple follow-up answers
			for i := 0; i < 5; i++ {
				engine.RecordFollowUpAnswer(session, q.ID, fmt.Sprintf("Follow up question %d?", i), fmt.Sprintf("Follow up answer %d. Adding more text here to increase payload size.", i))
			}

			// Record multiple iterations
			for i := 0; i < 3; i++ {
				engine.ReiterateAnswer(session, q.ID, fmt.Sprintf("Updated answer version %d", i), fmt.Sprintf("Reason for update %d", i))
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.GenerateSummary(session)
		if err != nil {
			b.Fatalf("GenerateSummary failed: %v", err)
		}
	}
}
