package state

import (
	"context"
	"testing"
	"time"
)

func TestStoreNavigationRepositoryAppendListByProjectAndRun(t *testing.T) {
	store := newRunArtifactTestStore(t)
	seedStateCRun(t, store, "proj_navigation", "run_navigation")
	ctx := context.Background()

	firstAt := time.Date(2026, 6, 30, 7, 0, 0, 1, time.FixedZone("offset", 3*60*60))
	secondAt := time.Date(2026, 6, 30, 8, 0, 0, 2, time.UTC)
	if err := store.AppendNavigationEvent(ctx, &NavigationEvent{ID: "nav_second", ProjectID: "proj_navigation", RunID: "run_navigation", FromStage: "interview", ToStage: "design", Reason: "advance", Actor: "operator", CreatedAt: secondAt}); err != nil {
		t.Fatalf("AppendNavigationEvent second failed: %v", err)
	}
	if err := store.AppendNavigationEvent(ctx, &NavigationEvent{ID: "nav_first", ProjectID: "proj_navigation", FromStage: "init", ToStage: "repo_analyze", Reason: "start", Actor: "system", CreatedAt: firstAt}); err != nil {
		t.Fatalf("AppendNavigationEvent first failed: %v", err)
	}

	events, err := store.ListNavigationEvents(ctx, NavigationEventListOptions{ProjectID: "proj_navigation"})
	if err != nil {
		t.Fatalf("ListNavigationEvents by project failed: %v", err)
	}
	if len(events) != 2 || events[0].ID != "nav_first" || events[1].ID != "nav_second" {
		t.Fatalf("navigation order = %v", navigationEventIDs(events))
	}
	if got, want := events[0].CreatedAt.Format(time.RFC3339Nano), firstAt.UTC().Format(time.RFC3339Nano); got != want {
		t.Fatalf("CreatedAt = %s, want %s", got, want)
	}
	if events[0].FromStage != "init" || events[0].ToStage != "repo_analyze" || events[0].Reason != "start" || events[0].Actor != "system" {
		t.Fatalf("navigation event mismatch: %+v", events[0])
	}

	runEvents, err := store.ListNavigationEvents(ctx, NavigationEventListOptions{ProjectID: "proj_navigation", RunID: "run_navigation"})
	if err != nil {
		t.Fatalf("ListNavigationEvents by run failed: %v", err)
	}
	if len(runEvents) != 1 || runEvents[0].ID != "nav_second" {
		t.Fatalf("run navigation events = %v", navigationEventIDs(runEvents))
	}
}

func TestStoreNavigationRepositoryForeignKeys(t *testing.T) {
	store := newRunArtifactTestStore(t)
	if err := store.AppendNavigationEvent(context.Background(), &NavigationEvent{ID: "nav_missing", ProjectID: "missing", ToStage: "design", Actor: "operator"}); err == nil {
		t.Fatal("AppendNavigationEvent with missing project succeeded, want failure")
	}
}

func navigationEventIDs(events []*NavigationEvent) []string {
	ids := make([]string, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}
	return ids
}
