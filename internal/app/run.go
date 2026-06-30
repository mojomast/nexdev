package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/controlplane"
	"github.com/mojomast/nexdev/internal/executor"
	"github.com/mojomast/nexdev/internal/observability"
	"github.com/mojomast/nexdev/internal/pipeline"
	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
)

type RunRequest struct {
	Prompt       string
	FromStage    string
	Stage        string
	Yes          bool
	Cheap        bool
	Brrrr        bool
	FakeProvider bool
}

type RunResult struct {
	ProjectID string            `json:"project_id"`
	RunID     string            `json:"run_id"`
	Status    string            `json:"status"`
	Artifacts []state.Artifact  `json:"artifacts"`
	Events    int               `json:"event_count"`
	Paths     map[string]string `json:"paths"`
}

const (
	appChangedFilesArtifactRelPath = ".nexdev/artifacts/changed_files.json"
	appRunSummaryArtifactRelPath   = ".nexdev/artifacts/run_summary.json"
)

type RunStarterService struct {
	runtime *Runtime
}

func (r *Runtime) RunFakeProvider(ctx context.Context, req RunRequest) (*RunResult, error) {
	if !req.FakeProvider {
		return nil, fmt.Errorf("fake provider mode must be explicit")
	}
	starter := &RunStarterService{runtime: r}
	run, err := starter.StartRun(ctx, controlplane.StartRunRequest{Prompt: req.Prompt, FromStage: req.FromStage, Stage: req.Stage, Yes: req.Yes, Cheap: req.Cheap, Brrrr: req.Brrrr})
	if err != nil {
		return nil, err
	}
	return r.runResult(ctx, run.ID)
}

