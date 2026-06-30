package contract

import (
	"encoding/json"
	"time"
)

const EventContractVersion = "nexdev-event-v1"

const (
	EventTypeHeartbeat             = "heartbeat"
	EventTypeRunStarted            = "run_started"
	EventTypeRunStatus             = "run_status"
	EventTypeStageTransition       = "stage_transition"
	EventTypeStageStatus           = "stage_status"
	EventTypeContentDelta          = "content_delta"
	EventTypeProviderCallStarted   = "provider_call_started"
	EventTypeProviderCallCompleted = "provider_call_completed"
	EventTypeProviderCallFailed    = "provider_call_failed"
	EventTypeArtifactUpdated       = "artifact_updated"
	EventTypePlanUpdated           = "plan_updated"
	EventTypeReviewRequired        = "review_required"
	EventTypeReviewCompleted       = "review_completed"
	EventTypeTaskStarted           = "task_started"
	EventTypeTaskProgress          = "task_progress"
	EventTypeTaskCompleted         = "task_completed"
	EventTypeTaskError             = "task_error"
	EventTypeTaskBlocked           = "task_blocked"
	EventTypeTaskPaused            = "task_paused"
	EventTypeTaskResumed           = "task_resumed"
	EventTypeTaskSkipped           = "task_skipped"
	EventTypeSteeringAdded         = "steering_added"
	EventTypeDetourRequested       = "detour_requested"
	EventTypeDetourCreated         = "detour_created"
	EventTypeDetourFailed          = "detour_failed"
	EventTypeBlockerCreated        = "blocker_created"
	EventTypeBlockerResolved       = "blocker_resolved"
	EventTypeVerifyStarted         = "verify_started"
	EventTypeVerifyCommandOutput   = "verify_command_output"
	EventTypeVerifyCompleted       = "verify_completed"
	EventTypeGitEvent              = "git_event"
	EventTypeCostUpdate            = "cost_update"
	EventTypeSecurityWarning       = "security_warning"
	EventTypePipelineError         = "pipeline_error"
	EventTypeDone                  = "done"
)

var RequiredEventTypes = []string{
	EventTypeHeartbeat,
	EventTypeRunStarted,
	EventTypeRunStatus,
	EventTypeStageTransition,
	EventTypeStageStatus,
	EventTypeContentDelta,
	EventTypeProviderCallStarted,
	EventTypeProviderCallCompleted,
	EventTypeProviderCallFailed,
	EventTypeArtifactUpdated,
	EventTypePlanUpdated,
	EventTypeReviewRequired,
	EventTypeReviewCompleted,
	EventTypeTaskStarted,
	EventTypeTaskProgress,
	EventTypeTaskCompleted,
	EventTypeTaskError,
	EventTypeTaskBlocked,
	EventTypeTaskPaused,
	EventTypeTaskResumed,
	EventTypeTaskSkipped,
	EventTypeSteeringAdded,
	EventTypeDetourRequested,
	EventTypeDetourCreated,
	EventTypeDetourFailed,
	EventTypeBlockerCreated,
	EventTypeBlockerResolved,
	EventTypeVerifyStarted,
	EventTypeVerifyCommandOutput,
	EventTypeVerifyCompleted,
	EventTypeGitEvent,
	EventTypeCostUpdate,
	EventTypeSecurityWarning,
	EventTypePipelineError,
	EventTypeDone,
}

const (
	EventSourceCore     = "core"
	EventSourceExecutor = "executor"
	EventSourceWorker   = "worker"
	EventSourceTUI      = "tui"
	EventSourceAPI      = "api"
	EventSourceMCP      = "mcp"
)

var RequiredEventSources = []string{
	EventSourceCore,
	EventSourceExecutor,
	EventSourceWorker,
	EventSourceTUI,
	EventSourceAPI,
	EventSourceMCP,
}

type EventEnvelope struct {
	EventID         string          `json:"event_id"`
	Sequence        int64           `json:"sequence"`
	ContractVersion string          `json:"contract_version"`
	Type            string          `json:"type"`
	ProjectID       string          `json:"project_id"`
	RunID           string          `json:"run_id"`
	Stage           string          `json:"stage,omitempty"`
	TaskID          string          `json:"task_id,omitempty"`
	Timestamp       time.Time       `json:"ts"`
	Source          string          `json:"source"`
	Payload         json.RawMessage `json:"payload"`
}
