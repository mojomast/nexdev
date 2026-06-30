package observability

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
)

func TestUsageRecorderPersistsCostAndAuditRecords(t *testing.T) {
	store := newUsageTestStore(t)
	seedUsageRun(t, store, "proj_usage", "run_usage")
	now := time.Date(2026, 6, 30, 14, 0, 0, 0, time.UTC)
	recorder := NewUsageRecorder(UsageRecorderConfig{
		Store:      store,
		ProjectID:  "proj_usage",
		RunID:      "run_usage",
		Stage:      "design",
		Currency:   "USD",
		Prices:     map[string]Price{"fake/fake-model": {InputPer1KUSD: 0.01, OutputPer1KUSD: 0.02}},
		Now:        func() time.Time { return now },
		NewID:      func(prefix string) string { return prefix + "_fixed" },
		AuditCalls: true,
	})

	ctx := ContextWithCorrelation(context.Background(), Correlation{TaskID: "T1.01", RequestID: "req_1", Source: "api", Actor: "operator"})
	err := recorder.RecordStructuredCall(ctx, provider.StructuredCallRecord{
		Slot:             provider.SlotDesign,
		Provider:         "fake",
		Model:            "fake-model",
		Attempts:         2,
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
		ValidationErrors: []string{"token=sk-1234567890abcdef"},
		StartedAt:        now.Add(-250 * time.Millisecond),
		CompletedAt:      now,
	})
	if err != nil {
		t.Fatalf("RecordStructuredCall failed: %v", err)
	}

	costs, err := store.ListCostRecords(context.Background(), state.CostListOptions{RunID: "run_usage"})
	if err != nil {
		t.Fatalf("ListCostRecords failed: %v", err)
	}
	if len(costs) != 1 {
		t.Fatalf("cost count = %d, want 1", len(costs))
	}
	if costs[0].RetryCount != 1 || costs[0].LatencyMS != 250 || costs[0].EstimatedUSD == nil || *costs[0].EstimatedUSD != 0.02 {
		t.Fatalf("cost record mismatch: %+v", costs[0])
	}
	if strings.Contains(string(costs[0].Metadata), "sk-1234567890abcdef") {
		t.Fatalf("cost metadata leaked secret: %s", costs[0].Metadata)
	}

	audits, err := store.ListAuditRecords(context.Background(), state.AuditListOptions{RunID: "run_usage"})
	if err != nil {
		t.Fatalf("ListAuditRecords failed: %v", err)
	}
	if len(audits) != 1 || audits[0].Action != "provider_call" || audits[0].Outcome != "success" {
		t.Fatalf("audit record mismatch: %+v", audits)
	}
}

func TestConfigureOTelDisabledByDefault(t *testing.T) {
	shutdown, err := ConfigureOTel(OTelConfig{})
	if err != nil {
		t.Fatalf("ConfigureOTel disabled returned error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("disabled OTel returned nil shutdown")
	}
	if err := shutdown(); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
}

func TestConfigureOTelEnabledRequiresEndpointWithoutNetwork(t *testing.T) {
	_, err := ConfigureOTel(OTelConfig{Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "endpoint") {
		t.Fatalf("expected endpoint validation error, got %v", err)
	}
}

func newUsageTestStore(t *testing.T) *state.Store {
	t.Helper()
	store, err := state.NewStore(t.TempDir() + "/usage.db")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func seedUsageRun(t *testing.T, store *state.Store, projectID, runID string) {
	t.Helper()
	_, err := store.DB().Exec(`
		INSERT OR IGNORE INTO projects (id, name, created_at, current_stage)
		VALUES (?, ?, ?, 'init')
	`, projectID, projectID, time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("seed project failed: %v", err)
	}
	_, err = store.DB().Exec(`
		INSERT INTO runs (id, project_id, status, started_at)
		VALUES (?, ?, 'running', ?)
	`, runID, projectID, time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano))
	if err != nil {
		t.Fatalf("seed run failed: %v", err)
	}
}
