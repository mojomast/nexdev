package state

import (
	"context"
	"testing"
	"time"
)

func TestStoreRunRepositoryCreateReadUpdateList(t *testing.T) {
	store := newRunArtifactTestStore(t)
	seedStateCProject(t, store, "proj_runs")
	ctx := context.Background()

	firstStarted := time.Date(2026, 6, 30, 6, 0, 0, 123456789, time.FixedZone("offset", -4*60*60))
	secondStarted := time.Date(2026, 6, 30, 11, 0, 0, 0, time.UTC)
	if err := store.CreateRun(ctx, &Run{
		ID:           "run_second",
		ProjectID:    "proj_runs",
		Status:       "pending",
		CurrentStage: "interview",
		StartedAt:    secondStarted,
		Metadata:     map[string]any{"operator": "test"},
	}); err != nil {
		t.Fatalf("CreateRun second failed: %v", err)
	}
	if err := store.CreateRun(ctx, &Run{
		ID:           "run_first",
		ProjectID:    "proj_runs",
		Status:       "running",
		CurrentStage: "repo_analyze",
		StartedAt:    firstStarted,
		Metadata:     map[string]any{"nested": map[string]any{"ok": true}},
	}); err != nil {
		t.Fatalf("CreateRun first failed: %v", err)
	}

	loaded, err := store.GetRun(ctx, "run_first")
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	if loaded.StartedAt.Location() != time.UTC {
		t.Fatalf("StartedAt location = %v, want UTC", loaded.StartedAt.Location())
	}
	if got, want := loaded.StartedAt.Format(time.RFC3339Nano), firstStarted.UTC().Format(time.RFC3339Nano); got != want {
		t.Fatalf("StartedAt = %s, want %s", got, want)
	}
	if nested, ok := loaded.Metadata["nested"].(map[string]any); !ok || nested["ok"] != true {
		t.Fatalf("metadata round-trip mismatch: %#v", loaded.Metadata)
	}

	if err := store.UpdateRunStatus(ctx, "run_first", "blocked"); err != nil {
		t.Fatalf("UpdateRunStatus failed: %v", err)
	}
	if err := store.UpdateRunCurrentStage(ctx, "run_first", "develop"); err != nil {
		t.Fatalf("UpdateRunCurrentStage failed: %v", err)
	}
	completedAt := time.Date(2026, 6, 30, 12, 0, 0, 7, time.FixedZone("offset", 2*60*60))
	if err := store.CompleteRun(ctx, "run_first", completedAt); err != nil {
		t.Fatalf("CompleteRun failed: %v", err)
	}
	cancelledAt := time.Date(2026, 6, 30, 13, 0, 0, 8, time.UTC)
	if err := store.CancelRun(ctx, "run_second", cancelledAt); err != nil {
		t.Fatalf("CancelRun failed: %v", err)
	}

	loaded, err = store.GetRun(ctx, "run_first")
	if err != nil {
		t.Fatalf("GetRun after update failed: %v", err)
	}
	if loaded.Status != "completed" || loaded.CurrentStage != "develop" {
		t.Fatalf("updated run = %+v", loaded)
	}
	if loaded.CompletedAt == nil || loaded.CompletedAt.Format(time.RFC3339Nano) != completedAt.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("CompletedAt = %v, want %s", loaded.CompletedAt, completedAt.UTC().Format(time.RFC3339Nano))
	}

	runs, err := store.ListRunsByProject(ctx, "proj_runs")
	if err != nil {
		t.Fatalf("ListRunsByProject failed: %v", err)
	}
	assertRunIDs(t, runs, []string{"run_first", "run_second"})

	second, err := store.GetRun(ctx, "run_second")
	if err != nil {
		t.Fatalf("GetRun second failed: %v", err)
	}
	if second.Status != "cancelled" || second.CancelledAt == nil {
		t.Fatalf("cancelled run = %+v", second)
	}
}

