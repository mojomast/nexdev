package state

import (
	"context"
	"math"
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

func TestStoreSummarizeCostForRunAggregatesProviders(t *testing.T) {
	store := newEventTestStore(t)
	seedEventRun(t, store, "proj_cost_summary", "run_cost_summary")
	ctx := context.Background()
	firstCost := 0.01
	secondCost := 0.02
	thirdCost := 0.03
	records := []*CostRecord{
		{ID: "cost_sum_1", ProjectID: "proj_cost_summary", RunID: "run_cost_summary", Provider: "alpha", Model: "m1", PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, EstimatedUSD: &firstCost, LatencyMS: 100, Currency: "USD"},
		{ID: "cost_sum_2", ProjectID: "proj_cost_summary", RunID: "run_cost_summary", Provider: "alpha", Model: "m2", PromptTokens: 30, CompletionTokens: 15, TotalTokens: 45, EstimatedUSD: &secondCost, LatencyMS: 300, Currency: "USD"},
		{ID: "cost_sum_3", ProjectID: "proj_cost_summary", RunID: "run_cost_summary", Provider: "beta", Model: "m1", PromptTokens: 7, CompletionTokens: 8, TotalTokens: 15, EstimatedUSD: &thirdCost, LatencyMS: 50, Currency: "USD"},
	}
	for _, record := range records {
		if err := store.CreateCostRecord(ctx, record); err != nil {
			t.Fatalf("CreateCostRecord failed: %v", err)
		}
	}

	summary, err := store.SummarizeCostForRun(ctx, "run_cost_summary")
	if err != nil {
		t.Fatalf("SummarizeCostForRun failed: %v", err)
	}
	if summary.RunID != "run_cost_summary" || len(summary.Providers) != 2 || !almostEqual(summary.TotalCostUSD, 0.06) {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	alpha := summary.Providers[0]
	if alpha.Provider != "alpha" || alpha.InputTokens != 40 || alpha.OutputTokens != 20 || alpha.TotalTokens != 60 || alpha.CallCount != 2 || alpha.AverageLatencyMS != 200 || !almostEqual(alpha.TotalCostUSD, 0.03) {
		t.Fatalf("unexpected alpha summary: %#v", alpha)
	}
	beta := summary.Providers[1]
	if beta.Provider != "beta" || beta.InputTokens != 7 || beta.OutputTokens != 8 || beta.TotalTokens != 15 || beta.CallCount != 1 || beta.AverageLatencyMS != 50 || !almostEqual(beta.TotalCostUSD, 0.03) {
		t.Fatalf("unexpected beta summary: %#v", beta)
	}
}

func TestStoreSummarizeCostForRunZeroRows(t *testing.T) {
	store := newEventTestStore(t)
	seedEventRun(t, store, "proj_cost_empty", "run_cost_empty")

	summary, err := store.SummarizeCostForRun(context.Background(), "run_cost_empty")
	if err != nil {
		t.Fatalf("SummarizeCostForRun failed: %v", err)
	}
	if summary.RunID != "run_cost_empty" || summary.TotalCostUSD != 0 || len(summary.Providers) != 0 {
		t.Fatalf("expected empty summary, got %#v", summary)
	}
}

func almostEqual(got, want float64) bool {
	return math.Abs(got-want) < 0.0000001
}
