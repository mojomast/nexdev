package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/state"
)

func TestReviewStageManualRequiresApprovalThenResumesFromMarker(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, env := seededReviewEnv(t, ctx, root, "proj_review_manual", "run_review_manual")
	stage := NewReviewStage(ReviewStageConfig{Mode: ReviewModeManual, Actor: "operator", ProjectRoot: root})

	err := stage.Run(ctx, env)
	var blocked *BlockedError
	if !errors.As(err, &blocked) || !strings.Contains(blocked.Reason, "review_required") {
		t.Fatalf("Run error = %v, want review_required BlockedError", err)
	}
	service := reviewServiceForTest(store, root, "proj_review_manual", "run_review_manual")
	approval, err := service.Approve(ctx, "operator", "risk accepted")
	if err != nil {
		t.Fatalf("Approve failed: %v", err)
	}
	if !approval.Approved || approval.Marker != reviewApprovedMarker || approval.PlanVersion != 1 {
		t.Fatalf("unexpected approval: %#v", approval)
	}
	if err := stage.Resume(ctx, env); err != nil {
		t.Fatalf("Resume after approval failed: %v", err)
	}
	output, err := stage.Output(ctx, env)
	if err != nil {
		t.Fatalf("Output failed: %v", err)
	}
	if output["marker"] != reviewApprovedMarker || output["approved"] != true {
		t.Fatalf("approval marker missing from output: %#v", output)
	}
}

func TestReviewServiceUpdateTaskIncrementsVersionAndRecordsPlanEdit(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, _ := seededReviewEnv(t, ctx, root, "proj_review_edit", "run_review_edit")
	service := reviewServiceForTest(store, root, "proj_review_edit", "run_review_edit")
	title := "Renamed planning task"
	notes := []string{"operator clarified scope"}

	nextVersion, err := service.UpdatePendingTask(ctx, "T1.01", ReviewTaskPatch{Title: &title, Notes: &notes}, "admin")
	if err != nil {
		t.Fatalf("UpdatePendingTask failed: %v", err)
	}
	if nextVersion != 2 {
		t.Fatalf("version = %d, want 2", nextVersion)
	}
	tasks, err := store.ListNexdevTasks(ctx, state.NexdevTaskListOptions{RunID: "run_review_edit", PlanVersion: 2})
	if err != nil {
		t.Fatalf("ListNexdevTasks failed: %v", err)
	}
	if len(tasks) != 2 || tasks[0].Spec.Title != title || tasks[0].Spec.Notes[0] != notes[0] {
		t.Fatalf("task update not persisted: %#v", tasks)
	}
	edits, err := store.ListPlanEditEventsByRun(ctx, "run_review_edit")
	if err != nil {
		t.Fatalf("ListPlanEditEventsByRun failed: %v", err)
	}
	if len(edits) != 1 || edits[0].EditType != ReviewEditUpdateTask || edits[0].PlanVersionBefore != 1 || edits[0].PlanVersionAfter != 2 || edits[0].Actor != "admin" {
		t.Fatalf("unexpected edit event: %#v", edits)
	}
}

func TestReviewServiceRejectsEditsToNonPendingTasks(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, _ := seededReviewEnv(t, ctx, root, "proj_review_nonpending", "run_review_nonpending")
	if err := store.UpdateNexdevTaskStatus(ctx, "T1.01", state.NexdevTaskStatusRunning); err != nil {
		t.Fatalf("UpdateNexdevTaskStatus failed: %v", err)
	}
	service := reviewServiceForTest(store, root, "proj_review_nonpending", "run_review_nonpending")
	title := "Should fail"
	if _, err := service.UpdatePendingTask(ctx, "T1.01", ReviewTaskPatch{Title: &title}, "admin"); err == nil || !strings.Contains(err.Error(), "not pending") {
		t.Fatalf("UpdatePendingTask error = %v, want non-pending rejection", err)
	}
}

