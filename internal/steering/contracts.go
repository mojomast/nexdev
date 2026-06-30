package steering

import "time"

// Source identifies the trusted control surface that accepted a steering message.
type Source string

const (
	SourceCLI Source = "cli"
	SourceAPI Source = "api"
	SourceTUI Source = "tui"
	SourceMCP Source = "mcp"
)

var AllSources = []Source{
	SourceCLI,
	SourceAPI,
	SourceTUI,
	SourceMCP,
}

// SafetyPolicyOverrideAllowed is intentionally false: steering can add context,
// but it cannot weaken safety policy, output schemas, or task acceptance criteria.
const SafetyPolicyOverrideAllowed = false

// Message is the durable steering event shape stored for a run or task.
type Message struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"project_id"`
	RunID         string    `json:"run_id"`
	TaskID        string    `json:"task_id,omitempty"`
	Message       string    `json:"message"`
	Summary       string    `json:"summary,omitempty"`
	Source        Source    `json:"source"`
	CreatedByRole string    `json:"created_by_role,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// Context is the selected steering context for prompt construction.
type Context struct {
	TaskID       string    `json:"task_id,omitempty"`
	Summary      string    `json:"summary,omitempty"`
	Messages     []Message `json:"messages"`
	MaxMessages  int       `json:"max_messages"`
	BudgetBytes  int       `json:"budget_bytes,omitempty"`
	Truncated    bool      `json:"truncated"`
	SafetyPinned bool      `json:"safety_pinned"`
}

// Store is the durable steering boundary used by later state-backed implementations.
type Store interface {
	AppendSteeringMessage(message Message) error
	SteeringContext(projectID, runID, taskID string, maxMessages int) (Context, error)
}

func IsKnownSource(source Source) bool {
	for _, candidate := range AllSources {
		if candidate == source {
			return true
		}
	}
	return false
}
