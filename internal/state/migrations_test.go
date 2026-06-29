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