func TestReviewServiceDeletePendingTaskIncrementsVersionAndRecordsPlanEdit(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, _ := seededReviewEnv(t, ctx, root, "proj_review_delete", "run_review_delete")
	service := reviewServiceForTest(store, root, "proj_review_delete", "run_review_delete")

	if _, err := service.DeletePendingTask(ctx, "T1.01", "admin"); err == nil || !strings.Contains(err.Error(), "depends") {
		t.Fatalf("DeletePendingTask dependency error = %v, want depends rejection", err)
	}
	nextVersion, err := service.DeletePendingTask(ctx, "T2.01", "admin")
	if err != nil {
		t.Fatalf("DeletePendingTask failed: %v", err)
	}
	if nextVersion != 2 {
		t.Fatalf("version = %d, want 2", nextVersion)
	}
	tasks, err := store.ListNexdevTasks(ctx, state.NexdevTaskListOptions{RunID: "run_review_delete", PlanVersion: 2})
	if err != nil {
		t.Fatalf("ListNexdevTasks failed: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Spec.ID != "T1.01" || tasks[0].PlanOrder != 1 {
		t.Fatalf("unexpected tasks after delete: %#v", tasks)
	}
	edits, err := store.ListPlanEditEventsByRun(ctx, "run_review_delete")
	if err != nil {
		t.Fatalf("ListPlanEditEventsByRun failed: %v", err)
	}
	if len(edits) != 1 || edits[0].EditType != ReviewEditDeleteTask || edits[0].TargetID != "T2.01" || edits[0].PlanVersionAfter != 2 {
		t.Fatalf("unexpected delete edit event: %#v", edits)
	}
}

func TestReviewStageCIRejectsHighRiskTaskWithoutTests(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	_, env := seededReviewEnvWithTasks(t, ctx, root, "proj_review_ci", "run_review_ci", []contract.TaskSpec{{ID: "T1.01", PhaseID: "phase_001", Title: "Dangerous change", Description: "Modify auth", ExpectedFiles: []string{"internal/auth.go"}, AcceptanceCriteria: []string{"safe"}, RiskLevel: "high", RequiredTools: []string{"write_file"}}})
	stage := NewReviewStage(ReviewStageConfig{Mode: ReviewModeCI, Actor: "ci", ProjectRoot: root})
	if err := stage.Run(ctx, env); err == nil || !strings.Contains(err.Error(), "high-risk") {
		t.Fatalf("CI review error = %v, want high-risk rejection", err)
	}
}

func TestReviewStageSkipRequiresExplicitAllowance(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	_, env := seededReviewEnv(t, ctx, root, "proj_review_skip", "run_review_skip")
	stage := NewReviewStage(ReviewStageConfig{Mode: ReviewModeSkip, ProjectRoot: root})
	if err := stage.Run(ctx, env); err == nil || !strings.Contains(err.Error(), "skip-review") {
		t.Fatalf("skip without allowance error = %v, want explicit allowance rejection", err)
	}
	stage = NewReviewStage(ReviewStageConfig{Mode: ReviewModeSkip, AllowSkipReview: true, Actor: "operator", ProjectRoot: root})
	if err := stage.Run(ctx, env); err != nil {
		t.Fatalf("skip with allowance failed: %v", err)
	}
}

func TestReviewStageAutoApprovesAndWritesDevelopPrerequisiteMarker(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, env := seededReviewEnv(t, ctx, root, "proj_review_auto", "run_review_auto")
	stage := NewReviewStage(ReviewStageConfig{Mode: ReviewModeAuto, Actor: "reviewer", ProjectRoot: root})
	if err := stage.Run(ctx, env); err != nil {
		t.Fatalf("auto review failed: %v", err)
	}
	var approval ReviewApproval
	if err := json.Unmarshal(readStageArtifact(t, root, reviewApprovalArtifactRelPath), &approval); err != nil {
		t.Fatalf("approval artifact invalid: %v", err)
	}
	if !approval.Approved || approval.Marker != reviewApprovedMarker || approval.TaskCount != 2 {
		t.Fatalf("unexpected approval artifact: %#v", approval)
	}
	artifacts, err := store.ListArtifacts(ctx, state.ArtifactListOptions{ProjectID: "proj_review_auto", RunID: "run_review_auto", Kind: reviewApprovalArtifactKind})
	if err != nil {
		t.Fatalf("ListArtifacts failed: %v", err)
	}
	if len(artifacts) != 1 || artifacts[0].Path != reviewApprovalArtifactRelPath {
		t.Fatalf("approval artifact not indexed: %#v", artifacts)
	}
}

func seededReviewEnv(t *testing.T, ctx context.Context, root, projectID, runID string) (*state.Store, StageEnv) {
	t.Helper()
	return seededReviewEnvWithTasks(t, ctx, root, projectID, runID, []contract.TaskSpec{
		{ID: "T1.01", PhaseID: "phase_001", Title: "Add review contracts", Description: "Implement review", ExpectedFiles: []string{"internal/pipeline/review.go"}, AcceptanceCriteria: []string{"review works"}, TestCommands: []string{"go test ./internal/pipeline"}, RiskLevel: "low", RequiredTools: []string{"write_file"}},
		{ID: "T2.01", PhaseID: "phase_002", Title: "Test review", Description: "Cover review", ExpectedFiles: []string{"internal/pipeline/review_test.go"}, Dependencies: []string{"T1.01"}, AcceptanceCriteria: []string{"tests pass"}, TestCommands: []string{"go test ./internal/pipeline"}, RiskLevel: "medium", RequiredTools: []string{"write_file"}},
	})
}

func seededReviewEnvWithTasks(t *testing.T, ctx context.Context, root, projectID, runID string, specs []contract.TaskSpec) (*state.Store, StageEnv) {
	t.Helper()
	store := newRepoAnalyzeStore(t)
	createStageProjectAndRun(t, ctx, store, projectID, runID)
	for i, spec := range specs {
		if err := store.CreateNexdevTask(ctx, &state.NexdevTask{ProjectID: projectID, RunID: runID, Status: state.NexdevTaskStatusPending, PlanVersion: 1, PlanOrder: i + 1, Spec: spec}); err != nil {
			t.Fatalf("CreateNexdevTask %s failed: %v", spec.ID, err)
		}
	}
	env := StageEnv{Project: testRepoAnalyzeProject{id: projectID}, Run: testRepoAnalyzeRun{id: runID}, Store: store}
	return store, env
}

func reviewServiceForTest(store *state.Store, root, projectID, runID string) *ReviewService {
	return &ReviewService{Store: store, ProjectID: projectID, RunID: runID, ProjectRoot: root}
}
