package state

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	return db
}

func TestMigrationManager_Initialize(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewMigrationManager(db)

	// Initialize should create migrations table
	if err := mgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Verify table exists
	var tableName string
	err := db.QueryRow(`
		SELECT name FROM sqlite_master 
		WHERE type='table' AND name='schema_migrations'
	`).Scan(&tableName)

	if err != nil {
		t.Fatalf("Migrations table not created: %v", err)
	}

	if tableName != "schema_migrations" {
		t.Errorf("Wrong table name: got %s, want schema_migrations", tableName)
	}
}

func TestMigrationManager_CurrentVersion(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewMigrationManager(db)

	// Initialize
	if err := mgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Current version should be 0 initially
	version, err := mgr.CurrentVersion()
	if err != nil {
		t.Fatalf("Failed to get current version: %v", err)
	}

	if version != 0 {
		t.Errorf("Initial version should be 0, got %d", version)
	}
}

func TestMigrationManager_Migrate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewMigrationManager(db)

	// Run migrations
	if err := mgr.Migrate(); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Verify current version
	version, err := mgr.CurrentVersion()
	if err != nil {
		t.Fatalf("Failed to get current version: %v", err)
	}

	expectedVersion := len(migrations)
	if version != expectedVersion {
		t.Errorf("Version mismatch: got %d, want %d", version, expectedVersion)
	}

	// Verify all tables were created
	tables := []string{
		"projects",
		"interview_data",
		"architectures",
		"phases",
		"tasks",
		"checkpoints",
		"token_usage",
		"rate_limits",
		"quotas",
		"token_stats_cache",
		"blockers",
		"config",
	}

	for _, table := range tables {
		var tableName string
		err := db.QueryRow(`
			SELECT name FROM sqlite_master 
			WHERE type='table' AND name=?
		`, table).Scan(&tableName)

		if err != nil {
			t.Errorf("Table %s not created: %v", table, err)
		}
	}
}

func TestMigrationManager_Migrate_Idempotent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewMigrationManager(db)

	// Run migrations twice
	if err := mgr.Migrate(); err != nil {
		t.Fatalf("First migration failed: %v", err)
	}

	if err := mgr.Migrate(); err != nil {
		t.Fatalf("Second migration failed: %v", err)
	}

	// Verify version is still correct
	version, err := mgr.CurrentVersion()
	if err != nil {
		t.Fatalf("Failed to get current version: %v", err)
	}

	expectedVersion := len(migrations)
	if version != expectedVersion {
		t.Errorf("Version mismatch after second migration: got %d, want %d", version, expectedVersion)
	}
}

func TestMigrationManager_Rollback(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewMigrationManager(db)

	// Run migrations
	if err := mgr.Migrate(); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	initialVersion, _ := mgr.CurrentVersion()

	// Rollback
	if err := mgr.Rollback(); err != nil {
		t.Fatalf("Failed to rollback: %v", err)
	}

	// Verify version decreased
	version, err := mgr.CurrentVersion()
	if err != nil {
		t.Fatalf("Failed to get current version: %v", err)
	}

	if version != initialVersion-1 {
		t.Errorf("Version after rollback: got %d, want %d", version, initialVersion-1)
	}
}

func TestMigrationManager_Rollback_NoMigrations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewMigrationManager(db)

	// Initialize but don't migrate
	if err := mgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Rollback should fail
	err := mgr.Rollback()
	if err == nil {
		t.Error("Rollback should fail when no migrations applied")
	}
}

func TestMigrationManager_MigrateToVersion(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewMigrationManager(db)

	// Initialize migrations table first
	if err := mgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Migrate to version 1
	if err := mgr.MigrateToVersion(1); err != nil {
		t.Fatalf("Failed to migrate to version 1: %v", err)
	}

	// Verify version
	version, err := mgr.CurrentVersion()
	if err != nil {
		t.Fatalf("Failed to get current version: %v", err)
	}

	if version != 1 {
		t.Errorf("Version mismatch: got %d, want 1", version)
	}

	// Migrate to version 0 (rollback)
	if err := mgr.MigrateToVersion(0); err != nil {
		t.Fatalf("Failed to migrate to version 0: %v", err)
	}

	// Verify version
	version, err = mgr.CurrentVersion()
	if err != nil {
		t.Fatalf("Failed to get current version: %v", err)
	}

	if version != 0 {
		t.Errorf("Version mismatch after rollback: got %d, want 0", version)
	}
}

