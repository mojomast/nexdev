package executor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/state"
	"github.com/mojomast/nexdev/internal/steering"
)

func TestNexdevExecutorFakeTaskCompletionPersistsStatusAndEvents(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store := seededExecutorStore(t, ctx, "proj_exec_complete", "run_exec_complete", []contract.TaskSpec{testTaskSpec("T1.01", "generated.txt")})
	exec := newExecutorForTest(t, store, root, "proj_exec_complete", "run_exec_complete", FakeWorker{Progress: map[string][]string{"T1.01": {"halfway"}}})

	reports, err := exec.RunPending(ctx)
	if err != nil {
		t.Fatalf("RunPending failed: %v", err)
	}
	if len(reports) != 1 || reports[0].Status != TaskStatusCompleted {
		t.Fatalf("unexpected reports: %#v", reports)
	}
	task, err := store.GetNexdevTask(ctx, "T1.01")
	if err != nil {
		t.Fatalf("GetNexdevTask failed: %v", err)
	}
	if task.Status != state.NexdevTaskStatusCompleted {
		t.Fatalf("task status = %q, want completed", task.Status)
	}
	events, err := store.ListEvents(ctx, state.EventListOptions{RunID: "run_exec_complete"})
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	assertEventTypes(t, events, []string{contract.EventTypeTaskStarted, contract.EventTypeTaskProgress, contract.EventTypeTaskCompleted})
}

