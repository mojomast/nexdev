package state

import (
	"context"
	"testing"
	"time"
)

func TestStoreAuthTokenRepositoryCreateReadListRevokeTouch(t *testing.T) {
	store := newRunArtifactTestStore(t)
	ctx := context.Background()

	createdAt := time.Date(2026, 6, 30, 8, 0, 0, 123, time.FixedZone("offset", -5*60*60))
	expiresAt := time.Date(2026, 7, 30, 8, 0, 0, 456, time.UTC)
	if err := store.CreateAuthToken(ctx, &AuthToken{
		ID:        "tok_first",
		TokenHash: "hash-first",
		Role:      "operator",
		Name:      "ops token",
		CreatedAt: createdAt,
		ExpiresAt: &expiresAt,
	}); err != nil {
		t.Fatalf("CreateAuthToken first failed: %v", err)
	}
	if err := store.CreateAuthToken(ctx, &AuthToken{
		ID:        "tok_second",
		TokenHash: "hash-second",
		Role:      "observer",
		CreatedAt: createdAt.Add(time.Hour),
	}); err != nil {
		t.Fatalf("CreateAuthToken second failed: %v", err)
	}

	loaded, err := store.GetAuthToken(ctx, "tok_first")
	if err != nil {
		t.Fatalf("GetAuthToken failed: %v", err)
	}
	if loaded.TokenHash != "hash-first" || loaded.Role != "operator" || loaded.Name != "ops token" {
		t.Fatalf("loaded auth token mismatch: %+v", loaded)
	}
	if got, want := loaded.CreatedAt.Format(time.RFC3339Nano), createdAt.UTC().Format(time.RFC3339Nano); got != want {
		t.Fatalf("CreatedAt = %s, want %s", got, want)
	}
	if loaded.ExpiresAt == nil || loaded.ExpiresAt.Format(time.RFC3339Nano) != expiresAt.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("ExpiresAt = %v, want %s", loaded.ExpiresAt, expiresAt.UTC().Format(time.RFC3339Nano))
	}

	byHash, err := store.GetAuthTokenByHash(ctx, "hash-first")
	if err != nil {
		t.Fatalf("GetAuthTokenByHash failed: %v", err)
	}
	if byHash.ID != "tok_first" {
		t.Fatalf("GetAuthTokenByHash ID = %s, want tok_first", byHash.ID)
	}

	lastUsedAt := time.Date(2026, 6, 30, 12, 0, 0, 789, time.FixedZone("offset", 3*60*60))
	if err := store.TouchAuthTokenLastUsed(ctx, "tok_first", lastUsedAt); err != nil {
		t.Fatalf("TouchAuthTokenLastUsed failed: %v", err)
	}
	revokedAt := time.Date(2026, 6, 30, 13, 0, 0, 987, time.UTC)
	if err := store.RevokeAuthToken(ctx, "tok_first", revokedAt); err != nil {
		t.Fatalf("RevokeAuthToken failed: %v", err)
	}
	loaded, err = store.GetAuthToken(ctx, "tok_first")
	if err != nil {
		t.Fatalf("GetAuthToken after update failed: %v", err)
	}
	if loaded.LastUsedAt == nil || loaded.LastUsedAt.Format(time.RFC3339Nano) != lastUsedAt.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("LastUsedAt = %v, want %s", loaded.LastUsedAt, lastUsedAt.UTC().Format(time.RFC3339Nano))
	}
	if loaded.RevokedAt == nil || loaded.RevokedAt.Format(time.RFC3339Nano) != revokedAt.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("RevokedAt = %v, want %s", loaded.RevokedAt, revokedAt.UTC().Format(time.RFC3339Nano))
	}

	tokens, err := store.ListAuthTokens(ctx)
	if err != nil {
		t.Fatalf("ListAuthTokens failed: %v", err)
	}
	if len(tokens) != 2 || tokens[0].ID != "tok_first" || tokens[1].ID != "tok_second" {
		t.Fatalf("auth token order = %v", authTokenIDs(tokens))
	}
}

func TestStoreAuthTokenRepositoryRejectsDuplicateHash(t *testing.T) {
	store := newRunArtifactTestStore(t)
	ctx := context.Background()
	if err := store.CreateAuthToken(ctx, &AuthToken{ID: "tok_one", TokenHash: "same-hash", Role: "admin"}); err != nil {
		t.Fatalf("CreateAuthToken first failed: %v", err)
	}
	if err := store.CreateAuthToken(ctx, &AuthToken{ID: "tok_two", TokenHash: "same-hash", Role: "admin"}); err == nil {
		t.Fatal("CreateAuthToken duplicate hash succeeded, want unique failure")
	}
}

func authTokenIDs(tokens []*AuthToken) []string {
	ids := make([]string, 0, len(tokens))
	for _, token := range tokens {
		ids = append(ids, token.ID)
	}
	return ids
}
