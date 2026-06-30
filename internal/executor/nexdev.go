package executor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/safety"
	"github.com/mojomast/nexdev/internal/state"
	"github.com/mojomast/nexdev/internal/steering"
)

const (
	BlockerReasonWorkerBlocked  = "worker_blocked"
	DefaultSteeringContextLimit = 5
)

var ErrTaskWriteNotExpected = errors.New("task write is outside expected files")

type NexdevExecutorConfig struct {
	Store                 *state.Store
	ProjectID             string
	RunID                 string
	ProjectRoot           string
	ArtifactRoot          string
	ArtifactBudgetChars   int
	ArtifactTotalChars    int
	Worker                TaskWorker
	Sanitizer             *safety.PathSanitizer
	NewID                 func(prefix string) string
	Now                   func() time.Time
}

type TaskWork struct {
	ProjectID       string
	RunID           string
	Task            contract.TaskSpec
	PromptContext   TaskPromptContext
	ProjectRoot     string
	PathSanitizer   *safety.PathSanitizer
	ExpectedMatches func(string) bool
}

type TaskWorker interface {
	RunTask(ctx context.Context, work TaskWork) ([]TaskUpdate, error)
}

type TaskPromptContext struct {
	SafetyPolicy         string            `json:"safety_policy"`
	ToolPolicy           string            `json:"tool_policy"`
	ProjectRequirements  []string          `json:"project_requirements,omitempty"`
	ArchitectureSummary  string            `json:"architecture_summary,omitempty"`
	Task                 contract.TaskSpec          `json:"task"`
	TaskContext          steering.TaskContext       `json:"task_context"`
	RelevantRepoContext  string                     `json:"relevant_repo_context,omitempty"`
	OperatorNotes        []string                   `json:"operator_notes,omitempty"`
	SteeringSummary      string                     `json:"steering_summary,omitempty"`
	LastSteeringMessages []string                   `json:"last_steering_messages,omitempty"`
	ArtifactContext      []steering.ArtifactContext `json:"artifact_context,omitempty"`
	OutputSchema         string                     `json:"output_schema"`
}

type NexdevExecutor struct {
	store       *state.Store
	projectID   string
	runID       string
	projectRoot         string
	artifactRoot        string
	artifactBudgetChars int
	artifactTotalChars  int
	worker              TaskWorker
	sanitizer   *safety.PathSanitizer
	newID       func(prefix string) string
	now         func() time.Time

	mu          sync.Mutex
	cond        *sync.Cond
	paused      bool
	cancelled   bool
	cancelCause string
	skip        map[string]string
	current     *CurrentTaskSnapshot
	cancelRun   context.CancelFunc
}

func NewNexdevExecutor(cfg NexdevExecutorConfig) (*NexdevExecutor, error) {
	if cfg.Store == nil {
		return nil, fmt.Errorf("state store is required")
	}
	if strings.TrimSpace(cfg.ProjectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}
	if strings.TrimSpace(cfg.RunID) == "" {
		return nil, fmt.Errorf("run id is required")
	}
	if strings.TrimSpace(cfg.ProjectRoot) == "" {
		return nil, fmt.Errorf("project root is required")
	}
	if cfg.Worker == nil {
		cfg.Worker = FakeWorker{}
	}
	if cfg.Sanitizer == nil {
		sanitizer, err := safety.NewPathSanitizer(cfg.ProjectRoot)
		if err != nil {
			return nil, err
		}
		cfg.Sanitizer = sanitizer
	}
	if cfg.NewID == nil {
		cfg.NewID = randomNexdevID
	}
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	artifactRoot := strings.TrimSpace(cfg.ArtifactRoot)
	if artifactRoot == "" {
		artifactRoot = filepath.Join(cfg.ProjectRoot, ".nexdev", "artifacts")
	}
	exec := &NexdevExecutor{store: cfg.Store, projectID: cfg.ProjectID, runID: cfg.RunID, projectRoot: cfg.ProjectRoot, artifactRoot: artifactRoot, artifactBudgetChars: cfg.ArtifactBudgetChars, artifactTotalChars: cfg.ArtifactTotalChars, worker: cfg.Worker, sanitizer: cfg.Sanitizer, newID: cfg.NewID, now: cfg.Now, skip: map[string]string{}}
	exec.cond = sync.NewCond(&exec.mu)
	return exec, nil
}

