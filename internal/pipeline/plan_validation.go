package pipeline

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mojomast/nexdev/internal/contract"
)

type devPlanArtifact struct {
	PlanVersion int                    `json:"plan_version"`
	Phases      []contract.PhaseSketch `json:"phases"`
	Tasks       []contract.TaskSpec    `json:"tasks"`
}

func canonicalizePhaseSketches(phases []contract.PhaseSketch) ([]contract.PhaseSketch, error) {
	seen := map[string]bool{}
	out := make([]contract.PhaseSketch, 0, len(phases))
	for _, phase := range phases {
		phase.Title = strings.TrimSpace(phase.Title)
		phase.Description = strings.TrimSpace(phase.Description)
		phase.EstimatedComplexity = strings.TrimSpace(phase.EstimatedComplexity)
		phase.Goals = nonEmptyStrings(phase.Goals)
		phase.Risks = nonEmptyStrings(phase.Risks)
		if phase.Title == "" {
			return nil, fmt.Errorf("phase title is required")
		}
		key := normalizePlanKey(phase.Title)
		if key == "" {
			key = normalizePlanKey(phase.Description)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		phase.Number = len(out) + 1
		phase.ID = fmt.Sprintf("phase_%03d", phase.Number)
		out = append(out, phase)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one phase is required")
	}
	return out, nil
}

func validateTaskPlan(phases []contract.PhaseSketch, tasks []contract.TaskSpec) error {
	if len(tasks) == 0 {
		return fmt.Errorf("at least one task is required")
	}
	phaseIDs := map[string]bool{}
	for _, phase := range phases {
		phaseIDs[phase.ID] = true
	}
	seenTasks := map[string]contract.TaskSpec{}
	for _, task := range tasks {
		if strings.TrimSpace(task.ID) == "" {
			return fmt.Errorf("task id is required")
		}
		if _, exists := seenTasks[task.ID]; exists {
			return fmt.Errorf("duplicate task id: %s", task.ID)
		}
		if !phaseIDs[task.PhaseID] {
			return fmt.Errorf("task %s references unknown phase %s", task.ID, task.PhaseID)
		}
		if strings.TrimSpace(task.Title) == "" {
			return fmt.Errorf("task %s title is required", task.ID)
		}
		if len(nonEmptyStrings(task.AcceptanceCriteria)) == 0 {
			return fmt.Errorf("task %s acceptance criteria are required", task.ID)
		}
		if taskLooksLikeWrite(task) && len(nonEmptyStrings(task.ExpectedFiles)) == 0 {
			return fmt.Errorf("write task %s expected files are required", task.ID)
		}
		for _, file := range task.ExpectedFiles {
			if err := validateExpectedFilePattern(file); err != nil {
				return fmt.Errorf("task %s expected file %q invalid: %w", task.ID, file, err)
			}
		}
		seenTasks[task.ID] = task
	}
	for _, task := range tasks {
		for _, dep := range nonEmptyStrings(task.Dependencies) {
			if dep == task.ID {
				return fmt.Errorf("task %s cannot depend on itself", task.ID)
			}
			if _, ok := seenTasks[dep]; !ok {
				return fmt.Errorf("task %s dependency not found: %s", task.ID, dep)
			}
		}
	}
	return rejectTaskDependencyCycles(tasks)
}

func taskLooksLikeWrite(task contract.TaskSpec) bool {
	for _, tool := range task.RequiredTools {
		lower := strings.ToLower(tool)
		if strings.Contains(lower, "write") || strings.Contains(lower, "edit") || strings.Contains(lower, "patch") || strings.Contains(lower, "apply") {
			return true
		}
	}
	text := strings.ToLower(strings.Join(append([]string{task.Title, task.Description}, task.Notes...), " "))
	writeWords := []string{"write", "edit", "modify", "create", "delete", "patch", "implement", "add", "update"}
	for _, word := range writeWords {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}

func validateExpectedFilePattern(pattern string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return fmt.Errorf("empty pattern")
	}
	clean := filepath.Clean(pattern)
	if filepath.IsAbs(pattern) || strings.HasPrefix(clean, "..") || clean == "." {
		return fmt.Errorf("must be a project-relative file or glob")
	}
	return nil
}

func rejectTaskDependencyCycles(tasks []contract.TaskSpec) error {
	deps := map[string][]string{}
	for _, task := range tasks {
		deps[task.ID] = append([]string(nil), nonEmptyStrings(task.Dependencies)...)
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("task dependency cycle detected at %s", id)
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		for _, dep := range deps[id] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		visiting[id] = false
		visited[id] = true
		return nil
	}
	for _, task := range tasks {
		if err := visit(task.ID); err != nil {
			return err
		}
	}
	return nil
}

func sortTasksByPhaseAndID(phases []contract.PhaseSketch, tasks []contract.TaskSpec) []contract.TaskSpec {
	phaseOrder := map[string]int{}
	for i, phase := range phases {
		phaseOrder[phase.ID] = i
	}
	out := append([]contract.TaskSpec(nil), tasks...)
	sort.SliceStable(out, func(i, j int) bool {
		if phaseOrder[out[i].PhaseID] != phaseOrder[out[j].PhaseID] {
			return phaseOrder[out[i].PhaseID] < phaseOrder[out[j].PhaseID]
		}
		return out[i].ID < out[j].ID
	})
	return out
}

var planKeyRe = regexp.MustCompile(`[^a-z0-9]+`)

func normalizePlanKey(value string) string {
	return strings.Trim(planKeyRe.ReplaceAllString(strings.ToLower(strings.TrimSpace(value)), "-"), "-")
}
