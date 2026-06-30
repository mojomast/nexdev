package state

import (
	"encoding/json"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
)

// Stage represents the current pipeline stage
type Stage string

const (
	StageInit      Stage = "init"
	StageInterview Stage = "interview"
	StageDesign    Stage = "design"
	StagePlan      Stage = "plan"
	StageReview    Stage = "review"
	StageDevelop   Stage = "develop"
	StageComplete  Stage = "complete"
)

// PhaseStatus represents the status of a phase
type PhaseStatus string

const (
	PhaseNotStarted PhaseStatus = "not_started"
	PhaseInProgress PhaseStatus = "in_progress"
	PhaseCompleted  PhaseStatus = "completed"
	PhaseBlocked    PhaseStatus = "blocked"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskNotStarted TaskStatus = "not_started"
	TaskInProgress TaskStatus = "in_progress"
	TaskCompleted  TaskStatus = "completed"
	TaskBlocked    TaskStatus = "blocked"
	TaskSkipped    TaskStatus = "skipped"
)

// Project represents a Geoffrussy project
type Project struct {
	ID           string
	Name         string
	CreatedAt    time.Time
	CurrentStage Stage
	CurrentPhase string
}

// Run represents a Nexdev pipeline run persisted in migration version 4.
type Run struct {
	ID           string
	ProjectID    string
	Status       string
	CurrentStage string
	StartedAt    time.Time
	CompletedAt  *time.Time
	CancelledAt  *time.Time
	Metadata     map[string]any
}

// StageRun represents one attempt at a canonical or pseudo pipeline stage.
type StageRun struct {
	ID          string
	RunID       string
	Stage       string
	Status      string
	Attempt     int
	StartedAt   *time.Time
	CompletedAt *time.Time
	Error       map[string]any
	Output      map[string]any
}

