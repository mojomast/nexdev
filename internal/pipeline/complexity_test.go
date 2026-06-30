package pipeline

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
)

func TestComplexityStageDeterministicLevels(t *testing.T) {
	for _, tc := range []struct {
		name      string
		interview contract.InterviewData
		repo      contract.RepoAnalysis
		wantLevel string
	}{
		{name: "trivial", interview: interviewWithRequirements("Rename a label"), wantLevel: "trivial"},
		{name: "small", interview: interviewWithRequirements("Add CLI flag", "Update docs", "Add tests"), repo: contract.RepoAnalysis{TestCommands: []string{"go test ./..."}}, wantLevel: "small"},
		{name: "medium", interview: interviewWithRequirements("Add CLI", "Update docs", "Add tests", "Improve logging"), repo: contract.RepoAnalysis{Languages: []string{"go", "typescript"}, Frameworks: []string{"cobra", "react", "vite"}, RiskNotes: []string{"prompt warning"}}, wantLevel: "medium"},
		{name: "large", interview: interviewWithRequirements("Add auth", "Add API", "Add DB migration"), repo: contract.RepoAnalysis{Languages: []string{"go", "typescript"}, Frameworks: []string{"cobra", "react", "vite"}, RiskNotes: []string{"risk 1"}}, wantLevel: "large"},
		{name: "epic", interview: interviewWithRequirements("Add auth", "Add API", "Add DB migration", "Add audit", "Add security review", "Add control plane", "Add SSE"), repo: contract.RepoAnalysis{Languages: []string{"go", "typescript"}, Frameworks: []string{"cobra", "react", "vite"}, RiskNotes: []string{"risk 1", "risk 2", "risk 3", "risk 4"}}, wantLevel: "epic"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stage := NewComplexityStage(provider.StructuredClient{}, ComplexityStageConfig{Interview: tc.interview, RepoAnalysis: tc.repo, ProjectRoot: t.TempDir()})
			if err := stage.Run(context.Background(), StageEnv{Project: testRepoAnalyzeProject{id: "proj_" + tc.name}}); err != nil {
				t.Fatalf("ComplexityStage.Run failed: %v", err)
			}
			if got := stage.Profile().Level; got != tc.wantLevel {
				t.Fatalf("level = %q, want %q; profile=%#v", got, tc.wantLevel, stage.Profile())
			}
		})
	}
}

func TestComplexityStageProviderRefinementCannotLowerVerificationFloor(t *testing.T) {
	fake := provider.NewFakeProvider(provider.WithFakeScripts([]provider.FakeScript{{
		Name: "complexity-refine",
		Responses: []provider.FakeResponse{{Content: `{
			"score":1,
			"level":"trivial",
			"recommended_phases":1,
			"risk_factors":[],
			"suggested_voices":["product"],
			"suggested_tests":["manual smoke test"],
			"rationale":"model attempted to simplify"
		}`}},
	}}))
	interview := interviewWithRequirements("Add auth", "Add API", "Add DB migration")
	repo := contract.RepoAnalysis{Languages: []string{"go", "typescript"}, TestCommands: []string{"go test ./..."}, LintCommands: []string{"go vet ./..."}, RiskNotes: []string{"security warning"}}
	stage := NewComplexityStage(newInterviewComplexityClient(t, fake), ComplexityStageConfig{Interview: interview, RepoAnalysis: repo, ProjectRoot: t.TempDir(), UseProviderRefine: true})

	if err := stage.Run(context.Background(), StageEnv{Project: testRepoAnalyzeProject{id: "proj_refine"}}); err != nil {
		t.Fatalf("ComplexityStage.Run failed: %v", err)
	}
	profile := stage.Profile()
	if profile.Level == "trivial" || profile.Score <= 1 {
		t.Fatalf("provider lowered deterministic complexity: %#v", profile)
	}
	assertContains(t, profile.SuggestedTests, "go test ./...")
	assertContains(t, profile.SuggestedTests, "go vet ./...")
	assertContains(t, profile.SuggestedTests, "manual smoke test")
	if calls := fake.Calls(); len(calls) != 1 || !strings.Contains(calls[0].Prompt, "deterministic_suggested_tests") {
		t.Fatalf("unexpected refinement prompt calls: %#v", calls)
	}
}

