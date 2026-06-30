package state

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type SteeringListOptions struct {
	RunID  string
	TaskID string
}

func (s *Store) AppendSteeringEvent(ctx context.Context, event *SteeringEvent) error {
	if event == nil {
		return fmt.Errorf("steering event is required")
	}
	if event.ID == "" {
		return fmt.Errorf("steering event id is required")
	}
	if event.ProjectID == "" {
		return fmt.Errorf("project id is required")
	}
	if event.RunID == "" {
		return fmt.Errorf("run id is required")
	}
	if event.Message == "" {
		return fmt.Errorf("steering message is required")
	}
	if event.Source == "" {
		return fmt.Errorf("steering source is required")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	} else {
		event.CreatedAt = event.CreatedAt.UTC()
	}

	return executeWithRetryContext(ctx, s.db, 25, 5, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO steering_events (id, project_id, run_id, task_id, message, summary, source, created_by_role, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, event.ID, event.ProjectID, event.RunID, nullableString(event.TaskID), event.Message, nullableString(event.Summary), event.Source, nullableString(event.CreatedByRole), formatTime(event.CreatedAt))
		if err != nil {
			return fmt.Errorf("failed to append steering event: %w", err)
		}
		return nil
	})
}

func (s *Store) ListSteeringEvents(ctx context.Context, opts SteeringListOptions) ([]*SteeringEvent, error) {
	if opts.RunID == "" {
		return nil, fmt.Errorf("run id is required")
	}
	query := `
		SELECT id, project_id, run_id, task_id, message, summary, source, created_by_role, created_at
		FROM steering_events
		WHERE run_id = ?
	`
	args := []any{opts.RunID}
	if opts.TaskID != "" {
		query += ` AND task_id = ?`
		args = append(args, opts.TaskID)
	}
	query += ` ORDER BY created_at ASC, id ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list steering events: %w", err)
	}
	defer rows.Close()

	var events []*SteeringEvent
	for rows.Next() {
		event, err := scanSteeringEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read steering events: %w", err)
	}
	return events, nil
}

func scanSteeringEvent(scanner rowScanner) (*SteeringEvent, error) {
	var event SteeringEvent
	var taskID sql.NullString
	var summary sql.NullString
	var createdByRole sql.NullString
	var createdAt string

	if err := scanner.Scan(&event.ID, &event.ProjectID, &event.RunID, &taskID, &event.Message, &summary, &event.Source, &createdByRole, &createdAt); err != nil {
		return nil, err
	}
	if taskID.Valid {
		event.TaskID = taskID.String
	}
	if summary.Valid {
		event.Summary = summary.String
	}
	if createdByRole.Valid {
		event.CreatedByRole = createdByRole.String
	}
	parsedCreatedAt, err := parseStoredTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse steering created_at: %w", err)
	}
	event.CreatedAt = parsedCreatedAt
	return &event, nil
}