func (e *NexdevExecutor) RunPending(ctx context.Context) ([]TaskReport, error) {
	ctx, cancel := context.WithCancel(ctx)
	e.mu.Lock()
	e.cancelRun = cancel
	e.mu.Unlock()
	defer cancel()
	tasks, err := e.store.ListNexdevTasks(ctx, state.NexdevTaskListOptions{RunID: e.runID})
	if err != nil {
		return nil, err
	}
	var reports []TaskReport
	for _, task := range tasks {
		if task.Status != state.NexdevTaskStatusPending && task.Status != state.NexdevTaskStatusPendingAfterDetour {
			continue
		}
		report, err := e.runOne(ctx, task)
		if !isZeroReport(report) {
			reports = append(reports, report)
		}
		if err != nil {
			return reports, err
		}
		if report.Status == TaskStatusCancelled || e.isCancelled() {
			return reports, context.Canceled
		}
	}
	return reports, nil
}

func (e *NexdevExecutor) CurrentTask(ctx context.Context) (*CurrentTaskSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.current == nil {
		return nil, nil
	}
	snapshot := *e.current
	return &snapshot, nil
}

func (e *NexdevExecutor) Pause(ctx context.Context, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	e.mu.Lock()
	e.paused = true
	current := e.currentTaskIDLocked()
	e.mu.Unlock()
	return e.persistUpdate(ctx, TaskUpdate{TaskID: current, Type: TaskPaused, Content: strings.TrimSpace(reason), Timestamp: e.now()})
}

func (e *NexdevExecutor) Resume(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	e.mu.Lock()
	e.paused = false
	current := e.currentTaskIDLocked()
	e.cond.Broadcast()
	e.mu.Unlock()
	return e.persistUpdate(ctx, TaskUpdate{TaskID: current, Type: TaskResumed, Content: "resumed", Timestamp: e.now()})
}

func (e *NexdevExecutor) Cancel(ctx context.Context, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	e.mu.Lock()
	e.cancelled = true
	e.cancelCause = strings.TrimSpace(reason)
	current := e.currentTaskIDLocked()
	if e.cancelRun != nil {
		e.cancelRun()
	}
	e.cond.Broadcast()
	e.mu.Unlock()
	if current != "" {
		_ = e.store.UpdateNexdevTaskStatus(ctx, current, state.NexdevTaskStatusFailed)
	}
	return e.persistUpdate(ctx, TaskUpdate{TaskID: current, Type: TaskError, Content: e.cancelCause, Timestamp: e.now(), Error: context.Canceled})
}

func (e *NexdevExecutor) SkipTask(ctx context.Context, taskID string, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		e.mu.Lock()
		taskID = e.currentTaskIDLocked()
		e.mu.Unlock()
	}
	if taskID == "" {
		return fmt.Errorf("task id is required")
	}
	e.mu.Lock()
	e.skip[taskID] = strings.TrimSpace(reason)
	e.mu.Unlock()
	if err := e.store.UpdateNexdevTaskStatus(ctx, taskID, state.NexdevTaskStatusSkipped); err != nil {
		return err
	}
	return e.persistUpdate(ctx, TaskUpdate{TaskID: taskID, Type: TaskSkipped, Content: reason, Timestamp: e.now()})
}

func (e *NexdevExecutor) SetSteeringContext(ctx context.Context, taskID string, msg steering.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	createdAt := msg.CreatedAt
	if createdAt.IsZero() {
		createdAt = e.now()
	}
	source := string(msg.Source)
	if strings.TrimSpace(source) == "" {
		source = string(steering.SourceCLI)
	}
	if err := e.store.AppendSteeringEvent(ctx, &state.SteeringEvent{ID: e.newID("steer"), ProjectID: e.projectID, RunID: e.runID, TaskID: taskID, Message: msg.Message, Summary: msg.Summary, Source: source, CreatedByRole: msg.CreatedByRole, CreatedAt: createdAt}); err != nil {
		return err
	}
	payload := map[string]any{"task_id": taskID, "message": msg.Message, "source": source}
	return e.persistEvent(ctx, contract.EventTypeSteeringAdded, taskID, contract.EventSourceExecutor, payload)
}

