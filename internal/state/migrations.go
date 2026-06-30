package state

import (
	"database/sql"
	"fmt"
	"time"
)

// Migration represents a database migration
type Migration struct {
	Version     int
	Description string
	Up          string
	Down        string
}

// migrations contains all database migrations in order
var migrations = []Migration{
	{
		Version:     1,
		Description: "Initial schema",
		Up:          Schema,
		Down: `
			DROP TABLE IF EXISTS config;
			DROP TABLE IF EXISTS blockers;
			DROP TABLE IF EXISTS token_stats_cache;
			DROP TABLE IF EXISTS quotas;
			DROP TABLE IF EXISTS rate_limits;
			DROP TABLE IF EXISTS token_usage;
			DROP TABLE IF EXISTS checkpoints;
			DROP TABLE IF EXISTS tasks;
			DROP TABLE IF EXISTS phases;
			DROP TABLE IF EXISTS architectures;
			DROP TABLE IF EXISTS interview_data;
			DROP TABLE IF EXISTS projects;
		`,
	},
	{
		Version:     2,
		Description: "Make rate_limits fields nullable",
		Up: `
			-- Create a new table with nullable columns
			CREATE TABLE IF NOT EXISTS rate_limits_new (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				provider TEXT NOT NULL,
				requests_remaining INTEGER,
				requests_limit INTEGER,
				reset_at TIMESTAMP,
				checked_at TIMESTAMP NOT NULL
			);

			-- Copy data from old table to new table
			INSERT INTO rate_limits_new (provider, requests_remaining, requests_limit, reset_at, checked_at)
			SELECT provider, requests_remaining, requests_limit, reset_at, checked_at 
			FROM rate_limits;

			-- Drop old table and rename new one
			DROP TABLE rate_limits;
			ALTER TABLE rate_limits_new RENAME TO rate_limits;

			-- Recreate indexes
			CREATE INDEX IF NOT EXISTS idx_rate_limits_provider ON rate_limits(provider);
		`,
		Down: `
			-- Create a new table with NOT NULL columns
			CREATE TABLE IF NOT EXISTS rate_limits_new (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				provider TEXT NOT NULL,
				requests_remaining INTEGER NOT NULL,
				requests_limit INTEGER NOT NULL,
				reset_at TIMESTAMP NOT NULL,
				checked_at TIMESTAMP NOT NULL
			);

			-- Copy data from old table to new table
			INSERT INTO rate_limits_new (provider, requests_remaining, requests_limit, reset_at, checked_at)
			SELECT provider, requests_remaining, requests_limit, reset_at, checked_at 
			FROM rate_limits;

			-- Drop old table and rename new one
			DROP TABLE rate_limits;
			ALTER TABLE rate_limits_new RENAME TO rate_limits;

			-- Recreate indexes
			CREATE INDEX IF NOT EXISTS idx_rate_limits_provider ON rate_limits(provider);
		`,
	},
	{
		Version:     3,
		Description: "Add cache table",
		Up: `
			CREATE TABLE IF NOT EXISTS cache (
				key TEXT PRIMARY KEY,
				value TEXT NOT NULL,
				created_at TIMESTAMP NOT NULL,
				expires_at TIMESTAMP
			);

			CREATE INDEX IF NOT EXISTS idx_cache_expires_at ON cache(expires_at);
		`,
		Down: `
			DROP TABLE IF EXISTS cache;
		`,
	},
	{
		Version:     4,
		Description: "Add Nexdev contract tables",
		Up: `
			CREATE TABLE IF NOT EXISTS runs (
				id TEXT PRIMARY KEY,
				project_id TEXT NOT NULL,
				status TEXT NOT NULL,
				current_stage TEXT,
				started_at TEXT NOT NULL,
				completed_at TEXT,
				cancelled_at TEXT,
				metadata_json TEXT NOT NULL DEFAULT '{}',
				FOREIGN KEY (project_id) REFERENCES projects(id)
			);

			CREATE TABLE IF NOT EXISTS stage_runs (
				id TEXT PRIMARY KEY,
				run_id TEXT NOT NULL,
				stage TEXT NOT NULL,
				status TEXT NOT NULL,
				attempt INTEGER NOT NULL DEFAULT 1,
				started_at TEXT,
				completed_at TEXT,
				error_json TEXT,
				output_json TEXT NOT NULL DEFAULT '{}',
				FOREIGN KEY (run_id) REFERENCES runs(id)
			);

			CREATE TABLE IF NOT EXISTS events (
				id TEXT PRIMARY KEY,
				run_id TEXT NOT NULL,
				sequence INTEGER NOT NULL,
				type TEXT NOT NULL,
				source TEXT NOT NULL,
				stage TEXT,
				task_id TEXT,
				payload_json TEXT NOT NULL,
				created_at TEXT NOT NULL,
				UNIQUE(run_id, sequence),
				FOREIGN KEY (run_id) REFERENCES runs(id)
			);

			CREATE TABLE IF NOT EXISTS artifacts (
				id TEXT PRIMARY KEY,
				project_id TEXT NOT NULL,
				run_id TEXT,
				kind TEXT NOT NULL,
				path TEXT NOT NULL,
				sha256 TEXT,
				version INTEGER NOT NULL DEFAULT 1,
				metadata_json TEXT NOT NULL DEFAULT '{}',
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				FOREIGN KEY (project_id) REFERENCES projects(id),
				FOREIGN KEY (run_id) REFERENCES runs(id)
			);

			CREATE TABLE IF NOT EXISTS hivemind_results (
				id TEXT PRIMARY KEY,
				project_id TEXT NOT NULL,
				run_id TEXT NOT NULL,
				cycle INTEGER NOT NULL,
				voice TEXT NOT NULL,
				result_json TEXT NOT NULL,
				created_at TEXT NOT NULL,
				FOREIGN KEY (project_id) REFERENCES projects(id),
				FOREIGN KEY (run_id) REFERENCES runs(id)
			);

			CREATE TABLE IF NOT EXISTS validate_results (
				id TEXT PRIMARY KEY,
				project_id TEXT NOT NULL,
				run_id TEXT NOT NULL,
				report_json TEXT NOT NULL,
				created_at TEXT NOT NULL,
				FOREIGN KEY (project_id) REFERENCES projects(id),
				FOREIGN KEY (run_id) REFERENCES runs(id)
			);

			CREATE TABLE IF NOT EXISTS steering_events (
				id TEXT PRIMARY KEY,
				project_id TEXT NOT NULL,
				run_id TEXT NOT NULL,
				task_id TEXT,
				message TEXT NOT NULL,
				summary TEXT,
				source TEXT NOT NULL,
				created_by_role TEXT,
				created_at TEXT NOT NULL,
				FOREIGN KEY (project_id) REFERENCES projects(id),
				FOREIGN KEY (run_id) REFERENCES runs(id)
			);

			CREATE TABLE IF NOT EXISTS detour_records (
				id TEXT PRIMARY KEY,
				project_id TEXT NOT NULL,
				run_id TEXT NOT NULL,
				trigger_task_id TEXT NOT NULL,
				reason TEXT NOT NULL,
				source TEXT NOT NULL,
				depth INTEGER NOT NULL,
				result_json TEXT NOT NULL,
				created_at TEXT NOT NULL,
				FOREIGN KEY (project_id) REFERENCES projects(id),
				FOREIGN KEY (run_id) REFERENCES runs(id)
			);

			CREATE TABLE IF NOT EXISTS navigation_events (
				id TEXT PRIMARY KEY,
				project_id TEXT NOT NULL,
				run_id TEXT,
				from_stage TEXT,
				to_stage TEXT NOT NULL,
				reason TEXT,
				actor TEXT NOT NULL,
				created_at TEXT NOT NULL,
				FOREIGN KEY (project_id) REFERENCES projects(id),
				FOREIGN KEY (run_id) REFERENCES runs(id)
			);

			CREATE TABLE IF NOT EXISTS plan_edit_events (
				id TEXT PRIMARY KEY,
				project_id TEXT NOT NULL,
				run_id TEXT NOT NULL,
				plan_version_before INTEGER NOT NULL,
				plan_version_after INTEGER NOT NULL,
				edit_type TEXT NOT NULL,
				target_id TEXT,
				patch_json TEXT NOT NULL,
				actor TEXT NOT NULL,
				created_at TEXT NOT NULL,
				FOREIGN KEY (project_id) REFERENCES projects(id),
				FOREIGN KEY (run_id) REFERENCES runs(id)
			);

			CREATE TABLE IF NOT EXISTS auth_tokens (
				id TEXT PRIMARY KEY,
				token_hash TEXT NOT NULL UNIQUE,
				role TEXT NOT NULL,
				name TEXT,
				created_at TEXT NOT NULL,
				expires_at TEXT,
				revoked_at TEXT,
				last_used_at TEXT
			);

			CREATE INDEX IF NOT EXISTS idx_runs_project_id ON runs(project_id);
			CREATE INDEX IF NOT EXISTS idx_stage_runs_run_stage ON stage_runs(run_id, stage);
			CREATE INDEX IF NOT EXISTS idx_events_run_sequence ON events(run_id, sequence);
			CREATE INDEX IF NOT EXISTS idx_events_run_type ON events(run_id, type);
			CREATE INDEX IF NOT EXISTS idx_artifacts_project_kind ON artifacts(project_id, kind);
			CREATE INDEX IF NOT EXISTS idx_artifacts_run_kind ON artifacts(run_id, kind);
			CREATE INDEX IF NOT EXISTS idx_hivemind_results_run_voice ON hivemind_results(run_id, voice);
			CREATE INDEX IF NOT EXISTS idx_validate_results_run_id ON validate_results(run_id);
			CREATE INDEX IF NOT EXISTS idx_steering_events_run_task ON steering_events(run_id, task_id);
			CREATE INDEX IF NOT EXISTS idx_detour_records_run_trigger ON detour_records(run_id, trigger_task_id);
			CREATE INDEX IF NOT EXISTS idx_navigation_events_project_created ON navigation_events(project_id, created_at);
			CREATE INDEX IF NOT EXISTS idx_plan_edit_events_run_created ON plan_edit_events(run_id, created_at);
		`,
		Down: `
			DROP INDEX IF EXISTS idx_plan_edit_events_run_created;
			DROP INDEX IF EXISTS idx_navigation_events_project_created;
			DROP INDEX IF EXISTS idx_detour_records_run_trigger;
			DROP INDEX IF EXISTS idx_steering_events_run_task;
			DROP INDEX IF EXISTS idx_validate_results_run_id;
			DROP INDEX IF EXISTS idx_hivemind_results_run_voice;
			DROP INDEX IF EXISTS idx_artifacts_run_kind;
			DROP INDEX IF EXISTS idx_artifacts_project_kind;
			DROP INDEX IF EXISTS idx_events_run_type;
			DROP INDEX IF EXISTS idx_events_run_sequence;
			DROP INDEX IF EXISTS idx_stage_runs_run_stage;
			DROP INDEX IF EXISTS idx_runs_project_id;

			DROP TABLE IF EXISTS auth_tokens;
			DROP TABLE IF EXISTS plan_edit_events;
			DROP TABLE IF EXISTS navigation_events;
			DROP TABLE IF EXISTS detour_records;
			DROP TABLE IF EXISTS steering_events;
			DROP TABLE IF EXISTS validate_results;
			DROP TABLE IF EXISTS hivemind_results;
			DROP TABLE IF EXISTS artifacts;
			DROP TABLE IF EXISTS events;
			DROP TABLE IF EXISTS stage_runs;
			DROP TABLE IF EXISTS runs;
		`,
	},
}

