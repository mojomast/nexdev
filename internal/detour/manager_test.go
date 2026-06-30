package detour

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
)

func TestWorkflowManagerBlockerTriggeredDetourRequest(t *testing.T) {
	ctx := context.Background()
	store := newDetourWorkflowStore(t, "proj_detour", "run_detour")
	seedTask(t, ctx, store, "proj_detour", "run_detour", "T1.01", 10)
	seedTask(t, ctx, store, "proj_detour", "run_detour", "T1.02", 30)
	if err := store.CreateNexdevBlocker(ctx, &state.NexdevBlocker{ID: "blk_detour", ProjectID: "proj_detour", RunID: "run_detour", TaskID: "T1.01", Reason: "missing_dependency", Description: "Need setup task"}); err != nil {
		t.Fatalf("CreateNexdevBlocker failed: %v", err)
	}

	fake := provider.NewFakeProvider(provider.WithFakeScripts([]provider.FakeScript{{PromptContains: "DetourResult", Responses: []provider.FakeResponse{{Content: `{"id":"detour_1","new_tasks":[{"id":"D1.01","phase_id":"phase_001","title":"Add setup","description":"Create setup","expected_files":["setup.txt"],"dependencies":[],"acceptance_criteria":["setup exists"],"test_commands":[],"risk_level":"low","required_tools":["write_file"],"notes":[]}],"spliced_after":"T1.01","id_conflicts":[],"depth":1}`}}}}))
	manager := newWorkflowManagerForTest(t, store, fake, 3)

	result, err := manager.RequestForBlocker(ctx, "run_detour", "blk_detour")
	if err != nil {
		t.Fatalf("RequestForBlocker failed: %v", err)
	}
	if result.Depth != 1 || result.SplicedAfter != "T1.01" || len(result.NewTasks) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	inserted, err := store.GetNexdevTask(ctx, "D1.01")
	if err != nil {
		t.Fatalf("GetNexdevTask inserted failed: %v", err)
	}
	if inserted.PlanOrder <= 10 || inserted.PlanOrder >= 30 {
		t.Fatalf("inserted plan order = %d, want between trigger and next", inserted.PlanOrder)
	}
	trigger, err := store.GetNexdevTask(ctx, "T1.01")
	if err != nil {
		t.Fatalf("GetNexdevTask trigger failed: %v", err)
	}
	if trigger.Status != state.NexdevTaskStatusPendingAfterDetour {
		t.Fatalf("trigger status = %q", trigger.Status)
	}
	records, err := store.ListDetourRecords(ctx, state.DetourRecordListOptions{RunID: "run_detour", TriggerTaskID: "T1.01"})
	if err != nil || len(records) != 1 {
		t.Fatalf("detour records = %d err=%v", len(records), err)
	}
	events, err := store.ListEvents(ctx, state.EventListOptions{RunID: "run_detour"})
	if err != nil || len(events) != 1 || events[0].Type != contract.EventTypeDetourCreated {
		t.Fatalf("events = %#v err=%v", events, err)
	}
}

func TestWorkflowManagerSplicesDensePlanImmediatelyAfterTrigger(t *testing.T) {
	ctx := context.Background()
	store := newDetourWorkflowStore(t, "proj_dense_detour", "run_dense_detour")
	seedTask(t, ctx, store, "proj_dense_detour", "run_dense_detour", "T1.01", 1)
	seedTask(t, ctx, store, "proj_dense_detour", "run_dense_detour", "T1.02", 2)
	seedTask(t, ctx, store, "proj_dense_detour", "run_dense_detour", "T1.03", 3)
	fake := provider.NewFakeProvider(provider.WithFakeScripts([]provider.FakeScript{{PromptContains: "DetourResult", Responses: []provider.FakeResponse{{Content: `{"id":"detour_dense","new_tasks":[{"id":"D1.01","phase_id":"phase_001","title":"Add first unblocker","description":"Create first","expected_files":["first.txt"],"dependencies":[],"acceptance_criteria":["first exists"],"test_commands":[],"risk_level":"low","required_tools":["write_file"],"notes":[]},{"id":"D1.02","phase_id":"phase_001","title":"Add second unblocker","description":"Create second","expected_files":["second.txt"],"dependencies":["D1.01"],"acceptance_criteria":["second exists"],"test_commands":[],"risk_level":"low","required_tools":["write_file"],"notes":[]}],"spliced_after":"T1.01","id_conflicts":[],"depth":1}`}}}}))
	manager := newWorkflowManagerForTest(t, store, fake, 3)

	result, err := manager.Request(ctx, contract.DetourRequest{ProjectID: "proj_dense_detour", RunID: "run_dense_detour", TriggerTaskID: "T1.01", Reason: "blocked", Context: "dense", Source: contract.DetourSourceOperatorManual})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if got, want := result.SplicedAfter, "T1.01"; got != want {
		t.Fatalf("SplicedAfter = %q, want %q", got, want)
	}
	tasks, err := store.ListNexdevTasks(ctx, state.NexdevTaskListOptions{RunID: "run_dense_detour", PlanVersion: 1})
	if err != nil {
		t.Fatalf("ListNexdevTasks failed: %v", err)
	}
	if got, want := stateTaskIDs(tasks), []string{"T1.01", "D1.01", "D1.02", "T1.02", "T1.03"}; !sameStringSlices(got, want) {
		t.Fatalf("task order = %v, want %v", got, want)
	}
	orders := map[string]int{}
	for _, task := range tasks {
		orders[task.Spec.ID] = task.PlanOrder
		if task.PlanVersion != 1 {
			t.Fatalf("task %s plan version = %d, want 1", task.Spec.ID, task.PlanVersion)
		}
	}
	if orders["T1.01"] != 1 || orders["D1.01"] != 2 || orders["D1.02"] != 3 || orders["T1.02"] != 4 || orders["T1.03"] != 5 {
		t.Fatalf("orders = %#v", orders)
	}
}