func TestComplexityStageRepairsInvalidStructuredOutput(t *testing.T) {
	fake := provider.NewFakeProvider(provider.WithFakeScripts([]provider.FakeScript{{
		Name: "complexity-repair",
		Responses: []provider.FakeResponse{
			{Content: `not json`},
			{Content: `{"score":4,"level":"small","recommended_phases":2,"risk_factors":[],"suggested_voices":["implementation"],"suggested_tests":["go test ./..."],"rationale":"fixed"}`},
		},
	}}))
	stage := NewComplexityStage(newInterviewComplexityClient(t, fake), ComplexityStageConfig{Interview: interviewWithRequirements("Add tests"), RepoAnalysis: contract.RepoAnalysis{TestCommands: []string{"go test ./..."}}, ProjectRoot: t.TempDir(), UseProviderRefine: true, MaxRepairAttempts: 1})
	if err := stage.Run(context.Background(), StageEnv{Project: testRepoAnalyzeProject{id: "proj_complexity_repair"}}); err != nil {
		t.Fatalf("ComplexityStage.Run failed: %v", err)
	}
	if calls := fake.Calls(); len(calls) != 2 || !strings.Contains(calls[1].Prompt, "decode structured JSON") {
		t.Fatalf("expected repair prompt, got %#v", calls)
	}
}

func TestComplexityStageWritesArtifactAndIndexes(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store := newRepoAnalyzeStore(t)
	projectID := "proj_complexity_artifact"
	runID := "run_complexity_artifact"
	createStageProjectAndRun(t, ctx, store, projectID, runID)
	stage := NewComplexityStage(provider.StructuredClient{}, ComplexityStageConfig{Interview: interviewWithRequirements("Add tests"), RepoAnalysis: contract.RepoAnalysis{TestCommands: []string{"go test ./..."}}, ProjectRoot: root})
	env := StageEnv{Project: testRepoAnalyzeProject{id: projectID}, Run: testRepoAnalyzeRun{id: runID}, Store: store}

	if err := stage.Run(ctx, env); err != nil {
		t.Fatalf("ComplexityStage.Run failed: %v", err)
	}
	artifactData := readStageArtifact(t, root, complexityArtifactRelPath)
	var artifact contract.ComplexityProfile
	if err := json.Unmarshal(artifactData, &artifact); err != nil {
		t.Fatalf("complexity artifact invalid JSON: %v", err)
	}
	if artifact.Score <= 0 || artifact.Level == "" {
		t.Fatalf("unexpected artifact profile: %#v", artifact)
	}
	artifacts, err := store.ListArtifacts(ctx, state.ArtifactListOptions{ProjectID: projectID, RunID: runID, Kind: string(contract.ArtifactKindComplexityProfile)})
	if err != nil {
		t.Fatalf("ListArtifacts failed: %v", err)
	}
	if len(artifacts) != 1 || artifacts[0].Path != complexityArtifactRelPath || artifacts[0].SHA256 == "" {
		t.Fatalf("unexpected artifact index: %#v", artifacts)
	}
}

func TestComplexityStageInterfaceValidateAndResume(t *testing.T) {
	var _ PipelineStage = NewComplexityStage(provider.StructuredClient{}, ComplexityStageConfig{})
	var _ StageOutputter = NewComplexityStage(provider.StructuredClient{}, ComplexityStageConfig{})
	stage := NewComplexityStage(provider.StructuredClient{}, ComplexityStageConfig{Interview: interviewWithRequirements("Add tests"), ProjectRoot: t.TempDir()})
	if stage.Name() != StageComplexity {
		t.Fatalf("unexpected stage name: %s", stage.Name())
	}
	if err := stage.Validate(context.Background(), StageEnv{}); err == nil {
		t.Fatal("Validate without project succeeded")
	}
	env := StageEnv{Project: testRepoAnalyzeProject{id: "proj_complexity_resume"}}
	if err := stage.Resume(context.Background(), env); err != nil {
		t.Fatalf("Resume failed: %v", err)
	}
	output, err := stage.Output(context.Background(), env)
	if err != nil {
		t.Fatalf("Output failed: %v", err)
	}
	if _, ok := output["score"]; !ok {
		t.Fatalf("expected score in output, got %#v", output)
	}
}

func interviewWithRequirements(requirements ...string) contract.InterviewData {
	return contract.InterviewData{Requirements: requirements, RiskTolerance: "medium", RawTranscript: strings.Join(requirements, "; ")}
}