// MigrationManager handles database migrations
type MigrationManager struct {
	db *sql.DB
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(db *sql.DB) *MigrationManager {
	return &MigrationManager{db: db}
}

// Initialize creates the migrations table if it doesn't exist
func (m *MigrationManager) Initialize() error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			description TEXT NOT NULL,
			applied_at TIMESTAMP NOT NULL
		);
	`
	_, err := m.db.Exec(query)
	return err
}

// CurrentVersion returns the current schema version
func (m *MigrationManager) CurrentVersion() (int, error) {
	var version int
	err := m.db.QueryRow(`
		SELECT COALESCE(MAX(version), 0) 
		FROM schema_migrations
	`).Scan(&version)

	if err != nil {
		return 0, fmt.Errorf("failed to get current version: %w", err)
	}

	return version, nil
}

// Migrate runs all pending migrations
func (m *MigrationManager) Migrate() error {
	if err := m.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize migrations table: %w", err)
	}

	currentVersion, err := m.CurrentVersion()
	if err != nil {
		return err
	}

	// Run all migrations newer than current version
	for _, migration := range migrations {
		if migration.Version <= currentVersion {
			continue
		}

		if err := m.applyMigration(migration); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
		}
	}

	return nil
}

// applyMigration applies a single migration
func (m *MigrationManager) applyMigration(migration Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute the migration
	if _, err := tx.Exec(migration.Up); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	// Record the migration
	_, err = tx.Exec(`
		INSERT INTO schema_migrations (version, description, applied_at)
		VALUES (?, ?, ?)
	`, migration.Version, migration.Description, time.Now())

	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Rollback rolls back the last migration
func (m *MigrationManager) Rollback() error {
	currentVersion, err := m.CurrentVersion()
	if err != nil {
		return err
	}

	if currentVersion == 0 {
		return fmt.Errorf("no migrations to rollback")
	}

	// Find the migration to rollback
	var targetMigration *Migration
	for i := range migrations {
		if migrations[i].Version == currentVersion {
			targetMigration = &migrations[i]
			break
		}
	}

	if targetMigration == nil {
		return fmt.Errorf("migration %d not found", currentVersion)
	}

	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute the rollback
	if _, err := tx.Exec(targetMigration.Down); err != nil {
		return fmt.Errorf("failed to execute rollback: %w", err)
	}

	// Remove the migration record
	_, err = tx.Exec(`
		DELETE FROM schema_migrations 
		WHERE version = ?
	`, currentVersion)

	if err != nil {
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// MigrateToVersion migrates to a specific version
func (m *MigrationManager) MigrateToVersion(targetVersion int) error {
	currentVersion, err := m.CurrentVersion()
	if err != nil {
		return err
	}

	if targetVersion == currentVersion {
		return nil
	}

	if targetVersion > currentVersion {
		// Migrate up
		for _, migration := range migrations {
			if migration.Version <= currentVersion || migration.Version > targetVersion {
				continue
			}

			if err := m.applyMigration(migration); err != nil {
				return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
			}
		}
	} else {
		// Migrate down
		for i := len(migrations) - 1; i >= 0; i-- {
			migration := migrations[i]
			if migration.Version > currentVersion || migration.Version <= targetVersion {
				continue
			}

			if err := m.Rollback(); err != nil {
				return fmt.Errorf("failed to rollback migration %d: %w", migration.Version, err)
			}
		}
	}

	return nil
}

// ListMigrations returns all available migrations
func (m *MigrationManager) ListMigrations() ([]Migration, error) {
	return migrations, nil
}

// GetAppliedMigrations returns all applied migrations
func (m *MigrationManager) GetAppliedMigrations() ([]Migration, error) {
	rows, err := m.db.Query(`
		SELECT version, description, applied_at
		FROM schema_migrations
		ORDER BY version
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	var applied []Migration
	for rows.Next() {
		var migration Migration
		var appliedAt time.Time

		if err := rows.Scan(&migration.Version, &migration.Description, &appliedAt); err != nil {
			return nil, fmt.Errorf("failed to scan migration: %w", err)
		}

		applied = append(applied, migration)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating migrations: %w", err)
	}

	return applied, nil
}