func stateTaskIDs(tasks []*state.NexdevTask) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.Spec.ID)
	}
	return ids
}

func TestWorkflowManagerValidatesGeneratedTasks(t *testing.T) {
	err := validateGeneratedTasks([]contract.TaskSpec{{ID: "D1.01", PhaseID: "phase_001", Title: "Implement unsafe write", RequiredTools: []string{"write_file"}}}, nil)
	if err == nil || !strings.Contains(err.Error(), "acceptance criteria") {
		t.Fatalf("expected acceptance criteria validation error, got %v", err)
	}
	err = validateGeneratedTasks([]contract.TaskSpec{{ID: "D1.01", PhaseID: "phase_001", Title: "Implement write", AcceptanceCriteria: []string{"done"}, RequiredTools: []string{"write_file"}}}, nil)
	if err == nil || !strings.Contains(err.Error(), "expected files") {
		t.Fatalf("expected expected_files validation error, got %v", err)
	}
	err = validateGeneratedTasks([]contract.TaskSpec{{ID: "D1.01", PhaseID: "phase_001", Title: "Task", AcceptanceCriteria: []string{"done"}, Dependencies: []string{"missing"}}}, nil)
	if err == nil || !strings.Contains(err.Error(), "dependency not found") {
		t.Fatalf("expected dependency validation error, got %v", err)
	}
}

func TestWorkflowManagerIDConflictDetection(t *testing.T) {
	ctx := context.Background()
	store := newDetourWorkflowStore(t, "proj_conflict", "run_conflict")
	seedTask(t, ctx, store, "proj_conflict", "run_conflict", "T1.01", 10)
	seedTask(t, ctx, store, "proj_conflict", "run_conflict", "T1.02", 30)
	fake := provider.NewFakeProvider(provider.WithFakeScripts([]provider.FakeScript{{PromptContains: "DetourResult", Responses: []provider.FakeResponse{{Content: `{"id":"detour_conflict","new_tasks":[{"id":"T1.02","phase_id":"phase_001","title":"Conflict","description":"Conflict","expected_files":[],"dependencies":[],"acceptance_criteria":["checked"],"test_commands":[],"risk_level":"low","required_tools":[],"notes":[]}],"spliced_after":"T1.01","id_conflicts":[],"depth":1}`}}}}))
	manager := newWorkflowManagerForTest(t, store, fake, 3)

	result, err := manager.Request(ctx, contract.DetourRequest{ProjectID: "proj_conflict", RunID: "run_conflict", TriggerTaskID: "T1.01", Reason: "blocked", Context: "conflict", Source: contract.DetourSourceOperatorManual})
	if err == nil || !strings.Contains(err.Error(), "id conflicts") {
		t.Fatalf("expected id conflict error, got %v", err)
	}
	if len(result.IDConflicts) != 1 || result.IDConflicts[0] != "T1.02" {
		t.Fatalf("IDConflicts = %#v", result.IDConflicts)
	}
}

