package detour

import (
	"strings"
	"testing"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/state"
)

func TestSpliceDetourTasksOrdersImmediatelyAfterTrigger(t *testing.T) {
	existing := []*state.NexdevTask{
		taskRow("T1.01", 1, 10),
		taskRow("T1.02", 1, 30),
	}
	result := contract.DetourResult{NewTasks: []contract.TaskSpec{{ID: "D1.01", PhaseID: "phase_001", Title: "Unblock", AcceptanceCriteria: []string{"unblocked"}}}}

	inserted, splice, err := SpliceDetourTasks(existing, "T1.01", result)
	if err != nil {
		t.Fatalf("SpliceDetourTasks failed: %v", err)
	}
	if len(inserted) != 1 || inserted[0].PlanOrder <= 10 || inserted[0].PlanOrder >= 30 {
		t.Fatalf("inserted order = %#v, want between 10 and 30", inserted)
	}
	if splice.SplicedAfter != "T1.01" || len(splice.InsertedTaskID) != 1 || splice.InsertedTaskID[0] != "D1.01" {
		t.Fatalf("unexpected splice result: %#v", splice)
	}
}

func TestSpliceDetourTasksDetectsIDConflicts(t *testing.T) {
	existing := []*state.NexdevTask{taskRow("T1.01", 1, 10), taskRow("T1.02", 1, 30)}
	result := contract.DetourResult{NewTasks: []contract.TaskSpec{{ID: "T1.02", PhaseID: "phase_001", Title: "Conflict", AcceptanceCriteria: []string{"validated"}}}}

	_, splice, err := SpliceDetourTasks(existing, "T1.01", result)
	if err == nil || !strings.Contains(err.Error(), "id conflicts") {
		t.Fatalf("expected id conflict error, got %v", err)
	}
	if len(splice.IDConflicts) != 1 || splice.IDConflicts[0] != "T1.02" {
		t.Fatalf("IDConflicts = %#v", splice.IDConflicts)
	}
}

func TestSpliceDetourTasksOrdersMultipleTasksInDensePlan(t *testing.T) {
	existing := []*state.NexdevTask{
		taskRow("T1.01", 3, 1),
		taskRow("T1.02", 3, 2),
		taskRow("T1.03", 3, 3),
	}
	result := contract.DetourResult{NewTasks: []contract.TaskSpec{
		{ID: "D1.01", PhaseID: "phase_001", Title: "First", AcceptanceCriteria: []string{"first"}},
		{ID: "D1.02", PhaseID: "phase_001", Title: "Second", AcceptanceCriteria: []string{"second"}},
	}}

	inserted, splice, err := SpliceDetourTasks(existing, "T1.01", result)
	if err != nil {
		t.Fatalf("SpliceDetourTasks failed: %v", err)
	}
	if len(inserted) != 2 || inserted[0].PlanOrder != 2 || inserted[1].PlanOrder != 3 {
		t.Fatalf("inserted orders = %#v, want dense immediate 2,3", inserted)
	}
	if inserted[0].PlanVersion != 3 || inserted[1].PlanVersion != 3 {
		t.Fatalf("inserted plan versions = %#v", inserted)
	}
	if got, want := splice.InsertedTaskID, []string{"D1.01", "D1.02"}; !sameStringSlices(got, want) {
		t.Fatalf("InsertedTaskID = %v, want %v", got, want)
	}
	if got, want := taskSpecIDs(splice.Tasks), []string{"T1.01", "D1.01", "D1.02", "T1.02", "T1.03"}; !sameStringSlices(got, want) {
		t.Fatalf("splice task order = %v, want %v", got, want)
	}
}

func taskRow(id string, version, order int) *state.NexdevTask {
	return &state.NexdevTask{ProjectID: "proj", RunID: "run", PlanVersion: version, PlanOrder: order, Spec: contract.TaskSpec{ID: id, PhaseID: "phase_001", Title: id, AcceptanceCriteria: []string{"done"}}}
}

func taskSpecIDs(tasks []contract.TaskSpec) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return ids
}

func sameStringSlices(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
