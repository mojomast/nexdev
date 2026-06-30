package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
)

type NexdevTaskListOptions struct {
	RunID       string
	PhaseID     string
	Status      string
	PlanVersion int
}

func (s *Store) CreateNexdevTask(ctx context.Context, task *NexdevTask) error {
	if task == nil {
		return fmt.Errorf("task is required")
	}
	if task.Spec.ID == "" {
		return fmt.Errorf("task id is required")
	}
	if task.ProjectID == "" {
		return fmt.Errorf("project id is required")
	}
	if task.RunID == "" {
		return fmt.Errorf("run id is required")
	}
	if task.Spec.PhaseID == "" {
		return fmt.Errorf("phase id is required")
	}
	if task.Spec.Title == "" {
		return fmt.Errorf("task title is required")
	}
	if len(task.Spec.AcceptanceCriteria) == 0 {
		return fmt.Errorf("task acceptance criteria are required")
	}
	if task.Status == "" {
		task.Status = NexdevTaskStatusPending
	}
	if task.PlanVersion == 0 {
		task.PlanVersion = 1
	}
	if task.PlanOrder < 0 {
		return fmt.Errorf("task plan order must be >= 0")
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now().UTC()
	} else {
		task.CreatedAt = task.CreatedAt.UTC()
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = task.CreatedAt
	} else {
		task.UpdatedAt = task.UpdatedAt.UTC()
	}

	jsonFields, err := marshalTaskJSONFields(task.Spec)
	if err != nil {
		return err
	}

	return executeWithRetryContext(ctx, s.db, 25, 5, func(tx *sql.Tx) error {
		for _, dependencyID := range task.Spec.Dependencies {
			if dependencyID == task.Spec.ID {
				return fmt.Errorf("task %s cannot depend on itself", task.Spec.ID)
			}
			var exists int
			if err := tx.QueryRowContext(ctx, `SELECT 1 FROM nexdev_tasks WHERE id = ? AND run_id = ?`, dependencyID, task.RunID).Scan(&exists); err != nil {
				if err == sql.ErrNoRows {
					return fmt.Errorf("task dependency not found: %s", dependencyID)
				}
				return fmt.Errorf("failed to validate task dependency %s: %w", dependencyID, err)
			}
		}

		_, err := tx.ExecContext(ctx, `
			INSERT INTO nexdev_tasks (
				id, project_id, run_id, phase_id, title, description, expected_files_json,
				dependencies_json, acceptance_criteria_json, test_commands_json, risk_level,
				required_tools_json, notes_json, status, plan_version, plan_order, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, task.Spec.ID, task.ProjectID, task.RunID, task.Spec.PhaseID, task.Spec.Title, task.Spec.Description,
			jsonFields.expectedFiles, jsonFields.dependencies, jsonFields.acceptanceCriteria, jsonFields.testCommands,
			task.Spec.RiskLevel, jsonFields.requiredTools, jsonFields.notes, task.Status, task.PlanVersion,
			task.PlanOrder, formatTime(task.CreatedAt), formatTime(task.UpdatedAt))
		if err != nil {
			return fmt.Errorf("failed to create Nexdev task: %w", err)
		}
		return nil
	})
}

func (s *Store) GetNexdevTask(ctx context.Context, taskID string) (*NexdevTask, error) {
	task, err := scanNexdevTask(s.db.QueryRowContext(ctx, selectNexdevTaskSQL()+` WHERE id = ?`, taskID))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("Nexdev task not found: %s", taskID)
	}
	if err != nil {
		return nil, err
	}
	return task, nil
}

func (s *Store) ListNexdevTasks(ctx context.Context, opts NexdevTaskListOptions) ([]*NexdevTask, error) {
	if opts.RunID == "" {
		return nil, fmt.Errorf("run id is required")
	}
	query := selectNexdevTaskSQL() + ` WHERE run_id = ?`
	args := []any{opts.RunID}
	if opts.PhaseID != "" {
		query += ` AND phase_id = ?`
		args = append(args, opts.PhaseID)
	}
	if opts.Status != "" {
		query += ` AND status = ?`
		args = append(args, opts.Status)
	}
	if opts.PlanVersion > 0 {
		query += ` AND plan_version = ?`
		args = append(args, opts.PlanVersion)
	}
	query += ` ORDER BY plan_version ASC, plan_order ASC, id ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list Nexdev tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*NexdevTask
	for rows.Next() {
		task, err := scanNexdevTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read Nexdev tasks: %w", err)
	}
	return tasks, nil
}

func (s *Store) UpdateNexdevTaskStatus(ctx context.Context, taskID, status string) error {
	if taskID == "" {
		return fmt.Errorf("task id is required")
	}
	if status == "" {
		return fmt.Errorf("task status is required")
	}
	return updateOne(ctx, s.db, `UPDATE nexdev_tasks SET status = ?, updated_at = ? WHERE id = ?`, status, formatTime(time.Now().UTC()), taskID)
}

// InsertNexdevTasksAfter inserts tasks immediately after triggerTaskID in the
// trigger task's plan version. Later tasks are shifted in stable relative order.
func (s *Store) InsertNexdevTasksAfter(ctx context.Context, triggerTaskID string, tasks []*NexdevTask) ([]*NexdevTask, error) {
	if triggerTaskID == "" {
		return nil, fmt.Errorf("trigger task id is required")
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("at least one task is required")
	}

	prepared := make([]*NexdevTask, 0, len(tasks))
	seen := map[string]bool{}
	now := time.Now().UTC()
	for _, task := range tasks {
		if task == nil {
			return nil, fmt.Errorf("task is required")
		}
		copyTask := *task
		if err := validateNexdevTaskForInsert(&copyTask); err != nil {
			return nil, err
		}
		if seen[copyTask.Spec.ID] {
			return nil, fmt.Errorf("duplicate task id: %s", copyTask.Spec.ID)
		}
		seen[copyTask.Spec.ID] = true
		if copyTask.Status == "" {
			copyTask.Status = NexdevTaskStatusPending
		}
		if copyTask.CreatedAt.IsZero() {
			copyTask.CreatedAt = now
		} else {
			copyTask.CreatedAt = copyTask.CreatedAt.UTC()
		}
		if copyTask.UpdatedAt.IsZero() {
			copyTask.UpdatedAt = copyTask.CreatedAt
		} else {
			copyTask.UpdatedAt = copyTask.UpdatedAt.UTC()
		}
		prepared = append(prepared, &copyTask)
	}

	return prepared, executeWithRetryContext(ctx, s.db, 25, 5, func(tx *sql.Tx) error {
		trigger, err := scanNexdevTask(tx.QueryRowContext(ctx, selectNexdevTaskSQL()+` WHERE id = ?`, triggerTaskID))
		if err == sql.ErrNoRows {
			return fmt.Errorf("trigger task not found: %s", triggerTaskID)
		}
		if err != nil {
			return err
		}

		for _, task := range prepared {
			if task.ProjectID == "" {
				task.ProjectID = trigger.ProjectID
			}
			if task.RunID == "" {
				task.RunID = trigger.RunID
			}
			if task.PlanVersion == 0 {
				task.PlanVersion = trigger.PlanVersion
			}
			if task.ProjectID != trigger.ProjectID || task.RunID != trigger.RunID || task.PlanVersion != trigger.PlanVersion {
				return fmt.Errorf("task %s must match trigger project, run, and plan version", task.Spec.ID)
			}
		}

		affectedRows, err := tx.QueryContext(ctx, `SELECT id, plan_order FROM nexdev_tasks WHERE run_id = ? AND plan_version = ? AND plan_order > ? ORDER BY plan_order DESC, id DESC`, trigger.RunID, trigger.PlanVersion, trigger.PlanOrder)
		if err != nil {
			return fmt.Errorf("failed to list tasks to shift: %w", err)
		}
		type shiftedTask struct {
			id    string
			order int
		}
		var shifted []shiftedTask
		for affectedRows.Next() {
			var task shiftedTask
			if err := affectedRows.Scan(&task.id, &task.order); err != nil {
				affectedRows.Close()
				return err
			}
			shifted = append(shifted, task)
		}
		if err := affectedRows.Close(); err != nil {
			return err
		}
		if err := affectedRows.Err(); err != nil {
			return fmt.Errorf("failed to read tasks to shift: %w", err)
		}

		updatedAt := formatTime(now)
		for _, task := range shifted {
			if _, err := tx.ExecContext(ctx, `UPDATE nexdev_tasks SET plan_order = ?, updated_at = ? WHERE id = ?`, task.order+len(prepared), updatedAt, task.id); err != nil {
				return fmt.Errorf("failed to shift task %s: %w", task.id, err)
			}
		}

		for i, task := range prepared {
			task.PlanOrder = trigger.PlanOrder + i + 1
			jsonFields, err := marshalTaskJSONFields(task.Spec)
			if err != nil {
				return err
			}
			for _, dependencyID := range task.Spec.Dependencies {
				if dependencyID == task.Spec.ID {
					return fmt.Errorf("task %s cannot depend on itself", task.Spec.ID)
				}
				var exists int
				if err := tx.QueryRowContext(ctx, `SELECT 1 FROM nexdev_tasks WHERE id = ? AND run_id = ?`, dependencyID, task.RunID).Scan(&exists); err != nil {
					if err == sql.ErrNoRows {
						return fmt.Errorf("task dependency not found: %s", dependencyID)
					}
					return fmt.Errorf("failed to validate task dependency %s: %w", dependencyID, err)
				}
			}
			_, err = tx.ExecContext(ctx, `
				INSERT INTO nexdev_tasks (
					id, project_id, run_id, phase_id, title, description, expected_files_json,
					dependencies_json, acceptance_criteria_json, test_commands_json, risk_level,
					required_tools_json, notes_json, status, plan_version, plan_order, created_at, updated_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, task.Spec.ID, task.ProjectID, task.RunID, task.Spec.PhaseID, task.Spec.Title, task.Spec.Description,
				jsonFields.expectedFiles, jsonFields.dependencies, jsonFields.acceptanceCriteria, jsonFields.testCommands,
				task.Spec.RiskLevel, jsonFields.requiredTools, jsonFields.notes, task.Status, task.PlanVersion,
				task.PlanOrder, formatTime(task.CreatedAt), formatTime(task.UpdatedAt))
			if err != nil {
				return fmt.Errorf("failed to create Nexdev task: %w", err)
			}
		}
		return nil
	})
}

func validateNexdevTaskForInsert(task *NexdevTask) error {
	if task == nil {
		return fmt.Errorf("task is required")
	}
	if task.Spec.ID == "" {
		return fmt.Errorf("task id is required")
	}
	if task.Spec.PhaseID == "" {
		return fmt.Errorf("phase id is required")
	}
	if task.Spec.Title == "" {
		return fmt.Errorf("task title is required")
	}
	if len(task.Spec.AcceptanceCriteria) == 0 {
		return fmt.Errorf("task acceptance criteria are required")
	}
	if task.PlanOrder < 0 {
		return fmt.Errorf("task plan order must be >= 0")
	}
	return nil
}

type taskJSONFields struct {
	expectedFiles      string
	dependencies       string
	acceptanceCriteria string
	testCommands       string
	requiredTools      string
	notes              string
}

func marshalTaskJSONFields(spec contract.TaskSpec) (taskJSONFields, error) {
	var fields taskJSONFields
	var err error
	if fields.expectedFiles, err = marshalStringSlice(spec.ExpectedFiles); err != nil {
		return fields, fmt.Errorf("failed to marshal expected files: %w", err)
	}
	if fields.dependencies, err = marshalStringSlice(spec.Dependencies); err != nil {
		return fields, fmt.Errorf("failed to marshal dependencies: %w", err)
	}
	if fields.acceptanceCriteria, err = marshalStringSlice(spec.AcceptanceCriteria); err != nil {
		return fields, fmt.Errorf("failed to marshal acceptance criteria: %w", err)
	}
	if fields.testCommands, err = marshalStringSlice(spec.TestCommands); err != nil {
		return fields, fmt.Errorf("failed to marshal test commands: %w", err)
	}
	if fields.requiredTools, err = marshalStringSlice(spec.RequiredTools); err != nil {
		return fields, fmt.Errorf("failed to marshal required tools: %w", err)
	}
	if fields.notes, err = marshalStringSlice(spec.Notes); err != nil {
		return fields, fmt.Errorf("failed to marshal notes: %w", err)
	}
	return fields, nil
}

func marshalStringSlice(values []string) (string, error) {
	if values == nil {
		values = []string{}
	}
	data, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshalStringSlice(value string) ([]string, error) {
	var values []string
	if err := json.Unmarshal([]byte(value), &values); err != nil {
		return nil, err
	}
	if values == nil {
		values = []string{}
	}
	return values, nil
}

func selectNexdevTaskSQL() string {
	return `SELECT id, project_id, run_id, phase_id, title, description, expected_files_json,
		dependencies_json, acceptance_criteria_json, test_commands_json, risk_level, required_tools_json,
		notes_json, status, plan_version, plan_order, created_at, updated_at FROM nexdev_tasks`
}

func scanNexdevTask(scanner rowScanner) (*NexdevTask, error) {
	var task NexdevTask
	var expectedFilesJSON, dependenciesJSON, acceptanceCriteriaJSON string
	var testCommandsJSON, requiredToolsJSON, notesJSON string
	var createdAt, updatedAt string

	if err := scanner.Scan(&task.Spec.ID, &task.ProjectID, &task.RunID, &task.Spec.PhaseID, &task.Spec.Title,
		&task.Spec.Description, &expectedFilesJSON, &dependenciesJSON, &acceptanceCriteriaJSON, &testCommandsJSON,
		&task.Spec.RiskLevel, &requiredToolsJSON, &notesJSON, &task.Status, &task.PlanVersion, &task.PlanOrder,
		&createdAt, &updatedAt); err != nil {
		return nil, err
	}

	var err error
	if task.Spec.ExpectedFiles, err = unmarshalStringSlice(expectedFilesJSON); err != nil {
		return nil, fmt.Errorf("failed to unmarshal expected files: %w", err)
	}
	if task.Spec.Dependencies, err = unmarshalStringSlice(dependenciesJSON); err != nil {
		return nil, fmt.Errorf("failed to unmarshal dependencies: %w", err)
	}
	if task.Spec.AcceptanceCriteria, err = unmarshalStringSlice(acceptanceCriteriaJSON); err != nil {
		return nil, fmt.Errorf("failed to unmarshal acceptance criteria: %w", err)
	}
	if task.Spec.TestCommands, err = unmarshalStringSlice(testCommandsJSON); err != nil {
		return nil, fmt.Errorf("failed to unmarshal test commands: %w", err)
	}
	if task.Spec.RequiredTools, err = unmarshalStringSlice(requiredToolsJSON); err != nil {
		return nil, fmt.Errorf("failed to unmarshal required tools: %w", err)
	}
	if task.Spec.Notes, err = unmarshalStringSlice(notesJSON); err != nil {
		return nil, fmt.Errorf("failed to unmarshal notes: %w", err)
	}
	if task.CreatedAt, err = parseStoredTime(createdAt); err != nil {
		return nil, fmt.Errorf("failed to parse task created_at: %w", err)
	}
	if task.UpdatedAt, err = parseStoredTime(updatedAt); err != nil {
		return nil, fmt.Errorf("failed to parse task updated_at: %w", err)
	}
	return &task, nil
}
