package pipeline

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/state"
)

const (
	ReviewModeManual ReviewMode = "manual"
	ReviewModeAuto   ReviewMode = "auto"
	ReviewModeCI     ReviewMode = "ci"
	ReviewModeSkip   ReviewMode = "skip"

	ReviewEditUpdateTask  = "update_task"
	ReviewEditDeleteTask  = "delete_task"
	ReviewEditApprovePlan = "approve_plan"

	reviewApprovalArtifactRelPath = ".nexdev/artifacts/review_approval.json"
	reviewApprovalArtifactKind    = "review_approval"
	reviewApprovedMarker          = "reviewed_approved_plan"
)

type ReviewMode string

type ReviewStageConfig struct {
	Mode            ReviewMode
	Actor           string
	AllowSkipReview bool
	ProjectRoot     string
}

type ReviewStage struct {
	config ReviewStageConfig
	output ReviewApproval
}

type ReviewApproval struct {
	Marker             string     `json:"marker"`
	Approved           bool       `json:"approved"`
	Mode               ReviewMode `json:"mode"`
	Actor              string     `json:"actor"`
	PlanVersion        int        `json:"plan_version"`
	TaskCount          int        `json:"task_count"`
	ApprovedAt         time.Time  `json:"approved_at"`
	RiskAcknowledgment string     `json:"risk_acknowledgment,omitempty"`
}

type ReviewTaskPatch struct {
	Title              *string   `json:"title,omitempty"`
	Description        *string   `json:"description,omitempty"`
	ExpectedFiles      *[]string `json:"expected_files,omitempty"`
	AcceptanceCriteria *[]string `json:"acceptance_criteria,omitempty"`
	TestCommands       *[]string `json:"test_commands,omitempty"`
	RiskLevel          *string   `json:"risk_level,omitempty"`
	RequiredTools      *[]string `json:"required_tools,omitempty"`
	Notes              *[]string `json:"notes,omitempty"`
}

type ReviewService struct {
	Store       *state.Store
	ProjectID   string
	RunID       string
	ProjectRoot string
}

func NewReviewStage(cfg ReviewStageConfig) *ReviewStage {
	return &ReviewStage{config: cfg}
}

func (s *ReviewStage) Name() Stage { return StageReview }

func (s *ReviewStage) Validate(ctx context.Context, env StageEnv) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if env.Project == nil {
		return fmt.Errorf("review requires project")
	}
	if env.Run == nil {
		return fmt.Errorf("review requires run")
	}
	if _, err := reviewStore(env); err != nil {
		return err
	}
	mode := s.reviewMode()
	if mode == ReviewModeSkip && !s.config.AllowSkipReview {
		return fmt.Errorf("review skip mode requires explicit skip-review allowance")
	}
	return nil
}

func (s *ReviewStage) Run(ctx context.Context, env StageEnv) error {
	if err := s.Validate(ctx, env); err != nil {
		return err
	}
	service, err := s.service(env)
	if err != nil {
		return err
	}
	if approval, ok, err := service.LoadApproval(ctx); err != nil {
		return err
	} else if ok {
		s.output = approval
		return nil
	}
	summary, err := service.PendingPlan(ctx)
	if err != nil {
		return err
	}
	switch s.reviewMode() {
	case ReviewModeManual:
		return &BlockedError{Reason: "review_required"}
	case ReviewModeAuto:
		approval, err := service.approveWithMode(ctx, ReviewModeAuto, s.actor(), "auto reviewer accepted plan")
		if err != nil {
			return err
		}
		s.output = approval
		return nil
	case ReviewModeCI:
		if err := rejectHighRiskWithoutTests(summary.Tasks); err != nil {
			return err
		}
		approval, err := service.approveWithMode(ctx, ReviewModeCI, s.actor(), "ci policy accepted plan")
		if err != nil {
			return err
		}
		s.output = approval
		return nil
	case ReviewModeSkip:
		approval, err := service.approveWithMode(ctx, ReviewModeSkip, s.actor(), "explicit skip-review")
		if err != nil {
			return err
		}
		s.output = approval
		return nil
	default:
		return fmt.Errorf("unknown review mode %q", s.reviewMode())
	}
}

