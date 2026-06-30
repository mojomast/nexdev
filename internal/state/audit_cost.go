package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mojomast/nexdev/internal/safety"
)

type AuditListOptions struct {
	ProjectID string
	RunID     string
	Action    string
	Limit     int
}

type CostListOptions struct {
	ProjectID string
	RunID     string
	Provider  string
	Limit     int
}

func (s *Store) CreateAuditRecord(ctx context.Context, record *AuditRecord) error {
	if record == nil {
		return fmt.Errorf("audit record is required")
	}
	if record.ID == "" {
		return fmt.Errorf("audit record id is required")
	}
	if record.ProjectID == "" {
		return fmt.Errorf("audit project id is required")
	}
	if record.Source == "" {
		return fmt.Errorf("audit source is required")
	}
	if record.Action == "" {
		return fmt.Errorf("audit action is required")
	}
	if record.Outcome == "" {
		return fmt.Errorf("audit outcome is required")
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	} else {
		record.CreatedAt = record.CreatedAt.UTC()
	}
	details, err := scrubRawJSONObject(record.Details)
	if err != nil {
		return fmt.Errorf("failed to scrub audit details: %w", err)
	}

	record.RequestID = safety.RedactSecrets(record.RequestID)
	record.Actor = safety.RedactSecrets(record.Actor)
	record.ActorRole = safety.RedactSecrets(record.ActorRole)
	record.Source = safety.RedactSecrets(record.Source)
	record.Action = safety.RedactSecrets(record.Action)
	record.ResourceType = safety.RedactSecrets(record.ResourceType)
	record.ResourceID = safety.RedactSecrets(record.ResourceID)
	record.Outcome = safety.RedactSecrets(record.Outcome)
	record.Details = json.RawMessage(details)

	return executeWithRetryContext(ctx, s.db, 25, 5, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO audit_log (id, project_id, run_id, request_id, actor, actor_role, source, action, resource_type, resource_id, outcome, details_json, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, record.ID, record.ProjectID, nullableString(record.RunID), nullableString(record.RequestID), nullableString(record.Actor), nullableString(record.ActorRole), record.Source, record.Action, nullableString(record.ResourceType), nullableString(record.ResourceID), record.Outcome, details, formatTime(record.CreatedAt))
		if err != nil {
			return fmt.Errorf("failed to create audit record: %w", err)
		}
		return nil
	})
}

func (s *Store) ListAuditRecords(ctx context.Context, opts AuditListOptions) ([]*AuditRecord, error) {
	if opts.ProjectID == "" && opts.RunID == "" {
		return nil, fmt.Errorf("project id or run id is required")
	}
	query := `
		SELECT id, project_id, run_id, request_id, actor, actor_role, source, action, resource_type, resource_id, outcome, details_json, created_at
		FROM audit_log
		WHERE 1 = 1
	`
	var args []any
	if opts.ProjectID != "" {
		query += ` AND project_id = ?`
		args = append(args, opts.ProjectID)
	}
	if opts.RunID != "" {
		query += ` AND run_id = ?`
		args = append(args, opts.RunID)
	}
	if opts.Action != "" {
		query += ` AND action = ?`
		args = append(args, opts.Action)
	}
	query += ` ORDER BY created_at ASC, id ASC`
	if opts.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, opts.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit records: %w", err)
	}
	defer rows.Close()

	var records []*AuditRecord
	for rows.Next() {
		record, err := scanAuditRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read audit records: %w", err)
	}
	return records, nil
}

func (s *Store) CreateCostRecord(ctx context.Context, record *CostRecord) error {
	if record == nil {
		return fmt.Errorf("cost record is required")
	}
	if record.ID == "" {
		return fmt.Errorf("cost record id is required")
	}
	if record.ProjectID == "" {
		return fmt.Errorf("cost project id is required")
	}
	if record.Provider == "" {
		return fmt.Errorf("cost provider is required")
	}
	if record.Model == "" {
		return fmt.Errorf("cost model is required")
	}
	if record.Currency == "" {
		record.Currency = "USD"
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	} else {
		record.CreatedAt = record.CreatedAt.UTC()
	}
	metadata, err := scrubRawJSONObject(record.Metadata)
	if err != nil {
		return fmt.Errorf("failed to scrub cost metadata: %w", err)
	}

	record.Stage = safety.RedactSecrets(record.Stage)
	record.TaskID = safety.RedactSecrets(record.TaskID)
	record.Provider = safety.RedactSecrets(record.Provider)
	record.Model = safety.RedactSecrets(record.Model)
	record.Currency = safety.RedactSecrets(record.Currency)
	record.Metadata = json.RawMessage(metadata)

	return executeWithRetryContext(ctx, s.db, 25, 5, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO cost_ledger (id, project_id, run_id, stage, task_id, provider, model, prompt_tokens, completion_tokens, total_tokens, estimated_usd, currency, retry_count, latency_ms, metadata_json, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, record.ID, record.ProjectID, nullableString(record.RunID), nullableString(record.Stage), nullableString(record.TaskID), record.Provider, record.Model, record.PromptTokens, record.CompletionTokens, record.TotalTokens, nullableFloat(record.EstimatedUSD), record.Currency, record.RetryCount, record.LatencyMS, metadata, formatTime(record.CreatedAt))
		if err != nil {
			return fmt.Errorf("failed to create cost record: %w", err)
		}
		return nil
	})
}