func TestMigrationManager_ListMigrations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewMigrationManager(db)

	// List migrations
	migs, err := mgr.ListMigrations()
	if err != nil {
		t.Fatalf("Failed to list migrations: %v", err)
	}

	if len(migs) != len(migrations) {
		t.Errorf("Migration count mismatch: got %d, want %d", len(migs), len(migrations))
	}
}

func TestMigrationManager_GetAppliedMigrations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewMigrationManager(db)

	// Initialize migrations table first
	if err := mgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Initially no migrations applied
	applied, err := mgr.GetAppliedMigrations()
	if err != nil {
		t.Fatalf("Failed to get applied migrations: %v", err)
	}

	if len(applied) != 0 {
		t.Errorf("Should have no applied migrations initially, got %d", len(applied))
	}

	// Run migrations
	if err := mgr.Migrate(); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Get applied migrations
	applied, err = mgr.GetAppliedMigrations()
	if err != nil {
		t.Fatalf("Failed to get applied migrations: %v", err)
	}

	if len(applied) != len(migrations) {
		t.Errorf("Applied migration count mismatch: got %d, want %d", len(applied), len(migrations))
	}

	// Verify first migration
	if len(applied) > 0 {
		if applied[0].Version != 1 {
			t.Errorf("First migration version: got %d, want 1", applied[0].Version)
		}
		if applied[0].Description != "Initial schema" {
			t.Errorf("First migration description: got %s, want Initial schema", applied[0].Description)
		}
	}
}

func TestMigrationManager_ForeignKeyConstraints(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewMigrationManager(db)

	// Run migrations
	if err := mgr.Migrate(); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Insert a project
	_, err := db.Exec(`
		INSERT INTO projects (id, name, created_at, current_stage)
		VALUES ('test-project', 'Test Project', datetime('now'), 'init')
	`)
	if err != nil {
		t.Fatalf("Failed to insert project: %v", err)
	}

	// Try to insert interview data with invalid project_id (should fail)
	_, err = db.Exec(`
		INSERT INTO interview_data (project_id, data)
		VALUES ('invalid-project', '{}')
	`)
	if err == nil {
		t.Error("Should fail to insert interview data with invalid project_id")
	}

	// Insert interview data with valid project_id (should succeed)
	_, err = db.Exec(`
		INSERT INTO interview_data (project_id, data)
		VALUES ('test-project', '{}')
	`)
	if err != nil {
		t.Errorf("Failed to insert interview data with valid project_id: %v", err)
	}
}

func TestMigrationManager_Indexes(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewMigrationManager(db)

	// Run migrations
	if err := mgr.Migrate(); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Verify indexes were created
	indexes := []string{
		"idx_phases_project_id",
		"idx_tasks_phase_id",
		"idx_token_usage_project_id",
		"idx_token_usage_phase_id",
		"idx_token_usage_provider",
		"idx_token_usage_timestamp",
		"idx_rate_limits_provider",
		"idx_quotas_provider",
		"idx_blockers_task_id",
		"idx_checkpoints_project_id",
	}

	for _, index := range indexes {
		var indexName string
		err := db.QueryRow(`
			SELECT name FROM sqlite_master 
			WHERE type='index' AND name=?
		`, index).Scan(&indexName)

		if err != nil {
			t.Errorf("Index %s not created: %v", index, err)
		}
	}
}

func TestMigrationManager_NexdevContractTablesAndIndexes(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewMigrationManager(db)
	if err := mgr.Migrate(); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	tables := []string{
		"runs",
		"stage_runs",
		"events",
		"artifacts",
		"hivemind_results",
		"validate_results",
		"steering_events",
		"detour_records",
		"navigation_events",
		"plan_edit_events",
		"auth_tokens",
		"nexdev_tasks",
		"nexdev_blockers",
	}

	for _, table := range tables {
		if !sqliteObjectExists(t, db, "table", table) {
			t.Errorf("Nexdev contract table %s not created", table)
		}
	}

	indexes := []string{
		"idx_events_run_sequence",
		"idx_events_run_type",
		"idx_runs_project_id",
		"idx_stage_runs_run_stage",
		"idx_artifacts_project_kind",
		"idx_artifacts_run_kind",
		"idx_hivemind_results_run_voice",
		"idx_validate_results_run_id",
		"idx_steering_events_run_task",
		"idx_detour_records_run_trigger",
		"idx_navigation_events_project_created",
		"idx_plan_edit_events_run_created",
		"idx_nexdev_tasks_run_order",
		"idx_nexdev_tasks_run_status",
		"idx_nexdev_tasks_phase",
		"idx_nexdev_blockers_run_status",
		"idx_nexdev_blockers_task",
	}

	for _, index := range indexes {
		if !sqliteObjectExists(t, db, "index", index) {
			t.Errorf("Nexdev contract index %s not created", index)
		}
	}
}