func (s *ReviewStage) Resume(ctx context.Context, env StageEnv) error { return s.Run(ctx, env) }

func (s *ReviewStage) Output(ctx context.Context, env StageEnv) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !s.output.Approved {
		return map[string]any{}, nil
	}
	return structToMap(s.output)
}

func (s *ReviewStage) reviewMode() ReviewMode {
	if s.config.Mode == "" {
		return ReviewModeManual
	}
	return s.config.Mode
}

func (s *ReviewStage) actor() string {
	if strings.TrimSpace(s.config.Actor) == "" {
		return "system"
	}
	return strings.TrimSpace(s.config.Actor)
}

func (s *ReviewStage) service(env StageEnv) (*ReviewService, error) {
	store, err := reviewStore(env)
	if err != nil {
		return nil, err
	}
	return &ReviewService{Store: store, ProjectID: env.Project.ProjectID(), RunID: env.Run.RunID(), ProjectRoot: s.projectRoot()}, nil
}

func (s *ReviewStage) projectRoot() string {
	if strings.TrimSpace(s.config.ProjectRoot) != "" {
		return s.config.ProjectRoot
	}
	return "."
}

func reviewStore(env StageEnv) (*state.Store, error) {
	store, ok := env.Store.(*state.Store)
	if !ok || store == nil {
		return nil, fmt.Errorf("review requires state store")
	}
	return store, nil
}

type pendingPlan struct {
	PlanVersion int
	Tasks       []*state.NexdevTask
}

func (s *ReviewService) PendingPlan(ctx context.Context) (pendingPlan, error) {
	if s == nil || s.Store == nil {
		return pendingPlan{}, fmt.Errorf("review service requires state store")
	}
	all, err := s.Store.ListNexdevTasks(ctx, state.NexdevTaskListOptions{RunID: s.RunID})
	if err != nil {
		return pendingPlan{}, err
	}
	if len(all) == 0 {
		return pendingPlan{}, fmt.Errorf("review requires persisted pending plan tasks")
	}
	latest := 0
	for _, task := range all {
		if task.PlanVersion > latest {
			latest = task.PlanVersion
		}
	}
	latestTasks := make([]*state.NexdevTask, 0, len(all))
	for _, task := range all {
		if task.PlanVersion == latest {
			latestTasks = append(latestTasks, task)
		}
	}
	if err := validateReviewableTasks(latestTasks); err != nil {
		return pendingPlan{}, err
	}
	return pendingPlan{PlanVersion: latest, Tasks: latestTasks}, nil
}

func (s *ReviewService) Approve(ctx context.Context, actor, riskAcknowledgment string) (ReviewApproval, error) {
	return s.approveWithMode(ctx, ReviewModeManual, actor, riskAcknowledgment)
}

func (s *ReviewService) approveWithMode(ctx context.Context, mode ReviewMode, actor, riskAcknowledgment string) (ReviewApproval, error) {
	plan, err := s.PendingPlan(ctx)
	if err != nil {
		return ReviewApproval{}, err
	}
	approval := ReviewApproval{Marker: reviewApprovedMarker, Approved: true, Mode: mode, Actor: defaultActor(actor), PlanVersion: plan.PlanVersion, TaskCount: len(plan.Tasks), ApprovedAt: time.Now().UTC(), RiskAcknowledgment: strings.TrimSpace(riskAcknowledgment)}
	if err := s.writeApproval(ctx, approval); err != nil {
		return ReviewApproval{}, err
	}
	if err := s.Store.CreatePlanEditEvent(ctx, &state.PlanEditEvent{ID: reviewEditID(s.RunID, ReviewEditApprovePlan, approval.PlanVersion, approval.Actor), ProjectID: s.ProjectID, RunID: s.RunID, PlanVersionBefore: plan.PlanVersion, PlanVersionAfter: plan.PlanVersion, EditType: ReviewEditApprovePlan, Patch: mustJSON(map[string]any{"approved": true, "marker": reviewApprovedMarker, "risk_acknowledgment": approval.RiskAcknowledgment}), Actor: approval.Actor, CreatedAt: approval.ApprovedAt}); err != nil {
		return ReviewApproval{}, err
	}
	return approval, nil
}

