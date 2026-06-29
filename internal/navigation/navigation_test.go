package navigation

import (
	"os"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/git"
	"github.com/mojomast/nexdev/internal/state"
)

func TestNavigateToStage(t *testing.T) {
	// Create temporary database
	tmpDB := t.TempDir() + "/test.db"
	defer os.Remove(tmpDB)

	store, err := state.NewStore(tmpDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create git manager (won't actually commit in test)
	gitMgr := git.NewManager(".")

	// Create navigator
	nav := NewNavigator(store, gitMgr)

	// Create test project
	project := &state.Project{
		ID:           "test-project-1",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageDesign,
		CurrentPhase: "",
	}

	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Navigate back to interview
	result, err := nav.NavigateToStage(project.ID, state.StageInterview)
	if err != nil {
		t.Errorf("failed to navigate to interview: %v", err)
	}

	if result == nil {
		t.Fatal("expected navigation result, got nil")
	}

	if result.FromStage != state.StageDesign {
		t.Errorf("expected from stage design, got %s", result.FromStage)
	}

	if result.ToStage != state.StageInterview {
		t.Errorf("expected to stage interview, got %s", result.ToStage)
	}

	// Verify project stage was updated
	updatedProject, err := store.GetProject(project.ID)
	if err != nil {
		t.Errorf("failed to get updated project: %v", err)
	}

	if updatedProject.CurrentStage != state.StageInterview {
		t.Errorf("expected current stage interview, got %s", updatedProject.CurrentStage)
	}
}

func TestValidateNavigation(t *testing.T) {
	tmpDB := t.TempDir() + "/test.db"
	defer os.Remove(tmpDB)

	store, err := state.NewStore(tmpDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	gitMgr := git.NewManager(".")
	nav := NewNavigator(store, gitMgr)

	testCases := []struct {
		name        string
		from        state.Stage
		to          state.Stage
		expectError bool
	}{
		{
			name:        "Can go back from design to interview",
			from:        state.StageDesign,
			to:          state.StageInterview,
			expectError: false,
		},
		{
			name:        "Can go forward one stage",
			from:        state.StageInterview,
			to:          state.StageDesign,
			expectError: false,
		},
		{
			name:        "Cannot skip stages",
			from:        state.StageInterview,
			to:          state.StagePlan,
			expectError: true,
		},
		{
			name:        "Cannot navigate to same stage",
			from:        state.StageDesign,
			to:          state.StageDesign,
			expectError: true,
		},
		{
			name:        "Can go back multiple stages",
			from:        state.StageDevelop,
			to:          state.StageInterview,
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := nav.ValidateNavigation(tc.from, tc.to)
			if tc.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

func TestGetNavigationOptions(t *testing.T) {
	tmpDB := t.TempDir() + "/test.db"
	defer os.Remove(tmpDB)

	store, err := state.NewStore(tmpDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	gitMgr := git.NewManager(".")
	nav := NewNavigator(store, gitMgr)

	// Create project at design stage
	project := &state.Project{
		ID:           "test-project-2",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageDesign,
		CurrentPhase: "",
	}

	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Get navigation options
	options, err := nav.GetNavigationOptions(project.ID)
	if err != nil {
		t.Errorf("failed to get navigation options: %v", err)
	}

	if options == nil {
		t.Fatal("expected navigation options, got nil")
	}

	if options.CurrentStage != state.StageDesign {
		t.Errorf("expected current stage design, got %s", options.CurrentStage)
	}

	// Should be able to go back to init and interview
	if len(options.CanGoBack) != 2 {
		t.Errorf("expected 2 backward options, got %d", len(options.CanGoBack))
	}

	// Should be able to go forward to plan
	if options.NextStage != state.StagePlan {
		t.Errorf("expected next stage plan, got %s", options.NextStage)
	}

	if len(options.CanGoForward) != 1 {
		t.Errorf("expected 1 forward option, got %d", len(options.CanGoForward))
	}
}

func TestDetermineArtifacts(t *testing.T) {
	tmpDB := t.TempDir() + "/test.db"
	defer os.Remove(tmpDB)

	store, err := state.NewStore(tmpDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	gitMgr := git.NewManager(".")
	nav := NewNavigator(store, gitMgr)

	// Create project
	project := &state.Project{
		ID:           "test-project-3",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StagePlan,
		CurrentPhase: "",
	}

	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Add some artifacts
	interviewData := &state.InterviewData{
		ProjectID:        project.ID,
		ProjectName:      project.Name,
		CreatedAt:        time.Now(),
		ProblemStatement: "Test problem",
	}

	if err := store.SaveInterviewData(project.ID, interviewData); err != nil {
		t.Fatalf("failed to save interview data: %v", err)
	}

	// Test determining artifacts when going back
	result := &NavigationResult{
		PreservedWork:        []string{},
		RegeneratedArtifacts: []string{},
	}

	err = nav.determineArtifacts(project.ID, state.StagePlan, state.StageInterview, result)
	if err != nil {
		t.Errorf("failed to determine artifacts: %v", err)
	}

	// Should preserve interview data
	if len(result.PreservedWork) == 0 {
		t.Error("expected preserved work, got none")
	}

	// Should indicate devplan may need regeneration
	if len(result.RegeneratedArtifacts) == 0 {
		t.Error("expected regenerated artifacts, got none")
	}
}

func TestHistoryTracker(t *testing.T) {
	tmpDB := t.TempDir() + "/test.db"
	defer os.Remove(tmpDB)

	store, err := state.NewStore(tmpDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	gitMgr := git.NewManager(".")
	tracker := NewHistoryTracker(store, gitMgr)

	projectID := "test-project"

	// Initially, history should be empty
	history, err := tracker.GetNavigationHistory(projectID)
	if err != nil {
		t.Errorf("failed to get empty navigation history: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d events", len(history))
	}

	// Record navigation events with slight delay to get different timestamps
	err = tracker.RecordNavigation(projectID, state.StageDesign, state.StageInterview)
	if err != nil {
		t.Fatalf("failed to record first navigation: %v", err)
	}

	// Sleep briefly to ensure different unix timestamps
	time.Sleep(1100 * time.Millisecond)

	err = tracker.RecordNavigation(projectID, state.StageInterview, state.StageDesign)
	if err != nil {
		t.Fatalf("failed to record second navigation: %v", err)
	}

	// Get history - should now have 2 events
	history, err = tracker.GetNavigationHistory(projectID)
	if err != nil {
		t.Errorf("failed to get navigation history: %v", err)
	}

	if len(history) != 2 {
		t.Fatalf("expected 2 history events, got %d", len(history))
	}

	// First event should be the earlier one
	if history[0].FromStage != state.StageDesign || history[0].ToStage != state.StageInterview {
		t.Errorf("first event: expected design->interview, got %s->%s", history[0].FromStage, history[0].ToStage)
	}

	if history[1].FromStage != state.StageInterview || history[1].ToStage != state.StageDesign {
		t.Errorf("second event: expected interview->design, got %s->%s", history[1].FromStage, history[1].ToStage)
	}

	// Verify project ID is set
	for _, event := range history {
		if event.ProjectID != projectID {
			t.Errorf("expected projectID %s, got %s", projectID, event.ProjectID)
		}
	}

	// Verify GetIterationCount works with real history
	count, err := tracker.GetIterationCount(projectID, state.StageInterview)
	if err != nil {
		t.Errorf("failed to get iteration count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 visit to interview, got %d", count)
	}

	count, err = tracker.GetIterationCount(projectID, state.StageDesign)
	if err != nil {
		t.Errorf("failed to get iteration count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 visit to design, got %d", count)
	}

	// History for a different project should be empty
	otherHistory, err := tracker.GetNavigationHistory("other-project")
	if err != nil {
		t.Errorf("failed to get other project history: %v", err)
	}
	if len(otherHistory) != 0 {
		t.Errorf("expected empty history for other project, got %d events", len(otherHistory))
	}
}

func TestCheckPrerequisites(t *testing.T) {
	tmpDB := t.TempDir() + "/test.db"
	defer os.Remove(tmpDB)

	store, err := state.NewStore(tmpDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	gitMgr := git.NewManager(".")
	nav := NewNavigator(store, gitMgr)

	projectID := "test-prereqs"

	// Create project
	project := &state.Project{
		ID:           projectID,
		Name:         "Test Prerequisites",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInit,
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	t.Run("Init to Interview succeeds with project", func(t *testing.T) {
		err := nav.checkPrerequisites(projectID, state.StageInit, state.StageInterview)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("Interview to Design fails without interview data", func(t *testing.T) {
		err := nav.checkPrerequisites(projectID, state.StageInterview, state.StageDesign)
		if err == nil {
			t.Error("expected error for missing interview data, got nil")
		}
	})

	// Add interview data
	interviewData := &state.InterviewData{
		ProjectID:        projectID,
		ProjectName:      "Test Prerequisites",
		CreatedAt:        time.Now(),
		ProblemStatement: "Test problem",
	}
	if err := store.SaveInterviewData(projectID, interviewData); err != nil {
		t.Fatalf("failed to save interview data: %v", err)
	}

	t.Run("Interview to Design succeeds with interview data", func(t *testing.T) {
		err := nav.checkPrerequisites(projectID, state.StageInterview, state.StageDesign)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("Design to Plan fails without architecture", func(t *testing.T) {
		err := nav.checkPrerequisites(projectID, state.StageDesign, state.StagePlan)
		if err == nil {
			t.Error("expected error for missing architecture, got nil")
		}
	})

	// Add architecture
	architecture := &state.Architecture{
		ProjectID: projectID,
		CreatedAt: time.Now(),
		Content:   "Test architecture",
	}
	if err := store.SaveArchitecture(projectID, architecture); err != nil {
		t.Fatalf("failed to save architecture: %v", err)
	}

	t.Run("Design to Plan succeeds with architecture", func(t *testing.T) {
		err := nav.checkPrerequisites(projectID, state.StageDesign, state.StagePlan)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("Plan to Review fails without phases", func(t *testing.T) {
		err := nav.checkPrerequisites(projectID, state.StagePlan, state.StageReview)
		if err == nil {
			t.Error("expected error for missing phases, got nil")
		}
	})

	// Add a phase
	phase := &state.Phase{
		ID:        "phase-1",
		ProjectID: projectID,
		Number:    1,
		Title:     "Phase 1",
		Status:    state.PhaseNotStarted,
		CreatedAt: time.Now(),
	}
	if err := store.SavePhase(phase); err != nil {
		t.Fatalf("failed to save phase: %v", err)
	}

	t.Run("Plan to Review succeeds with phases", func(t *testing.T) {
		err := nav.checkPrerequisites(projectID, state.StagePlan, state.StageReview)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("Review to Develop succeeds with phases", func(t *testing.T) {
		err := nav.checkPrerequisites(projectID, state.StageReview, state.StageDevelop)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("Develop to Complete fails without tasks", func(t *testing.T) {
		err := nav.checkPrerequisites(projectID, state.StageDevelop, state.StageComplete)
		if err == nil {
			t.Error("expected error for missing tasks, got nil")
		}
	})

	// Add tasks - one incomplete
	task1 := &state.Task{
		ID:          "task-1",
		PhaseID:     phase.ID,
		Number:      "1.1",
		Description: "First task",
		Status:      state.TaskCompleted,
	}
	task2 := &state.Task{
		ID:          "task-2",
		PhaseID:     phase.ID,
		Number:      "1.2",
		Description: "Second task",
		Status:      state.TaskInProgress,
	}
	if err := store.SaveTask(task1); err != nil {
		t.Fatalf("failed to save task1: %v", err)
	}
	if err := store.SaveTask(task2); err != nil {
		t.Fatalf("failed to save task2: %v", err)
	}

	t.Run("Develop to Complete fails with incomplete tasks", func(t *testing.T) {
		err := nav.checkPrerequisites(projectID, state.StageDevelop, state.StageComplete)
		if err == nil {
			t.Error("expected error for incomplete tasks, got nil")
		}
	})

	// Complete the second task
	if err := store.UpdateTaskStatus(task2.ID, state.TaskCompleted); err != nil {
		t.Fatalf("failed to complete task2: %v", err)
	}

	t.Run("Develop to Complete succeeds with all tasks complete", func(t *testing.T) {
		err := nav.checkPrerequisites(projectID, state.StageDevelop, state.StageComplete)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("Unknown target stage returns error", func(t *testing.T) {
		err := nav.checkPrerequisites(projectID, state.StageInit, state.Stage("unknown"))
		if err == nil {
			t.Error("expected error for unknown stage, got nil")
		}
	})
}