func TestMigrationManager_NexdevMigrationPreservesGeoffrussyState(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewMigrationManager(db)
	if err := mgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize migrations table: %v", err)
	}
	if err := mgr.MigrateToVersion(3); err != nil {
		t.Fatalf("Failed to migrate to geoffrussy schema version: %v", err)
	}

	_, err := db.Exec(`
		INSERT INTO projects (id, name, created_at, current_stage)
		VALUES ('proj_legacy', 'Legacy Project', '2026-06-29T00:00:00Z', 'develop')
		;
		INSERT INTO phases (id, project_id, number, title, content, status, created_at)
		VALUES ('phase_legacy', 'proj_legacy', 1, 'Legacy Phase', 'legacy', 'not_started', '2026-06-29T00:00:00Z')
		;
		INSERT INTO tasks (id, phase_id, number, description, status)
		VALUES ('legacy_task', 'phase_legacy', '1.01', 'legacy task', 'not_started')
		;
		INSERT INTO blockers (id, task_id, description, created_at)
		VALUES ('legacy_blocker', 'legacy_task', 'legacy blocker', '2026-06-29T00:00:00Z')
	`)
	if err != nil {
		t.Fatalf("Failed to seed geoffrussy-compatible project: %v", err)
	}

	if err := mgr.Migrate(); err != nil {
		t.Fatalf("Failed to apply Nexdev additive migration: %v", err)
	}

	var name string
	err = db.QueryRow("SELECT name FROM projects WHERE id = 'proj_legacy'").Scan(&name)
	if err != nil {
		t.Fatalf("Failed to read seeded geoffrussy project after Nexdev migration: %v", err)
	}
	if name != "Legacy Project" {
		t.Fatalf("Seeded project changed after migration: got %q", name)
	}

	if !sqliteObjectExists(t, db, "table", "runs") {
		t.Fatal("Nexdev runs table missing after migration from geoffrussy-compatible state")
	}
	if !sqliteObjectExists(t, db, "table", "tasks") || !sqliteObjectExists(t, db, "table", "blockers") {
		t.Fatal("legacy geoffrussy task/blocker tables missing after Nexdev migration")
	}
	if !sqliteObjectExists(t, db, "table", "nexdev_tasks") || !sqliteObjectExists(t, db, "table", "nexdev_blockers") {
		t.Fatal("Nexdev task/blocker tables missing after migration from geoffrussy-compatible state")
	}
	var legacyTaskDescription string
	if err := db.QueryRow("SELECT description FROM tasks WHERE id = 'legacy_task'").Scan(&legacyTaskDescription); err != nil {
		t.Fatalf("Failed to read legacy task after Nexdev migration: %v", err)
	}
	if legacyTaskDescription != "legacy task" {
		t.Fatalf("legacy task changed after migration: got %q", legacyTaskDescription)
	}
}

func TestMigrationManager_NexdevEventSequenceUnique(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewMigrationManager(db)
	if err := mgr.Migrate(); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	_, err := db.Exec(`
		INSERT INTO projects (id, name, created_at, current_stage)
		VALUES ('proj_test', 'Test Project', '2026-06-29T00:00:00Z', 'init');
		INSERT INTO runs (id, project_id, status, started_at)
		VALUES ('run_test', 'proj_test', 'running', '2026-06-29T00:00:00Z');
		INSERT INTO events (id, run_id, sequence, type, source, payload_json, created_at)
		VALUES ('evt_one', 'run_test', 1, 'run_started', 'core', '{}', '2026-06-29T00:00:00Z');
	`)
	if err != nil {
		t.Fatalf("Failed to seed event sequence test rows: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO events (id, run_id, sequence, type, source, payload_json, created_at)
		VALUES ('evt_two', 'run_test', 1, 'run_status', 'core', '{}', '2026-06-29T00:00:01Z')
	`)
	if err == nil {
		t.Fatal("Expected duplicate event sequence for a run to fail")
	}
}

func TestStore_BusyTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	var busyTimeout int
	if err := store.DB().QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("Failed to check busy timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("Busy timeout mismatch: got %d, want 5000", busyTimeout)
	}
}

func sqliteObjectExists(t *testing.T, db *sql.DB, objectType, name string) bool {
	t.Helper()

	var found string
	err := db.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type = ? AND name = ?
	`, objectType, name).Scan(&found)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("Failed to query sqlite object %s %s: %v", objectType, name, err)
	}

	return found == name
}
