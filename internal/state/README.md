# State Store Package

This package implements the SQLite-based state persistence layer for Geoffrussy.

## Overview

The state store provides:
- Database schema definition for all project data
- Migration system for schema versioning
- Store interface for database operations
- Health checking and connection management

## Database Schema

The schema includes the following tables:

### Core Tables
- **projects**: Main project information
- **interview_data**: Structured interview responses (JSON)
- **architectures**: Generated architecture documents
- **phases**: Development phases
- **tasks**: Individual tasks within phases

### Tracking Tables
- **checkpoints**: Saved states for rollback
- **token_usage**: API token consumption tracking
- **rate_limits**: API rate limit information
- **quotas**: API quota information
- **token_stats_cache**: Cached token statistics
- **blockers**: Task blockers and resolutions
- **config**: System configuration key-value pairs

### Indexes
Performance indexes are created on:
- Foreign key columns (project_id, phase_id, task_id)
- Query columns (provider, timestamp)

## Migration System

The migration system provides:
- **Versioned migrations**: Each migration has a version number
- **Up/Down migrations**: Support for both applying and rolling back
- **Transaction safety**: All migrations run in transactions
- **Migration tracking**: Applied migrations are recorded in `schema_migrations` table

### Migration Operations

```go
// Create a new store (automatically runs migrations)
store, err := NewStore("/path/to/database.db")

// Get current schema version
version, err := store.MigrationManager().CurrentVersion()

// Rollback last migration
err := store.MigrationManager().Rollback()

// Migrate to specific version
err := store.MigrationManager().MigrateToVersion(1)

// List all migrations
migrations, err := store.MigrationManager().ListMigrations()

// Get applied migrations
applied, err := store.MigrationManager().GetAppliedMigrations()
```

## Store Operations

```go
// Create a new store
store, err := NewStore("/path/to/database.db")
if err != nil {
    log.Fatal(err)
}
defer store.Close()

// Health check
if err := store.HealthCheck(); err != nil {
    log.Fatal("Database unhealthy:", err)
}

// Get database connection for queries
db := store.DB()

// Start a transaction
tx, err := store.BeginTx()
if err != nil {
    log.Fatal(err)
}
defer tx.Rollback()

// ... perform operations ...

tx.Commit()
```

## Features

### Foreign Key Constraints
Foreign keys are enabled to maintain referential integrity:
- Cascading deletes for dependent records
- SET NULL for optional references

### WAL Mode
Write-Ahead Logging (WAL) is enabled for:
- Better concurrency
- Improved performance
- Crash recovery

### In-Memory Database
For testing, use `:memory:` as the database path:
```go
store, err := NewStore(":memory:")
```

## Testing

The package includes comprehensive tests:
- **store_test.go**: Tests for Store creation, health checks, and configuration
- **migrations_test.go**: Tests for migration system functionality

Run tests with:
```bash
go test -v ./internal/state/
```

## Requirements Satisfied

This implementation satisfies:
- **Requirement 1.4**: Database creation during initialization
- **Requirement 14.1**: SQLite-based persistence
- **Requirement 14.8**: Database corruption detection (via health check)

## Future Enhancements

Future migrations can be added to the `migrations` slice in `migrations.go`:

```go
var migrations = []Migration{
    {
        Version:     1,
        Description: "Initial schema",
        Up:          Schema,
        Down:        "...",
    },
    {
        Version:     2,
        Description: "Add new feature",
        Up:          "ALTER TABLE ...",
        Down:        "ALTER TABLE ...",
    },
}
```

## Notes

- The schema uses TEXT for IDs (UUIDs or similar)
- Timestamps are stored as TIMESTAMP type
- JSON columns store structured data
- All tables use appropriate indexes for query performance
