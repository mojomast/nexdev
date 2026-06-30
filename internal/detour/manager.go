package detour

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
)

const DepthExceededBlockerReason = "detour_depth_exceeded"

type IDGenerator func(prefix string) string

type WorkflowManagerConfig struct {
	Store              *state.Store
	StructuredProvider provider.StructuredClient
	MaxDepth           int
	Now                func() time.Time
	NewID              IDGenerator
	DesignSummary      string
	DesignArtifactPath string
	RepoContext        string
}

type WorkflowManager struct {
	store              *state.Store
	structuredProvider provider.StructuredClient
	maxDepth           int
	now                func() time.Time
	newID              IDGenerator
	designSummary      string
	designArtifactPath string
	repoContext        string
}

func NewWorkflowManager(cfg WorkflowManagerConfig) (*WorkflowManager, error) {
	if cfg.Store == nil {
		return nil, fmt.Errorf("detour workflow requires store")
	}
	maxDepth := cfg.MaxDepth
	if maxDepth <= 0 {
		maxDepth = DefaultMaxDepth
	}
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	newID := cfg.NewID
	if newID == nil {
		newID = func(prefix string) string { return fmt.Sprintf("%s_%d", prefix, now().UnixNano()) }
	}
	return &WorkflowManager{store: cfg.Store, structuredProvider: cfg.StructuredProvider, maxDepth: maxDepth, now: now, newID: newID, designSummary: cfg.DesignSummary, designArtifactPath: cfg.DesignArtifactPath, repoContext: cfg.RepoContext}, nil
}

func (m *WorkflowManager) RequestForBlocker(ctx context.Context, runID, blockerID string) (contract.DetourResult, error) {
	if runID == "" {
		return contract.DetourResult{}, fmt.Errorf("run id is required")
	}
	if blockerID == "" {
		return contract.DetourResult{}, fmt.Errorf("blocker id is required")
	}
	blocker, err := m.findOpenBlocker(ctx, runID, blockerID)
	if err != nil {
		return contract.DetourResult{}, err
	}
	req := contract.DetourRequest{ProjectID: blocker.ProjectID, RunID: blocker.RunID, TriggerTaskID: blocker.TaskID, Reason: blocker.Reason, Context: blocker.Description, Source: contract.DetourSourceBlockerAuto}
	return m.Request(ctx, req)
}

