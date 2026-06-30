package state

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (s *Store) CreateRun(ctx context.Context, run *Run) error {
	if run == nil {
		return fmt.Errorf("run is required")
	}
	if run.ID == "" {
		return fmt.Errorf("run id is required")
	}
	if run.ProjectID == "" {
		return fmt.Errorf("project id is required")
	}
	if run.Status == "" {
		return fmt.Errorf("run status is required")
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = time.Now().UTC()
	} else {
		run.StartedAt = run.StartedAt.UTC()
	}
	metadataJSON, err := jsonObjectString(run.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal run metadata: %w", err)
	}

	return executeWithRetryContext(ctx, s.db, 25, 5, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO runs (id, project_id, status, current_stage, started_at, completed_at, cancelled_at, metadata_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, run.ID, run.ProjectID, run.Status, nullableString(run.CurrentStage), formatTime(run.StartedAt), formatOptionalTime(run.CompletedAt), formatOptionalTime(run.CancelledAt), metadataJSON)
		if err != nil {
			return fmt.Errorf("failed to create run: %w", err)
		}
		return nil
	})
}

func (s *Store) GetRun(ctx context.Context, runID string) (*Run, error) {
	run, err := scanRun(s.db.QueryRowContext(ctx, `
		SELECT id, project_id, status, current_stage, started_at, completed_at, cancelled_at, metadata_json
		FROM runs
		WHERE id = ?
	`, runID))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("run not found: %s", runID)
	}
	if err != nil {
		return nil, err
	}
	return run, nil
}

func (s *Store) UpdateRunStatus(ctx context.Context, runID, status string) error {
	if runID == "" {
		return fmt.Errorf("run id is required")
	}
	if status == "" {
		return fmt.Errorf("run status is required")
	}
	return updateOne(ctx, s.db, `UPDATE runs SET status = ? WHERE id = ?`, status, runID)
}

func (s *Store) UpdateRunCurrentStage(ctx context.Context, runID, currentStage string) error {
	if runID == "" {
		return fmt.Errorf("run id is required")
	}
	return updateOne(ctx, s.db, `UPDATE runs SET current_stage = ? WHERE id = ?`, nullableString(currentStage), runID)
}

func (s *Store) CompleteRun(ctx context.Context, runID string, completedAt time.Time) error {
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	return updateOne(ctx, s.db, `UPDATE runs SET status = 'completed', completed_at = ? WHERE id = ?`, formatTime(completedAt), runID)
}

func (s *Store) CancelRun(ctx context.Context, runID string, cancelledAt time.Time) error {
	if cancelledAt.IsZero() {
		cancelledAt = time.Now().UTC()
	}
	return updateOne(ctx, s.db, `UPDATE runs SET status = 'cancelled', cancelled_at = ? WHERE id = ?`, formatTime(cancelledAt), runID)
}

func (s *Store) ListRunsByProject(ctx context.Context, projectID string) ([]*Run, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project id is required")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, status, current_stage, started_at, completed_at, cancelled_at, metadata_json
		FROM runs
		WHERE project_id = ?
		ORDER BY started_at ASC, id ASC
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list runs: %w", err)
	}
	defer rows.Close()

	var runs []*Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read runs: %w", err)
	}
	return runs, nil
}

func (s *Store) CreateStageRun(ctx context.Context, stageRun *StageRun) error {
	if stageRun == nil {
		return fmt.Errorf("stage run is required")
	}
	if stageRun.ID == "" {
		return fmt.Errorf("stage run id is required")
	}
	if stageRun.RunID == "" {
		return fmt.Errorf("run id is required")
	}
	if stageRun.Stage == "" {
		return fmt.Errorf("stage is required")
	}
	if stageRun.Status == "" {
		return fmt.Errorf("stage run status is required")
	}
	if stageRun.Attempt == 0 {
		stageRun.Attempt = 1
	}
	outputJSON, err := jsonObjectString(stageRun.Output)
	if err != nil {
		return fmt.Errorf("failed to marshal stage output: %w", err)
	}
	errorJSON, err := optionalJSONObjectString(stageRun.Error)
	if err != nil {
		return fmt.Errorf("failed to marshal stage error: %w", err)
	}

	return executeWithRetryContext(ctx, s.db, 25, 5, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO stage_runs (id, run_id, stage, status, attempt, started_at, completed_at, error_json, output_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, stageRun.ID, stageRun.RunID, stageRun.Stage, stageRun.Status, stageRun.Attempt, formatOptionalTime(stageRun.StartedAt), formatOptionalTime(stageRun.CompletedAt), errorJSON, outputJSON)
		if err != nil {
			return fmt.Errorf("failed to create stage run: %w", err)
		}
		return nil
	})
}

func (s *Store) GetStageRun(ctx context.Context, stageRunID string) (*StageRun, error) {
	stageRun, err := scanStageRun(s.db.QueryRowContext(ctx, `
		SELECT id, run_id, stage, status, attempt, started_at, completed_at, error_json, output_json
		FROM stage_runs
		WHERE id = ?
	`, stageRunID))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("stage run not found: %s", stageRunID)
	}
	if err != nil {
		return nil, err
	}
	return stageRun, nil
}

func (s *Store) UpdateStageRunStatus(ctx context.Context, stageRunID, status string) error {
	if status == "" {
		return fmt.Errorf("stage run status is required")
	}
	return updateOne(ctx, s.db, `UPDATE stage_runs SET status = ? WHERE id = ?`, status, stageRunID)
}