func TestStoreStageRunRepositoryCreateReadUpdateList(t *testing.T) {
	store := newRunArtifactTestStore(t)
	seedStateCRun(t, store, "proj_stage_runs", "run_stage_runs")
	ctx := context.Background()

	startedAt := time.Date(2026, 6, 30, 14, 0, 0, 1, time.FixedZone("offset", -7*60*60))
	if err := store.CreateStageRun(ctx, &StageRun{
		ID:        "stage_one",
		RunID:     "run_stage_runs",
		Stage:     "design",
		Status:    "running",
		StartedAt: &startedAt,
		Output:    map[string]any{"draft": "ok"},
	}); err != nil {
		t.Fatalf("CreateStageRun first failed: %v", err)
	}
	if err := store.CreateStageRun(ctx, &StageRun{
		ID:      "stage_two",
		RunID:   "run_stage_runs",
		Stage:   "validate",
		Status:  "pending",
		Attempt: 2,
	}); err != nil {
		t.Fatalf("CreateStageRun second failed: %v", err)
	}

	loaded, err := store.GetStageRun(ctx, "stage_one")
	if err != nil {
		t.Fatalf("GetStageRun failed: %v", err)
	}
	if loaded.Attempt != 1 {
		t.Fatalf("default attempt = %d, want 1", loaded.Attempt)
	}
	if loaded.StartedAt == nil || loaded.StartedAt.Format(time.RFC3339Nano) != startedAt.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("StartedAt = %v, want %s", loaded.StartedAt, startedAt.UTC().Format(time.RFC3339Nano))
	}
	if loaded.Output["draft"] != "ok" {
		t.Fatalf("output round-trip mismatch: %#v", loaded.Output)
	}

	if err := store.UpdateStageRunStatus(ctx, "stage_one", "failed"); err != nil {
		t.Fatalf("UpdateStageRunStatus failed: %v", err)
	}
	if err := store.UpdateStageRunAttempt(ctx, "stage_one", 2); err != nil {
		t.Fatalf("UpdateStageRunAttempt failed: %v", err)
	}
	if err := store.UpdateStageRunOutput(ctx, "stage_one", map[string]any{"report": map[string]any{"passed": false}}); err != nil {
		t.Fatalf("UpdateStageRunOutput failed: %v", err)
	}
	if err := store.UpdateStageRunError(ctx, "stage_one", map[string]any{"message": "boom", "retryable": true}); err != nil {
		t.Fatalf("UpdateStageRunError failed: %v", err)
	}
	completedAt := time.Date(2026, 6, 30, 15, 0, 0, 2, time.UTC)
	if err := store.CompleteStageRun(ctx, "stage_one", completedAt); err != nil {
		t.Fatalf("CompleteStageRun failed: %v", err)
	}

	loaded, err = store.GetStageRun(ctx, "stage_one")
	if err != nil {
		t.Fatalf("GetStageRun after update failed: %v", err)
	}
	if loaded.Status != "completed" || loaded.Attempt != 2 {
		t.Fatalf("updated stage run = %+v", loaded)
	}
	if loaded.Error["message"] != "boom" || loaded.Error["retryable"] != true {
		t.Fatalf("error round-trip mismatch: %#v", loaded.Error)
	}
	if report, ok := loaded.Output["report"].(map[string]any); !ok || report["passed"] != false {
		t.Fatalf("output update mismatch: %#v", loaded.Output)
	}

	stageRuns, err := store.ListStageRunsByRun(ctx, "run_stage_runs")
	if err != nil {
		t.Fatalf("ListStageRunsByRun failed: %v", err)
	}
	assertStageRunIDs(t, stageRuns, []string{"stage_two", "stage_one"})
}

func TestStoreRunStageForeignKeys(t *testing.T) {
	store := newRunArtifactTestStore(t)
	ctx := context.Background()

	if err := store.CreateRun(ctx, &Run{ID: "run_missing_project", ProjectID: "missing", Status: "running"}); err == nil {
		t.Fatal("CreateRun with missing project succeeded, want FK failure")
	}
	if err := store.CreateStageRun(ctx, &StageRun{ID: "stage_missing_run", RunID: "missing", Stage: "design", Status: "pending"}); err == nil {
		t.Fatal("CreateStageRun with missing run succeeded, want FK failure")
	}
}

func newRunArtifactTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(t.TempDir() + "/state_c.db")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	})
	return store
}

func seedStateCProject(t *testing.T, store *Store, projectID string) {
	t.Helper()
	_, err := store.DB().Exec(`
		INSERT OR IGNORE INTO projects (id, name, created_at, current_stage)
		VALUES (?, ?, ?, 'init')
	`, projectID, projectID, time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("seed project failed: %v", err)
	}
}

func seedStateCRun(t *testing.T, store *Store, projectID, runID string) {
	t.Helper()
	seedStateCProject(t, store, projectID)
	if err := store.CreateRun(context.Background(), &Run{
		ID:        runID,
		ProjectID: projectID,
		Status:    "running",
		StartedAt: time.Date(2026, 6, 30, 1, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("seed run failed: %v", err)
	}
}

func assertRunIDs(t *testing.T, runs []*Run, want []string) {
	t.Helper()
	if len(runs) != len(want) {
		t.Fatalf("run count = %d, want %d", len(runs), len(want))
	}
	for i, run := range runs {
		if run.ID != want[i] {
			t.Fatalf("run[%d] = %s, want %s", i, run.ID, want[i])
		}
	}
}

func assertStageRunIDs(t *testing.T, stageRuns []*StageRun, want []string) {
	t.Helper()
	if len(stageRuns) != len(want) {
		t.Fatalf("stage run count = %d, want %d", len(stageRuns), len(want))
	}
	for i, stageRun := range stageRuns {
		if stageRun.ID != want[i] {
			t.Fatalf("stageRun[%d] = %s, want %s", i, stageRun.ID, want[i])
		}
	}
}
