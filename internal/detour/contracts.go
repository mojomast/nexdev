package detour

import (
	"context"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/pipeline"
)

type Request = contract.DetourRequest
type Result = contract.DetourResult

const DefaultMaxDepth = 3

// RequestContext captures the bounded state needed before generating detour tasks.
type RequestContext struct {
	Request       contract.DetourRequest `json:"request"`
	CurrentTask   contract.TaskSpec      `json:"current_task"`
	NeighborTasks []contract.TaskSpec    `json:"neighbor_tasks,omitempty"`
	BlockerID     string                 `json:"blocker_id,omitempty"`
	PhaseID       string                 `json:"phase_id,omitempty"`
	DesignSummary string                 `json:"design_summary,omitempty"`
	RepoContext   string                 `json:"repo_context,omitempty"`
	CurrentDepth  int                    `json:"current_depth"`
	MaxDepth      int                    `json:"max_depth"`
}

// Generator is the provider-routed detour task producer. M1 defines only the boundary.
type Generator interface {
	GenerateDetour(ctx context.Context, request RequestContext) (contract.DetourResult, error)
}

// SpliceRequest describes the pure plan mutation expected after a detour result is approved.
type SpliceRequest struct {
	ProjectID     string                `json:"project_id"`
	RunID         string                `json:"run_id"`
	PhaseID       string                `json:"phase_id"`
	TriggerTaskID string                `json:"trigger_task_id"`
	ExistingTasks []contract.TaskSpec   `json:"existing_tasks"`
	Result        contract.DetourResult `json:"result"`
}

type SpliceResult struct {
	Tasks          []contract.TaskSpec `json:"tasks"`
	InsertedTaskID []string            `json:"inserted_task_ids"`
	SplicedAfter   string              `json:"spliced_after"`
	IDConflicts    []string            `json:"id_conflicts,omitempty"`
	PlanVersion    int                 `json:"plan_version"`
	EventStage     pipeline.Stage      `json:"event_stage"`
}

type Splicer interface {
	SpliceDetour(ctx context.Context, request SpliceRequest) (SpliceResult, error)
}

type DepthDecision string

const (
	DepthDecisionAllow DepthDecision = "allow"
	DepthDecisionBlock DepthDecision = "block"
)

type DepthCheck struct {
	CurrentDepth  int           `json:"current_depth"`
	MaxDepth      int           `json:"max_depth"`
	Decision      DepthDecision `json:"decision"`
	BlockerReason string        `json:"blocker_reason,omitempty"`
}

type DepthPolicy interface {
	CheckDepth(currentDepth, maxDepth int) DepthCheck
}
