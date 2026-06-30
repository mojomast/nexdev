package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

func (s *Store) CreatePlanEditEvent(ctx context.Context, event *PlanEditEvent) error {
	if event == nil {
		return fmt.Errorf("plan edit event is required")
	}
	if event.ID == "" {
		return fmt.Errorf("plan edit event id is required")
	}
	if event.ProjectID == "" {
		return fmt.Errorf("project id is required")
	}
	if event.RunID == "" {
		return fmt.Errorf("run id is required")
	}
	if event.PlanVersionBefore < 0 || event.PlanVersionAfter < 0 {
		return fmt.Errorf("plan versions must be non-negative")
	}
	if event.EditType == "" {
		return fmt.Errorf("plan edit type is required")
	}
	if event.Actor == "" {
		return fmt.Errorf("plan edit actor is required")
	}
	if len(event.Patch) == 0 {
		event.Patch = json.RawMessage(`{}`)
	}
	if !json.Valid(event.Patch) {
		return fmt.Errorf("plan edit patch must be valid JSON")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	} else {
		event.CreatedAt = event.CreatedAt.UTC()
	}

	return executeWithRetryContext(ctx, s.db, 25, 5, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO plan_edit_events (id, project_id, run_id, plan_version_before, plan_version_after, edit_type, target_id, patch_json, actor, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, event.ID, event.ProjectID, event.RunID, event.PlanVersionBefore, event.PlanVersionAfter, event.EditType, nullableString(event.TargetID), string(event.Patch), event.Actor, formatTime(event.CreatedAt))
		if err != nil {
			return fmt.Errorf("failed to create plan edit event: %w", err)
		}
		return nil
	})
}

func (s *Store) ListPlanEditEventsByRun(ctx context.Context, runID string) ([]*PlanEditEvent, error) {
	if runID == "" {
		return nil, fmt.Errorf("run id is required")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, run_id, plan_version_before, plan_version_after, edit_type, target_id, patch_json, actor, created_at
		FROM plan_edit_events
		WHERE run_id = ?
		ORDER BY created_at ASC, id ASC
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to list plan edit events: %w", err)
	}
	defer rows.Close()

	var events []*PlanEditEvent
	for rows.Next() {
		event, err := scanPlanEditEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read plan edit events: %w", err)
	}
	return events, nil
}

func scanPlanEditEvent(scanner rowScanner) (*PlanEditEvent, error) {
	var event PlanEditEvent
	var targetID sql.NullString
	var patchJSON string
	var createdAt string

	if err := scanner.Scan(&event.ID, &event.ProjectID, &event.RunID, &event.PlanVersionBefore, &event.PlanVersionAfter, &event.EditType, &targetID, &patchJSON, &event.Actor, &createdAt); err != nil {
		return nil, err
	}
	if targetID.Valid {
		event.TargetID = targetID.String
	}
	event.Patch = json.RawMessage(patchJSON)
	parsedCreatedAt, err := parseStoredTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plan edit created_at: %w", err)
	}
	event.CreatedAt = parsedCreatedAt
	return &event, nil
}