func (s *ReviewService) UpdatePendingTask(ctx context.Context, taskID string, patch ReviewTaskPatch, actor string) (int, error) {
	plan, err := s.PendingPlan(ctx)
	if err != nil {
		return 0, err
	}
	tasks := cloneReviewTasks(plan.Tasks)
	idx := -1
	for i := range tasks {
		if tasks[i].Spec.ID == taskID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return 0, fmt.Errorf("pending task not found: %s", taskID)
	}
	if tasks[idx].Status != state.NexdevTaskStatusPending {
		return 0, fmt.Errorf("cannot edit non-pending task %s with status %s", taskID, tasks[idx].Status)
	}
	applyReviewPatch(&tasks[idx].Spec, patch)
	if err := validateReviewableTasks(tasks); err != nil {
		return 0, err
	}
	nextVersion := plan.PlanVersion + 1
	if err := s.updateTaskRows(ctx, tasks, nextVersion); err != nil {
		return 0, err
	}
	if err := s.Store.CreatePlanEditEvent(ctx, &state.PlanEditEvent{ID: reviewEditID(s.RunID, ReviewEditUpdateTask, nextVersion, taskID), ProjectID: s.ProjectID, RunID: s.RunID, PlanVersionBefore: plan.PlanVersion, PlanVersionAfter: nextVersion, EditType: ReviewEditUpdateTask, TargetID: taskID, Patch: mustJSON(patch), Actor: defaultActor(actor)}); err != nil {
		return 0, err
	}
	return nextVersion, nil
}

func (s *ReviewService) DeletePendingTask(ctx context.Context, taskID, actor string) (int, error) {
	plan, err := s.PendingPlan(ctx)
	if err != nil {
		return 0, err
	}
	tasks := cloneReviewTasks(plan.Tasks)
	idx := -1
	for i, task := range tasks {
		if task.Spec.ID == taskID {
			idx = i
		}
		for _, dep := range task.Spec.Dependencies {
			if dep == taskID {
				return 0, fmt.Errorf("cannot delete task %s while task %s depends on it", taskID, task.Spec.ID)
			}
		}
	}
	if idx < 0 {
		return 0, fmt.Errorf("pending task not found: %s", taskID)
	}
	if tasks[idx].Status != state.NexdevTaskStatusPending {
		return 0, fmt.Errorf("cannot delete non-pending task %s with status %s", taskID, tasks[idx].Status)
	}
	tasks = append(tasks[:idx], tasks[idx+1:]...)
	if len(tasks) == 0 {
		return 0, fmt.Errorf("cannot delete the last pending plan task")
	}
	if err := validateReviewableTasks(tasks); err != nil {
		return 0, err
	}
	nextVersion := plan.PlanVersion + 1
	if err := s.deleteTaskRow(ctx, taskID, tasks, nextVersion); err != nil {
		return 0, err
	}
	if err := s.Store.CreatePlanEditEvent(ctx, &state.PlanEditEvent{ID: reviewEditID(s.RunID, ReviewEditDeleteTask, nextVersion, taskID), ProjectID: s.ProjectID, RunID: s.RunID, PlanVersionBefore: plan.PlanVersion, PlanVersionAfter: nextVersion, EditType: ReviewEditDeleteTask, TargetID: taskID, Patch: mustJSON(map[string]any{"deleted_task_id": taskID}), Actor: defaultActor(actor)}); err != nil {
		return 0, err
	}
	return nextVersion, nil
}

