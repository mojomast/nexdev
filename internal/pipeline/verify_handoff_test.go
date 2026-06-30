package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/safety"
	"github.com/mojomast/nexdev/internal/state"
)

func TestVerifyAndHandoffStagesWriteArtifacts(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, env := seededDevelopEnv(t, ctx, root, "proj_verify_handoff", "run_verify_handoff")
	if err := os.WriteFile(filepath.Join(root, "develop.txt"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}

	verify := NewVerifyStage(VerifyStageConfig{ProjectRoot: root})
	if err := verify.Run(ctx, env); err != nil {
		t.Fatalf("VerifyStage.Run failed: %v", err)
	}
	var report verifyReport
	if err := json.Unmarshal(readStageArtifact(t, root, verifyReportArtifactRelPath), &report); err != nil {
		t.Fatalf("verify report invalid: %v", err)
	}
	if !report.Passed || len(report.ChangedFiles) != 1 || report.ChangedFiles[0].Path != "develop.txt" || report.ChangedFiles[0].SHA256 == "" {
		t.Fatalf("unexpected verify report: %#v", report)
	}
	events, err := store.ListEvents(ctx, state.EventListOptions{RunID: env.Run.RunID()})
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	if countEvents(events, contract.EventTypeVerifyStarted) != 1 || countEvents(events, contract.EventTypeVerifyCompleted) != 1 {
		t.Fatalf("verify events missing: %#v", events)
	}
	estimated := 0.125
	if err := store.CreateCostRecord(ctx, &state.CostRecord{ID: "cost_handoff_1", ProjectID: env.Project.ProjectID(), RunID: env.Run.RunID(), Stage: "design", Provider: "fake token=sk-providersecret1234567890", Model: "fake-model", PromptTokens: 20, CompletionTokens: 30, TotalTokens: 50, EstimatedUSD: &estimated, LatencyMS: 150, Currency: "USD"}); err != nil {
		t.Fatalf("CreateCostRecord failed: %v", err)
	}

	handoff := NewHandoffStage(HandoffStageConfig{ProjectRoot: root, Request: "finish token=sk-testsecret1234567890"})
	if err := handoff.Run(ctx, env); err != nil {
		t.Fatalf("HandoffStage.Run failed: %v", err)
	}
	changedJSON := string(readStageArtifact(t, root, changedFilesArtifactRelPath))
	if !strings.Contains(changedJSON, "develop.txt") {
		t.Fatalf("changed files artifact missing path: %s", changedJSON)
	}
	var summary runSummaryArtifact
	if err := json.Unmarshal(readStageArtifact(t, root, runSummaryArtifactRelPath), &summary); err != nil {
		t.Fatalf("run summary invalid: %v", err)
	}
	if summary.RunID != env.Run.RunID() || len(summary.ChangedFiles) != 1 {
		t.Fatalf("unexpected run summary: %#v", summary)
	}
	if summary.TotalCostUSD == nil || *summary.TotalCostUSD != estimated || len(summary.ProviderUsage) != 1 {
		t.Fatalf("run summary missing provider usage: %#v", summary)
	}
	usage := summary.ProviderUsage[0]
	if usage.InputTokens != 20 || usage.OutputTokens != 30 || usage.TotalTokens != 50 || usage.CallCount != 1 || usage.AverageLatencyMS != 150 || usage.TotalCostUSD != estimated || strings.Contains(usage.Provider, "sk-providersecret") {
		t.Fatalf("unexpected provider usage: %#v", usage)
	}
	handoffMD := string(readStageArtifact(t, root, handoffArtifactRelPath))
	if !strings.Contains(handoffMD, "develop.txt") || strings.Contains(handoffMD, "sk-testsecret") {
		t.Fatalf("handoff markdown missing changed file or leaked secret:\n%s", handoffMD)
	}
}

func TestHandoffStageUsesGitDiffWhenBaseRefAvailable(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	root := t.TempDir()
	store, env := seededDevelopEnv(t, ctx, root, "proj_handoff_git", "run_handoff_git")
	runPipelineGit(t, root, "init")
	runPipelineGit(t, root, "config", "user.email", "nexdev@example.invalid")
	runPipelineGit(t, root, "config", "user.name", "Nexdev Test")
	writePipelineFile(t, root, "develop.txt", "before")
	runPipelineGit(t, root, "add", ".")
	runPipelineGit(t, root, "commit", "-m", "base")
	writePipelineFile(t, root, "develop.txt", "after")
	writePipelineFile(t, root, "git-only.txt", "added")
	runPipelineGit(t, root, "add", "git-only.txt")

	if err := store.CreateCostRecord(ctx, &state.CostRecord{ID: "cost_handoff_git", ProjectID: env.Project.ProjectID(), RunID: env.Run.RunID(), Stage: "design", Provider: "fake", Model: "fake-model", TotalTokens: 1, Currency: "USD"}); err != nil {
		t.Fatalf("CreateCostRecord failed: %v", err)
	}
	handoff := NewHandoffStage(HandoffStageConfig{ProjectRoot: root, GitBaseRef: "HEAD", Request: "git diff"})
	if err := handoff.Run(ctx, env); err != nil {
		t.Fatalf("HandoffStage.Run failed: %v", err)
	}
	var artifact struct {
		ChangedFiles []contract.ChangedFile `json:"changed_files"`
	}
	if err := json.Unmarshal(readStageArtifact(t, root, changedFilesArtifactRelPath), &artifact); err != nil {
		t.Fatalf("changed files artifact invalid: %v", err)
	}
	byPath := map[string]contract.ChangedFile{}
	for _, file := range artifact.ChangedFiles {
		byPath[file.Path] = file
	}
	if byPath["develop.txt"].Status != "M" || byPath["git-only.txt"].Status != "A" {
		t.Fatalf("changed files did not come from git diff: %#v", artifact.ChangedFiles)
	}
	if len(byPath["develop.txt"].OwningTasks) != 1 || byPath["develop.txt"].OwningTasks[0] != "T1.01" {
		t.Fatalf("expected task owner preserved for expected file: %#v", byPath["develop.txt"])
	}
	if len(byPath["git-only.txt"].OwningTasks) != 0 {
		t.Fatalf("unexpected owner for git-only file: %#v", byPath["git-only.txt"])
	}
}

func TestDetectChangedFilesFallsBackWhenGitUnavailable(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, env := seededDevelopEnv(t, ctx, root, "proj_handoff_git_fallback", "run_handoff_git_fallback")
	writePipelineFile(t, root, "develop.txt", "changed")

	changed, err := detectChangedFiles(ctx, store, env.Run.RunID(), root, "missing-base-ref", "HEAD")
	if err != nil {
		t.Fatalf("detectChangedFiles failed: %v", err)
	}
	if len(changed) != 1 || changed[0].Path != "develop.txt" || changed[0].Status != "modified" {
		t.Fatalf("expected fallback to expected-file manifest, got %#v", changed)
	}
}

func TestVerifyStageDeniesCommandsWithoutExecuting(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	_, env := seededDevelopEnv(t, ctx, root, "proj_verify_deny", "run_verify_deny")
	stage := NewVerifyStage(VerifyStageConfig{ProjectRoot: root, Commands: []string{"go test ./..."}})

	if err := stage.Run(ctx, env); err != nil {
		t.Fatalf("VerifyStage.Run failed: %v", err)
	}
	var report verifyReport
	if err := json.Unmarshal(readStageArtifact(t, root, verifyReportArtifactRelPath), &report); err != nil {
		t.Fatalf("verify report invalid: %v", err)
	}
	if report.Passed || len(report.Commands) != 1 || report.Commands[0].ExitCode != 126 || !strings.Contains(report.Commands[0].StderrTail, "denied") || report.Commands[0].Attempts != 1 {
		t.Fatalf("unexpected denied command report: %#v", report)
	}
}

func TestVerifyStagePassingCommandWritesPassStatus(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	_, env := seededDevelopEnv(t, ctx, root, "proj_verify_pass", "run_verify_pass")
	policy := safety.DefaultToolPolicy()
	policy.Shell.AllowCommands = []string{"go version"}
	stage := NewVerifyStage(VerifyStageConfig{ProjectRoot: root, Commands: []string{"go version"}, Policy: policy})

	if err := stage.Run(ctx, env); err != nil {
		t.Fatalf("VerifyStage.Run failed: %v", err)
	}
	var report verifyReport
	if err := json.Unmarshal(readStageArtifact(t, root, verifyReportArtifactRelPath), &report); err != nil {
		t.Fatalf("verify report invalid: %v", err)
	}
	if !report.Passed || len(report.Commands) != 1 || !report.Commands[0].Passed || report.Commands[0].ExitCode != 0 || report.Commands[0].Attempts != 1 {
		t.Fatalf("unexpected pass report: %#v", report)
	}
}

func TestVerifyStageOutputCapTruncatesReport(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	_, env := seededDevelopEnv(t, ctx, root, "proj_verify_truncate", "run_verify_truncate")
	policy := safety.DefaultToolPolicy()
	policy.Shell.AllowCommands = []string{"go env"}
	stage := NewVerifyStage(VerifyStageConfig{ProjectRoot: root, Commands: []string{"go env"}, Policy: policy, OutputCapBytes: 8})

	if err := stage.Run(ctx, env); err != nil {
		t.Fatalf("VerifyStage.Run failed: %v", err)
	}
	var report verifyReport
	if err := json.Unmarshal(readStageArtifact(t, root, verifyReportArtifactRelPath), &report); err != nil {
		t.Fatalf("verify report invalid: %v", err)
	}
	if len(report.Commands) != 1 || !report.Commands[0].OutputTruncated || len(report.Commands[0].StdoutTail) > 8 {
		t.Fatalf("output was not capped: %#v", report.Commands)
	}
}

func TestVerifyStageRepairLoopRerunsUpToCapThenFails(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	_, env := seededDevelopEnv(t, ctx, root, "proj_verify_repair", "run_verify_repair")
	policy := safety.DefaultToolPolicy()
	policy.Shell.AllowCommands = []string{"false"}
	stage := NewVerifyStage(VerifyStageConfig{ProjectRoot: root, Commands: []string{"false"}, Policy: policy, RepairAttempts: 2})

	if err := stage.Run(ctx, env); err != nil {
		t.Fatalf("VerifyStage.Run failed: %v", err)
	}
	var report verifyReport
	if err := json.Unmarshal(readStageArtifact(t, root, verifyReportArtifactRelPath), &report); err != nil {
		t.Fatalf("verify report invalid: %v", err)
	}
	if report.Passed || len(report.Commands) != 1 || report.Commands[0].Attempts != 3 || len(report.RepairAttempts) != 2 {
		t.Fatalf("unexpected repair report: %#v", report)
	}
}

func TestVerifyStageTimeoutMarksPendingCommandsTimedOut(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	_, env := seededDevelopEnv(t, ctx, root, "proj_verify_timeout", "run_verify_timeout")
	policy := safety.DefaultToolPolicy()
	policy.Shell.AllowCommands = []string{"sleep 1", "go version"}
	stage := NewVerifyStage(VerifyStageConfig{ProjectRoot: root, Commands: []string{"sleep 1", "go version"}, Policy: policy, TotalTimeout: 10 * time.Millisecond})

	if err := stage.Run(ctx, env); err != nil {
		t.Fatalf("VerifyStage.Run failed: %v", err)
	}
	var report verifyReport
	if err := json.Unmarshal(readStageArtifact(t, root, verifyReportArtifactRelPath), &report); err != nil {
		t.Fatalf("verify report invalid: %v", err)
	}
	if report.Passed || len(report.Commands) != 2 || !report.Commands[0].TimedOut || !report.Commands[1].TimedOut || report.Commands[1].Attempts != 0 {
		t.Fatalf("pending command was not timed out: %#v", report.Commands)
	}
}

func TestVerifyStageFakeRepairSeamRecordsAttempt(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	_, env := seededDevelopEnv(t, ctx, root, "proj_verify_fake_repair", "run_verify_fake_repair")
	policy := safety.DefaultToolPolicy()
	policy.Shell.AllowCommands = []string{"false"}
	called := 0
	stage := NewVerifyStage(VerifyStageConfig{ProjectRoot: root, Commands: []string{"false"}, Policy: policy, RepairAttempts: 1, RepairFunc: func(context.Context, VerifyRepairAttempt) error {
		called++
		return nil
	}})

	if err := stage.Run(ctx, env); err != nil {
		t.Fatalf("VerifyStage.Run failed: %v", err)
	}
	var report verifyReport
	if err := json.Unmarshal(readStageArtifact(t, root, verifyReportArtifactRelPath), &report); err != nil {
		t.Fatalf("verify report invalid: %v", err)
	}
	if called != 1 || len(report.RepairAttempts) != 1 || report.Commands[0].Attempts != 2 {
		t.Fatalf("fake repair seam not used: called=%d report=%#v", called, report)
	}
}

func runPipelineGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

func writePipelineFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