func (s *RunStarterService) StartRun(ctx context.Context, req controlplane.StartRunRequest) (*state.Run, error) {
	if s == nil || s.runtime == nil {
		return nil, fmt.Errorf("runtime is required")
	}
	r := s.runtime
	runID, err := r.nextRunID(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	run := &state.Run{ID: runID, ProjectID: r.ProjectID, Status: "pending", StartedAt: now, Metadata: map[string]any{"fake_provider": true, "prompt": req.Prompt}}
	if err := r.Store.CreateRun(ctx, run); err != nil {
		return nil, err
	}
	if err := r.persistRunStarted(ctx, run, req); err != nil {
		return nil, err
	}
	if err := r.runPipeline(ctx, run, req); err != nil {
		return nil, err
	}
	return r.Store.GetRun(ctx, run.ID)
}

func (r *Runtime) runPipeline(ctx context.Context, run *state.Run, req controlplane.StartRunRequest) error {
	client, err := r.fakeStructuredClient(run.ID)
	if err != nil {
		return err
	}
	var repo contract.RepoAnalysis
	var interview contract.InterviewData
	var complexity contract.ComplexityProfile
	var designMarkdown string
	var synthesis contract.HivemindSynthesis
	var validation contract.ValidationReport
	var phases []contract.PhaseSketch

	runner, err := pipeline.NewRunner(r.Store, pipeline.WithIDGenerator(sequenceID()), pipeline.WithPrerequisiteProvider(fakePrerequisites{}))
	if err != nil {
		return err
	}
	register := func(stage pipeline.PipelineStage) error {
		return runner.Register(stage)
	}
	if err := register(runStageFunc{name: pipeline.StageRepoAnalyze, run: func(ctx context.Context, env pipeline.StageEnv) error {
		stage := pipeline.NewRepoAnalyzeStage(r.ProjectRoot)
		if err := stage.Run(ctx, env); err != nil {
			return err
		}
		repo = stage.Analysis()
		return nil
	}, out: func(context.Context, pipeline.StageEnv) (map[string]any, error) { return structToMap(repo) }}); err != nil {
		return err
	}
	request := strings.TrimSpace(req.Prompt)
	if request == "" {
		request = "Run the deterministic Nexdev fake-provider E2E pipeline."
	}
	if err := register(runStageFunc{name: pipeline.StageInterview, run: func(ctx context.Context, env pipeline.StageEnv) error {
		stage := pipeline.NewInterviewStage(client, pipeline.InterviewStageConfig{Request: request, RepoAnalysis: repo, YesMode: true, CI: true, ArtifactProjectRoot: r.ProjectRoot})
		if err := stage.Run(ctx, env); err != nil {
			return err
		}
		interview = stage.Data()
		return nil
	}, out: func(context.Context, pipeline.StageEnv) (map[string]any, error) { return structToMap(interview) }}); err != nil {
		return err
	}
	if err := register(runStageFunc{name: pipeline.StageComplexity, run: func(ctx context.Context, env pipeline.StageEnv) error {
		stage := pipeline.NewComplexityStage(client, pipeline.ComplexityStageConfig{Interview: interview, RepoAnalysis: repo, ProjectRoot: r.ProjectRoot, UseProviderRefine: false})
		if err := stage.Run(ctx, env); err != nil {
			return err
		}
		complexity = stage.Profile()
		return nil
	}, out: func(context.Context, pipeline.StageEnv) (map[string]any, error) { return structToMap(complexity) }}); err != nil {
		return err
	}
	if err := register(runStageFunc{name: pipeline.StageDesign, run: func(ctx context.Context, env pipeline.StageEnv) error {
		stage := pipeline.NewDesignStage(client, pipeline.DesignStageConfig{Interview: interview, RepoAnalysis: repo, Complexity: complexity, ProjectRoot: r.ProjectRoot, MaxIterations: 1})
		if err := stage.Run(ctx, env); err != nil {
			return err
		}
		designMarkdown = stage.Result().DesignMarkdown
		return nil
	}, out: func(context.Context, pipeline.StageEnv) (map[string]any, error) {
		return map[string]any{"design_markdown": designMarkdown}, nil
	}}); err != nil {
		return err
	}
	if err := register(runStageFunc{name: pipeline.StageHivemind, run: func(ctx context.Context, env pipeline.StageEnv) error {
		stage := pipeline.NewHivemindStage(client, pipeline.HivemindStageConfig{Interview: interview, RepoAnalysis: repo, Complexity: complexity, DesignMarkdown: designMarkdown, ProjectRoot: r.ProjectRoot, Voices: []string{"skeptic", "security", "test"}, Parallel: false, Cycle: 1})
		if err := stage.Run(ctx, env); err != nil {
			return err
		}
		synthesis = stage.Result().Synthesis
		return nil
	}, out: func(context.Context, pipeline.StageEnv) (map[string]any, error) { return structToMap(synthesis) }}); err != nil {
		return err
	}
	if err := register(runStageFunc{name: pipeline.StageValidate, run: func(ctx context.Context, env pipeline.StageEnv) error {
		stage := pipeline.NewValidateStage(client, pipeline.ValidateStageConfig{Interview: interview, RepoAnalysis: repo, Complexity: complexity, DesignMarkdown: designMarkdown, HivemindSynthesis: synthesis, ProjectRoot: r.ProjectRoot})
		if err := stage.Run(ctx, env); err != nil {
			return err
		}
		validation = stage.Report()
		return nil
	}, out: func(context.Context, pipeline.StageEnv) (map[string]any, error) { return structToMap(validation) }}); err != nil {
		return err
	}
	if err := register(runStageFunc{name: pipeline.StagePlanSketch, run: func(ctx context.Context, env pipeline.StageEnv) error {
		stage := pipeline.NewPlanSketchStage(client, pipeline.PlanSketchStageConfig{Interview: interview, RepoAnalysis: repo, Complexity: complexity, DesignMarkdown: designMarkdown, ValidationReport: validation, ProjectRoot: r.ProjectRoot})
		if err := stage.Run(ctx, env); err != nil {
			return err
		}
		phases = stage.Phases()
		return nil
	}, out: func(context.Context, pipeline.StageEnv) (map[string]any, error) {
		return map[string]any{"phases": phases}, nil
	}}); err != nil {
		return err
	}
	if err := register(runStageFunc{name: pipeline.StagePlanDetail, run: func(ctx context.Context, env pipeline.StageEnv) error {
		stage := pipeline.NewPlanDetailStage(client, pipeline.PlanDetailStageConfig{Interview: interview, RepoAnalysis: repo, Complexity: complexity, DesignMarkdown: designMarkdown, Phases: phases, ProjectRoot: r.ProjectRoot, PlanVersion: 1})
		return stage.Run(ctx, env)
	}}); err != nil {
		return err
	}
	if err := register(pipeline.NewReviewStage(pipeline.ReviewStageConfig{Mode: pipeline.ReviewModeAuto, Actor: "fake-provider-e2e", ProjectRoot: r.ProjectRoot})); err != nil {
		return err
	}
	if err := register(pipeline.NewDevelopStage(pipeline.DevelopStageConfig{ProjectRoot: r.ProjectRoot, Worker: fakeE2EWorker()})); err != nil {
		return err
	}
	if err := register(pipeline.NewVerifyStage(pipeline.VerifyStageConfig{ProjectRoot: r.ProjectRoot})); err != nil {
		return err
	}
	if err := register(pipeline.NewHandoffStage(pipeline.HandoffStageConfig{ProjectRoot: r.ProjectRoot, Request: request})); err != nil {
		return err
	}
	if err := register(pipeline.NewCompleteStage()); err != nil {
		return err
	}

	env := pipeline.StageEnv{Project: projectRef{id: r.ProjectID}, Run: runRef{id: run.ID}, Store: r.Store, Config: r.Config}
	from := pipeline.StageRepoAnalyze
	if req.FromStage != "" {
		from = pipeline.Stage(req.FromStage)
	}
	opts := pipeline.RunOptions{RunID: run.ID, From: from}
	if req.Stage != "" {
		opts.SingleStage = pipeline.Stage(req.Stage)
	}
	if err := runner.Run(ctx, env, opts); err != nil {
		return err
	}
	return r.rewriteCompletedRunSummary(ctx, run.ID)
}

func (r *Runtime) fakeStructuredClient(runID string) (provider.StructuredClient, error) {
	registry := map[string]provider.ProviderFactory{provider.FakeProviderName: func() provider.Provider { return provider.NewFakeProvider() }}
	router, err := provider.NewRouterWithRegistry(provider.Selection{Provider: provider.FakeProviderName, Model: "fake-model"}, nil, registry)
	if err != nil {
		return provider.StructuredClient{}, err
	}
	fake := provider.NewFakeProvider(provider.WithFakeScripts(fakeScripts()))
	recorder := observability.NewUsageRecorder(observability.UsageRecorderConfig{Store: r.Store, ProjectID: r.ProjectID, RunID: runID, AuditCalls: true})
	return provider.StructuredClient{Router: router, Providers: map[string]provider.Provider{provider.FakeProviderName: fake}, Recorder: recorder}, nil
}

func fakeScripts() []provider.FakeScript {
	design := "## Product behavior\nFake local pipeline completion.\n## User flows\nCLI runs headless.\n## System boundaries\nNo network or shell.\n## Data model\nSQLite state and artifacts.\n## API/CLI/TUI changes\nCLI emits JSON.\n## Execution model\nFake provider and fake worker.\n## Security and privacy constraints\nSecrets are redacted.\n## Failure modes and rollback\nFailures persist stage state.\n## Verification strategy\nVerify artifacts and replay.\n## Migration/backward compatibility\nNo destructive migration.\n"
	voice := func(name string) string {
		return fmt.Sprintf(`{"voice":%q,"findings":[],"severity":"low","verdict":"approve","confidence":1}`, name)
	}
	return []provider.FakeScript{
		{Name: "interview", PromptContains: "Nexdev's interview stage", Responses: []provider.FakeResponse{{Content: `{"requirements":["Complete a deterministic fake-provider E2E run"],"constraints":["No external network","No real providers","Redact secrets"],"open_questions":[],"user_personas":["CI"],"non_goals":["Real provider smoke"],"acceptance_signals":["Run reaches complete","Artifacts exist","SSE replay works"],"risk_tolerance":"low","target_users":["developers"],"raw_transcript":"fake-provider e2e"}`, TokensInput: 10, TokensOutput: 20}}},
		{Name: "design", PromptContains: "Nexdev's design stage", Responses: []provider.FakeResponse{{Content: fmt.Sprintf(`{"design_markdown":%q,"critique":{"findings":[]},"actionable":false,"metadata":{"summary":"fake e2e design","assumptions":["local only"],"open_risks":[],"provider_note":"fake"}}`, design), TokensInput: 10, TokensOutput: 30}}},
		{Name: "synthesis", PromptContains: "hivemind synthesis stage", Responses: []provider.FakeResponse{{Content: `{"consensus_findings":[],"required_changes":[],"optional_changes":[],"disagreements":[],"final_verdict":"approve"}`}}},
		{Name: "voice-skeptic", PromptContains: "voice=skeptic", Responses: []provider.FakeResponse{{Content: voice("skeptic")}}},
		{Name: "voice-security", PromptContains: "voice=security", Responses: []provider.FakeResponse{{Content: voice("security")}}},
		{Name: "voice-test", PromptContains: "voice=test", Responses: []provider.FakeResponse{{Content: voice("test")}}},
		{Name: "validate", PromptContains: "validation stage", Responses: []provider.FakeResponse{{Content: `{"ambiguities":[],"conflicts":[],"missing_prereqs":[],"blockers":[],"hallucination_risks":[],"verdict":"pass"}`}}},
		{Name: "plan-sketch", PromptContains: "plan_sketch stage", Responses: []provider.FakeResponse{{Content: `[{"id":"phase_001","number":1,"title":"Fake E2E Implementation","description":"Exercise the deterministic fake-provider pipeline","estimated_complexity":"small","goals":["write safe fixture file"],"risks":[]}]`}}},
		{Name: "plan-detail", PromptContains: "plan_detail stage", Responses: []provider.FakeResponse{{Content: `[{"id":"T1.01","phase_id":"phase_001","title":"Write fake E2E output","description":"Create a deterministic project file through the fake worker","expected_files":["generated/fake_e2e.txt"],"dependencies":[],"acceptance_criteria":["file is written safely"],"test_commands":[],"risk_level":"low","required_tools":["write_file"],"notes":["fake-provider e2e"]}]`}}},
	}
}

func fakeE2EWorker() executor.FakeWorker {
	return executor.FakeWorker{Progress: map[string][]string{"T1.01": []string{"fake worker starting"}}, Writes: map[string][]executor.FakeWrite{"T1.01": []executor.FakeWrite{{Path: "generated/fake_e2e.txt", Content: "nexdev fake-provider e2e\n"}}}}
}

func (r *Runtime) persistRunStarted(ctx context.Context, run *state.Run, req controlplane.StartRunRequest) error {
	data, err := json.Marshal(map[string]any{"fake_provider": true, "from_stage": req.FromStage, "stage": req.Stage})
	if err != nil {
		return err
	}
	_, err = r.Store.PersistEvent(ctx, contract.EventEnvelope{EventID: "evt_" + run.ID + "_started", ProjectID: r.ProjectID, RunID: run.ID, Type: contract.EventTypeRunStarted, Source: contract.EventSourceCore, Timestamp: run.StartedAt, Payload: data})
	return err
}

func (r *Runtime) nextRunID(ctx context.Context) (string, error) {
	runs, err := r.Store.ListRunsByProject(ctx, r.ProjectID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("run_fake_%03d", len(runs)+1), nil
}

func (r *Runtime) runResult(ctx context.Context, runID string) (*RunResult, error) {
	run, err := r.Store.GetRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	artifacts, err := r.Store.ListArtifacts(ctx, state.ArtifactListOptions{ProjectID: r.ProjectID, RunID: runID})
	if err != nil {
		return nil, err
	}
	events, err := r.Store.ListEvents(ctx, state.EventListOptions{RunID: runID})
	if err != nil {
		return nil, err
	}
	return &RunResult{ProjectID: r.ProjectID, RunID: runID, Status: run.Status, Artifacts: derefArtifacts(artifacts), Events: len(events), Paths: map[string]string{"state_dir": r.StateDir, "artifacts_dir": filepath.Join(r.ProjectRoot, ".nexdev", "artifacts")}}, nil
}

func (r *Runtime) rewriteCompletedRunSummary(ctx context.Context, runID string) error {
	run, err := r.Store.GetRun(ctx, runID)
	if err != nil {
		return err
	}
	stages, err := r.Store.ListStageRunsByRun(ctx, runID)
	if err != nil {
		return err
	}
	changed := readChangedFiles(filepath.Join(r.ProjectRoot, appChangedFilesArtifactRelPath))
	summary := contract.RunSummary{ProjectID: r.ProjectID, RunID: runID, Status: run.Status, StartedAt: run.StartedAt.UTC().Format(time.RFC3339Nano), ChangedFiles: changed}
	if run.CompletedAt != nil {
		summary.CompletedAt = run.CompletedAt.UTC().Format(time.RFC3339Nano)
	}
	for _, stage := range stages {
		item := contract.StageSummary{Stage: stage.Stage, Status: stage.Status}
		if stage.CompletedAt != nil {
			item.CompletedAt = stage.CompletedAt.UTC().Format(time.RFC3339Nano)
		}
		summary.Stages = append(summary.Stages, item)
	}
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := filepath.Join(r.ProjectRoot, appRunSummaryArtifactRelPath)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	hash := sha256.Sum256(data)
	return r.Store.UpsertArtifact(ctx, &state.Artifact{ID: artifactID(r.ProjectID, runID, string(contract.ArtifactKindRunSummary), appRunSummaryArtifactRelPath), ProjectID: r.ProjectID, RunID: runID, Kind: string(contract.ArtifactKindRunSummary), Path: appRunSummaryArtifactRelPath, SHA256: hex.EncodeToString(hash[:]), Version: 1, Metadata: map[string]any{"stage": string(pipeline.StageHandoff)}, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()})
}

type runStageFunc struct {
	name pipeline.Stage
	run  func(context.Context, pipeline.StageEnv) error
	out  func(context.Context, pipeline.StageEnv) (map[string]any, error)
}

func (s runStageFunc) Name() pipeline.Stage                                      { return s.name }
func (s runStageFunc) Validate(ctx context.Context, env pipeline.StageEnv) error { return ctx.Err() }
func (s runStageFunc) Run(ctx context.Context, env pipeline.StageEnv) error      { return s.run(ctx, env) }
func (s runStageFunc) Resume(ctx context.Context, env pipeline.StageEnv) error {
	return s.Run(ctx, env)
}
func (s runStageFunc) Output(ctx context.Context, env pipeline.StageEnv) (map[string]any, error) {
	if s.out == nil {
		return map[string]any{}, ctx.Err()
	}
	return s.out(ctx, env)
}

type fakePrerequisites struct{}

func (fakePrerequisites) Snapshot(ctx context.Context, env pipeline.StageEnv, target pipeline.Stage) (pipeline.PrerequisiteSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return pipeline.PrerequisiteSnapshot{}, err
	}
	return pipeline.NewPrerequisiteSnapshot(
		pipeline.RequirementProjectExists,
		pipeline.RequirementInterviewDataExists,
		pipeline.RequirementRepoAnalysisExists,
		pipeline.RequirementDesignDraftExists,
		pipeline.RequirementLatestHivemindSynthesisExists,
		pipeline.RequirementValidationPassedOrWarningsAccepted,
		pipeline.RequirementPhaseSketchExists,
		pipeline.RequirementDetailedPlanExists,
		pipeline.RequirementReviewedApprovedPlanExists,
		pipeline.RequirementDevelopHasNoRunningTasks,
		pipeline.RequirementVerifyCompleteOrExplicitlySkipped,
		pipeline.RequirementHandoffExists,
		pipeline.RequirementAllRequiredReportsExist,
	), nil
}

type projectRef struct{ id string }

func (p projectRef) ProjectID() string { return p.id }

type runRef struct{ id string }

func (r runRef) RunID() string { return r.id }

func sequenceID() func(string) string {
	var n uint64
	return func(prefix string) string { return fmt.Sprintf("%s_fake_%04d", prefix, atomic.AddUint64(&n, 1)) }
}

func structToMap(v any) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func derefArtifacts(in []*state.Artifact) []state.Artifact {
	out := make([]state.Artifact, 0, len(in))
	for _, item := range in {
		if item != nil {
			out = append(out, *item)
		}
	}
	return out
}

func artifactID(projectID, runID, kind, relPath string) string {
	sum := sha256.Sum256([]byte(projectID + ":" + runID + ":" + kind))
	return "artifact_" + hex.EncodeToString(sum[:8])
}

func readChangedFiles(path string) []contract.ChangedFile {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var wrapper struct {
		ChangedFiles []contract.ChangedFile `json:"changed_files"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil
	}
	return wrapper.ChangedFiles
}
