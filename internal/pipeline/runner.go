package pipeline

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/state"
)

var ErrStageSkipped = errors.New("stage skipped")

type BlockedError struct {
	Reason string
	Err    error
}

func (e *BlockedError) Error() string {
	if e == nil {
		return "stage blocked"
	}
	if e.Reason != "" {
		return "stage blocked: " + e.Reason
	}
	if e.Err != nil {
		return "stage blocked: " + e.Err.Error()
	}
	return "stage blocked"
}

func (e *BlockedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type StageOutputter interface {
	Output(ctx context.Context, env StageEnv) (map[string]any, error)
}

type PrerequisiteProvider interface {
	Snapshot(ctx context.Context, env StageEnv, target Stage) (PrerequisiteSnapshot, error)
}

type PrerequisiteProviderFunc func(ctx context.Context, env StageEnv, target Stage) (PrerequisiteSnapshot, error)

func (f PrerequisiteProviderFunc) Snapshot(ctx context.Context, env StageEnv, target Stage) (PrerequisiteSnapshot, error) {
	return f(ctx, env, target)
}

type IDGenerator func(prefix string) string

type Runner struct {
	store         *state.Store
	stages        map[Stage]PipelineStage
	prerequisites PrerequisiteProvider
	newID         IDGenerator
	now           func() time.Time
}

type RunnerOption func(*Runner)

func WithPrerequisiteProvider(provider PrerequisiteProvider) RunnerOption {
	return func(r *Runner) {
		r.prerequisites = provider
	}
}

func WithIDGenerator(generator IDGenerator) RunnerOption {
	return func(r *Runner) {
		r.newID = generator
	}
}

func WithClock(clock func() time.Time) RunnerOption {
	return func(r *Runner) {
		r.now = clock
	}
}

func NewRunner(store *state.Store, opts ...RunnerOption) (*Runner, error) {
	if store == nil {
		return nil, fmt.Errorf("state store is required")
	}
	runner := &Runner{
		store:         store,
		stages:        map[Stage]PipelineStage{},
		prerequisites: defaultPrerequisiteProvider{},
		newID:         randomID,
		now:           func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		opt(runner)
	}
	if runner.prerequisites == nil {
		runner.prerequisites = defaultPrerequisiteProvider{}
	}
	if runner.newID == nil {
		runner.newID = randomID
	}
	if runner.now == nil {
		runner.now = func() time.Time { return time.Now().UTC() }
	}
	return runner, nil
}

func (r *Runner) Register(stage PipelineStage) error {
	if stage == nil {
		return fmt.Errorf("stage is required")
	}
	name := stage.Name()
	if !IsCanonicalStage(name) {
		return fmt.Errorf("runner stage must be canonical: %s", name)
	}
	if _, exists := r.stages[name]; exists {
		return fmt.Errorf("stage already registered: %s", name)
	}
	r.stages[name] = stage
	return nil
}

type RunOptions struct {
	RunID         string
	From          Stage
	SingleStage   Stage
	Prerequisites *PrerequisiteSnapshot
}

func (r *Runner) Run(ctx context.Context, env StageEnv, opts RunOptions) error {
	if opts.RunID == "" {
		return fmt.Errorf("run id is required")
	}
	run, err := r.store.GetRun(ctx, opts.RunID)
	if err != nil {
		return err
	}
	env = r.prepareEnv(env, run)

	stages, err := r.stagesForRun(ctx, env, run, opts)
	if err != nil {
		return err
	}
	if len(stages) == 0 {
		return r.completeRun(ctx, run)
	}
	if err := r.store.UpdateRunStatus(ctx, run.ID, "running"); err != nil {
		return err
	}

	for _, stageName := range stages {
		stage := r.stages[stageName]
		if stage == nil {
			return fmt.Errorf("stage not registered: %s", stageName)
		}

		stageRun, err := r.ensureStageRun(ctx, run.ID, stageName)
		if err != nil {
			return err
		}
		status := StageStatus(stageRun.Status)
		if status == StageStatusCompleted || status == StageStatusSkipped {
			continue
		}

		snapshot, err := r.snapshot(ctx, env, stageName, opts.Prerequisites)
		if err != nil {
			return err
		}
		if err := ValidatePrerequisites(stageName, snapshot); err != nil {
			return err
		}

		if err := r.store.UpdateRunCurrentStage(ctx, run.ID, string(stageName)); err != nil {
			return err
		}

		if err := stage.Validate(ctx, env); err != nil {
			if errors.Is(err, ErrStageSkipped) {
				output, outputErr := r.stageOutput(ctx, env, stage)
				if outputErr != nil {
					return outputErr
				}
				if err := r.transition(ctx, run, stageRun, StageStatusSkipped, output, nil); err != nil {
					return err
				}
				continue
			}
			return err
		}

		if status != StageStatusRunning {
			if err := r.transition(ctx, run, stageRun, StageStatusRunning, nil, nil); err != nil {
				return err
			}
		}

		var stageErr error
		if status == StageStatusBlocked || status == StageStatusFailed || status == StageStatusRunning {
			stageErr = stage.Resume(ctx, env)
		} else {
			stageErr = stage.Run(ctx, env)
		}

		output, outputErr := r.stageOutput(ctx, env, stage)
		if outputErr != nil {
			return outputErr
		}
		if stageErr == nil {
			if err := r.transition(ctx, run, stageRun, StageStatusCompleted, output, nil); err != nil {
				return err
			}
			continue
		}

		errorData := map[string]any{"message": stageErr.Error()}
		var blocked *BlockedError
		if errors.As(stageErr, &blocked) {
			if blocked.Reason != "" {
				errorData["reason"] = blocked.Reason
			}
			if err := r.transition(ctx, run, stageRun, StageStatusBlocked, output, errorData); err != nil {
				return err
			}
			_ = r.store.UpdateRunStatus(ctx, run.ID, "blocked")
			return stageErr
		}
		if err := r.transition(ctx, run, stageRun, StageStatusFailed, output, errorData); err != nil {
			return err
		}
		_ = r.store.UpdateRunStatus(ctx, run.ID, "failed")
		return stageErr
	}

	return r.completeRun(ctx, run)
}

func (r *Runner) Resume(ctx context.Context, env StageEnv, runID string) error {
	return r.Run(ctx, env, RunOptions{RunID: runID})
}

func (r *Runner) stagesForRun(ctx context.Context, env StageEnv, run *state.Run, opts RunOptions) ([]Stage, error) {
	if opts.SingleStage != "" {
		if !IsCanonicalStage(opts.SingleStage) {
			return nil, fmt.Errorf("single stage must be canonical: %s", opts.SingleStage)
		}
		return []Stage{opts.SingleStage}, nil
	}
	start := opts.From
	if start == "" {
		var err error
		start, err = r.resumeStage(ctx, run)
		if err != nil {
			return nil, err
		}
	}
	if !IsCanonicalStage(start) {
		return nil, fmt.Errorf("start stage must be canonical: %s", start)
	}
	startIndex, _ := StageIndex(start)
	return append([]Stage(nil), CanonicalStages[startIndex:]...), nil
}

func (r *Runner) resumeStage(ctx context.Context, run *state.Run) (Stage, error) {
	stageRuns, err := r.store.ListStageRunsByRun(ctx, run.ID)
	if err != nil {
		return "", err
	}
	byStage := latestStageRuns(stageRuns)
	if run.CurrentStage != "" {
		stage := Stage(run.CurrentStage)
		if !IsCanonicalStage(stage) {
			return "", fmt.Errorf("persisted current stage is not canonical: %s", stage)
		}
		stageRun := byStage[stage]
		if stageRun == nil || !terminalStatus(StageStatus(stageRun.Status)) {
			return stage, nil
		}
		currentIndex, _ := StageIndex(stage)
		for _, next := range CanonicalStages[currentIndex+1:] {
			nextRun := byStage[next]
			if nextRun == nil || !terminalStatus(StageStatus(nextRun.Status)) {
				return next, nil
			}
		}
		return "", nil
	}
	for _, stage := range CanonicalStages {
		stageRun := byStage[stage]
		if stageRun == nil || !terminalStatus(StageStatus(stageRun.Status)) {
			return stage, nil
		}
	}
	return "", nil
}

func (r *Runner) ensureStageRun(ctx context.Context, runID string, stage Stage) (*state.StageRun, error) {
	stageRuns, err := r.store.ListStageRunsByRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	if existing := latestStageRuns(stageRuns)[stage]; existing != nil {
		return existing, nil
	}
	stageRun := &state.StageRun{
		ID:     r.newID("stage"),
		RunID:  runID,
		Stage:  string(stage),
		Status: string(StageStatusPending),
		Output: map[string]any{},
	}
	if err := r.store.CreateStageRun(ctx, stageRun); err != nil {
		return nil, err
	}
	return stageRun, nil
}

func (r *Runner) transition(ctx context.Context, run *state.Run, stageRun *state.StageRun, to StageStatus, output map[string]any, errorData map[string]any) error {
	from := StageStatus(stageRun.Status)
	if err := ValidateStatusTransition(from, to); err != nil {
		return err
	}
	if output != nil && (to == StageStatusCompleted || to == StageStatusSkipped || to == StageStatusBlocked || to == StageStatusFailed) {
		if err := r.store.UpdateStageRunOutput(ctx, stageRun.ID, output); err != nil {
			return err
		}
		stageRun.Output = output
	}
	if errorData != nil {
		if err := r.store.UpdateStageRunError(ctx, stageRun.ID, errorData); err != nil {
			return err
		}
		stageRun.Error = errorData
	}
	if to == StageStatusCompleted {
		if err := r.store.CompleteStageRun(ctx, stageRun.ID, r.now()); err != nil {
			return err
		}
	} else {
		if err := r.store.UpdateStageRunStatus(ctx, stageRun.ID, string(to)); err != nil {
			return err
		}
	}
	stageRun.Status = string(to)
	return r.emitStageStatus(ctx, run, stageRun, from, to)
}

func (r *Runner) completeRun(ctx context.Context, run *state.Run) error {
	if err := r.store.CompleteRun(ctx, run.ID, r.now()); err != nil {
		return err
	}
	event := contract.EventEnvelope{
		EventID:   r.newID("evt"),
		RunID:     run.ID,
		ProjectID: run.ProjectID,
		Type:      contract.EventTypeDone,
		Source:    contract.EventSourceCore,
		Payload:   json.RawMessage(`{"status":"completed"}`),
	}
	_, err := r.store.PersistEvent(ctx, event)
	return err
}

func (r *Runner) emitStageStatus(ctx context.Context, run *state.Run, stageRun *state.StageRun, from, to StageStatus) error {
	payload, err := json.Marshal(map[string]any{
		"stage":        stageRun.Stage,
		"stage_run_id": stageRun.ID,
		"from_status":  string(from),
		"status":       string(to),
	})
	if err != nil {
		return err
	}
	event := contract.EventEnvelope{
		EventID:   r.newID("evt"),
		RunID:     run.ID,
		ProjectID: run.ProjectID,
		Type:      contract.EventTypeStageStatus,
		Source:    contract.EventSourceCore,
		Stage:     stageRun.Stage,
		Payload:   payload,
	}
	_, err = r.store.PersistEvent(ctx, event)
	return err
}

func (r *Runner) snapshot(ctx context.Context, env StageEnv, target Stage, explicit *PrerequisiteSnapshot) (PrerequisiteSnapshot, error) {
	if explicit != nil {
		return *explicit, nil
	}
	return r.prerequisites.Snapshot(ctx, env, target)
}

func (r *Runner) stageOutput(ctx context.Context, env StageEnv, stage PipelineStage) (map[string]any, error) {
	outputter, ok := stage.(StageOutputter)
	if !ok {
		return map[string]any{}, nil
	}
	output, err := outputter.Output(ctx, env)
	if err != nil {
		return nil, err
	}
	if output == nil {
		output = map[string]any{}
	}
	return output, nil
}

func (r *Runner) prepareEnv(env StageEnv, run *state.Run) StageEnv {
	if env.Store == nil {
		env.Store = r.store
	}
	if env.Run == nil {
		env.Run = runRef{id: run.ID}
	}
	return env
}

func latestStageRuns(stageRuns []*state.StageRun) map[Stage]*state.StageRun {
	latest := map[Stage]*state.StageRun{}
	for _, stageRun := range stageRuns {
		if stageRun == nil {
			continue
		}
		latest[Stage(stageRun.Stage)] = stageRun
	}
	return latest
}

func terminalStatus(status StageStatus) bool {
	return status == StageStatusCompleted || status == StageStatusSkipped
}

type defaultPrerequisiteProvider struct{}

func (defaultPrerequisiteProvider) Snapshot(ctx context.Context, env StageEnv, target Stage) (PrerequisiteSnapshot, error) {
	snapshot := PrerequisiteSnapshot{Satisfied: map[RequirementKey]bool{}, SkippedStages: map[Stage]bool{}}
	if env.Project != nil {
		snapshot.Satisfied[RequirementProjectExists] = true
	}
	store, ok := env.Store.(*state.Store)
	if !ok || env.Run == nil {
		return snapshot, nil
	}
	stageRuns, err := store.ListStageRunsByRun(ctx, env.Run.RunID())
	if err != nil {
		return PrerequisiteSnapshot{}, err
	}
	for _, stageRun := range stageRuns {
		if StageStatus(stageRun.Status) == StageStatusSkipped {
			snapshot.SkippedStages[Stage(stageRun.Stage)] = true
		}
	}
	return snapshot, nil
}

type runRef struct{ id string }

func (r runRef) RunID() string { return r.id }

func randomID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UTC().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}
