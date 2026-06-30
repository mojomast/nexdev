package state

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type ArtifactListOptions struct {
	ProjectID string
	RunID     string
	Kind      string
}

func (s *Store) UpsertArtifact(ctx context.Context, artifact *Artifact) error {
	if artifact == nil {
		return fmt.Errorf("artifact is required")
	}
	if artifact.ID == "" {
		return fmt.Errorf("artifact id is required")
	}
	if artifact.ProjectID == "" {
		return fmt.Errorf("project id is required")
	}
	if artifact.Kind == "" {
		return fmt.Errorf("artifact kind is required")
	}
	if artifact.Path == "" {
		return fmt.Errorf("artifact path is required")
	}
	if artifact.Version == 0 {
		artifact.Version = 1
	}
	if artifact.CreatedAt.IsZero() {
		artifact.CreatedAt = time.Now().UTC()
	} else {
		artifact.CreatedAt = artifact.CreatedAt.UTC()
	}
	if artifact.UpdatedAt.IsZero() {
		artifact.UpdatedAt = artifact.CreatedAt
	} else {
		artifact.UpdatedAt = artifact.UpdatedAt.UTC()
	}
	metadataJSON, err := jsonObjectString(artifact.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal artifact metadata: %w", err)
	}

	return executeWithRetryContext(ctx, s.db, 25, 5, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO artifacts (id, project_id, run_id, kind, path, sha256, version, metadata_json, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				project_id = excluded.project_id,
				run_id = excluded.run_id,
				kind = excluded.kind,
				path = excluded.path,
				sha256 = excluded.sha256,
				version = excluded.version,
				metadata_json = excluded.metadata_json,
				updated_at = excluded.updated_at
		`, artifact.ID, artifact.ProjectID, nullableString(artifact.RunID), artifact.Kind, artifact.Path, nullableString(artifact.SHA256), artifact.Version, metadataJSON, formatTime(artifact.CreatedAt), formatTime(artifact.UpdatedAt))
		if err != nil {
			return fmt.Errorf("failed to upsert artifact: %w", err)
		}
		return nil
	})
}

func (s *Store) GetArtifact(ctx context.Context, artifactID string) (*Artifact, error) {
	artifact, err := scanArtifact(s.db.QueryRowContext(ctx, `
		SELECT id, project_id, run_id, kind, path, sha256, version, metadata_json, created_at, updated_at
		FROM artifacts
		WHERE id = ?
	`, artifactID))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("artifact not found: %s", artifactID)
	}
	if err != nil {
		return nil, err
	}
	return artifact, nil
}

func (s *Store) ListArtifacts(ctx context.Context, opts ArtifactListOptions) ([]*Artifact, error) {
	if opts.ProjectID == "" {
		return nil, fmt.Errorf("project id is required")
	}
	query := `
		SELECT id, project_id, run_id, kind, path, sha256, version, metadata_json, created_at, updated_at
		FROM artifacts
		WHERE project_id = ?
	`
	args := []any{opts.ProjectID}
	if opts.RunID != "" {
		query += ` AND run_id = ?`
		args = append(args, opts.RunID)
	}
	if opts.Kind != "" {
		query += ` AND kind = ?`
		args = append(args, opts.Kind)
	}
	query += ` ORDER BY updated_at ASC, id ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list artifacts: %w", err)
	}
	defer rows.Close()

	var artifacts []*Artifact
	for rows.Next() {
		artifact, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read artifacts: %w", err)
	}
	return artifacts, nil
}

func scanArtifact(scanner rowScanner) (*Artifact, error) {
	var artifact Artifact
	var runID sql.NullString
	var sha256 sql.NullString
	var metadataJSON string
	var createdAt string
	var updatedAt string

	if err := scanner.Scan(&artifact.ID, &artifact.ProjectID, &runID, &artifact.Kind, &artifact.Path, &sha256, &artifact.Version, &metadataJSON, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if runID.Valid {
		artifact.RunID = runID.String
	}
	if sha256.Valid {
		artifact.SHA256 = sha256.String
	}
	parsedCreatedAt, err := parseStoredTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse artifact created_at: %w", err)
	}
	artifact.CreatedAt = parsedCreatedAt
	parsedUpdatedAt, err := parseStoredTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse artifact updated_at: %w", err)
	}
	artifact.UpdatedAt = parsedUpdatedAt
	if err := unmarshalJSON(metadataJSON, &artifact.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal artifact metadata: %w", err)
	}
	if artifact.Metadata == nil {
		artifact.Metadata = map[string]any{}
	}
	return &artifact, nil
}
