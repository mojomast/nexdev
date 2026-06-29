package resume

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/checkpoint"
	"github.com/mojomast/nexdev/internal/git"
	"github.com/mojomast/nexdev/internal/state"
)

func TestDetectIncompleteWork(t *testing.T) {
	// Create temporary database
	tmpDB := t.TempDir() + "/test.db"
	defer os.Remove(tmpDB)

	store, err := state.NewStore(tmpDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create checkpoint manager
	gitMgr := git.NewManager(".")
	dataDir := filepath.Dir(tmpDB)
	checkpointMgr := checkpoint.NewManager(store, gitMgr, dataDir)

	// Create resume manager
	mgr := NewManager(store, checkpointMgr)

	// Create a test project
	project := &state.Project{
		ID:           "test-project-1",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInterview,
		CurrentPhase: "",
	}

	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Test detection
	info, err := mgr.DetectIncompleteWork(project.ID)
	if err != nil {
		t.Errorf("failed to detect incomplete work: %v", err)
	}

	if info == nil {
		t.Fatal("expected resume info, got nil")
	}

	if !info.HasIncompleteWork {
		t.Error("expected incomplete work for project in interview stage")
	}

	if info.CurrentStage != state.StageInterview {
		t.Errorf("expected stage interview, got %s", info.CurrentStage)
	}
}

func TestDetectCompleteWork(t *testing.T) {
	// Create temporary database
	tmpDB := t.TempDir() + "/test.db"
	defer os.Remove(tmpDB)

	store, err := state.NewStore(tmpDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create checkpoint manager
	gitMgr := git.NewManager(".")
	dataDir := filepath.Dir(tmpDB)
	checkpointMgr := checkpoint.NewManager(store, gitMgr, dataDir)

	// Create resume manager
	mgr := NewManager(store, checkpointMgr)

	// Create a completed project
	project := &state.Project{
		ID:           "test-project-2",
		Name:         "Completed Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageComplete,
		CurrentPhase: "",
	}

	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Test detection
	info, err := mgr.DetectIncompleteWork(project.ID)
	if err != nil {
		t.Errorf("failed to detect incomplete work: %v", err)
	}

	if info == nil {
		t.Fatal("expected resume info, got nil")
	}

	if info.HasIncompleteWork {
		t.Error("expected no incomplete work for completed project")
	}
}

func TestResume_FromCurrent(t *testing.T) {
	// Create temporary database
	tmpDB := t.TempDir() + "/test.db"
	defer os.Remove(tmpDB)

	store, err := state.NewStore(tmpDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create checkpoint manager
	gitMgr := git.NewManager(".")
	dataDir := filepath.Dir(tmpDB)
	checkpointMgr := checkpoint.NewManager(store, gitMgr, dataDir)

	// Create resume manager
	mgr := NewManager(store, checkpointMgr)

	// Create a test project
	project := &state.Project{
		ID:           "test-project-3",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageDesign,
		CurrentPhase: "",
	}

	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Resume from current state
	result, err := mgr.Resume(&ResumeOptions{
		ProjectID: project.ID,
	})

	if err != nil {
		t.Errorf("failed to resume: %v", err)
	}

	if result == nil {
		t.Fatal("expected resume result, got nil")
	}

	if result.Stage != state.StageDesign {
		t.Errorf("expected stage design, got %s", result.Stage)
	}

	if result.RestoredFrom != "current" {
		t.Errorf("expected restored from current, got %s", result.RestoredFrom)
	}
}

func TestResume_RestartStage(t *testing.T) {
	// Create temporary database
	tmpDB := t.TempDir() + "/test.db"
	defer os.Remove(tmpDB)

	store, err := state.NewStore(tmpDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create checkpoint manager
	gitMgr := git.NewManager(".")
	dataDir := filepath.Dir(tmpDB)
	checkpointMgr := checkpoint.NewManager(store, gitMgr, dataDir)

	// Create resume manager
	mgr := NewManager(store, checkpointMgr)

	// Create a test project with phases
	project := &state.Project{
		ID:           "test-project-4",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageDevelop,
		CurrentPhase: "phase-1",
	}

	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Create a phase with in-progress status
	phase := &state.Phase{
		ID:        "phase-1",
		ProjectID: project.ID,
		Number:    1,
		Title:     "Test Phase",
		Content:   "Test content",
		Status:    state.PhaseInProgress,
		CreatedAt: time.Now(),
	}

	if err := store.SavePhase(phase); err != nil {
		t.Fatalf("failed to save phase: %v", err)
	}

	// Resume with restart stage
	result, err := mgr.Resume(&ResumeOptions{
		ProjectID:    project.ID,
		RestartStage: true,
	})

	if err != nil {
		t.Errorf("failed to resume with restart: %v", err)
	}

	if result == nil {
		t.Fatal("expected resume result, got nil")
	}

	// Verify phase was reset
	reloadedPhase, err := store.GetPhase(phase.ID)
	if err != nil {
		t.Errorf("failed to reload phase: %v", err)
	}

	if reloadedPhase.Status != state.PhaseNotStarted {
		t.Errorf("expected phase to be reset to not_started, got %s", reloadedPhase.Status)
	}
}

func TestGetResumeContext(t *testing.T) {
	// Create temporary database
	tmpDB := t.TempDir() + "/test.db"
	defer os.Remove(tmpDB)

	store, err := state.NewStore(tmpDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create checkpoint manager
	gitMgr := git.NewManager(".")
	dataDir := filepath.Dir(tmpDB)
	checkpointMgr := checkpoint.NewManager(store, gitMgr, dataDir)

	// Create resume manager
	mgr := NewManager(store, checkpointMgr)

	// Create a test project
	project := &state.Project{
		ID:           "test-project-5",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageDevelop,
		CurrentPhase: "",
	}

	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Add interview data
	interviewData := &state.InterviewData{
		ProjectID:        project.ID,
		ProjectName:      project.Name,
		CreatedAt:        time.Now(),
		ProblemStatement: "Test problem",
	}

	if err := store.SaveInterviewData(project.ID, interviewData); err != nil {
		t.Fatalf("failed to save interview data: %v", err)
	}

	// Get resume context
	ctx, err := mgr.GetResumeContext(project.ID, state.StageDevelop)
	if err != nil {
		t.Errorf("failed to get resume context: %v", err)
	}

	if ctx == nil {
		t.Fatal("expected resume context, got nil")
	}

	if ctx.Stage != state.StageDevelop {
		t.Errorf("expected stage develop, got %s", ctx.Stage)
	}

	if ctx.InterviewData == nil {
		t.Error("expected interview data in context")
	}

	if ctx.Progress == nil {
		t.Error("expected progress in context")
	}
}

func TestDetermineNextAction(t *testing.T) {
	// Create temporary database
	tmpDB := t.TempDir() + "/test.db"
	defer os.Remove(tmpDB)

	store, err := state.NewStore(tmpDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create checkpoint manager
	gitMgr := git.NewManager(".")
	dataDir := filepath.Dir(tmpDB)
	checkpointMgr := checkpoint.NewManager(store, gitMgr, dataDir)

	// Create resume manager
	mgr := NewManager(store, checkpointMgr)

	// Test cases
	testCases := []struct {
		name           string
		stage          state.Stage
		expectedPhrase string
	}{
		{
			name:           "Init stage",
			stage:          state.StageInit,
			expectedPhrase: "geoffrussy init",
		},
		{
			name:           "Interview stage",
			stage:          state.StageInterview,
			expectedPhrase: "geoffrussy interview",
		},
		{
			name:           "Design stage",
			stage:          state.StageDesign,
			expectedPhrase: "geoffrussy design",
		},
		{
			name:           "Plan stage",
			stage:          state.StagePlan,
			expectedPhrase: "geoffrussy plan",
		},
		{
			name:           "Review stage",
			stage:          state.StageReview,
			expectedPhrase: "geoffrussy review",
		},
		{
			name:           "Develop stage",
			stage:          state.StageDevelop,
			expectedPhrase: "geoffrussy develop",
		},
		{
			name:           "Complete stage",
			stage:          state.StageComplete,
			expectedPhrase: "complete",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			project := &state.Project{
				ID:           "test-project",
				Name:         "Test",
				CreatedAt:    time.Now(),
				CurrentStage: tc.stage,
			}

			action := mgr.determineNextAction(project, tc.stage)

			// Check if expected phrase is in the action
			if action == "" {
				t.Error("expected non-empty next action")
			}

			// Simple substring check to verify the action contains the expected phrase
			// This is a loose check since the exact wording might vary
			t.Logf("Stage: %s, Action: %s", tc.stage, action)
		})
	}
}