func (s *Store) ListCostRecords(ctx context.Context, opts CostListOptions) ([]*CostRecord, error) {
	if opts.ProjectID == "" && opts.RunID == "" {
		return nil, fmt.Errorf("project id or run id is required")
	}
	query := `
		SELECT id, project_id, run_id, stage, task_id, provider, model, prompt_tokens, completion_tokens, total_tokens, estimated_usd, currency, retry_count, latency_ms, metadata_json, created_at
		FROM cost_ledger
		WHERE 1 = 1
	`
	var args []any
	if opts.ProjectID != "" {
		query += ` AND project_id = ?`
		args = append(args, opts.ProjectID)
	}
	if opts.RunID != "" {
		query += ` AND run_id = ?`
		args = append(args, opts.RunID)
	}
	if opts.Provider != "" {
		query += ` AND provider = ?`
		args = append(args, opts.Provider)
	}
	query += ` ORDER BY created_at ASC, id ASC`
	if opts.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, opts.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list cost records: %w", err)
	}
	defer rows.Close()

	var records []*CostRecord
	for rows.Next() {
		record, err := scanCostRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read cost records: %w", err)
	}
	return records, nil
}

func scanAuditRecord(scanner interface{ Scan(dest ...any) error }) (*AuditRecord, error) {
	var record AuditRecord
	var runID, requestID, actor, actorRole, resourceType, resourceID sql.NullString
	var details string
	var createdAt string
	if err := scanner.Scan(&record.ID, &record.ProjectID, &runID, &requestID, &actor, &actorRole, &record.Source, &record.Action, &resourceType, &resourceID, &record.Outcome, &details, &createdAt); err != nil {
		return nil, err
	}
	parsedAt, err := parseStoredTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse audit timestamp: %w", err)
	}
	record.RunID = nullStringValue(runID)
	record.RequestID = nullStringValue(requestID)
	record.Actor = nullStringValue(actor)
	record.ActorRole = nullStringValue(actorRole)
	record.ResourceType = nullStringValue(resourceType)
	record.ResourceID = nullStringValue(resourceID)
	record.Details = json.RawMessage(details)
	record.CreatedAt = parsedAt
	return &record, nil
}

func scanCostRecord(scanner interface{ Scan(dest ...any) error }) (*CostRecord, error) {
	var record CostRecord
	var runID, stage, taskID sql.NullString
	var estimated sql.NullFloat64
	var metadata string
	var createdAt string
	if err := scanner.Scan(&record.ID, &record.ProjectID, &runID, &stage, &taskID, &record.Provider, &record.Model, &record.PromptTokens, &record.CompletionTokens, &record.TotalTokens, &estimated, &record.Currency, &record.RetryCount, &record.LatencyMS, &metadata, &createdAt); err != nil {
		return nil, err
	}
	parsedAt, err := parseStoredTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cost timestamp: %w", err)
	}
	record.RunID = nullStringValue(runID)
	record.Stage = nullStringValue(stage)
	record.TaskID = nullStringValue(taskID)
	if estimated.Valid {
		record.EstimatedUSD = &estimated.Float64
	}
	record.Metadata = json.RawMessage(metadata)
	record.CreatedAt = parsedAt
	return &record, nil
}

func scrubRawJSONObject(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return `{}`, nil
	}
	if !json.Valid(raw) {
		return "", fmt.Errorf("JSON must be valid")
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	redacted, err := json.Marshal(scrubJSONValue(value))
	if err != nil {
		return "", err
	}
	return string(redacted), nil
}

func scrubJSONValue(value any) any {
	switch typed := value.(type) {
	case string:
		return safety.RedactSecrets(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = scrubJSONValue(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = scrubJSONValue(item)
		}
		return out
	default:
		return value
	}
}

func nullableFloat(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