func TestWorkflowManagerDepthExceededCreatesBlockerAndDoesNotSkip(t *testing.T) {
	ctx := context.Background()
	store := newDetourWorkflowStore(t, "proj_depth", "run_depth")
	seedTask(t, ctx, store, "proj_depth", "run_depth", "D3.01", 10)
	manager := newWorkflowManagerForTest(t, store, provider.NewFakeProvider(), 3)

	_, err := manager.Request(ctx, contract.DetourRequest{ProjectID: "proj_depth", RunID: "run_depth", TriggerTaskID: "D3.01", Reason: "still_blocked", Context: "too deep", Source: contract.DetourSourceOperatorManual})
	if err == nil || !strings.Contains(err.Error(), DepthExceededBlockerReason) {
		t.Fatalf("expected depth exceeded error, got %v", err)
	}
	blockers, err := store.ListNexdevBlockers(ctx, state.NexdevBlockerListOptions{RunID: "run_depth", TaskID: "D3.01", Status: state.NexdevBlockerStatusOpen})
	if err != nil || len(blockers) != 1 || blockers[0].Reason != DepthExceededBlockerReason {
		t.Fatalf("depth blockers = %#v err=%v", blockers, err)
	}
	task, err := store.GetNexdevTask(ctx, "D3.01")
	if err != nil {
		t.Fatalf("GetNexdevTask failed: %v", err)
	}
	if task.Status == state.NexdevTaskStatusSkipped {
		t.Fatal("depth exceeded silently skipped task")
	}
	if task.Status != state.NexdevTaskStatusBlocked {
		t.Fatalf("task status = %q, want blocked", task.Status)
	}
	records, err := store.ListDetourRecords(ctx, state.DetourRecordListOptions{RunID: "run_depth"})
	if err != nil || len(records) != 0 {
		t.Fatalf("depth-exceeded detour records = %d err=%v", len(records), err)
	}
}

func TestWorkflowManagerFakeProviderStructuredRepair(t *testing.T) {
	ctx := context.Background()
	store := newDetourWorkflowStore(t, "proj_repair", "run_repair")
	seedTask(t, ctx, store, "proj_repair", "run_repair", "T1.01", 10)
	seedTask(t, ctx, store, "proj_repair", "run_repair", "T1.02", 30)
	fake := provider.NewFakeProvider(provider.WithFakeScripts([]provider.FakeScript{{PromptContains: "DetourResult", Responses: []provider.FakeResponse{{Content: `not-json`}, {Content: `{"id":"detour_repair","new_tasks":[{"id":"D1.01","phase_id":"phase_001","title":"Repair output","description":"Repair output","expected_files":["repair.txt"],"dependencies":[],"acceptance_criteria":["valid"],"test_commands":[],"risk_level":"low","required_tools":["write_file"],"notes":[]}],"spliced_after":"T1.01","id_conflicts":[],"depth":1}`}}}}))
	manager := newWorkflowManagerForTest(t, store, fake, 3)

	if _, err := manager.Request(ctx, contract.DetourRequest{ProjectID: "proj_repair", RunID: "run_repair", TriggerTaskID: "T1.01", Reason: "repair", Context: "repair", Source: contract.DetourSourceOperatorManual}); err != nil {
		t.Fatalf("Request failed after repair: %v", err)
	}
	if calls := fake.Calls(); len(calls) != 2 {
		t.Fatalf("fake provider calls = %d, want 2", len(calls))
	}
}

func newWorkflowManagerForTest(t *testing.T, store *state.Store, fake *provider.FakeProvider, maxDepth int) *WorkflowManager {
	t.Helper()
	router, err := provider.NewRouterWithRegistry(provider.Selection{Provider: provider.FakeProviderName, Model: "fake-model"}, nil, map[string]provider.ProviderFactory{provider.FakeProviderName: func() provider.Provider { return fake }})
	if err != nil {
		t.Fatalf("NewRouterWithRegistry failed: %v", err)
	}
	manager, err := NewWorkflowManager(WorkflowManagerConfig{Store: store, StructuredProvider: provider.StructuredClient{Router: router, Providers: map[string]provider.Provider{provider.FakeProviderName: fake}}, MaxDepth: maxDepth, DesignSummary: "design summary", DesignArtifactPath: ".nexdev/artifacts/design_draft.md", RepoContext: "repo context", Now: func() time.Time { return time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC) }, NewID: fixtureIDGenerator()})
	if err != nil {
		t.Fatalf("NewWorkflowManager failed: %v", err)
	}
	return manager
}

func newDetourWorkflowStore(t *testing.T, projectID, runID string) *state.Store {
	t.Helper()
	store, err := state.NewStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.CreateProject(&state.Project{ID: projectID, Name: projectID, CreatedAt: time.Now().UTC(), CurrentStage: state.StageDevelop}); err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}
	if err := store.CreateRun(context.Background(), &state.Run{ID: runID, ProjectID: projectID, Status: "running", CurrentStage: "develop"}); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}
	return store
}

func seedTask(t *testing.T, ctx context.Context, store *state.Store, projectID, runID, taskID string, order int) {
	t.Helper()
	spec := contract.TaskSpec{ID: taskID, PhaseID: "phase_001", Title: taskID, Description: "task", AcceptanceCriteria: []string{"done"}}
	if err := store.CreateNexdevTask(ctx, &state.NexdevTask{ProjectID: projectID, RunID: runID, Spec: spec, Status: state.NexdevTaskStatusPending, PlanVersion: 1, PlanOrder: order}); err != nil {
		t.Fatalf("CreateNexdevTask %s failed: %v", taskID, err)
	}
}

func fixtureIDGenerator() IDGenerator {
	counters := map[string]int{}
	return func(prefix string) string {
		counters[prefix]++
		return fmt.Sprintf("%s_%02d", prefix, counters[prefix])
	}
}