func (s *ReviewService) LoadApproval(ctx context.Context) (ReviewApproval, bool, error) {
	data, err := os.ReadFile(filepath.Join(s.projectRoot(), reviewApprovalArtifactRelPath))
	if err != nil {
		if os.IsNotExist(err) {
			return ReviewApproval{}, false, nil
		}
		return ReviewApproval{}, false, err
	}
	var approval ReviewApproval
	if err := json.Unmarshal(data, &approval); err != nil {
		return ReviewApproval{}, false, err
	}
	return approval, approval.Approved && approval.Marker == reviewApprovedMarker, nil
}

func (s *ReviewService) writeApproval(ctx context.Context, approval ReviewApproval) error {
	data, err := json.MarshalIndent(approval, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := filepath.Join(s.projectRoot(), reviewApprovalArtifactRelPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	hash := sha256.Sum256(data)
	idHash := sha256.Sum256([]byte(s.ProjectID + ":" + s.RunID + ":" + reviewApprovalArtifactKind))
	now := time.Now().UTC()
	return s.Store.UpsertArtifact(ctx, &state.Artifact{ID: "artifact_" + hex.EncodeToString(idHash[:8]), ProjectID: s.ProjectID, RunID: s.RunID, Kind: reviewApprovalArtifactKind, Path: reviewApprovalArtifactRelPath, SHA256: hex.EncodeToString(hash[:]), Version: approval.PlanVersion, Metadata: map[string]any{"stage": string(StageReview), "marker": reviewApprovedMarker}, CreatedAt: now, UpdatedAt: now})
}

func (s *ReviewService) updateTaskRows(ctx context.Context, tasks []*state.NexdevTask, nextVersion int) error {
	tx, err := s.Store.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, task := range tasks {
		if err := updateTaskRow(ctx, tx, task, nextVersion, i+1); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *ReviewService) deleteTaskRow(ctx context.Context, taskID string, tasks []*state.NexdevTask, nextVersion int) error {
	tx, err := s.Store.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if result, err := tx.ExecContext(ctx, `DELETE FROM nexdev_tasks WHERE id = ? AND run_id = ?`, taskID, s.RunID); err != nil {
		return err
	} else if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("delete pending task affected %d rows", affected)
	}
	for i, task := range tasks {
		if err := updateTaskRow(ctx, tx, task, nextVersion, i+1); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func updateTaskRow(ctx context.Context, tx *sql.Tx, task *state.NexdevTask, planVersion, planOrder int) error {
	expectedFiles, _ := json.Marshal(nonEmptyStrings(task.Spec.ExpectedFiles))
	dependencies, _ := json.Marshal(nonEmptyStrings(task.Spec.Dependencies))
	acceptanceCriteria, _ := json.Marshal(nonEmptyStrings(task.Spec.AcceptanceCriteria))
	testCommands, _ := json.Marshal(nonEmptyStrings(task.Spec.TestCommands))
	requiredTools, _ := json.Marshal(nonEmptyStrings(task.Spec.RequiredTools))
	notes, _ := json.Marshal(nonEmptyStrings(task.Spec.Notes))
	_, err := tx.ExecContext(ctx, `UPDATE nexdev_tasks SET title = ?, description = ?, expected_files_json = ?, dependencies_json = ?, acceptance_criteria_json = ?, test_commands_json = ?, risk_level = ?, required_tools_json = ?, notes_json = ?, plan_version = ?, plan_order = ?, updated_at = ? WHERE id = ? AND run_id = ?`, task.Spec.Title, task.Spec.Description, string(expectedFiles), string(dependencies), string(acceptanceCriteria), string(testCommands), task.Spec.RiskLevel, string(requiredTools), string(notes), planVersion, planOrder, time.Now().UTC().Format(time.RFC3339Nano), task.Spec.ID, task.RunID)
	if err != nil {
		return fmt.Errorf("update review task %s: %w", task.Spec.ID, err)
	}
	return nil
}

func validateReviewableTasks(tasks []*state.NexdevTask) error {
	if len(tasks) == 0 {
		return fmt.Errorf("review requires at least one task")
	}
	phases := phasesFromTasks(tasks)
	specs := make([]contract.TaskSpec, 0, len(tasks))
	for _, task := range tasks {
		if task.Status != state.NexdevTaskStatusPending {
			return fmt.Errorf("task %s is not pending: %s", task.Spec.ID, task.Status)
		}
		specs = append(specs, task.Spec)
	}
	return validateTaskPlan(phases, specs)
}

func phasesFromTasks(tasks []*state.NexdevTask) []contract.PhaseSketch {
	seen := map[string]bool{}
	var ids []string
	for _, task := range tasks {
		if !seen[task.Spec.PhaseID] {
			seen[task.Spec.PhaseID] = true
			ids = append(ids, task.Spec.PhaseID)
		}
	}
	sort.Strings(ids)
	phases := make([]contract.PhaseSketch, 0, len(ids))
	for i, id := range ids {
		phases = append(phases, contract.PhaseSketch{ID: id, Number: i + 1, Title: id})
	}
	return phases
}

func rejectHighRiskWithoutTests(tasks []*state.NexdevTask) error {
	for _, task := range tasks {
		if strings.EqualFold(strings.TrimSpace(task.Spec.RiskLevel), "high") || strings.EqualFold(strings.TrimSpace(task.Spec.RiskLevel), "critical") {
			if len(nonEmptyStrings(task.Spec.TestCommands)) == 0 {
				return fmt.Errorf("ci review rejects high-risk task %s without test commands", task.Spec.ID)
			}
		}
	}
	return nil
}

func cloneReviewTasks(tasks []*state.NexdevTask) []*state.NexdevTask {
	out := make([]*state.NexdevTask, 0, len(tasks))
	for _, task := range tasks {
		clone := *task
		clone.Spec.ExpectedFiles = append([]string(nil), task.Spec.ExpectedFiles...)
		clone.Spec.Dependencies = append([]string(nil), task.Spec.Dependencies...)
		clone.Spec.AcceptanceCriteria = append([]string(nil), task.Spec.AcceptanceCriteria...)
		clone.Spec.TestCommands = append([]string(nil), task.Spec.TestCommands...)
		clone.Spec.RequiredTools = append([]string(nil), task.Spec.RequiredTools...)
		clone.Spec.Notes = append([]string(nil), task.Spec.Notes...)
		out = append(out, &clone)
	}
	return out
}

func applyReviewPatch(spec *contract.TaskSpec, patch ReviewTaskPatch) {
	if patch.Title != nil {
		spec.Title = strings.TrimSpace(*patch.Title)
	}
	if patch.Description != nil {
		spec.Description = strings.TrimSpace(*patch.Description)
	}
	if patch.ExpectedFiles != nil {
		spec.ExpectedFiles = nonEmptyStrings(*patch.ExpectedFiles)
	}
	if patch.AcceptanceCriteria != nil {
		spec.AcceptanceCriteria = nonEmptyStrings(*patch.AcceptanceCriteria)
	}
	if patch.TestCommands != nil {
		spec.TestCommands = nonEmptyStrings(*patch.TestCommands)
	}
	if patch.RiskLevel != nil {
		spec.RiskLevel = strings.TrimSpace(*patch.RiskLevel)
	}
	if patch.RequiredTools != nil {
		spec.RequiredTools = nonEmptyStrings(*patch.RequiredTools)
	}
	if patch.Notes != nil {
		spec.Notes = nonEmptyStrings(*patch.Notes)
	}
}

func defaultActor(actor string) string {
	if strings.TrimSpace(actor) == "" {
		return "operator"
	}
	return strings.TrimSpace(actor)
}

func reviewEditID(runID, editType string, version int, target string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d:%s:%d", runID, editType, version, target, time.Now().UTC().UnixNano())))
	return "edit_" + hex.EncodeToString(hash[:8])
}

func mustJSON(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}

func (s *ReviewService) projectRoot() string {
	if strings.TrimSpace(s.ProjectRoot) != "" {
		return s.ProjectRoot
	}
	return "."
}
