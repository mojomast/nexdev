package state

import "time"

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
