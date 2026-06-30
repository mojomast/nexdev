package state

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type NavigationEventListOptions struct {
	ProjectID string
	RunID     string
}

func (s *Store) AppendNavigationEvent(ctx context.Context, event *NavigationEvent) error {
	if event == nil {
		return fmt.Errorf("navigation event is required")
	}
	if event.ID == "" {
		return fmt.Errorf("navigation event id is required")
	}
	if event.ProjectID == "" {
		return fmt.Errorf("project id is required")
	}
	if event.ToStage == "" {
		return fmt.Errorf("to stage is required")
	}
	if event.Actor == "" {
		return fmt.Errorf("navigation actor is required")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	} else {
		event.CreatedAt = event.CreatedAt.UTC()
	}

	return executeWithRetryContext(ctx, s.db, 25, 5, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO navigation_events (id, project_id, run_id, from_stage, to_stage, reason, actor, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, event.ID, event.ProjectID, nullableString(event.RunID), nullableString(event.FromStage), event.ToStage, nullableString(event.Reason), event.Actor, formatTime(event.CreatedAt))
		if err != nil {
			return fmt.Errorf("failed to append navigation event: %w", err)
		}
		return nil
	})
}

func (s *Store) ListNavigationEvents(ctx context.Context, opts NavigationEventListOptions) ([]*NavigationEvent, error) {
	if opts.ProjectID == "" {
		return nil, fmt.Errorf("project id is required")
	}
	query := `
		SELECT id, project_id, run_id, from_stage, to_stage, reason, actor, created_at
		FROM navigation_events
		WHERE project_id = ?
	`
	args := []any{opts.ProjectID}
	if opts.RunID != "" {
		query += ` AND run_id = ?`
		args = append(args, opts.RunID)
	}
	query += ` ORDER BY created_at ASC, id ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list navigation events: %w", err)
	}
	defer rows.Close()

	var events []*NavigationEvent
	for rows.Next() {
		event, err := scanNavigationEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read navigation events: %w", err)
	}
	return events, nil
}

func scanNavigationEvent(scanner rowScanner) (*NavigationEvent, error) {
	var event NavigationEvent
	var runID sql.NullString
	var fromStage sql.NullString
	var reason sql.NullString
	var createdAt string

	if err := scanner.Scan(&event.ID, &event.ProjectID, &runID, &fromStage, &event.ToStage, &reason, &event.Actor, &createdAt); err != nil {
		return nil, err
	}
	if runID.Valid {
		event.RunID = runID.String
	}
	if fromStage.Valid {
		event.FromStage = fromStage.String
	}
	if reason.Valid {
		event.Reason = reason.String
	}
	parsedCreatedAt, err := parseStoredTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse navigation created_at: %w", err)
	}
	event.CreatedAt = parsedCreatedAt
	return &event, nil
}
