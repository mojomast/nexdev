package state

import (
	"context"
	"testing"
	"time"
)

func TestStoreSteeringRepositoryAppendListByRunAndTask(t *testing.T) {
	store := newRunArtifactTestStore(t)
	seedStateCRun(t, store, "proj_steering", "run_steering")
	ctx := context.Background()

	firstAt := time.Date(2026, 6, 30, 9, 0, 0, 1, time.FixedZone("offset", 4*60*60))
	secondAt := time.Date(2026, 6, 30, 10, 0, 0, 2, time.UTC)
	if err := store.AppendSteeringEvent(ctx, &SteeringEvent{ID: "steer_second", ProjectID: "proj_steering", RunID: "run_steering", TaskID: "T1.02", Message: "do later", Summary: "later", Source: "api", CreatedByRole: "operator", CreatedAt: secondAt}); err != nil {
		t.Fatalf("AppendSteeringEvent second failed: %v", err)
	}
	if err := store.AppendSteeringEvent(ctx, &SteeringEvent{ID: "steer_first", ProjectID: "proj_steering", RunID: "run_steering", TaskID: "T1.01", Message: "use small diff", Summary: "small", Source: "cli", CreatedByRole: "admin", CreatedAt: firstAt}); err != nil {
		t.Fatalf("AppendSteeringEvent first failed: %v", err)
	}

	events, err := store.ListSteeringEvents(ctx, SteeringListOptions{RunID: "run_steering"})
	if err != nil {
		t.Fatalf("ListSteeringEvents by run failed: %v", err)
	}
	if len(events) != 2 || events[0].ID != "steer_first" || events[1].ID != "steer_second" {
		t.Fatalf("steering order = %v", steeringEventIDs(events))
	}
	if got, want := events[0].CreatedAt.Format(time.RFC3339Nano), firstAt.UTC().Format(time.RFC3339Nano); got != want {
		t.Fatalf("CreatedAt = %s, want %s", got, want)
	}
	if events[0].Message != "use small diff" || events[0].Summary != "small" || events[0].Source != "cli" || events[0].CreatedByRole != "admin" {
		t.Fatalf("steering event mismatch: %+v", events[0])
	}

	taskEvents, err := store.ListSteeringEvents(ctx, SteeringListOptions{RunID: "run_steering", TaskID: "T1.02"})
	if err != nil {
		t.Fatalf("ListSteeringEvents by task failed: %v", err)
	}
	if len(taskEvents) != 1 || taskEvents[0].ID != "steer_second" {
		t.Fatalf("task steering events = %v", steeringEventIDs(taskEvents))
	}
}

func TestStoreSteeringRepositoryForeignKeys(t *testing.T) {
	store := newRunArtifactTestStore(t)
	if err := store.AppendSteeringEvent(context.Background(), &SteeringEvent{ID: "steer_missing", ProjectID: "missing", RunID: "missing", Message: "msg", Source: "api"}); err == nil {
		t.Fatal("AppendSteeringEvent with missing FK succeeded, want failure")
	}
}

func steeringEventIDs(events []*SteeringEvent) []string {
	ids := make([]string, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}
	return ids
}
