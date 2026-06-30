package pipeline

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/state"
)

func TestRunnerRunsCanonicalStagesInOrderAndPersistsEvents(t *testing.T) {
	ctx := context.Background()
	store, runID := newRunnerTestStore(t)
	runner := newTestRunner(t, store, allPrerequisitesProvider(store))
	var calls []Stage
	for _, stage := range CanonicalStages {
		registerTestStage(t, runner, &fakeRunnerStage{name: stage, calls: &calls, output: map[string]any{"stage": string(stage)}})
	}

	if err := runner.Run(ctx, runnerTestEnv(), RunOptions{RunID: runID}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !reflect.DeepEqual(calls, CanonicalStages) {
		t.Fatalf("stage calls mismatch\ngot:  %#v\nwant: %#v", calls, CanonicalStages)
	}
	loadedRun, err := store.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	if loadedRun.Status != "completed" || loadedRun.CurrentStage != string(StageComplete) {
		t.Fatalf("run after completion = %+v", loadedRun)
	}
	stageRuns, err := store.ListStageRunsByRun(ctx, runID)
	if err != nil {
		t.Fatalf("ListStageRunsByRun failed: %v", err)
	}
	if len(stageRuns) != len(CanonicalStages) {
		t.Fatalf("stage run count = %d, want %d", len(stageRuns), len(CanonicalStages))
	}
	for _, stageRun := range stageRuns {
		if stageRun.Status != string(StageStatusCompleted) {
			t.Fatalf("stage %s status = %s, want completed", stageRun.Stage, stageRun.Status)
		}
		if stageRun.Output["stage"] != stageRun.Stage {
			t.Fatalf("stage %s output mismatch: %#v", stageRun.Stage, stageRun.Output)
		}
	}
	events, err := store.ListEvents(ctx, state.EventListOptions{RunID: runID})
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	if got, want := countEvents(events, contract.EventTypeStageStatus), len(CanonicalStages)*2; got != want {
		t.Fatalf("stage_status event count = %d, want %d", got, want)
	}
	if events[len(events)-1].Type != contract.EventTypeDone {
		t.Fatalf("last event type = %s, want done", events[len(events)-1].Type)
	}
}

func TestRunnerRejectsMissingPrerequisite(t *testing.T) {
	ctx := context.Background()
	store, runID := newRunnerTestStore(t)
	runner := newTestRunner(t, store, PrerequisiteProviderFunc(func(context.Context, StageEnv, Stage) (PrerequisiteSnapshot, error) {
		return PrerequisiteSnapshot{}, nil
	}))
	registerTestStage(t, runner, &fakeRunnerStage{name: StageRepoAnalyze})

	err := runner.Run(ctx, runnerTestEnv(), RunOptions{RunID: runID, From: StageRepoAnalyze, SingleStage: StageRepoAnalyze})
	var prereqErr *PrerequisiteError
	if !errors.As(err, &prereqErr) {
		t.Fatalf("Run error = %T %v, want PrerequisiteError", err, err)
	}
	if !reflect.DeepEqual(prereqErr.Missing, []RequirementKey{RequirementProjectExists}) {
		t.Fatalf("missing prerequisites = %#v", prereqErr.Missing)
	}
}

func TestRunnerResumeSelectsPersistedNextStage(t *testing.T) {
	ctx := context.Background()
	store, runID := newRunnerTestStore(t)
	completedAt := time.Date(2026, 6, 30, 16, 0, 0, 0, time.UTC)
	if err := store.CreateStageRun(ctx, &state.StageRun{ID: "stage_done", RunID: runID, Stage: string(StageRepoAnalyze), Status: string(StageStatusCompleted), CompletedAt: &completedAt}); err != nil {
		t.Fatalf("CreateStageRun completed failed: %v", err)
	}
	if err := store.UpdateRunCurrentStage(ctx, runID, string(StageRepoAnalyze)); err != nil {
		t.Fatalf("UpdateRunCurrentStage failed: %v", err)
	}
	runner := newTestRunner(t, store, allPrerequisitesProvider(store))
	var calls []Stage
	for _, stage := range CanonicalStages[2:] {
		registerTestStage(t, runner, &fakeRunnerStage{name: stage, calls: &calls})
	}

	if err := runner.Resume(ctx, runnerTestEnv(), runID); err != nil {
		t.Fatalf("Resume failed: %v", err)
	}
	if len(calls) == 0 || calls[0] != StageInterview {
		t.Fatalf("resume calls = %#v, want first interview", calls)
	}
	for _, called := range calls {
		if called == StageRepoAnalyze {
			t.Fatalf("resume reran completed stage: %#v", calls)
		}
	}
}

func TestLatestStageRunsSortsDuplicateStagesDeterministically(t *testing.T) {
	older := &state.StageRun{ID: "stage_001", Stage: string(StageInterview), Status: string(StageStatusCompleted)}
	newer := &state.StageRun{ID: "stage_999", Stage: string(StageInterview), Status: string(StageStatusRunning)}
	other := &state.StageRun{ID: "stage_100", Stage: string(StageDesign), Status: string(StageStatusPending)}

	latest := latestStageRuns([]*state.StageRun{newer, nil, other, older})
	if latest[StageInterview] != newer {
		t.Fatalf("latest interview = %#v, want lexicographically newest duplicate", latest[StageInterview])
	}
	if latest[StageDesign] != other {
		t.Fatalf("latest design = %#v, want original design run", latest[StageDesign])
	}
}

func TestRunnerEnforcesPersistedStatusTransitions(t *testing.T) {
	ctx := context.Background()
	store, runID := newRunnerTestStore(t)
	if err := store.CreateStageRun(ctx, &state.StageRun{ID: "stage_cancelled", RunID: runID, Stage: string(StageRepoAnalyze), Status: string(StageStatusCancelled)}); err != nil {
		t.Fatalf("CreateStageRun failed: %v", err)
	}
	runner := newTestRunner(t, store, allPrerequisitesProvider(store))
	registerTestStage(t, runner, &fakeRunnerStage{name: StageRepoAnalyze})

	err := runner.Run(ctx, runnerTestEnv(), RunOptions{RunID: runID, SingleStage: StageRepoAnalyze})
	if err == nil || !errors.Is(err, context.Canceled) && err.Error() != "invalid stage status transition cancelled -> running" {
		t.Fatalf("Run error = %v, want invalid transition", err)
	}
}

func TestRunnerPersistsBlockedAndFailedStages(t *testing.T) {
	ctx := context.Background()

	blockedStore, blockedRunID := newRunnerTestStore(t)
	blockedRunner := newTestRunner(t, blockedStore, allPrerequisitesProvider(blockedStore))
	registerTestStage(t, blockedRunner, &fakeRunnerStage{name: StageDevelop, runErr: &BlockedError{Reason: "needs_input"}, output: map[string]any{"partial": true}})
	if err := blockedRunner.Run(ctx, runnerTestEnv(), RunOptions{RunID: blockedRunID, SingleStage: StageDevelop}); err == nil {
		t.Fatal("blocked stage succeeded, want error")
	}
	blocked := latestSingleStageRun(t, blockedStore, blockedRunID, StageDevelop)
	if blocked.Status != string(StageStatusBlocked) || blocked.Output["partial"] != true || blocked.Error["reason"] != "needs_input" {
		t.Fatalf("blocked stage run = %+v", blocked)
	}

	failedStore, failedRunID := newRunnerTestStore(t)
	failedRunner := newTestRunner(t, failedStore, allPrerequisitesProvider(failedStore))
	registerTestStage(t, failedRunner, &fakeRunnerStage{name: StageVerify, runErr: fmt.Errorf("verify exploded")})
	if err := failedRunner.Run(ctx, runnerTestEnv(), RunOptions{RunID: failedRunID, SingleStage: StageVerify}); err == nil {
		t.Fatal("failed stage succeeded, want error")
	}
	failed := latestSingleStageRun(t, failedStore, failedRunID, StageVerify)
	if failed.Status != string(StageStatusFailed) || failed.Error["message"] != "verify exploded" {
		t.Fatalf("failed stage run = %+v", failed)
	}
}

func TestRunnerPersistsSkippedCheckpoint(t *testing.T) {
	ctx := context.Background()
	store, runID := newRunnerTestStore(t)
	runner := newTestRunner(t, store, allPrerequisitesProvider(store))
	registerTestStage(t, runner, &fakeRunnerStage{name: StageHivemind, validateErr: ErrStageSkipped, output: map[string]any{"skipped_reason": "disabled"}})

	if err := runner.Run(ctx, runnerTestEnv(), RunOptions{RunID: runID, SingleStage: StageHivemind}); err != nil {
		t.Fatalf("Run skipped stage failed: %v", err)
	}
	stageRun := latestSingleStageRun(t, store, runID, StageHivemind)
	if stageRun.Status != string(StageStatusSkipped) || stageRun.Output["skipped_reason"] != "disabled" {
		t.Fatalf("skipped stage run = %+v", stageRun)
	}
}

type fakeRunnerStage struct {
	name        Stage
	calls       *[]Stage
	validateErr error
	runErr      error
	resumeErr   error
	output      map[string]any
}

func (s *fakeRunnerStage) Name() Stage { return s.name }

func (s *fakeRunnerStage) Validate(context.Context, StageEnv) error { return s.validateErr }

func (s *fakeRunnerStage) Run(context.Context, StageEnv) error {
	if s.calls != nil {
		*s.calls = append(*s.calls, s.name)
	}
	return s.runErr
}

func (s *fakeRunnerStage) Resume(context.Context, StageEnv) error {
	if s.calls != nil {
		*s.calls = append(*s.calls, s.name)
	}
	return s.resumeErr
}

func (s *fakeRunnerStage) Output(context.Context, StageEnv) (map[string]any, error) {
	return s.output, nil
}

type testProjectRef struct{ id string }

func (p testProjectRef) ProjectID() string { return p.id }

func runnerTestEnv() StageEnv {
	return StageEnv{Project: testProjectRef{id: "proj_runner"}}
}

func newRunnerTestStore(t *testing.T) (*state.Store, string) {
	t.Helper()
	store, err := state.NewStore(t.TempDir() + "/pipeline_runner.db")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	})
	if err := store.CreateProject(&state.Project{ID: "proj_runner", Name: "Runner Project", CreatedAt: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC), CurrentStage: state.StageInit}); err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}
	runID := "run_runner"
	if err := store.CreateRun(context.Background(), &state.Run{ID: runID, ProjectID: "proj_runner", Status: "pending", StartedAt: time.Date(2026, 6, 30, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}
	return store, runID
}

func newTestRunner(t *testing.T, store *state.Store, provider PrerequisiteProvider) *Runner {
	t.Helper()
	seq := 0
	runner, err := NewRunner(store,
		WithPrerequisiteProvider(provider),
		WithIDGenerator(func(prefix string) string {
			seq++
			return fmt.Sprintf("%s_%03d", prefix, seq)
		}),
		WithClock(func() time.Time { return time.Date(2026, 6, 30, 2, 0, 0, seq, time.UTC) }),
	)
	if err != nil {
		t.Fatalf("NewRunner failed: %v", err)
	}
	return runner
}

func registerTestStage(t *testing.T, runner *Runner, stage PipelineStage) {
	t.Helper()
	if err := runner.Register(stage); err != nil {
		t.Fatalf("Register(%s) failed: %v", stage.Name(), err)
	}
}

func allPrerequisitesProvider(store *state.Store) PrerequisiteProvider {
	return PrerequisiteProviderFunc(func(ctx context.Context, env StageEnv, target Stage) (PrerequisiteSnapshot, error) {
		snapshot := NewPrerequisiteSnapshot(
			RequirementProjectExists,
			RequirementInterviewDataExists,
			RequirementRepoAnalysisExists,
			RequirementDesignDraftExists,
			RequirementLatestHivemindSynthesisExists,
			RequirementValidationPassedOrWarningsAccepted,
			RequirementPhaseSketchExists,
			RequirementDetailedPlanExists,
			RequirementReviewedApprovedPlanExists,
			RequirementDevelopHasNoRunningTasks,
			RequirementVerifyCompleteOrExplicitlySkipped,
			RequirementHandoffExists,
			RequirementAllRequiredReportsExist,
		)
		if env.Run != nil {
			stageRuns, err := store.ListStageRunsByRun(ctx, env.Run.RunID())
			if err != nil {
				return PrerequisiteSnapshot{}, err
			}
			snapshot.SkippedStages = map[Stage]bool{}
			for _, stageRun := range stageRuns {
				if stageRun.Status == string(StageStatusSkipped) {
					snapshot.SkippedStages[Stage(stageRun.Stage)] = true
				}
			}
		}
		return snapshot, nil
	})
}

func latestSingleStageRun(t *testing.T, store *state.Store, runID string, stage Stage) *state.StageRun {
	t.Helper()
	stageRuns, err := store.ListStageRunsByRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("ListStageRunsByRun failed: %v", err)
	}
	var found *state.StageRun
	for _, stageRun := range stageRuns {
		if stageRun.Stage == string(stage) {
			found = stageRun
		}
	}
	if found == nil {
		t.Fatalf("stage run not found for %s", stage)
	}
	return found
}

func countEvents(events []contract.EventEnvelope, eventType string) int {
	count := 0
	for _, event := range events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}
