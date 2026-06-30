package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
)

var (
	ErrEventSequenceConflict = errors.New("event sequence conflict")
	ErrEventNotFound         = errors.New("event not found")
)

type EventListOptions struct {
	RunID         string
	AfterSequence int64
	AfterEventID  string
	Limit         int
}

func (s *Store) PersistEvent(ctx context.Context, event contract.EventEnvelope) (contract.EventEnvelope, error) {
	if event.EventID == "" {
		return contract.EventEnvelope{}, fmt.Errorf("event id is required")
	}
	if event.RunID == "" {
		return contract.EventEnvelope{}, fmt.Errorf("run id is required")
	}
	if event.Type == "" {
		return contract.EventEnvelope{}, fmt.Errorf("event type is required")
	}
	if event.Source == "" {
		return contract.EventEnvelope{}, fmt.Errorf("event source is required")
	}

	if len(event.Payload) == 0 {
		event.Payload = json.RawMessage(`{}`)
	}
	if !json.Valid(event.Payload) {
		return contract.EventEnvelope{}, fmt.Errorf("event payload must be valid JSON")
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	} else {
		event.Timestamp = event.Timestamp.UTC()
	}
	event.ContractVersion = contract.EventContractVersion

	if err := executeWithRetryContext(ctx, s.db, 25, 5, func(tx *sql.Tx) error {
		if err := lockRunForEventAppend(ctx, tx, event.RunID); err != nil {
			return err
		}

		var maxSequence sql.NullInt64
		if err := tx.QueryRowContext(ctx, `SELECT MAX(sequence) FROM events WHERE run_id = ?`, event.RunID).Scan(&maxSequence); err != nil {
			return fmt.Errorf("failed to read current event sequence: %w", err)
		}

		nextSequence := int64(1)
		if maxSequence.Valid {
			nextSequence = maxSequence.Int64 + 1
		}
		if event.Sequence == 0 {
			event.Sequence = nextSequence
		} else if event.Sequence != nextSequence {
			return fmt.Errorf("%w: got %d, want next sequence %d for run %s", ErrEventSequenceConflict, event.Sequence, nextSequence, event.RunID)
		}

		_, err := tx.ExecContext(ctx, `
			INSERT INTO events (id, run_id, sequence, type, source, stage, task_id, payload_json, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, event.EventID, event.RunID, event.Sequence, event.Type, event.Source, nullableString(event.Stage), nullableString(event.TaskID), string(event.Payload), event.Timestamp.Format(time.RFC3339Nano))
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed: events.run_id, events.sequence") {
				return fmt.Errorf("%w: %v", ErrEventSequenceConflict, err)
			}
			return fmt.Errorf("failed to persist event: %w", err)
		}

		return nil
	}); err != nil {
		return contract.EventEnvelope{}, err
	}

	stored, err := s.GetEvent(ctx, event.EventID)
	if err != nil {
		return contract.EventEnvelope{}, err
	}
	return stored, nil
}

func (s *Store) GetEvent(ctx context.Context, eventID string) (contract.EventEnvelope, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT events.id, events.run_id, runs.project_id, events.sequence, events.type, events.source,
		       events.stage, events.task_id, events.payload_json, events.created_at
		FROM events
		JOIN runs ON runs.id = events.run_id
		WHERE events.id = ?
	`, eventID)

	event, err := scanEvent(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return contract.EventEnvelope{}, fmt.Errorf("%w: %s", ErrEventNotFound, eventID)
		}
		return contract.EventEnvelope{}, err
	}
	return event, nil
}

func (s *Store) ListEvents(ctx context.Context, opts EventListOptions) ([]contract.EventEnvelope, error) {
	if opts.RunID == "" {
		return nil, fmt.Errorf("run id is required")
	}

	afterSequence := opts.AfterSequence
	if opts.AfterEventID != "" {
		eventSequence, err := s.EventSequenceForID(ctx, opts.RunID, opts.AfterEventID)
		if err != nil {
			return nil, err
		}
		if eventSequence > afterSequence {
			afterSequence = eventSequence
		}
	}

	query := `
		SELECT events.id, events.run_id, runs.project_id, events.sequence, events.type, events.source,
		       events.stage, events.task_id, events.payload_json, events.created_at
		FROM events
		JOIN runs ON runs.id = events.run_id
		WHERE events.run_id = ? AND events.sequence > ?
		ORDER BY events.sequence ASC
	`
	args := []any{opts.RunID, afterSequence}
	if opts.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, opts.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}
	defer rows.Close()

	var events []contract.EventEnvelope
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read events: %w", err)
	}

	return events, nil
}

func (s *Store) EventSequenceForID(ctx context.Context, runID, eventID string) (int64, error) {
	var sequence int64
	err := s.db.QueryRowContext(ctx, `
		SELECT sequence FROM events WHERE run_id = ? AND id = ?
	`, runID, eventID).Scan(&sequence)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("%w: %s", ErrEventNotFound, eventID)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to map event id to sequence: %w", err)
	}
	return sequence, nil
}

func lockRunForEventAppend(ctx context.Context, tx *sql.Tx, runID string) error {
	result, err := tx.ExecContext(ctx, `UPDATE runs SET metadata_json = metadata_json WHERE id = ?`, runID)
	if err != nil {
		return fmt.Errorf("failed to lock run for event append: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to verify run lock: %w", err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("run not found: %s", runID)
	}
	return nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

type eventScanner interface {
	Scan(dest ...any) error
}

func scanEvent(scanner eventScanner) (contract.EventEnvelope, error) {
	var event contract.EventEnvelope
	var stage sql.NullString
	var taskID sql.NullString
	var payload string
	var createdAt string

	if err := scanner.Scan(
		&event.EventID,
		&event.RunID,
		&event.ProjectID,
		&event.Sequence,
		&event.Type,
		&event.Source,
		&stage,
		&taskID,
		&payload,
		&createdAt,
	); err != nil {
		return contract.EventEnvelope{}, err
	}

	parsedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return contract.EventEnvelope{}, fmt.Errorf("failed to parse event timestamp %q: %w", createdAt, err)
	}

	event.ContractVersion = contract.EventContractVersion
	event.Timestamp = parsedAt.UTC()
	event.Payload = json.RawMessage(payload)
	if stage.Valid {
		event.Stage = stage.String
	}
	if taskID.Valid {
		event.TaskID = taskID.String
	}

	return event, nil
}
