package executor

import (
	"context"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/steering"
)

// TaskStatus is the executor-facing status for a planned task.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusSkipped   TaskStatus = "skipped"
	TaskStatusBlocked   TaskStatus = "blocked"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// CurrentTaskSnapshot is a safe query result for live control surfaces.
type CurrentTaskSnapshot struct {
	ProjectID string            `json:"project_id"`
	RunID     string            `json:"run_id"`
	Stage     string            `json:"stage"`
	Task      contract.TaskSpec `json:"task"`
	Status    TaskStatus        `json:"status"`
	StartedAt time.Time         `json:"started_at,omitempty"`
}

// Control defines context-aware executor controls. Implementations land in M8.
type Control interface {
	CurrentTask(ctx context.Context) (*CurrentTaskSnapshot, error)
	Pause(ctx context.Context, reason string) error
	Resume(ctx context.Context) error
	Cancel(ctx context.Context, reason string) error
	SkipTask(ctx context.Context, taskID string, reason string) error
	SetSteeringContext(ctx context.Context, taskID string, msg steering.Message) error
}

// TaskUpdateEvent describes how an executor TaskUpdate becomes a durable event.
type TaskUpdateEvent struct {
	EventType string `json:"event_type"`
	Source    string `json:"source"`
	Stage     string `json:"stage"`
}

var TaskUpdateEventMapping = map[UpdateType]TaskUpdateEvent{
	TaskStarted:   {EventType: contract.EventTypeTaskStarted, Source: contract.EventSourceExecutor, Stage: "develop"},
	TaskProgress:  {EventType: contract.EventTypeTaskProgress, Source: contract.EventSourceExecutor, Stage: "develop"},
	TaskCompleted: {EventType: contract.EventTypeTaskCompleted, Source: contract.EventSourceExecutor, Stage: "develop"},
	TaskError:     {EventType: contract.EventTypeTaskError, Source: contract.EventSourceExecutor, Stage: "develop"},
	TaskBlocked:   {EventType: contract.EventTypeTaskBlocked, Source: contract.EventSourceExecutor, Stage: "develop"},
	TaskPaused:    {EventType: contract.EventTypeTaskPaused, Source: contract.EventSourceExecutor, Stage: "develop"},
	TaskResumed:   {EventType: contract.EventTypeTaskResumed, Source: contract.EventSourceExecutor, Stage: "develop"},
	TaskSkipped:   {EventType: contract.EventTypeTaskSkipped, Source: contract.EventSourceExecutor, Stage: "develop"},
}

func EventMappingForTaskUpdate(updateType UpdateType) (TaskUpdateEvent, bool) {
	mapping, ok := TaskUpdateEventMapping[updateType]
	return mapping, ok
}

// TaskReport is the executor completion report shape for later state/artifact use.
type TaskReport struct {
	ProjectID          string                   `json:"project_id"`
	RunID              string                   `json:"run_id"`
	Task               contract.TaskSpec        `json:"task"`
	Status             TaskStatus               `json:"status"`
	Summary            string                   `json:"summary"`
	AcceptanceResults  []AcceptanceResult       `json:"acceptance_results"`
	ChangedFiles       []contract.ChangedFile   `json:"changed_files,omitempty"`
	TestCommands       []contract.CommandResult `json:"test_commands,omitempty"`
	BlockerID          string                   `json:"blocker_id,omitempty"`
	SteeringMessageIDs []string                 `json:"steering_message_ids,omitempty"`
	StartedAt          time.Time                `json:"started_at,omitempty"`
	CompletedAt        time.Time                `json:"completed_at,omitempty"`
}

type AcceptanceResult struct {
	Criterion string `json:"criterion"`
	Satisfied bool   `json:"satisfied"`
	Evidence  string `json:"evidence,omitempty"`
}
