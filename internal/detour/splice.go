package detour

import (
	"fmt"
	"sort"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/pipeline"
	"github.com/mojomast/nexdev/internal/state"
)

// PersistedSpliceTask is the state-facing task row produced by detour splicing.
type PersistedSpliceTask struct {
	Spec        contract.TaskSpec
	PlanVersion int
	PlanOrder   int
}

// SpliceDetourTasks assigns deterministic plan orders for new detour tasks
// immediately after the trigger task. State persistence shifts later tasks when
// a dense plan has no integer gaps.
func SpliceDetourTasks(existing []*state.NexdevTask, triggerTaskID string, result contract.DetourResult) ([]PersistedSpliceTask, SpliceResult, error) {
	if triggerTaskID == "" {
		return nil, SpliceResult{}, fmt.Errorf("trigger task id is required")
	}
	if len(result.NewTasks) == 0 {
		return nil, SpliceResult{}, fmt.Errorf("detour result must include at least one task")
	}

	sorted := append([]*state.NexdevTask(nil), existing...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].PlanVersion != sorted[j].PlanVersion {
			return sorted[i].PlanVersion < sorted[j].PlanVersion
		}
		if sorted[i].PlanOrder != sorted[j].PlanOrder {
			return sorted[i].PlanOrder < sorted[j].PlanOrder
		}
		return sorted[i].Spec.ID < sorted[j].Spec.ID
	})

	existingIDs := map[string]bool{}
	triggerIndex := -1
	for i, task := range sorted {
		if task == nil {
			continue
		}
		existingIDs[task.Spec.ID] = true
		if task.Spec.ID == triggerTaskID {
			triggerIndex = i
		}
	}
	if triggerIndex < 0 {
		return nil, SpliceResult{}, fmt.Errorf("trigger task %s not found", triggerTaskID)
	}

	conflicts := make([]string, 0)
	seenNew := map[string]bool{}
	for _, task := range result.NewTasks {
		if existingIDs[task.ID] || seenNew[task.ID] {
			conflicts = append(conflicts, task.ID)
		}
		seenNew[task.ID] = true
	}
	splice := SpliceResult{SplicedAfter: triggerTaskID, IDConflicts: conflicts, EventStage: pipeline.StageDetour}
	if len(conflicts) > 0 {
		return nil, splice, fmt.Errorf("detour task id conflicts: %v", conflicts)
	}

	trigger := sorted[triggerIndex]
	persisted := make([]PersistedSpliceTask, 0, len(result.NewTasks))
	allSpecs := make([]contract.TaskSpec, 0, len(sorted)+len(result.NewTasks))
	for i, task := range sorted {
		if task != nil {
			allSpecs = append(allSpecs, task.Spec)
		}
		if i == triggerIndex {
			for j, newTask := range result.NewTasks {
				order := trigger.PlanOrder + j + 1
				persisted = append(persisted, PersistedSpliceTask{Spec: newTask, PlanVersion: trigger.PlanVersion, PlanOrder: order})
				splice.InsertedTaskID = append(splice.InsertedTaskID, newTask.ID)
				splice.Tasks = append(splice.Tasks, newTask)
				allSpecs = append(allSpecs, newTask)
			}
		}
	}
	splice.PlanVersion = trigger.PlanVersion
	splice.Tasks = allSpecs
	return persisted, splice, nil
}
