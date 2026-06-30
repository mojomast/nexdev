package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type DetourRecordListOptions struct {
	RunID         string
	TriggerTaskID string
}

func (s *Store) CreateDetourRecord(ctx context.Context, record *DetourRecord) error {
	if record == nil {
		return fmt.Errorf("detour record is required")
	}
	if record.ID == "" {
		return fmt.Errorf("detour record id is required")
	}
	if record.ProjectID == "" {
		return fmt.Errorf("project id is required")
	}
	if record.RunID == "" {
		return fmt.Errorf("run id is required")
	}
	if record.TriggerTaskID == "" {
		return fmt.Errorf("trigger task id is required")
	}
	if record.Reason == "" {
		return fmt.Errorf("detour reason is required")
	}
	if record.Source == "" {
		return fmt.Errorf("detour source is required")
	}
	if len(record.Result) == 0 {
		record.Result = json.RawMessage(`{}`)
	}
	if !json.Valid(record.Result) {
		return fmt.Errorf("detour result must be valid JSON")
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	} else {
		record.CreatedAt = record.CreatedAt.UTC()
	}

	return executeWithRetryContext(ctx, s.db, 25, 5, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO detour_records (id, project_id, run_id, trigger_task_id, reason, source, depth, result_json, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, record.ID, record.ProjectID, record.RunID, record.TriggerTaskID, record.Reason, record.Source, record.Depth, string(record.Result), formatTime(record.CreatedAt))
		if err != nil {
			return fmt.Errorf("failed to create detour record: %w", err)
		}
		return nil
	})
}

func (s *Store) ListDetourRecords(ctx context.Context, opts DetourRecordListOptions) ([]*DetourRecord, error) {
	if opts.RunID == "" {
		return nil, fmt.Errorf("run id is required")
	}
	query := `
		SELECT id, project_id, run_id, trigger_task_id, reason, source, depth, result_json, created_at
		FROM detour_records
		WHERE run_id = ?
	`
	args := []any{opts.RunID}
	if opts.TriggerTaskID != "" {
		query += ` AND trigger_task_id = ?`
		args = append(args, opts.TriggerTaskID)
	}
	query += ` ORDER BY created_at ASC, id ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list detour records: %w", err)
	}
	defer rows.Close()

	var records []*DetourRecord
	for rows.Next() {
		record, err := scanDetourRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read detour records: %w", err)
	}
	return records, nil
}

func scanDetourRecord(scanner rowScanner) (*DetourRecord, error) {
	var record DetourRecord
	var resultJSON string
	var createdAt string

	if err := scanner.Scan(&record.ID, &record.ProjectID, &record.RunID, &record.TriggerTaskID, &record.Reason, &record.Source, &record.Depth, &resultJSON, &createdAt); err != nil {
		return nil, err
	}
	record.Result = json.RawMessage(resultJSON)
	parsedCreatedAt, err := parseStoredTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse detour created_at: %w", err)
	}
	record.CreatedAt = parsedCreatedAt
	return &record, nil
}
