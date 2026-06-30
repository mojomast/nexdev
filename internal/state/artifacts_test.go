package state

import (
	"context"
	"testing"
	"time"
)

func TestStoreArtifactRepositoryUpsertGetList(t *testing.T) {
	store := newRunArtifactTestStore(t)
	seedStateCRun(t, store, "proj_artifacts", "run_artifacts")
	ctx := context.Background()

	createdAt := time.Date(2026, 6, 30, 16, 0, 0, 9, time.FixedZone("offset", -4*60*60))
	updatedAt := time.Date(2026, 6, 30, 16, 5, 0, 10, time.UTC)
	if err := store.UpsertArtifact(ctx, &Artifact{
		ID:        "artifact_design",
		ProjectID: "proj_artifacts",
		RunID:     "run_artifacts",
		Kind:      "design_draft",
		Path:      ".nexdev/artifacts/design_draft.md",
		SHA256:    "abc123",
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Metadata:  map[string]any{"stage": "design", "nested": map[string]any{"voice_count": float64(3)}},
	}); err != nil {
		t.Fatalf("UpsertArtifact failed: %v", err)
	}
	if err := store.UpsertArtifact(ctx, &Artifact{
		ID:        "artifact_global",
		ProjectID: "proj_artifacts",
		Kind:      "repo_analysis",
		Path:      ".nexdev/artifacts/repo_analysis.json",
		CreatedAt: createdAt.Add(-time.Minute),
		UpdatedAt: updatedAt.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("UpsertArtifact global failed: %v", err)
	}

	loaded, err := store.GetArtifact(ctx, "artifact_design")
	if err != nil {
		t.Fatalf("GetArtifact failed: %v", err)
	}
	if loaded.CreatedAt.Location() != time.UTC || loaded.UpdatedAt.Location() != time.UTC {
		t.Fatalf("artifact timestamps are not UTC: created=%v updated=%v", loaded.CreatedAt.Location(), loaded.UpdatedAt.Location())
	}
	if got, want := loaded.CreatedAt.Format(time.RFC3339Nano), createdAt.UTC().Format(time.RFC3339Nano); got != want {
		t.Fatalf("CreatedAt = %s, want %s", got, want)
	}
	if loaded.Version != 1 || loaded.SHA256 != "abc123" {
		t.Fatalf("loaded artifact = %+v", loaded)
	}
	if nested, ok := loaded.Metadata["nested"].(map[string]any); !ok || nested["voice_count"] != float64(3) {
		t.Fatalf("metadata round-trip mismatch: %#v", loaded.Metadata)
	}

	upsertUpdatedAt := updatedAt.Add(time.Hour)
	if err := store.UpsertArtifact(ctx, &Artifact{
		ID:        "artifact_design",
		ProjectID: "proj_artifacts",
		RunID:     "run_artifacts",
		Kind:      "design_draft",
		Path:      ".nexdev/artifacts/design_draft_v2.md",
		SHA256:    "def456",
		Version:   2,
		CreatedAt: createdAt.Add(-time.Hour),
		UpdatedAt: upsertUpdatedAt,
		Metadata:  map[string]any{"stage": "design", "revision": float64(2)},
	}); err != nil {
		t.Fatalf("UpsertArtifact update failed: %v", err)
	}
	loaded, err = store.GetArtifact(ctx, "artifact_design")
	if err != nil {
		t.Fatalf("GetArtifact after update failed: %v", err)
	}
	if loaded.Path != ".nexdev/artifacts/design_draft_v2.md" || loaded.SHA256 != "def456" || loaded.Version != 2 {
		t.Fatalf("updated artifact = %+v", loaded)
	}
	if loaded.CreatedAt.Format(time.RFC3339Nano) != createdAt.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("upsert changed created_at to %s", loaded.CreatedAt.Format(time.RFC3339Nano))
	}
	if loaded.Metadata["revision"] != float64(2) {
		t.Fatalf("metadata update mismatch: %#v", loaded.Metadata)
	}

	all, err := store.ListArtifacts(ctx, ArtifactListOptions{ProjectID: "proj_artifacts"})
	if err != nil {
		t.Fatalf("ListArtifacts all failed: %v", err)
	}
	assertArtifactIDs(t, all, []string{"artifact_global", "artifact_design"})

	byRun, err := store.ListArtifacts(ctx, ArtifactListOptions{ProjectID: "proj_artifacts", RunID: "run_artifacts"})
	if err != nil {
		t.Fatalf("ListArtifacts by run failed: %v", err)
	}
	assertArtifactIDs(t, byRun, []string{"artifact_design"})

	byKind, err := store.ListArtifacts(ctx, ArtifactListOptions{ProjectID: "proj_artifacts", Kind: "repo_analysis"})
	if err != nil {
		t.Fatalf("ListArtifacts by kind failed: %v", err)
	}
	assertArtifactIDs(t, byKind, []string{"artifact_global"})
}

func TestStoreArtifactForeignKeys(t *testing.T) {
	store := newRunArtifactTestStore(t)
	ctx := context.Background()

	if err := store.UpsertArtifact(ctx, &Artifact{ID: "artifact_missing_project", ProjectID: "missing", Kind: "handoff", Path: ".nexdev/artifacts/handoff.md"}); err == nil {
		t.Fatal("UpsertArtifact with missing project succeeded, want FK failure")
	}
	seedStateCProject(t, store, "proj_artifact_fk")
	if err := store.UpsertArtifact(ctx, &Artifact{ID: "artifact_missing_run", ProjectID: "proj_artifact_fk", RunID: "missing", Kind: "handoff", Path: ".nexdev/artifacts/handoff.md"}); err == nil {
		t.Fatal("UpsertArtifact with missing run succeeded, want FK failure")
	}
}

func assertArtifactIDs(t *testing.T, artifacts []*Artifact, want []string) {
	t.Helper()
	if len(artifacts) != len(want) {
		t.Fatalf("artifact count = %d, want %d", len(artifacts), len(want))
	}
	for i, artifact := range artifacts {
		if artifact.ID != want[i] {
			t.Fatalf("artifact[%d] = %s, want %s", i, artifact.ID, want[i])
		}
	}
}
