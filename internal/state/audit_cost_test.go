package state

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestStoreAuditRecordsAreDurableAndRedacted(t *testing.T) {
	store := newEventTestStore(t)
	seedEventRun(t, store, "proj_audit", "run_audit")

	createdAt := time.Date(2026, 6, 30, 13, 0, 0, 0, time.UTC)
	err := store.CreateAuditRecord(context.Background(), &AuditRecord{
		ID:           "audit_1",
		ProjectID:    "proj_audit",
		RunID:        "run_audit",
		RequestID:    "Bearer abc.def-ghi",
		Actor:        "operator api_key=sk-1234567890abcdef",
		ActorRole:    "operator",
		Source:       "api",
		Action:       "pause",
		ResourceType: "run",
		ResourceID:   "run_audit",
		Outcome:      "success",
		Details:      []byte(`{"reason":"password=hunter2"}`),
		CreatedAt:    createdAt,
	})
	if err != nil {
		t.Fatalf("CreateAuditRecord failed: %v", err)
	}

	records, err := store.ListAuditRecords(context.Background(), AuditListOptions{ProjectID: "proj_audit"})
	if err != nil {
		t.Fatalf("ListAuditRecords failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}
	record := records[0]
	joined := record.RequestID + record.Actor + string(record.Details)
	if strings.Contains(joined, "abc.def-ghi") || strings.Contains(joined, "sk-1234567890abcdef") || strings.Contains(joined, "hunter2") {
		t.Fatalf("audit record leaked secret: %+v details=%s", record, record.Details)
	}
	if !strings.Contains(joined, "[REDACTED]") {
		t.Fatalf("audit record missing redaction marker: %+v details=%s", record, record.Details)
	}
	if got := record.CreatedAt.Format(time.RFC3339Nano); got != createdAt.Format(time.RFC3339Nano) {
		t.Fatalf("created_at = %s, want %s", got, createdAt.Format(time.RFC3339Nano))
	}
}

func TestStoreCostRecordsAreDurableAndRedacted(t *testing.T) {
	store := newEventTestStore(t)
	seedEventRun(t, store, "proj_cost", "run_cost")
	estimated := 0.0123

	err := store.CreateCostRecord(context.Background(), &CostRecord{
		ID:               "cost_1",
		ProjectID:        "proj_cost",
		RunID:            "run_cost",
		Stage:            "design",
		TaskID:           "T1.01",
		Provider:         "fake",
		Model:            "fake-model",
		PromptTokens:     7,
		CompletionTokens: 11,
		TotalTokens:      18,
		EstimatedUSD:     &estimated,
		Currency:         "USD",
		RetryCount:       2,
		LatencyMS:        123,
		Metadata:         []byte(`{"error":"Bearer abc.def-secret"}`),
	})
	if err != nil {
		t.Fatalf("CreateCostRecord failed: %v", err)
	}

	records, err := store.ListCostRecords(context.Background(), CostListOptions{RunID: "run_cost"})
	if err != nil {
		t.Fatalf("ListCostRecords failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}
	record := records[0]
	if record.PromptTokens != 7 || record.CompletionTokens != 11 || record.TotalTokens != 18 || record.RetryCount != 2 || record.LatencyMS != 123 {
		t.Fatalf("cost counters mismatch: %+v", record)
	}
	if record.EstimatedUSD == nil || *record.EstimatedUSD != estimated {
		t.Fatalf("estimated cost mismatch: %+v", record.EstimatedUSD)
	}
	if strings.Contains(string(record.Metadata), "abc.def-secret") || !strings.Contains(string(record.Metadata), "Bearer [REDACTED]") {
		t.Fatalf("cost metadata was not redacted: %s", record.Metadata)
	}
}
