package state

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestStorePlanEditRepositoryCreateListByRun(t *testing.T) {
	store := newRunArtifactTestStore(t)
	seedStateCRun(t, store, "proj_plan_edits", "run_plan_edits")
	ctx := context.Background()

	firstAt := time.Date(2026, 6, 30, 15, 0, 0, 111, time.FixedZone("offset", 7*60*60))
	secondAt := time.Date(2026, 6, 30, 16, 0, 0, 222, time.UTC)
	if err := store.CreatePlanEditEvent(ctx, &PlanEditEvent{ID: "edit_second", ProjectID: "proj_plan_edits", RunID: "run_plan_edits", PlanVersionBefore: 2, PlanVersionAfter: 3, EditType: "delete_task", TargetID: "T1.02", Patch: json.RawMessage(`{"op":"remove","path":"/tasks/1"}`), Actor: "admin", CreatedAt: secondAt}); err != nil {
		t.Fatalf("CreatePlanEditEvent second failed: %v", err)
	}
	if err := store.CreatePlanEditEvent(ctx, &PlanEditEvent{ID: "edit_first", ProjectID: "proj_plan_edits", RunID: "run_plan_edits", PlanVersionBefore: 1, PlanVersionAfter: 2, EditType: "rename_task", TargetID: "T1.01", Patch: json.RawMessage(`{"title":"New title"}`), Actor: "operator", CreatedAt: firstAt}); err != nil {
		t.Fatalf("CreatePlanEditEvent first failed: %v", err)
	}

	events, err := store.ListPlanEditEventsByRun(ctx, "run_plan_edits")
	if err != nil {
		t.Fatalf("ListPlanEditEventsByRun failed: %v", err)
	}
	if len(events) != 2 || events[0].ID != "edit_first" || events[1].ID != "edit_second" {
		t.Fatalf("plan edit order = %v", planEditEventIDs(events))
	}
	if got, want := events[0].CreatedAt.Format(time.RFC3339Nano), firstAt.UTC().Format(time.RFC3339Nano); got != want {
		t.Fatalf("CreatedAt = %s, want %s", got, want)
	}
	if events[0].PlanVersionBefore != 1 || events[0].PlanVersionAfter != 2 || events[0].EditType != "rename_task" || events[0].TargetID != "T1.01" || events[0].Actor != "operator" {
		t.Fatalf("plan edit event mismatch: %+v", events[0])
	}
	if string(events[0].Patch) != `{"title":"New title"}` {
		t.Fatalf("patch round-trip = %s", events[0].Patch)
	}
}

func TestStorePlanEditRepositoryForeignKeysAndJSONValidation(t *testing.T) {
	store := newRunArtifactTestStore(t)
	ctx := context.Background()
	if err := store.CreatePlanEditEvent(ctx, &PlanEditEvent{ID: "edit_bad_json", ProjectID: "proj", RunID: "run", PlanVersionBefore: 1, PlanVersionAfter: 2, EditType: "rename_task", Patch: json.RawMessage(`{bad}`), Actor: "admin"}); err == nil {
		t.Fatal("CreatePlanEditEvent invalid JSON succeeded, want failure")
	}
	if err := store.CreatePlanEditEvent(ctx, &PlanEditEvent{ID: "edit_missing", ProjectID: "missing", RunID: "missing", PlanVersionBefore: 1, PlanVersionAfter: 2, EditType: "rename_task", Actor: "admin"}); err == nil {
		t.Fatal("CreatePlanEditEvent with missing FK succeeded, want failure")
	}
}

func planEditEventIDs(events []*PlanEditEvent) []string {
	ids := make([]string, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}
	return ids
}
