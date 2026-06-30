package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type NexdevBlockerListOptions struct {
	RunID  string
	TaskID string
	Status string
}

func (s *Store) CreateNexdevBlocker(ctx context.Context, blocker *NexdevBlocker) error {
	if blocker == nil {
		return fmt.Errorf("blocker is required")
	}
	if blocker.ID == "" {
		return fmt.Errorf("blocker id is required")
	}
	if blocker.ProjectID == "" {
		return fmt.Errorf("project id is required")
	}
	if blocker.RunID == "" {
		return fmt.Errorf("run id is required")
	}
	if blocker.Reason == "" {
		return fmt.Errorf("blocker reason is required")
	}
	if blocker.Description == "" {
		return fmt.Errorf("blocker description is required")
	}
	if blocker.Status == "" {
		blocker.Status = NexdevBlockerStatusOpen
	}
	if len(blocker.Metadata) == 0 {
		blocker.Metadata = json.RawMessage(`{}`)
	}
	if !json.Valid(blocker.Metadata) {
		return fmt.Errorf("blocker metadata must be valid JSON")
	}
	if blocker.CreatedAt.IsZero() {
		blocker.CreatedAt = time.Now().UTC()
	} else {
		blocker.CreatedAt = blocker.CreatedAt.UTC()
	}
	if blocker.ResolvedAt != nil {
		resolvedAt := blocker.ResolvedAt.UTC()
		blocker.ResolvedAt = &resolvedAt
	}

	return executeWithRetryContext(ctx, s.db, 25, 5, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO nexdev_blockers (id, project_id, run_id, task_id, reason, description, status, resolution, metadata_json, created_at, resolved_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, blocker.ID, blocker.ProjectID, blocker.RunID, nullableString(blocker.TaskID), blocker.Reason, blocker.Description,
			blocker.Status, nullableString(blocker.Resolution), string(blocker.Metadata), formatTime(blocker.CreatedAt), formatOptionalTime(blocker.ResolvedAt))
		if err != nil {
			return fmt.Errorf("failed to create Nexdev blocker: %w", err)
		}
		return nil
	})
}

func (s *Store) ListNexdevBlockers(ctx context.Context, opts NexdevBlockerListOptions) ([]*NexdevBlocker, error) {
	if opts.RunID == "" {
		return nil, fmt.Errorf("run id is required")
	}
	query := selectNexdevBlockerSQL() + ` WHERE run_id = ?`
	args := []any{opts.RunID}
	if opts.TaskID != "" {
		query += ` AND task_id = ?`
		args = append(args, opts.TaskID)
	}
	if opts.Status != "" {
		query += ` AND status = ?`
		args = append(args, opts.Status)
	}
	query += ` ORDER BY created_at ASC, id ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list Nexdev blockers: %w", err)
	}
	defer rows.Close()

	var blockers []*NexdevBlocker
	for rows.Next() {
		blocker, err := scanNexdevBlocker(rows)
		if err != nil {
			return nil, err
		}
		blockers = append(blockers, blocker)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read Nexdev blockers: %w", err)
	}
	return blockers, nil
}

func (s *Store) ResolveNexdevBlocker(ctx context.Context, blockerID, resolution string, resolvedAt time.Time) error {
	if blockerID == "" {
		return fmt.Errorf("blocker id is required")
	}
	if resolution == "" {
		return fmt.Errorf("blocker resolution is required")
	}
	if resolvedAt.IsZero() {
		resolvedAt = time.Now().UTC()
	}
	return updateOne(ctx, s.db, `UPDATE nexdev_blockers SET status = ?, resolution = ?, resolved_at = ? WHERE id = ?`, NexdevBlockerStatusResolved, resolution, formatTime(resolvedAt), blockerID)
}

func selectNexdevBlockerSQL() string {
	return `SELECT id, project_id, run_id, task_id, reason, description, status, resolution, metadata_json, created_at, resolved_at FROM nexdev_blockers`
}

func scanNexdevBlocker(scanner rowScanner) (*NexdevBlocker, error) {
	var blocker NexdevBlocker
	var taskID, resolution, resolvedAt sql.NullString
	var metadataJSON, createdAt string
	if err := scanner.Scan(&blocker.ID, &blocker.ProjectID, &blocker.RunID, &taskID, &blocker.Reason, &blocker.Description,
		&blocker.Status, &resolution, &metadataJSON, &createdAt, &resolvedAt); err != nil {
		return nil, err
	}
	if taskID.Valid {
		blocker.TaskID = taskID.String
	}
	if resolution.Valid {
		blocker.Resolution = resolution.String
	}
	if !json.Valid([]byte(metadataJSON)) {
		return nil, fmt.Errorf("blocker metadata is invalid JSON")
	}
	blocker.Metadata = json.RawMessage(metadataJSON)
	parsedCreatedAt, err := parseStoredTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse blocker created_at: %w", err)
	}
	blocker.CreatedAt = parsedCreatedAt
	blocker.ResolvedAt, err = parseOptionalStoredTime(resolvedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse blocker resolved_at: %w", err)
	}
	return &blocker, nil
}