// Artifact represents a SQLite-indexed pipeline artifact. Disk files are written elsewhere.
type Artifact struct {
	ID        string
	ProjectID string
	RunID     string
	Kind      string
	Path      string
	SHA256    string
	Version   int
	Metadata  map[string]any
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AuthToken stores only the server-side token hash and authorization metadata.
type AuthToken struct {
	ID         string
	TokenHash  string
	Role       string
	Name       string
	CreatedAt  time.Time
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
	LastUsedAt *time.Time
}

// SteeringEvent is durable operator context for a run or task.
type SteeringEvent struct {
	ID            string
	ProjectID     string
	RunID         string
	TaskID        string
	Message       string
	Summary       string
	Source        string
	CreatedByRole string
	CreatedAt     time.Time
}

// DetourRecord persists the result of a manual or automatic detour request.
type DetourRecord struct {
	ID            string
	ProjectID     string
	RunID         string
	TriggerTaskID string
	Reason        string
	Source        string
	Depth         int
	Result        json.RawMessage
	CreatedAt     time.Time
}

// NavigationEvent records explicit stage navigation decisions.
type NavigationEvent struct {
	ID        string
	ProjectID string
	RunID     string
	FromStage string
	ToStage   string
	Reason    string
	Actor     string
	CreatedAt time.Time
}

// PlanEditEvent records review-time plan mutations and version changes.
type PlanEditEvent struct {
	ID                string
	ProjectID         string
	RunID             string
	PlanVersionBefore int
	PlanVersionAfter  int
	EditType          string
	TargetID          string
	Patch             json.RawMessage
	Actor             string
	CreatedAt         time.Time
}

// NexdevTask persists the reviewed TaskSpec plus execution-facing ordering/status metadata.
type NexdevTask struct {
	Spec        contract.TaskSpec
	ProjectID   string
	RunID       string
	Status      string
	PlanVersion int
	PlanOrder   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NexdevBlocker records a blocker in the Nexdev event/control-plane shape.
type NexdevBlocker struct {
	ID          string
	ProjectID   string
	RunID       string
	TaskID      string
	Reason      string
	Description string
	Status      string
	Resolution  string
	Metadata    json.RawMessage
	CreatedAt   time.Time
	ResolvedAt  *time.Time
}

// AuditRecord is a durable, redacted record of security/control-plane/operator-sensitive actions.
type AuditRecord struct {
	ID           string
	ProjectID    string
	RunID        string
	RequestID    string
	Actor        string
	ActorRole    string
	Source       string
	Action       string
	ResourceType string
	ResourceID   string
	Outcome      string
	Details      json.RawMessage
	CreatedAt    time.Time
}

// CostRecord is one durable provider usage/cost ledger entry.
type CostRecord struct {
	ID               string
	ProjectID        string
	RunID            string
	Stage            string
	TaskID           string
	Provider         string
	Model            string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	EstimatedUSD     *float64
	Currency         string
	RetryCount       int
	LatencyMS        int64
	Metadata         json.RawMessage
	CreatedAt        time.Time
}

const (
	NexdevTaskStatusPending            = "pending"
	NexdevTaskStatusRunning            = "running"
	NexdevTaskStatusCompleted          = "completed"
	NexdevTaskStatusBlocked            = "blocked"
	NexdevTaskStatusSkipped            = "skipped"
	NexdevTaskStatusFailed             = "failed"
	NexdevTaskStatusPendingAfterDetour = "pending_after_detour"

	NexdevBlockerStatusOpen     = "open"
	NexdevBlockerStatusResolved = "resolved"
)

// InterviewData contains all gathered requirements
type InterviewData struct {
	ProjectID         string
	ProjectName       string
	CreatedAt         time.Time
	ProblemStatement  string
	TargetUsers       []string
	SuccessMetrics    []string
	TechnicalStack    TechStack
	Integrations      []Integration
	Scope             Scope
	Constraints       []string
	Assumptions       []string
	Unknowns          []string
	RefinementHistory []Refinement
	RawSession        string // Stores the complete session state as JSON
}

// TechStack represents the technology stack
type TechStack struct {
	Backend        TechChoice
	Frontend       TechChoice
	Database       TechChoice
	Cache          TechChoice
	Infrastructure TechChoice
}

// TechChoice represents a technology choice
type TechChoice struct {
	Language  string
	Framework string
	Version   string
	Rationale string
}

// Integration represents an external integration
type Integration struct {
	Name     string
	Type     string
	Purpose  string
	Required bool
}

// Scope represents the project scope
type Scope struct {
	MVPFeatures    []string
	Phase2Features []string
	Timeline       string
	Resources      string
}

// Refinement represents a refinement iteration
type Refinement struct {
	Iteration  int
	Timestamp  time.Time
	Changes    []string
	ApprovedBy string
}

// Architecture represents the system design
type Architecture struct {
	ProjectID string
	Content   string // Markdown content
	CreatedAt time.Time
}

// Phase represents a development phase
type Phase struct {
	ID          string
	ProjectID   string
	Number      int
	Title       string
	Content     string // Full phase content (markdown)
	Status      PhaseStatus
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
}

// Task represents a single development task
type Task struct {
	ID          string
	PhaseID     string
	Number      string
	Description string
	Status      TaskStatus
	StartedAt   *time.Time
	CompletedAt *time.Time
}

// Checkpoint represents a saved state
type Checkpoint struct {
	ID        string
	ProjectID string
	Name      string
	GitTag    string
	CreatedAt time.Time
	Metadata  map[string]string
}

// TokenUsage tracks API usage
type TokenUsage struct {
	ID           int
	ProjectID    string
	PhaseID      string
	TaskID       string
	Provider     string
	Model        string
	TokensInput  int
	TokensOutput int
	Cost         float64
	Timestamp    time.Time
}

// RateLimitInfo contains rate limit information
type RateLimitInfo struct {
	Provider          string
	RequestsRemaining *int
	RequestsLimit     *int
	ResetAt           *time.Time
	CheckedAt         time.Time
}

// QuotaInfo contains quota information
type QuotaInfo struct {
	Provider        string
	TokensRemaining *int
	TokensLimit     *int
	CostRemaining   *float64
	CostLimit       *float64
	ResetAt         time.Time
	CheckedAt       time.Time
}

// TokenStats contains token usage statistics
type TokenStats struct {
	TotalInput  int
	TotalOutput int
	ByProvider  map[string]int
	ByPhase     map[string]int
	LastUpdated time.Time
}

// CostStats contains cost statistics
type CostStats struct {
	TotalCost   float64
	ByProvider  map[string]float64
	ByPhase     map[string]float64
	LastUpdated time.Time
}

// Blocker represents an issue preventing progress
type Blocker struct {
	ID          string
	TaskID      string
	Description string
	Resolution  string
	CreatedAt   time.Time
	ResolvedAt  *time.Time
}