func TestFakeWorkerRejectsUnexpectedWrite(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store := seededExecutorStore(t, ctx, "proj_exec_reject", "run_exec_reject", []contract.TaskSpec{testTaskSpec("T1.01", "allowed.txt")})
	exec := newExecutorForTest(t, store, root, "proj_exec_reject", "run_exec_reject", FakeWorker{Writes: map[string][]FakeWrite{"T1.01": {{Path: "unexpected.txt", Content: "nope"}}}})

	_, err := exec.RunPending(ctx)
	if !errors.Is(err, ErrTaskWriteNotExpected) {
		t.Fatalf("RunPending error = %v, want ErrTaskWriteNotExpected", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "unexpected.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("unexpected write exists or stat failed unexpectedly: %v", statErr)
	}
}

func TestFakeWorkerAllowsExpectedWrite(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store := seededExecutorStore(t, ctx, "proj_exec_write", "run_exec_write", []contract.TaskSpec{testTaskSpec("T1.01", "allowed/*.txt")})
	exec := newExecutorForTest(t, store, root, "proj_exec_write", "run_exec_write", FakeWorker{Writes: map[string][]FakeWrite{"T1.01": {{Path: "allowed/out.txt", Content: "safe"}}}})

	if _, err := exec.RunPending(ctx); err != nil {
		t.Fatalf("RunPending failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "allowed/out.txt"))
	if err != nil {
		t.Fatalf("expected write missing: %v", err)
	}
	if string(data) != "safe" {
		t.Fatalf("written content = %q", data)
	}
}

func TestNexdevExecutorBlockerCreatesTaskEventAndBlocker(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store := seededExecutorStore(t, ctx, "proj_exec_block", "run_exec_block", []contract.TaskSpec{testTaskSpec("T1.01", "allowed.txt")})
	exec := newExecutorForTest(t, store, root, "proj_exec_block", "run_exec_block", FakeWorker{Blockers: map[string]FakeBlocker{"T1.01": {Reason: "missing input", Description: "needs operator"}}})

	reports, err := exec.RunPending(ctx)
	if err != nil {
		t.Fatalf("RunPending failed: %v", err)
	}
	if len(reports) != 1 || reports[0].Status != TaskStatusBlocked || reports[0].BlockerID == "" {
		t.Fatalf("unexpected blocked report: %#v", reports)
	}
	blockers, err := store.ListNexdevBlockers(ctx, state.NexdevBlockerListOptions{RunID: "run_exec_block", Status: state.NexdevBlockerStatusOpen})
	if err != nil {
		t.Fatalf("ListNexdevBlockers failed: %v", err)
	}
	if len(blockers) != 1 || blockers[0].TaskID != "T1.01" || blockers[0].Reason != BlockerReasonWorkerBlocked {
		t.Fatalf("unexpected blockers: %#v", blockers)
	}
	events, err := store.ListEvents(ctx, state.EventListOptions{RunID: "run_exec_block"})
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	assertEventTypes(t, events, []string{contract.EventTypeTaskStarted, contract.EventTypeTaskBlocked, contract.EventTypeBlockerCreated})
}

func TestNexdevExecutorControlsAndCurrentTask(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store := seededExecutorStore(t, ctx, "proj_exec_controls", "run_exec_controls", []contract.TaskSpec{testTaskSpec("T1.01", "one.txt"), testTaskSpec("T2.01", "two.txt")})
	exec := newExecutorForTest(t, store, root, "proj_exec_controls", "run_exec_controls", blockingWorker{ready: make(chan struct{}), release: make(chan struct{})})

	if err := exec.Pause(ctx, "hold"); err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	if err := exec.SkipTask(ctx, "T1.01", "not needed"); err != nil {
		t.Fatalf("SkipTask failed: %v", err)
	}
	runDone := make(chan error, 1)
	worker := exec.worker.(blockingWorker)
	go func() {
		_, err := exec.RunPending(ctx)
		runDone <- err
	}()
	select {
	case err := <-runDone:
		t.Fatalf("RunPending finished while paused: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	if err := exec.Resume(ctx); err != nil {
		t.Fatalf("Resume failed: %v", err)
	}
	<-worker.ready
	current, err := exec.CurrentTask(ctx)
	if err != nil {
		t.Fatalf("CurrentTask failed: %v", err)
	}
	if current == nil || current.Task.ID != "T2.01" {
		t.Fatalf("current task = %#v, want T2.01", current)
	}
	if err := exec.Cancel(ctx, "stop"); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}
	close(worker.release)
	if err := <-runDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("RunPending error = %v, want context.Canceled", err)
	}
}

func TestNexdevExecutorSteeringPersistsAndInfluencesPromptContext(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store := seededExecutorStore(t, ctx, "proj_exec_steer", "run_exec_steer", []contract.TaskSpec{testTaskSpec("T1.01", "one.txt")})
	capture := &capturingWorker{}
	exec := newExecutorForTest(t, store, root, "proj_exec_steer", "run_exec_steer", capture)
	if err := exec.SetSteeringContext(ctx, "T1.01", steering.Message{Message: "prefer small change", Summary: "small", Source: steering.SourceCLI, CreatedByRole: "operator"}); err != nil {
		t.Fatalf("SetSteeringContext failed: %v", err)
	}
	if _, err := exec.RunPending(ctx); err != nil {
		t.Fatalf("RunPending failed: %v", err)
	}
	if capture.context.SteeringSummary != "small" || len(capture.context.LastSteeringMessages) != 1 || capture.context.LastSteeringMessages[0] != "prefer small change" {
		t.Fatalf("steering context not passed: %#v", capture.context)
	}
}

type blockingWorker struct {
	ready   chan struct{}
	release chan struct{}
}

func (w blockingWorker) RunTask(ctx context.Context, work TaskWork) ([]TaskUpdate, error) {
	close(w.ready)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-w.release:
		return []TaskUpdate{{TaskID: work.Task.ID, PhaseID: work.Task.PhaseID, Type: TaskCompleted, Content: "released"}}, nil
	}
}

type capturingWorker struct{ context TaskPromptContext }

func (w *capturingWorker) RunTask(ctx context.Context, work TaskWork) ([]TaskUpdate, error) {
	w.context = work.PromptContext
	return []TaskUpdate{{TaskID: work.Task.ID, PhaseID: work.Task.PhaseID, Type: TaskCompleted, Content: "captured"}}, nil
}

func newExecutorForTest(t *testing.T, store *state.Store, root, projectID, runID string, worker TaskWorker) *NexdevExecutor {
	t.Helper()
	seq := 0
	exec, err := NewNexdevExecutor(NexdevExecutorConfig{Store: store, ProjectID: projectID, RunID: runID, ProjectRoot: root, Worker: worker, NewID: func(prefix string) string {
		seq++
		return prefix + "_test_" + strings.ReplaceAll(runID, "_", "") + "_" + string(rune('a'+seq))
	}})
	if err != nil {
		t.Fatalf("NewNexdevExecutor failed: %v", err)
	}
	return exec
}

func seededExecutorStore(t *testing.T, ctx context.Context, projectID, runID string, specs []contract.TaskSpec) *state.Store {
	t.Helper()
	store, err := state.NewStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.CreateProject(&state.Project{ID: projectID, Name: projectID, CreatedAt: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC), CurrentStage: state.StageInit}); err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}
	if err := store.CreateRun(ctx, &state.Run{ID: runID, ProjectID: projectID, Status: "running", CurrentStage: "develop", StartedAt: time.Date(2026, 6, 30, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}
	for i, spec := range specs {
		if err := store.CreateNexdevTask(ctx, &state.NexdevTask{ProjectID: projectID, RunID: runID, Spec: spec, Status: state.NexdevTaskStatusPending, PlanVersion: 1, PlanOrder: i + 1}); err != nil {
			t.Fatalf("CreateNexdevTask failed: %v", err)
		}
	}
	return store
}

func testTaskSpec(id, expectedFile string) contract.TaskSpec {
	return contract.TaskSpec{ID: id, PhaseID: "phase_001", Title: "Task " + id, Description: "test task", ExpectedFiles: []string{expectedFile}, AcceptanceCriteria: []string{"done"}, RiskLevel: "low", RequiredTools: []string{"write_file"}}
}

func assertEventTypes(t *testing.T, events []contract.EventEnvelope, want []string) {
	t.Helper()
	if len(events) != len(want) {
		t.Fatalf("event count = %d, want %d: %#v", len(events), len(want), events)
	}
	for i := range want {
		if events[i].Type != want[i] {
			t.Fatalf("event[%d] = %q, want %q", i, events[i].Type, want[i])
		}
	}
}
