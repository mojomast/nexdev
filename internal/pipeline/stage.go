package pipeline

import (
	"context"
	"log/slog"
)

// Stage is the canonical pipeline stage identifier persisted and exposed by Nexdev.
type Stage string

const (
	StageInit        Stage = "init"
	StageRepoAnalyze Stage = "repo_analyze"
	StageInterview   Stage = "interview"
	StageComplexity  Stage = "complexity"
	StageDesign      Stage = "design"
	StageHivemind    Stage = "hivemind"
	StageValidate    Stage = "validate"
	StagePlanSketch  Stage = "plan_sketch"
	StagePlanDetail  Stage = "plan_detail"
	StageReview      Stage = "review"
	StageDevelop     Stage = "develop"
	StageVerify      Stage = "verify"
	StageHandoff     Stage = "handoff"
	StageComplete    Stage = "complete"

	// StageDetour is a pseudo-stage reachable only from develop and returning to develop.
	StageDetour Stage = "detour"
)

// CanonicalStages is the SPEC 9.1 order. It intentionally excludes pseudo-stages.
var CanonicalStages = []Stage{
	StageInit,
	StageRepoAnalyze,
	StageInterview,
	StageComplexity,
	StageDesign,
	StageHivemind,
	StageValidate,
	StagePlanSketch,
	StagePlanDetail,
	StageReview,
	StageDevelop,
	StageVerify,
	StageHandoff,
	StageComplete,
}

// AllStages includes canonical stages plus pseudo-stages for validation and contracts.
var AllStages = append(append([]Stage(nil), CanonicalStages...), StageDetour)

// PipelineStage is implemented by each concrete stage once stage behavior lands.
type PipelineStage interface {
	Name() Stage
	Run(ctx context.Context, env StageEnv) error
	Validate(ctx context.Context, env StageEnv) error
	Resume(ctx context.Context, env StageEnv) error
}

// ProjectRef and RunRef are intentionally tiny to keep pipeline independent of state models.
type ProjectRef interface{ ProjectID() string }
type RunRef interface{ RunID() string }

// Store is a placeholder consumer-side boundary for future stage/run/artifact repositories.
// TODO(M3/M5): replace or extend with focused repository methods after state contracts exist.
type Store interface{}

// ProviderRouter is the stage-facing provider boundary. Concrete providers remain elsewhere.
// TODO(M4): align with internal/provider router once that contract is frozen.
type ProviderRouter interface{}

// Config is the resolved runtime configuration boundary for stages.
// TODO(M2): align with internal/config.Config once config contracts are frozen.
type Config interface{}

// EventPublisher is the minimal event boundary stages need without importing controlplane.
type EventPublisher interface {
	PublishStageStatus(ctx context.Context, stage Stage, status StageStatus) error
}

// GitManager is a local boundary for git operations used by later stages.
type GitManager interface{}

// PolicyEngine is a local boundary for path/tool policy checks used by later stages.
type PolicyEngine interface{}

// StageEnv carries dependencies into stages without importing concrete domain packages.
type StageEnv struct {
	Project   ProjectRef
	Run       RunRef
	Store     Store
	Providers ProviderRouter
	Config    Config
	Events    EventPublisher
	Git       GitManager
	Safety    PolicyEngine
	Logger    *slog.Logger
}

// StageIndex returns the canonical stage index. Pseudo-stages are not indexed.
func StageIndex(stage Stage) (int, bool) {
	for i, candidate := range CanonicalStages {
		if candidate == stage {
			return i, true
		}
	}
	return 0, false
}

func IsCanonicalStage(stage Stage) bool {
	_, ok := StageIndex(stage)
	return ok
}

func IsPseudoStage(stage Stage) bool {
	return stage == StageDetour
}

func IsKnownStage(stage Stage) bool {
	return IsCanonicalStage(stage) || IsPseudoStage(stage)
}