func (m *WorkflowManager) Request(ctx context.Context, req contract.DetourRequest) (contract.DetourResult, error) {
	if err := validateRequest(req); err != nil {
		return contract.DetourResult{}, err
	}
	requestContext, tasks, err := m.captureContext(ctx, req)
	if err != nil {
		return contract.DetourResult{}, err
	}

	depthCheck := CheckDepth(requestContext.CurrentDepth, requestContext.MaxDepth)
	if depthCheck.Decision == DepthDecisionBlock {
		return contract.DetourResult{}, m.createDepthExceededBlocker(ctx, req, requestContext.CurrentDepth)
	}

	var result contract.DetourResult
	if _, err := m.structuredProvider.CallStructured(ctx, provider.SlotPlanDetail, buildPrompt(requestContext), &result, provider.StructuredOptions{
		MaxRepairAttempts: -1,
		Validate: func(candidate any) error {
			candidateResult, ok := candidate.(*contract.DetourResult)
			if !ok {
				return fmt.Errorf("candidate is not DetourResult")
			}
			prepareGeneratedTasks(candidateResult, requestContext.CurrentDepth+1, requestContext.CurrentTask.PhaseID)
			return validateGeneratedTasks(candidateResult.NewTasks, append(existingTaskSpecs(tasks), candidateResult.NewTasks...))
		},
	}); err != nil {
		return contract.DetourResult{}, err
	}

	depth := requestContext.CurrentDepth + 1
	prepareGeneratedTasks(&result, depth, requestContext.CurrentTask.PhaseID)
	if result.ID == "" {
		result.ID = m.newID(fmt.Sprintf("detour_d%d", depth))
	}
	result.Depth = depth
	result.SplicedAfter = req.TriggerTaskID
	if err := validateGeneratedTasks(result.NewTasks, append(existingTaskSpecs(tasks), result.NewTasks...)); err != nil {
		return contract.DetourResult{}, err
	}

	inserted, splice, err := SpliceDetourTasks(tasks, req.TriggerTaskID, result)
	if err != nil {
		if len(splice.IDConflicts) > 0 {
			result.IDConflicts = append([]string(nil), splice.IDConflicts...)
		}
		return result, err
	}
	newRows := make([]*state.NexdevTask, 0, len(inserted))
	for _, task := range inserted {
		newRows = append(newRows, &state.NexdevTask{ProjectID: req.ProjectID, RunID: req.RunID, Spec: task.Spec, Status: state.NexdevTaskStatusPending, PlanVersion: task.PlanVersion, PlanOrder: task.PlanOrder, CreatedAt: m.now(), UpdatedAt: m.now()})
	}
	if _, err := m.store.InsertNexdevTasksAfter(ctx, req.TriggerTaskID, newRows); err != nil {
		return result, err
	}
	if err := m.store.UpdateNexdevTaskStatus(ctx, req.TriggerTaskID, state.NexdevTaskStatusPendingAfterDetour); err != nil {
		return result, err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return result, err
	}
	if err := m.store.CreateDetourRecord(ctx, &state.DetourRecord{ID: result.ID, ProjectID: req.ProjectID, RunID: req.RunID, TriggerTaskID: req.TriggerTaskID, Reason: req.Reason, Source: string(req.Source), Depth: result.Depth, Result: resultJSON, CreatedAt: m.now()}); err != nil {
		return result, err
	}
	if err := m.persistDetourCreatedEvent(ctx, req, result); err != nil {
		return result, err
	}
	return result, nil
}

func validateRequest(req contract.DetourRequest) error {
	if req.ProjectID == "" || req.RunID == "" || req.TriggerTaskID == "" || req.Reason == "" || req.Source == "" {
		return fmt.Errorf("project_id, run_id, trigger_task_id, reason, and source are required")
	}
	return nil
}

func (m *WorkflowManager) captureContext(ctx context.Context, req contract.DetourRequest) (RequestContext, []*state.NexdevTask, error) {
	tasks, err := m.store.ListNexdevTasks(ctx, state.NexdevTaskListOptions{RunID: req.RunID})
	if err != nil {
		return RequestContext{}, nil, err
	}
	currentIndex := -1
	for i, task := range tasks {
		if task.Spec.ID == req.TriggerTaskID {
			currentIndex = i
			break
		}
	}
	if currentIndex < 0 {
		return RequestContext{}, nil, fmt.Errorf("trigger task %s not found", req.TriggerTaskID)
	}
	currentTask := tasks[currentIndex]
	neighbors := []contract.TaskSpec{}
	if currentIndex > 0 {
		neighbors = append(neighbors, tasks[currentIndex-1].Spec)
	}
	if currentIndex+1 < len(tasks) {
		neighbors = append(neighbors, tasks[currentIndex+1].Spec)
	}

	blockerID := ""
	blockers, err := m.store.ListNexdevBlockers(ctx, state.NexdevBlockerListOptions{RunID: req.RunID, TaskID: req.TriggerTaskID, Status: state.NexdevBlockerStatusOpen})
	if err == nil && len(blockers) > 0 {
		blockerID = blockers[len(blockers)-1].ID
	}

	return RequestContext{Request: req, CurrentTask: currentTask.Spec, NeighborTasks: neighbors, BlockerID: blockerID, PhaseID: currentTask.Spec.PhaseID, DesignSummary: m.contextDesignSummary(), RepoContext: m.repoContext, CurrentDepth: depthFromTaskID(req.TriggerTaskID), MaxDepth: m.maxDepth}, tasks, nil
}

