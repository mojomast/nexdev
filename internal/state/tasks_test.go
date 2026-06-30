package state

import (
	"context"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
)

func TestStoreNexdevTaskRepositoryCreateListStatusAndDependencies(t *testing.T) {
	store := newRunArtifactTestStore(t)
	seedStateCRun(t, store, "proj_tasks", "run_tasks")
	ctx := context.Background()
	firstAt := time.Date(2026, 6, 30, 8, 0, 0, 7, time.FixedZone("offset", -5*60*60))

	first := &NexdevTask{ProjectID: "proj_tasks", RunID: "run_tasks", Status: NexdevTaskStatusPending, PlanVersion: 2, PlanOrder: 1, CreatedAt: firstAt, Spec: contract.TaskSpec{
		ID:                 "T1.01",
		PhaseID:            "phase_001",
		Title:              "Bootstrap contract",
		Description:        "Create the base contract.",
		ExpectedFiles:      []string{"internal/state/tasks.go", "internal/state/tasks_test.go"},
		AcceptanceCriteria: []string{"task persists"},
		TestCommands:       []string{"go test ./internal/state"},
		RiskLevel:          "medium",
		RequiredTools:      []string{"write_file"},
		Notes:              []string{"preserve legacy tables"},
	}}
	if err := store.CreateNexdevTask(ctx, first); err != nil {
		t.Fatalf("CreateNexdevTask first failed: %v", err)
	}

	second := &NexdevTask{ProjectID: "proj_tasks", RunID: "run_tasks", Status: NexdevTaskStatusPending, PlanVersion: 2, PlanOrder: 2, Spec: contract.TaskSpec{
		ID:                 "T1.02",
		PhaseID:            "phase_001",
		Title:              "Use contract",
		Description:        "Consume the base task.",
		Dependencies:       []string{"T1.01"},
		AcceptanceCriteria: []string{"dependency preserved"},
		RiskLevel:          "low",
	}}
	if err := store.CreateNexdevTask(ctx, second); err != nil {
		t.Fatalf("CreateNexdevTask second failed: %v", err)
	}
	if err := store.UpdateNexdevTaskStatus(ctx, "T1.02", NexdevTaskStatusRunning); err != nil {
		t.Fatalf("UpdateNexdevTaskStatus failed: %v", err)
	}

	tasks, err := store.ListNexdevTasks(ctx, NexdevTaskListOptions{RunID: "run_tasks", PlanVersion: 2})
	if err != nil {
		t.Fatalf("ListNexdevTasks failed: %v", err)
	}
	if len(tasks) != 2 || tasks[0].Spec.ID != "T1.01" || tasks[1].Spec.ID != "T1.02" {
		t.Fatalf("task order = %v", nexdevTaskIDs(tasks))
	}
	if got, want := tasks[0].CreatedAt.Format(time.RFC3339Nano), firstAt.UTC().Format(time.RFC3339Nano); got != want {
		t.Fatalf("CreatedAt = %s, want %s", got, want)
	}
	if got := tasks[0].Spec.ExpectedFiles; len(got) != 2 || got[0] != "internal/state/tasks.go" || got[1] != "internal/state/tasks_test.go" {
		t.Fatalf("ExpectedFiles = %#v", got)
	}
	if got := tasks[1].Spec.Dependencies; len(got) != 1 || got[0] != "T1.01" {
		t.Fatalf("Dependencies = %#v", got)
	}
	if tasks[1].Status != NexdevTaskStatusRunning {
		t.Fatalf("second status = %s", tasks[1].Status)
	}

	running, err := store.ListNexdevTasks(ctx, NexdevTaskListOptions{RunID: "run_tasks", Status: NexdevTaskStatusRunning})
	if err != nil {
		t.Fatalf("ListNexdevTasks by status failed: %v", err)
	}
	if len(running) != 1 || running[0].Spec.ID != "T1.02" {
		t.Fatalf("running tasks = %v", nexdevTaskIDs(running))
	}
}

func TestStoreNexdevTaskRepositoryConstraints(t *testing.T) {
	store := newRunArtifactTestStore(t)
	seedStateCRun(t, store, "proj_task_constraints", "run_task_constraints")
	ctx := context.Background()

	missingDependency := &NexdevTask{ProjectID: "proj_task_constraints", RunID: "run_task_constraints", PlanOrder: 1, Spec: contract.TaskSpec{ID: "T2.01", PhaseID: "phase_002", Title: "Missing dep", Dependencies: []string{"missing"}, AcceptanceCriteria: []string{"fails"}}}
	if err := store.CreateNexdevTask(ctx, missingDependency); err == nil {
		t.Fatal("CreateNexdevTask with missing dependency succeeded, want failure")
	}

	missingAcceptance := &NexdevTask{ProjectID: "proj_task_constraints", RunID: "run_task_constraints", PlanOrder: 1, Spec: contract.TaskSpec{ID: "T2.02", PhaseID: "phase_002", Title: "No acceptance"}}
	if err := store.CreateNexdevTask(ctx, missingAcceptance); err == nil {
		t.Fatal("CreateNexdevTask with missing acceptance criteria succeeded, want failure")
	}

	valid := &NexdevTask{ProjectID: "proj_task_constraints", RunID: "run_task_constraints", PlanOrder: 1, Spec: contract.TaskSpec{ID: "T2.03", PhaseID: "phase_002", Title: "Valid", AcceptanceCriteria: []string{"passes"}}}
	if err := store.CreateNexdevTask(ctx, valid); err != nil {
		t.Fatalf("CreateNexdevTask valid failed: %v", err)
	}
	duplicateOrder := &NexdevTask{ProjectID: "proj_task_constraints", RunID: "run_task_constraints", PlanOrder: 1, Spec: contract.TaskSpec{ID: "T2.04", PhaseID: "phase_002", Title: "Duplicate order", AcceptanceCriteria: []string{"fails"}}}
	if err := store.CreateNexdevTask(ctx, duplicateOrder); err == nil {
		t.Fatal("CreateNexdevTask duplicate plan order succeeded, want failure")
	}
	missingRun := &NexdevTask{ProjectID: "proj_task_constraints", RunID: "missing", PlanOrder: 2, Spec: contract.TaskSpec{ID: "T2.05", PhaseID: "phase_002", Title: "Missing run", AcceptanceCriteria: []string{"fails"}}}
	if err := store.CreateNexdevTask(ctx, missingRun); err == nil {
		t.Fatal("CreateNexdevTask missing run succeeded, want FK failure")
	}
}

func nexdevTaskIDs(tasks []*NexdevTask) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.Spec.ID)
	}
	return ids
}