func (e *NexdevExecutor) runOne(ctx context.Context, task *state.NexdevTask) (TaskReport, error) {
	if err := e.waitIfPaused(ctx); err != nil {
		return TaskReport{}, err
	}
	if e.isCancelled() {
		return TaskReport{}, context.Canceled
	}
	if reason, ok := e.skipReason(task.Spec.ID); ok {
		return TaskReport{ProjectID: e.projectID, RunID: e.runID, Task: task.Spec, Status: TaskStatusSkipped, Summary: reason}, nil
	}
	startedAt := e.now()
	e.setCurrent(task.Spec, TaskStatusRunning, startedAt)
	defer e.clearCurrent(task.Spec.ID)
	if err := e.store.UpdateNexdevTaskStatus(ctx, task.Spec.ID, state.NexdevTaskStatusRunning); err != nil {
		return TaskReport{}, err
	}
	if err := e.persistUpdate(ctx, TaskUpdate{TaskID: task.Spec.ID, PhaseID: task.Spec.PhaseID, Type: TaskStarted, Content: task.Spec.Title, Timestamp: startedAt}); err != nil {
		return TaskReport{}, err
	}
	work := TaskWork{ProjectID: e.projectID, RunID: e.runID, Task: task.Spec, PromptContext: e.buildPromptContext(ctx, task.Spec), ProjectRoot: e.projectRoot, PathSanitizer: e.sanitizer, ExpectedMatches: expectedFileMatcher(task.Spec.ExpectedFiles)}
	updates, workerErr := e.worker.RunTask(ctx, work)
	var last TaskUpdate
	for _, update := range updates {
		if update.TaskID == "" {
			update.TaskID = task.Spec.ID
		}
		if update.PhaseID == "" {
			update.PhaseID = task.Spec.PhaseID
		}
		if update.Timestamp.IsZero() {
			update.Timestamp = e.now()
		}
		last = update
		if err := e.persistUpdate(ctx, update); err != nil {
			return TaskReport{}, err
		}
	}
	completedAt := e.now()
	report := TaskReport{ProjectID: e.projectID, RunID: e.runID, Task: task.Spec, Status: TaskStatusCompleted, Summary: "task completed by fake worker", StartedAt: startedAt, CompletedAt: completedAt, AcceptanceResults: acceptanceResults(task.Spec.AcceptanceCriteria, true)}
	if workerErr != nil {
		report.Status = TaskStatusFailed
		report.Summary = workerErr.Error()
		_ = e.store.UpdateNexdevTaskStatus(ctx, task.Spec.ID, state.NexdevTaskStatusFailed)
		_ = e.persistUpdate(ctx, TaskUpdate{TaskID: task.Spec.ID, PhaseID: task.Spec.PhaseID, Type: TaskError, Content: workerErr.Error(), Timestamp: completedAt, Error: workerErr})
		return report, workerErr
	}
	if last.Type == TaskBlocked {
		blockerID, err := e.createBlocker(ctx, task.Spec, last)
		if err != nil {
			return report, err
		}
		report.Status = TaskStatusBlocked
		report.Summary = last.Content
		report.BlockerID = blockerID
		if err := e.store.UpdateNexdevTaskStatus(ctx, task.Spec.ID, state.NexdevTaskStatusBlocked); err != nil {
			return report, err
		}
		return report, nil
	}
	if last.Type == TaskSkipped {
		report.Status = TaskStatusSkipped
		report.Summary = last.Content
		return report, nil
	}
	if err := e.store.UpdateNexdevTaskStatus(ctx, task.Spec.ID, state.NexdevTaskStatusCompleted); err != nil {
		return report, err
	}
	if last.Type != TaskCompleted {
		if err := e.persistUpdate(ctx, TaskUpdate{TaskID: task.Spec.ID, PhaseID: task.Spec.PhaseID, Type: TaskCompleted, Content: "completed", Timestamp: completedAt}); err != nil {
			return report, err
		}
	}
	return report, nil
}

