package state

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
)

func TestStoreNexdevBlockerRepositoryCreateResolveList(t *testing.T) {
	store := newRunArtifactTestStore(t)
	seedStateCRun(t, store, "proj_blockers", "run_blockers")
	ctx := context.Background()
	seedNexdevTask(t, store, "proj_blockers", "run_blockers", "T3.01", 1)

	firstAt := time.Date(2026, 6, 30, 11, 0, 0, 5, time.FixedZone("offset", 2*60*60))
	secondAt := time.Date(2026, 6, 30, 12, 0, 0, 6, time.UTC)
	if err := store.CreateNexdevBlocker(ctx, &NexdevBlocker{ID: "blk_second", ProjectID: "proj_blockers", RunID: "run_blockers", TaskID: "T3.01", Reason: "provider_error", Description: "second", Metadata: json.RawMessage(`{"retryable":true}`), CreatedAt: secondAt}); err != nil {
		t.Fatalf("CreateNexdevBlocker second failed: %v", err)
	}
	if err := store.CreateNexdevBlocker(ctx, &NexdevBlocker{ID: "blk_first", ProjectID: "proj_blockers", RunID: "run_blockers", TaskID: "T3.01", Reason: "missing_input", Description: "first", CreatedAt: firstAt}); err != nil {
		t.Fatalf("CreateNexdevBlocker first failed: %v", err)
	}

	resolvedAt := time.Date(2026, 6, 30, 13, 0, 0, 7, time.FixedZone("offset", -7*60*60))
	if err := store.ResolveNexdevBlocker(ctx, "blk_first", "operator supplied missing input", resolvedAt); err != nil {
		t.Fatalf("ResolveNexdevBlocker failed: %v", err)
	}

	blockers, err := store.ListNexdevBlockers(ctx, NexdevBlockerListOptions{RunID: "run_blockers"})
	if err != nil {
		t.Fatalf("ListNexdevBlockers failed: %v", err)
	}
	if len(blockers) != 2 || blockers[0].ID != "blk_first" || blockers[1].ID != "blk_second" {
		t.Fatalf("blocker order = %v", nexdevBlockerIDs(blockers))
	}
	if got, want := blockers[0].CreatedAt.Format(time.RFC3339Nano), firstAt.UTC().Format(time.RFC3339Nano); got != want {
		t.Fatalf("CreatedAt = %s, want %s", got, want)
	}
	if blockers[0].Status != NexdevBlockerStatusResolved || blockers[0].Resolution == "" || blockers[0].ResolvedAt == nil {
		t.Fatalf("resolved blocker mismatch: %+v", blockers[0])
	}
	if got, want := blockers[0].ResolvedAt.Format(time.RFC3339Nano), resolvedAt.UTC().Format(time.RFC3339Nano); got != want {
		t.Fatalf("ResolvedAt = %s, want %s", got, want)
	}
	if string(blockers[1].Metadata) != `{"retryable":true}` {
		t.Fatalf("metadata = %s", blockers[1].Metadata)
	}

	open, err := store.ListNexdevBlockers(ctx, NexdevBlockerListOptions{RunID: "run_blockers", TaskID: "T3.01", Status: NexdevBlockerStatusOpen})
	if err != nil {
		t.Fatalf("ListNexdevBlockers filtered failed: %v", err)
	}
	if len(open) != 1 || open[0].ID != "blk_second" {
		t.Fatalf("open blockers = %v", nexdevBlockerIDs(open))
	}
}

func TestStoreNexdevBlockerRepositoryConstraints(t *testing.T) {
	store := newRunArtifactTestStore(t)
	seedStateCRun(t, store, "proj_blocker_constraints", "run_blocker_constraints")
	ctx := context.Background()

	if err := store.CreateNexdevBlocker(ctx, &NexdevBlocker{ID: "blk_bad_json", ProjectID: "proj_blocker_constraints", RunID: "run_blocker_constraints", Reason: "bad", Description: "bad", Metadata: json.RawMessage(`{bad}`)}); err == nil {
		t.Fatal("CreateNexdevBlocker invalid JSON succeeded, want failure")
	}
	if err := store.CreateNexdevBlocker(ctx, &NexdevBlocker{ID: "blk_missing_run", ProjectID: "proj_blocker_constraints", RunID: "missing", Reason: "bad", Description: "bad"}); err == nil {
		t.Fatal("CreateNexdevBlocker missing run succeeded, want FK failure")
	}
	if err := store.CreateNexdevBlocker(ctx, &NexdevBlocker{ID: "blk_missing_task", ProjectID: "proj_blocker_constraints", RunID: "run_blocker_constraints", TaskID: "missing", Reason: "bad", Description: "bad"}); err == nil {
		t.Fatal("CreateNexdevBlocker missing task succeeded, want FK failure")
	}
	if err := store.ResolveNexdevBlocker(ctx, "missing", "resolution", time.Time{}); err == nil {
		t.Fatal("ResolveNexdevBlocker missing blocker succeeded, want failure")
	}
}

func seedNexdevTask(t *testing.T, store *Store, projectID, runID, taskID string, order int) {
	t.Helper()
	if err := store.CreateNexdevTask(context.Background(), &NexdevTask{ProjectID: projectID, RunID: runID, PlanOrder: order, Spec: contract.TaskSpec{ID: taskID, PhaseID: "phase_003", Title: taskID, AcceptanceCriteria: []string{"seeded"}}}); err != nil {
		t.Fatalf("seed Nexdev task failed: %v", err)
	}
}

func nexdevBlockerIDs(blockers []*NexdevBlocker) []string {
	ids := make([]string, 0, len(blockers))
	for _, blocker := range blockers {
		ids = append(ids, blocker.ID)
	}
	return ids
}