func (s *Store) UpdateStageRunOutput(ctx context.Context, stageRunID string, output map[string]any) error {
	outputJSON, err := jsonObjectString(output)
	if err != nil {
		return fmt.Errorf("failed to marshal stage output: %w", err)
	}
	return updateOne(ctx, s.db, `UPDATE stage_runs SET output_json = ? WHERE id = ?`, outputJSON, stageRunID)
}

func (s *Store) UpdateStageRunError(ctx context.Context, stageRunID string, errorData map[string]any) error {
	errorJSON, err := optionalJSONObjectString(errorData)
	if err != nil {
		return fmt.Errorf("failed to marshal stage error: %w", err)
	}
	return updateOne(ctx, s.db, `UPDATE stage_runs SET error_json = ? WHERE id = ?`, errorJSON, stageRunID)
}

func (s *Store) UpdateStageRunAttempt(ctx context.Context, stageRunID string, attempt int) error {
	if attempt < 1 {
		return fmt.Errorf("stage run attempt must be >= 1")
	}
	return updateOne(ctx, s.db, `UPDATE stage_runs SET attempt = ? WHERE id = ?`, attempt, stageRunID)
}

func (s *Store) CompleteStageRun(ctx context.Context, stageRunID string, completedAt time.Time) error {
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	return updateOne(ctx, s.db, `UPDATE stage_runs SET status = 'completed', completed_at = ? WHERE id = ?`, formatTime(completedAt), stageRunID)
}

func (s *Store) ListStageRunsByRun(ctx context.Context, runID string) ([]*StageRun, error) {
	if runID == "" {
		return nil, fmt.Errorf("run id is required")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, run_id, stage, status, attempt, started_at, completed_at, error_json, output_json
		FROM stage_runs
		WHERE run_id = ?
		ORDER BY started_at ASC, id ASC
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to list stage runs: %w", err)
	}
	defer rows.Close()

	var stageRuns []*StageRun
	for rows.Next() {
		stageRun, err := scanStageRun(rows)
		if err != nil {
			return nil, err
		}
		stageRuns = append(stageRuns, stageRun)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read stage runs: %w", err)
	}
	return stageRuns, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRun(scanner rowScanner) (*Run, error) {
	var run Run
	var currentStage sql.NullString
	var startedAt string
	var completedAt sql.NullString
	var cancelledAt sql.NullString
	var metadataJSON string

	if err := scanner.Scan(&run.ID, &run.ProjectID, &run.Status, &currentStage, &startedAt, &completedAt, &cancelledAt, &metadataJSON); err != nil {
		return nil, err
	}
	parsedStartedAt, err := parseStoredTime(startedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse run started_at: %w", err)
	}
	run.StartedAt = parsedStartedAt
	run.CompletedAt, err = parseOptionalStoredTime(completedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse run completed_at: %w", err)
	}
	run.CancelledAt, err = parseOptionalStoredTime(cancelledAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse run cancelled_at: %w", err)
	}
	if currentStage.Valid {
		run.CurrentStage = currentStage.String
	}
	if err := unmarshalJSON(metadataJSON, &run.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal run metadata: %w", err)
	}
	if run.Metadata == nil {
		run.Metadata = map[string]any{}
	}
	return &run, nil
}

func scanStageRun(scanner rowScanner) (*StageRun, error) {
	var stageRun StageRun
	var startedAt sql.NullString
	var completedAt sql.NullString
	var errorJSON sql.NullString
	var outputJSON string

	if err := scanner.Scan(&stageRun.ID, &stageRun.RunID, &stageRun.Stage, &stageRun.Status, &stageRun.Attempt, &startedAt, &completedAt, &errorJSON, &outputJSON); err != nil {
		return nil, err
	}
	var err error
	stageRun.StartedAt, err = parseOptionalStoredTime(startedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse stage started_at: %w", err)
	}
	stageRun.CompletedAt, err = parseOptionalStoredTime(completedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse stage completed_at: %w", err)
	}
	if errorJSON.Valid && errorJSON.String != "" {
		if err := unmarshalJSON(errorJSON.String, &stageRun.Error); err != nil {
			return nil, fmt.Errorf("failed to unmarshal stage error: %w", err)
		}
	}
	if err := unmarshalJSON(outputJSON, &stageRun.Output); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stage output: %w", err)
	}
	if stageRun.Output == nil {
		stageRun.Output = map[string]any{}
	}
	return &stageRun, nil
}

func updateOne(ctx context.Context, db *sql.DB, query string, args ...any) error {
	return executeWithRetryContext(ctx, db, 25, 5, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return err
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to verify update: %w", err)
		}
		if rowsAffected != 1 {
			return fmt.Errorf("row not found")
		}
		return nil
	})
}

func jsonObjectString(value map[string]any) (string, error) {
	if value == nil {
		value = map[string]any{}
	}
	return marshalJSON(value)
}

func optionalJSONObjectString(value map[string]any) (any, error) {
	if value == nil {
		return nil, nil
	}
	return marshalJSON(value)
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func formatOptionalTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	formatted := formatTime(*t)
	return formatted
}

func parseStoredTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func parseOptionalStoredTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid || value.String == "" {
		return nil, nil
	}
	parsed, err := parseStoredTime(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