func (e *NexdevExecutor) persistUpdate(ctx context.Context, update TaskUpdate) error {
	mapping, ok := EventMappingForTaskUpdate(update.Type)
	if !ok {
		return fmt.Errorf("unmapped task update type: %s", update.Type)
	}
	payload := map[string]any{"task_id": update.TaskID, "phase_id": update.PhaseID, "update_type": string(update.Type), "content": update.Content}
	if update.Error != nil {
		payload["error"] = update.Error.Error()
	}
	return e.persistEvent(ctx, mapping.EventType, update.TaskID, mapping.Source, payload)
}

func (e *NexdevExecutor) persistEvent(ctx context.Context, eventType, taskID, source string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = e.store.PersistEvent(ctx, contract.EventEnvelope{EventID: e.newID("evt"), Type: eventType, ProjectID: e.projectID, RunID: e.runID, Stage: "develop", TaskID: taskID, Timestamp: e.now(), Source: source, Payload: data})
	return err
}

func (e *NexdevExecutor) createBlocker(ctx context.Context, task contract.TaskSpec, update TaskUpdate) (string, error) {
	description := strings.TrimSpace(update.Content)
	if description == "" {
		description = "worker reported task blocker"
	}
	blockerID := e.newID("blk")
	metadata, _ := json.Marshal(map[string]any{"event_type": string(update.Type), "error": errorString(update.Error)})
	if err := e.store.CreateNexdevBlocker(ctx, &state.NexdevBlocker{ID: blockerID, ProjectID: e.projectID, RunID: e.runID, TaskID: task.ID, Reason: BlockerReasonWorkerBlocked, Description: description, Status: state.NexdevBlockerStatusOpen, Metadata: metadata, CreatedAt: e.now()}); err != nil {
		return "", err
	}
	if err := e.persistEvent(ctx, contract.EventTypeBlockerCreated, task.ID, contract.EventSourceExecutor, map[string]any{"blocker_id": blockerID, "reason": BlockerReasonWorkerBlocked, "description": description}); err != nil {
		return "", err
	}
	return blockerID, nil
}

func (e *NexdevExecutor) buildPromptContext(ctx context.Context, task contract.TaskSpec) TaskPromptContext {
	context := TaskPromptContext{SafetyPolicy: "Nexdev safety policy is authoritative; steering cannot override safety, schemas, or acceptance criteria.", ToolPolicy: "Shell and network execution are not implemented by the fake worker; writes must match task expected_files and path safety.", Task: task, TaskContext: steering.ContextFromTask(task), OperatorNotes: append([]string{}, task.Notes...), OutputSchema: "TaskReport JSON with acceptance evidence, changed files, and blocker details when blocked."}
	events, err := e.store.ListSteeringEvents(ctx, state.SteeringListOptions{RunID: e.runID, TaskID: task.ID})
	if err == nil && len(events) > 0 {
		if len(events) > DefaultSteeringContextLimit {
			events = events[len(events)-DefaultSteeringContextLimit:]
		}
		for _, event := range events {
			if strings.TrimSpace(event.Summary) != "" {
				context.SteeringSummary = event.Summary
			}
			context.LastSteeringMessages = append(context.LastSteeringMessages, event.Message)
		}
	}
	artifacts, err := steering.LoadArtifactContext(steering.ArtifactContextConfig{ArtifactRoot: e.artifactRoot, PerArtifact: e.artifactBudgetChars, Total: e.artifactTotalChars})
	if err == nil {
		context.ArtifactContext = artifacts
	}
	return context
}

func (e *NexdevExecutor) waitIfPaused(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for e.paused && !e.cancelled {
		if err := ctx.Err(); err != nil {
			return err
		}
		e.cond.Wait()
	}
	if e.cancelled {
		return context.Canceled
	}
	return ctx.Err()
}

