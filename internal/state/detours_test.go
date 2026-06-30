package state

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestStoreDetourRepositoryCreateListByRunAndTrigger(t *testing.T) {
	store := newRunArtifactTestStore(t)
	seedStateCRun(t, store, "proj_detours", "run_detours")
	ctx := context.Background()

	firstAt := time.Date(2026, 6, 30, 9, 30, 0, 11, time.FixedZone("offset", 2*60*60))
	secondAt := time.Date(2026, 6, 30, 10, 30, 0, 12, time.UTC)
	if err := store.CreateDetourRecord(ctx, &DetourRecord{ID: "detour_second", ProjectID: "proj_detours", RunID: "run_detours", TriggerTaskID: "T1.02", Reason: "second", Source: "operator_manual", Depth: 2, Result: json.RawMessage(`{"new_tasks":[{"id":"D2.01"}]}`), CreatedAt: secondAt}); err != nil {
		t.Fatalf("CreateDetourRecord second failed: %v", err)
	}
	if err := store.CreateDetourRecord(ctx, &DetourRecord{ID: "detour_first", ProjectID: "proj_detours", RunID: "run_detours", TriggerTaskID: "T1.01", Reason: "blocked", Source: "blocker_auto", Depth: 1, Result: json.RawMessage(`{"new_tasks":[{"id":"D1.01"}]}`), CreatedAt: firstAt}); err != nil {
		t.Fatalf("CreateDetourRecord first failed: %v", err)
	}

	records, err := store.ListDetourRecords(ctx, DetourRecordListOptions{RunID: "run_detours"})
	if err != nil {
		t.Fatalf("ListDetourRecords by run failed: %v", err)
	}
	if len(records) != 2 || records[0].ID != "detour_first" || records[1].ID != "detour_second" {
		t.Fatalf("detour order = %v", detourRecordIDs(records))
	}
	if got, want := records[0].CreatedAt.Format(time.RFC3339Nano), firstAt.UTC().Format(time.RFC3339Nano); got != want {
		t.Fatalf("CreatedAt = %s, want %s", got, want)
	}
	if string(records[0].Result) != `{"new_tasks":[{"id":"D1.01"}]}` || records[0].Source != "blocker_auto" || records[0].Depth != 1 {
		t.Fatalf("detour record mismatch: %+v", records[0])
	}

	triggerRecords, err := store.ListDetourRecords(ctx, DetourRecordListOptions{RunID: "run_detours", TriggerTaskID: "T1.02"})
	if err != nil {
		t.Fatalf("ListDetourRecords by trigger failed: %v", err)
	}
	if len(triggerRecords) != 1 || triggerRecords[0].ID != "detour_second" {
		t.Fatalf("trigger detour records = %v", detourRecordIDs(triggerRecords))
	}
}

func TestStoreDetourRepositoryForeignKeysAndJSONValidation(t *testing.T) {
	store := newRunArtifactTestStore(t)
	ctx := context.Background()
	if err := store.CreateDetourRecord(ctx, &DetourRecord{ID: "detour_bad_json", ProjectID: "proj", RunID: "run", TriggerTaskID: "T1.01", Reason: "bad", Source: "operator_manual", Result: json.RawMessage(`{bad}`)}); err == nil {
		t.Fatal("CreateDetourRecord invalid JSON succeeded, want failure")
	}
	if err := store.CreateDetourRecord(ctx, &DetourRecord{ID: "detour_missing", ProjectID: "missing", RunID: "missing", TriggerTaskID: "T1.01", Reason: "blocked", Source: "blocker_auto"}); err == nil {
		t.Fatal("CreateDetourRecord with missing FK succeeded, want failure")
	}
}

func detourRecordIDs(records []*DetourRecord) []string {
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	return ids
}
