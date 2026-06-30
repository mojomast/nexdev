package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/executor"
	"github.com/mojomast/nexdev/internal/state"
)

func TestDevelopStageRequiresReviewApprovalMarker(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, env := seededDevelopEnv(t, ctx, root, "proj_develop_prereq", "run_develop_prereq")
	stage := NewDevelopStage(DevelopStageConfig{ProjectRoot: root, Worker: executor.FakeWorker{Writes: map[string][]executor.FakeWrite{"T1.01": {{Path: "develop.txt", Content: "no"}}}}})

	if err := stage.Run(ctx, env); err == nil || !strings.Contains(err.Error(), "reviewed approved plan") {
		t.Fatalf("Run without approval error = %v, want prerequisite failure", err)
	}
	task, err := store.GetNexdevTask(ctx, "T1.01")
	if err != nil {
		t.Fatalf("GetNexdevTask failed: %v", err)
	}
	if task.Status != state.NexdevTaskStatusPending {
		t.Fatalf("task status = %q, want pending", task.Status)
	}
	if _, err := os.Stat(filepath.Join(root, "develop.txt")); !os.IsNotExist(err) {
		t.Fatalf("develop wrote file before review approval: %v", err)
	}
}

func TestDevelopStageRunsApprovedPendingTasks(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, env := seededDevelopEnv(t, ctx, root, "proj_develop_run", "run_develop_run")
	writeReviewApproval(t, root)
	stage := NewDevelopStage(DevelopStageConfig{ProjectRoot: root, Worker: executor.FakeWorker{Writes: map[string][]executor.FakeWrite{"T1.01": {{Path: "develop.txt", Content: "yes"}}}}})

	if err := stage.Run(ctx, env); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	task, err := store.GetNexdevTask(ctx, "T1.01")
	if err != nil {
		t.Fatalf("GetNexdevTask failed: %v", err)
	}
	if task.Status != state.NexdevTaskStatusCompleted {
		t.Fatalf("task status = %q, want completed", task.Status)
	}
	data, err := os.ReadFile(filepath.Join(root, "develop.txt"))
	if err != nil || string(data) != "yes" {
		t.Fatalf("develop write = %q, %v", data, err)
	}
	output, err := stage.Output(ctx, env)
	if err != nil {
		t.Fatalf("Output failed: %v", err)
	}
	if output["task_report_count"] != 1 {
		t.Fatalf("task_report_count = %#v, want 1", output["task_report_count"])
	}
}

func seededDevelopEnv(t *testing.T, ctx context.Context, root, projectID, runID string) (*state.Store, StageEnv) {
	t.Helper()
	store := newRepoAnalyzeStore(t)
	createStageProjectAndRun(t, ctx, store, projectID, runID)
	spec := contract.TaskSpec{ID: "T1.01", PhaseID: "phase_001", Title: "Develop task", Description: "safe write", ExpectedFiles: []string{"develop.txt"}, AcceptanceCriteria: []string{"written"}, RiskLevel: "low", RequiredTools: []string{"write_file"}}
	if err := store.CreateNexdevTask(ctx, &state.NexdevTask{ProjectID: projectID, RunID: runID, Spec: spec, Status: state.NexdevTaskStatusPending, PlanVersion: 1, PlanOrder: 1}); err != nil {
		t.Fatalf("CreateNexdevTask failed: %v", err)
	}
	return store, StageEnv{Project: testRepoAnalyzeProject{id: projectID}, Run: testRepoAnalyzeRun{id: runID}, Store: store}
}

func writeReviewApproval(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, reviewApprovalArtifactRelPath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	data, err := json.Marshal(ReviewApproval{Marker: reviewApprovedMarker, Approved: true, Mode: ReviewModeManual, Actor: "operator", PlanVersion: 1, TaskCount: 1})
	if err != nil {
		t.Fatalf("Marshal approval failed: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile approval failed: %v", err)
	}
}