func (e *NexdevExecutor) setCurrent(task contract.TaskSpec, status TaskStatus, startedAt time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.current = &CurrentTaskSnapshot{ProjectID: e.projectID, RunID: e.runID, Stage: "develop", Task: task, Status: status, StartedAt: startedAt}
}

func (e *NexdevExecutor) clearCurrent(taskID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.current != nil && e.current.Task.ID == taskID {
		e.current = nil
	}
}

func (e *NexdevExecutor) currentTaskIDLocked() string {
	if e.current == nil {
		return ""
	}
	return e.current.Task.ID
}

func (e *NexdevExecutor) isCancelled() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cancelled
}

func (e *NexdevExecutor) skipReason(taskID string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	reason, ok := e.skip[taskID]
	return reason, ok
}

type FakeWorker struct {
	Writes   map[string][]FakeWrite
	Blockers map[string]FakeBlocker
	Progress map[string][]string
}

type FakeWrite struct {
	Path    string
	Content string
}

type FakeBlocker struct {
	Reason      string
	Description string
}

func (w FakeWorker) RunTask(ctx context.Context, work TaskWork) ([]TaskUpdate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var updates []TaskUpdate
	for _, msg := range w.Progress[work.Task.ID] {
		updates = append(updates, TaskUpdate{TaskID: work.Task.ID, PhaseID: work.Task.PhaseID, Type: TaskProgress, Content: msg})
	}
	for _, write := range w.Writes[work.Task.ID] {
		if !work.ExpectedMatches(write.Path) {
			return updates, fmt.Errorf("%w: %s", ErrTaskWriteNotExpected, write.Path)
		}
		abs, err := work.PathSanitizer.ValidateWrite(write.Path)
		if err != nil {
			return updates, err
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
			return updates, err
		}
		if err := os.WriteFile(abs, []byte(write.Content), 0644); err != nil {
			return updates, err
		}
		updates = append(updates, TaskUpdate{TaskID: work.Task.ID, PhaseID: work.Task.PhaseID, Type: TaskProgress, Content: "wrote " + filepath.ToSlash(write.Path)})
	}
	if blocker, ok := w.Blockers[work.Task.ID]; ok {
		description := strings.TrimSpace(blocker.Description)
		if description == "" {
			description = strings.TrimSpace(blocker.Reason)
		}
		updates = append(updates, TaskUpdate{TaskID: work.Task.ID, PhaseID: work.Task.PhaseID, Type: TaskBlocked, Content: description})
		return updates, nil
	}
	updates = append(updates, TaskUpdate{TaskID: work.Task.ID, PhaseID: work.Task.PhaseID, Type: TaskCompleted, Content: "fake worker completed task"})
	return updates, nil
}

func expectedFileMatcher(patterns []string) func(string) bool {
	cleanPatterns := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(filepath.Clean(strings.TrimSpace(pattern)))
		if pattern != "." && pattern != "" {
			cleanPatterns = append(cleanPatterns, pattern)
		}
	}
	return func(path string) bool {
		path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
		for _, pattern := range cleanPatterns {
			if pattern == path || pattern == "**" {
				return true
			}
			if strings.HasSuffix(pattern, "/**") {
				prefix := strings.TrimSuffix(pattern, "/**")
				if path == prefix || strings.HasPrefix(path, prefix+"/") {
					return true
				}
			}
			if ok, _ := filepath.Match(pattern, path); ok {
				return true
			}
		}
		return false
	}
}

func acceptanceResults(criteria []string, satisfied bool) []AcceptanceResult {
	results := make([]AcceptanceResult, 0, len(criteria))
	for _, criterion := range criteria {
		results = append(results, AcceptanceResult{Criterion: criterion, Satisfied: satisfied, Evidence: "fake worker deterministic result"})
	}
	return results
}

func randomNexdevID(prefix string) string {
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(data[:])
}

func isZeroReport(report TaskReport) bool {
	return report.ProjectID == "" && report.RunID == "" && report.Task.ID == ""
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