func (m *WorkflowManager) contextDesignSummary() string {
	if m.designSummary != "" && m.designArtifactPath != "" {
		return fmt.Sprintf("%s\nDesign artifact: %s", m.designSummary, filepath.Clean(m.designArtifactPath))
	}
	if m.designArtifactPath != "" {
		return fmt.Sprintf("Design artifact: %s", filepath.Clean(m.designArtifactPath))
	}
	if m.designSummary != "" {
		return m.designSummary
	}
	return "Design summary unavailable; see .nexdev/artifacts/design_draft.md when present."
}

func (m *WorkflowManager) findOpenBlocker(ctx context.Context, runID, blockerID string) (*state.NexdevBlocker, error) {
	blockers, err := m.store.ListNexdevBlockers(ctx, state.NexdevBlockerListOptions{RunID: runID, Status: state.NexdevBlockerStatusOpen})
	if err != nil {
		return nil, err
	}
	for _, blocker := range blockers {
		if blocker.ID == blockerID {
			return blocker, nil
		}
	}
	return nil, fmt.Errorf("open blocker not found: %s", blockerID)
}

func (m *WorkflowManager) createDepthExceededBlocker(ctx context.Context, req contract.DetourRequest, currentDepth int) error {
	payload, _ := json.Marshal(map[string]any{"trigger_task_id": req.TriggerTaskID, "current_depth": currentDepth, "max_depth": m.maxDepth, "source": req.Source})
	if err := m.store.CreateNexdevBlocker(ctx, &state.NexdevBlocker{ID: m.newID("blk_detour_depth"), ProjectID: req.ProjectID, RunID: req.RunID, TaskID: req.TriggerTaskID, Reason: DepthExceededBlockerReason, Description: fmt.Sprintf("detour depth %d reached max depth %d", currentDepth, m.maxDepth), Status: state.NexdevBlockerStatusOpen, Metadata: payload, CreatedAt: m.now()}); err != nil {
		return err
	}
	_ = m.store.UpdateNexdevTaskStatus(ctx, req.TriggerTaskID, state.NexdevTaskStatusBlocked)
	_ = m.store.UpdateRunStatus(ctx, req.RunID, "blocked")
	return fmt.Errorf("%s: current depth %d max depth %d", DepthExceededBlockerReason, currentDepth, m.maxDepth)
}

func (m *WorkflowManager) persistDetourCreatedEvent(ctx context.Context, req contract.DetourRequest, result contract.DetourResult) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = m.store.PersistEvent(ctx, contract.EventEnvelope{EventID: m.newID("evt_detour_created"), Type: contract.EventTypeDetourCreated, ProjectID: req.ProjectID, RunID: req.RunID, Stage: "detour", TaskID: req.TriggerTaskID, Source: contract.EventSourceCore, Payload: payload, Timestamp: m.now()})
	return err
}

func buildPrompt(ctx RequestContext) string {
	data, _ := json.MarshalIndent(ctx, "", "  ")
	return "SYSTEM POLICY\nReturn only JSON matching DetourResult. Generate the minimum task set needed to unblock the trigger task. Use provider slot plan_detail for detour planning because no dedicated detour slot exists.\n\nTRUSTED CONFIG\nDetour tasks must be small and valid TaskSpec objects. Write/edit tasks need expected_files. Dependencies may only refer to existing tasks in context or returned detour tasks.\n\nUNTRUSTED REPO CONTEXT\n" + ctx.RepoContext + "\n\nTASK\nCreate a minimal detour for this request/context:\n" + string(data)
}

func prepareGeneratedTasks(result *contract.DetourResult, depth int, phaseID string) {
	if result == nil {
		return
	}
	for i := range result.NewTasks {
		if result.NewTasks[i].ID == "" {
			result.NewTasks[i].ID = fmt.Sprintf("D%d.%02d", depth, i+1)
		}
		if result.NewTasks[i].PhaseID == "" {
			result.NewTasks[i].PhaseID = phaseID
		}
	}
}

