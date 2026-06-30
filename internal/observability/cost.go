package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/safety"
	"github.com/mojomast/nexdev/internal/state"
)

type UsageStore interface {
	CreateCostRecord(ctx context.Context, record *state.CostRecord) error
	CreateAuditRecord(ctx context.Context, record *state.AuditRecord) error
}

type Price struct {
	InputPer1KUSD  float64
	OutputPer1KUSD float64
}

type UsageRecorderConfig struct {
	Store      UsageStore
	Logger     *slog.Logger
	Currency   string
	Prices     map[string]Price
	Now        func() time.Time
	NewID      func(prefix string) string
	ProjectID  string
	RunID      string
	Stage      string
	TaskID     string
	AuditCalls bool
}

type UsageRecorder struct {
	store      UsageStore
	logger     *slog.Logger
	currency   string
	prices     map[string]Price
	now        func() time.Time
	newID      func(prefix string) string
	defaults   Correlation
	auditCalls bool
}

var usageIDCounter atomic.Uint64

func NewUsageRecorder(cfg UsageRecorderConfig) *UsageRecorder {
	currency := cfg.Currency
	if currency == "" {
		currency = "USD"
	}
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	newID := cfg.NewID
	if newID == nil {
		newID = func(prefix string) string { return fmt.Sprintf("%s_%d", prefix, usageIDCounter.Add(1)) }
	}
	return &UsageRecorder{
		store:      cfg.Store,
		logger:     cfg.Logger,
		currency:   currency,
		prices:     clonePrices(cfg.Prices),
		now:        now,
		newID:      newID,
		defaults:   Correlation{ProjectID: cfg.ProjectID, RunID: cfg.RunID, Stage: cfg.Stage, TaskID: cfg.TaskID},
		auditCalls: cfg.AuditCalls,
	}
}

func (r *UsageRecorder) RecordStructuredCall(ctx context.Context, record provider.StructuredCallRecord) error {
	if r == nil {
		return nil
	}
	correlation := mergeCorrelation(r.defaults, CorrelationFromContext(ctx))
	metadata := map[string]any{
		"slot":              string(record.Slot),
		"attempts":          record.Attempts,
		"validation_errors": redactStringSlice(record.ValidationErrors),
	}
	if record.Error != "" {
		metadata["error"] = safety.RedactSecrets(record.Error)
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal usage metadata: %w", err)
	}

	var estimated *float64
	if price, ok := r.prices[priceKey(record.Provider, record.Model)]; ok {
		value := float64(record.PromptTokens)/1000*price.InputPer1KUSD + float64(record.CompletionTokens)/1000*price.OutputPer1KUSD
		estimated = &value
	}

	createdAt := r.now().UTC()
	if r.store != nil && correlation.ProjectID != "" {
		cost := &state.CostRecord{
			ID:               r.newID("cost"),
			ProjectID:        correlation.ProjectID,
			RunID:            correlation.RunID,
			Stage:            correlation.Stage,
			TaskID:           correlation.TaskID,
			Provider:         record.Provider,
			Model:            record.Model,
			PromptTokens:     record.PromptTokens,
			CompletionTokens: record.CompletionTokens,
			TotalTokens:      record.TotalTokens,
			EstimatedUSD:     estimated,
			Currency:         r.currency,
			RetryCount:       max(0, record.Attempts-1),
			LatencyMS:        max64(0, record.CompletedAt.Sub(record.StartedAt).Milliseconds()),
			Metadata:         metadataJSON,
			CreatedAt:        createdAt,
		}
		if err := r.store.CreateCostRecord(ctx, cost); err != nil {
			return err
		}
		if r.auditCalls {
			outcome := "success"
			if record.Error != "" {
				outcome = "failed"
			}
			audit := &state.AuditRecord{ID: r.newID("audit"), ProjectID: correlation.ProjectID, RunID: correlation.RunID, RequestID: correlation.RequestID, Actor: correlation.Actor, ActorRole: correlation.ActorRole, Source: defaultString(correlation.Source, "provider"), Action: "provider_call", ResourceType: "model", ResourceID: record.Provider + "/" + record.Model, Outcome: outcome, Details: metadataJSON, CreatedAt: createdAt}
			if err := r.store.CreateAuditRecord(ctx, audit); err != nil {
				return err
			}
		}
	}

	if r.logger != nil {
		attrs := []slog.Attr{Provider(record.Provider), Model(record.Model), Stage(correlation.Stage), TaskID(correlation.TaskID), RunID(correlation.RunID)}
		if record.Error != "" {
			r.logger.LogAttrs(ctx, slog.LevelWarn, "provider call recorded", append(attrs, slog.String("error", safety.RedactSecrets(record.Error)))...)
		} else {
			r.logger.LogAttrs(ctx, slog.LevelInfo, "provider call recorded", attrs...)
		}
	}
	return nil
}

func mergeCorrelation(defaults, override Correlation) Correlation {
	out := defaults
	if override.ProjectID != "" {
		out.ProjectID = override.ProjectID
	}
	if override.RunID != "" {
		out.RunID = override.RunID
	}
	if override.Stage != "" {
		out.Stage = override.Stage
	}
	if override.TaskID != "" {
		out.TaskID = override.TaskID
	}
	if override.RequestID != "" {
		out.RequestID = override.RequestID
	}
	if override.Source != "" {
		out.Source = override.Source
	}
	if override.Actor != "" {
		out.Actor = override.Actor
	}
	if override.ActorRole != "" {
		out.ActorRole = override.ActorRole
	}
	return out
}

func clonePrices(prices map[string]Price) map[string]Price {
	out := make(map[string]Price, len(prices))
	for key, value := range prices {
		out[key] = value
	}
	return out
}

func priceKey(providerName, model string) string { return providerName + "/" + model }

func redactStringSlice(values []string) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = safety.RedactSecrets(value)
	}
	return out
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
