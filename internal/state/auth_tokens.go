package state

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (s *Store) CreateAuthToken(ctx context.Context, token *AuthToken) error {
	if token == nil {
		return fmt.Errorf("auth token is required")
	}
	if token.ID == "" {
		return fmt.Errorf("auth token id is required")
	}
	if token.TokenHash == "" {
		return fmt.Errorf("token hash is required")
	}
	if token.Role == "" {
		return fmt.Errorf("auth token role is required")
	}
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now().UTC()
	} else {
		token.CreatedAt = token.CreatedAt.UTC()
	}

	return executeWithRetryContext(ctx, s.db, 25, 5, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO auth_tokens (id, token_hash, role, name, created_at, expires_at, revoked_at, last_used_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, token.ID, token.TokenHash, token.Role, nullableString(token.Name), formatTime(token.CreatedAt), formatOptionalTime(token.ExpiresAt), formatOptionalTime(token.RevokedAt), formatOptionalTime(token.LastUsedAt))
		if err != nil {
			return fmt.Errorf("failed to create auth token: %w", err)
		}
		return nil
	})
}

func (s *Store) GetAuthToken(ctx context.Context, tokenID string) (*AuthToken, error) {
	token, err := scanAuthToken(s.db.QueryRowContext(ctx, `
		SELECT id, token_hash, role, name, created_at, expires_at, revoked_at, last_used_at
		FROM auth_tokens
		WHERE id = ?
	`, tokenID))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("auth token not found: %s", tokenID)
	}
	if err != nil {
		return nil, err
	}
	return token, nil
}

func (s *Store) GetAuthTokenByHash(ctx context.Context, tokenHash string) (*AuthToken, error) {
	token, err := scanAuthToken(s.db.QueryRowContext(ctx, `
		SELECT id, token_hash, role, name, created_at, expires_at, revoked_at, last_used_at
		FROM auth_tokens
		WHERE token_hash = ?
	`, tokenHash))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("auth token not found")
	}
	if err != nil {
		return nil, err
	}
	return token, nil
}

func (s *Store) ListAuthTokens(ctx context.Context) ([]*AuthToken, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, token_hash, role, name, created_at, expires_at, revoked_at, last_used_at
		FROM auth_tokens
		ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list auth tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*AuthToken
	for rows.Next() {
		token, err := scanAuthToken(rows)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read auth tokens: %w", err)
	}
	return tokens, nil
}

func (s *Store) RevokeAuthToken(ctx context.Context, tokenID string, revokedAt time.Time) error {
	if revokedAt.IsZero() {
		revokedAt = time.Now().UTC()
	}
	return updateOne(ctx, s.db, `UPDATE auth_tokens SET revoked_at = ? WHERE id = ?`, formatTime(revokedAt), tokenID)
}

func (s *Store) TouchAuthTokenLastUsed(ctx context.Context, tokenID string, lastUsedAt time.Time) error {
	if lastUsedAt.IsZero() {
		lastUsedAt = time.Now().UTC()
	}
	return updateOne(ctx, s.db, `UPDATE auth_tokens SET last_used_at = ? WHERE id = ?`, formatTime(lastUsedAt), tokenID)
}

func scanAuthToken(scanner rowScanner) (*AuthToken, error) {
	var token AuthToken
	var name sql.NullString
	var createdAt string
	var expiresAt sql.NullString
	var revokedAt sql.NullString
	var lastUsedAt sql.NullString

	if err := scanner.Scan(&token.ID, &token.TokenHash, &token.Role, &name, &createdAt, &expiresAt, &revokedAt, &lastUsedAt); err != nil {
		return nil, err
	}
	if name.Valid {
		token.Name = name.String
	}
	parsedCreatedAt, err := parseStoredTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse auth token created_at: %w", err)
	}
	token.CreatedAt = parsedCreatedAt
	token.ExpiresAt, err = parseOptionalStoredTime(expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse auth token expires_at: %w", err)
	}
	token.RevokedAt, err = parseOptionalStoredTime(revokedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse auth token revoked_at: %w", err)
	}
	token.LastUsedAt, err = parseOptionalStoredTime(lastUsedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse auth token last_used_at: %w", err)
	}
	return &token, nil
}