func validateGeneratedTasks(newTasks []contract.TaskSpec, available []contract.TaskSpec) error {
	if len(newTasks) == 0 {
		return fmt.Errorf("at least one detour task is required")
	}
	availableIDs := map[string]bool{}
	for _, task := range available {
		availableIDs[task.ID] = true
	}
	newIDs := map[string]bool{}
	for _, task := range newTasks {
		if strings.TrimSpace(task.ID) == "" || strings.TrimSpace(task.PhaseID) == "" || strings.TrimSpace(task.Title) == "" {
			return fmt.Errorf("detour task id, phase_id, and title are required")
		}
		if newIDs[task.ID] {
			return fmt.Errorf("duplicate detour task id: %s", task.ID)
		}
		newIDs[task.ID] = true
		if len(nonEmptyStrings(task.AcceptanceCriteria)) == 0 {
			return fmt.Errorf("detour task %s acceptance criteria are required", task.ID)
		}
		if taskLooksLikeWrite(task) && len(nonEmptyStrings(task.ExpectedFiles)) == 0 {
			return fmt.Errorf("write detour task %s expected files are required", task.ID)
		}
		for _, file := range task.ExpectedFiles {
			if err := validateExpectedFilePattern(file); err != nil {
				return fmt.Errorf("detour task %s expected file %q invalid: %w", task.ID, file, err)
			}
		}
		for _, dep := range nonEmptyStrings(task.Dependencies) {
			if dep == task.ID {
				return fmt.Errorf("detour task %s cannot depend on itself", task.ID)
			}
			if !availableIDs[dep] && !newIDs[dep] {
				return fmt.Errorf("detour task %s dependency not found: %s", task.ID, dep)
			}
		}
	}
	return rejectDependencyCycles(newTasks)
}

func existingTaskSpecs(tasks []*state.NexdevTask) []contract.TaskSpec {
	out := make([]contract.TaskSpec, 0, len(tasks))
	for _, task := range tasks {
		if task != nil {
			out = append(out, task.Spec)
		}
	}
	return out
}

func CheckDepth(currentDepth, maxDepth int) DepthCheck {
	if maxDepth <= 0 {
		maxDepth = DefaultMaxDepth
	}
	if currentDepth >= maxDepth {
		return DepthCheck{CurrentDepth: currentDepth, MaxDepth: maxDepth, Decision: DepthDecisionBlock, BlockerReason: DepthExceededBlockerReason}
	}
	return DepthCheck{CurrentDepth: currentDepth, MaxDepth: maxDepth, Decision: DepthDecisionAllow}
}

var detourIDRe = regexp.MustCompile(`^D(\d+)\.`)

func depthFromTaskID(taskID string) int {
	matches := detourIDRe.FindStringSubmatch(taskID)
	if len(matches) != 2 {
		return 0
	}
	var depth int
	_, _ = fmt.Sscanf(matches[1], "%d", &depth)
	return depth
}

func nonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func taskLooksLikeWrite(task contract.TaskSpec) bool {
	for _, tool := range task.RequiredTools {
		lower := strings.ToLower(tool)
		if strings.Contains(lower, "write") || strings.Contains(lower, "edit") || strings.Contains(lower, "patch") || strings.Contains(lower, "apply") {
			return true
		}
	}
	text := strings.ToLower(strings.Join(append([]string{task.Title, task.Description}, task.Notes...), " "))
	for _, word := range []string{"write", "edit", "modify", "create", "delete", "patch", "implement", "add", "update"} {
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

func rejectDependencyCycles(tasks []contract.TaskSpec) error {
	deps := map[string][]string{}
	for _, task := range tasks {
		deps[task.ID] = append([]string(nil), nonEmptyStrings(task.Dependencies)...)
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if !depsHasTask(deps, id) {
			return nil
		}
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
	keys := make([]string, 0, len(deps))
	for id := range deps {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	for _, id := range keys {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func depsHasTask(deps map[string][]string, id string) bool {
	_, ok := deps[id]
	return ok
}
