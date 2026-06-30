package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mojomast/nexdev/internal/contract"
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
	var report contract.VerifyReport
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

	handoff := NewHandoffStage(HandoffStageConfig{ProjectRoot: root, Request: "finish token=sk-testsecret1234567890"})
	if err := handoff.Run(ctx, env); err != nil {
		t.Fatalf("HandoffStage.Run failed: %v", err)
	}
	changedJSON := string(readStageArtifact(t, root, changedFilesArtifactRelPath))
	if !strings.Contains(changedJSON, "develop.txt") {
		t.Fatalf("changed files artifact missing path: %s", changedJSON)
	}
	var summary contract.RunSummary
	if err := json.Unmarshal(readStageArtifact(t, root, runSummaryArtifactRelPath), &summary); err != nil {
		t.Fatalf("run summary invalid: %v", err)
	}
	if summary.RunID != env.Run.RunID() || len(summary.ChangedFiles) != 1 {
		t.Fatalf("unexpected run summary: %#v", summary)
	}
	handoffMD := string(readStageArtifact(t, root, handoffArtifactRelPath))
	if !strings.Contains(handoffMD, "develop.txt") || strings.Contains(handoffMD, "sk-testsecret") {
		t.Fatalf("handoff markdown missing changed file or leaked secret:\n%s", handoffMD)
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
	var report contract.VerifyReport
	if err := json.Unmarshal(readStageArtifact(t, root, verifyReportArtifactRelPath), &report); err != nil {
		t.Fatalf("verify report invalid: %v", err)
	}
	if report.Passed || len(report.Commands) != 1 || report.Commands[0].ExitCode != 126 || !strings.Contains(report.Commands[0].StderrTail, "denied") {
		t.Fatalf("unexpected denied command report: %#v", report)
	}
}
