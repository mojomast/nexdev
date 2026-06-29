# Design Document: Geoffrussy AI Coding Agent

## Overview

Geoffrussy is a sophisticated AI-powered development orchestration platform built in Go that transforms project ideas into executable code through a structured four-stage pipeline. The system emphasizes deep understanding through interactive interviews, comprehensive architecture design, detailed phase planning, and automated validation before any code is written. Each stage includes review checkpoints where users can approve, iterate, or refine outputs, ensuring alignment between human intent and AI execution.

The core innovation is the separation of planning from execution, with each stage producing Git-tracked artifacts that serve as living documentation. This approach provides transparency, enables rollback, and creates an audit trail of all decisions.

## Architecture

### System Context

```
┌─────────────────────────────────────────────────────────────────┐
│                         Developer                               │
│                    (Terminal Interface)                         │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Geoffrussy CLI                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │  Interview   │→ │   Design     │→ │   DevPlan    │→ Review  │
│  │   Engine     │  │  Generator   │  │  Generator   │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │    Task      │  │     Git      │  │    State     │          │
│  │  Executor    │  │   Manager    │  │    Store     │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                    External Services                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │   OpenAI     │  │  Anthropic   │  │    Ollama    │          │
│  │     API      │  │     API      │  │   (Local)    │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │  OpenCode    │  │ Firmware.ai  │  │ Requesty.ai  │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
│  ┌──────────────┐  ┌──────────────┐                            │
│  │    Z.ai      │  │    Kimi      │                            │
│  └──────────────┘  └──────────────┘                            │
└─────────────────────────────────────────────────────────────────┘
```

### High-Level Architecture

Geoffrussy follows a layered architecture with clear separation of concerns:

1. **Presentation Layer**: CLI and Terminal UI (Cobra + Bubbletea)
2. **Pipeline Layer**: Interview, Design, DevPlan, Review stages
3. **Execution Layer**: Task execution and live monitoring
4. **Integration Layer**: API Bridge for multi-model support
5. **Persistence Layer**: State Store (SQLite) and Git Manager
6. **Infrastructure Layer**: Configuration, logging, error handling


## Components and Interfaces

### 1. CLI Component (Cobra)

**Responsibility**: Provide command-line interface for all Geoffrussy operations

**Commands**:
- `geoffrussy init` - Initialize project configuration
- `geoffrussy interview` - Start or resume interview phase
- `geoffrussy design` - Generate or review architecture
- `geoffrussy plan` - Generate or review DevPlan
- `geoffrussy review` - Run phase review and validation
- `geoffrussy develop` - Execute development phases
- `geoffrussy status` - Show current progress
- `geoffrussy stats` - Show token usage and cost statistics
- `geoffrussy quota` - Check rate limits and quotas for all providers
- `geoffrussy checkpoint` - Create or list checkpoints
- `geoffrussy rollback` - Rollback to a checkpoint

**Interface**:
```go
type CLI interface {
    Execute() error
    RegisterCommand(cmd *cobra.Command)
    GetConfig() *Config
}
```

### 2. Terminal UI Component (Bubbletea)

**Responsibility**: Provide interactive terminal interface for complex workflows

**Models**:
- InterviewModel: Multi-step interview with progress tracking
- ExecutionModel: Real-time streaming output display
- ReviewModel: Display review results with selection
- StatusModel: Show project status dashboard

**Interface**:
```go
type TerminalUI interface {
    RunInterview(questions []Question) (*InterviewResult, error)
    StreamExecution(taskChan <-chan TaskUpdate) error
    ShowReview(report ReviewReport) (*ReviewDecision, error)
    ShowStatus(status ProjectStatus) error
}
```

### 3. Interview Engine

**Responsibility**: Conduct five-phase interview to gather project requirements

**Phases**:
1. Project Essence (5-10 min)
2. Technical Constraints (5-10 min)
3. Integration Points (5-10 min)
4. Scope Definition (5-10 min)
5. Refinement & Validation (10-15 min)

**Interface**:
```go
type InterviewEngine interface {
    StartInterview() error
    ResumeInterview() error
    AskQuestion(phase Phase, question Question) (*Answer, error)
    GenerateFollowUp(answer Answer) ([]Question, error)
    ProposeDefaults(context InterviewContext) ([]Default, error)
    Summarize() (*InterviewSummary, error)
    SaveProgress() error
    ExportJSON() (*InterviewData, error)
}

type InterviewData struct {
    ProjectID       string
    ProjectName     string
    CreatedAt       time.Time
    ProblemStatement string
    TargetUsers     []string
    SuccessMetrics  []string
    TechnicalStack  TechStack
    Integrations    []Integration
    Scope           Scope
    Constraints     []string
    Assumptions     []string
    Unknowns        []string
    RefinementHistory []Refinement
}
```


### 4. Design Generator

**Responsibility**: Generate comprehensive system architecture from interview data

**Sections Generated**:
- System Overview (diagrams)
- Component Breakdown
- Data Flow Diagrams
- Technology Rationale
- Scaling Strategy
- API Contract
- Database Schema
- Security Approach
- Observability Strategy
- Deployment Architecture
- Risk Assessment
- Assumptions & Unknowns

**Interface**:
```go
type DesignGenerator interface {
    GenerateArchitecture(interview InterviewData) (*Architecture, error)
    GenerateSystemDiagram(components []Component) (string, error)
    GenerateDataFlow(journey UserJourney) (string, error)
    ExplainTechnologyChoice(tech Technology) (string, error)
    AssessRisks(architecture Architecture) ([]Risk, error)
    ExportMarkdown() (string, error)
}

type Architecture struct {
    SystemOverview    string
    Components        []Component
    DataFlows         []DataFlow
    TechRationale     map[string]string
    ScalingStrategy   ScalingPlan
    APIContract       APISpec
    DatabaseSchema    Schema
    SecurityApproach  SecurityPlan
    Observability     ObservabilityPlan
    Deployment        DeploymentPlan
    Risks             []Risk
    Assumptions       []string
    Unknowns          []string
}
```

### 5. DevPlan Generator

**Responsibility**: Convert architecture into 7-10 executable phases with tasks

**Phase Structure**:
- Phase 000: Setup & Infrastructure
- Phase 001: Database & Models
- Phase 002: Core API
- Phase 003: Authentication & Authorization
- Phase 004: Frontend Foundation
- Phase 005: Real-time Sync
- Phase 006: Integrations
- Phase 007: Testing & Validation
- Phase 008: Performance & Observability
- Phase 009: Deployment & Hardening

**Interface**:
```go
type DevPlanGenerator interface {
    GeneratePhases(architecture Architecture, interview InterviewData) ([]Phase, error)
    CreatePhaseFile(phase Phase) error
    EstimateTokens(phase Phase) (int, error)
    EstimateCost(tokens int, model Model) (float64, error)
    MergePhases(phase1, phase2 Phase) (*Phase, error)
    SplitPhase(phase Phase) ([]Phase, error)
    ReorderPhases(phases []Phase, newOrder []int) ([]Phase, error)
    ExportMasterPlan() (string, error)
}

type Phase struct {
    ID              string
    Number          int
    Title           string
    Objective       string
    SuccessCriteria []string
    Dependencies    []string
    Tasks           []Task
    EstimatedTokens int
    EstimatedCost   float64
    Status          PhaseStatus
}

type Task struct {
    ID                  string
    Number              string
    Description         string
    AcceptanceCriteria  []string
    ImplementationNotes []string
    BlockersEncountered []Blocker
    Status              TaskStatus
}
```


### 6. Phase Reviewer

**Responsibility**: Validate and improve generated DevPlan phases

**Review Criteria**:
- Clarity: Are tasks clearly actionable?
- Completeness: Are acceptance criteria unambiguous?
- Dependencies: Are phase dependencies correct?
- Scope: Is the phase appropriately sized?
- Risks: What could go wrong?
- Feasibility: Is it realistic to complete?
- Testing: Are there gaps in test coverage?
- Integration: Does it integrate cleanly?

**Interface**:
```go
type PhaseReviewer interface {
    ReviewPhase(phase Phase) (*PhaseReview, error)
    ReviewAllPhases(phases []Phase) (*ReviewReport, error)
    CheckCrossPhaseIssues(phases []Phase) ([]Issue, error)
    GenerateImprovements(issues []Issue) ([]Improvement, error)
    ApplyImprovements(phase Phase, improvements []Improvement) (*Phase, error)
}

type ReviewReport struct {
    Timestamp        time.Time
    TotalPhases      int
    IssuesFound      int
    SeverityBreakdown map[Severity]int
    PhaseReviews     []PhaseReview
    CrossPhaseIssues []Issue
    Summary          string
}

type PhaseReview struct {
    PhaseID string
    Status  ReviewStatus
    Issues  []Issue
}

type Issue struct {
    Type        IssueType
    Severity    Severity
    Description string
    Suggestion  string
}

type Severity string
const (
    Critical Severity = "critical"
    Warning  Severity = "warning"
    Info     Severity = "info"
)
```

### 7. API Bridge

**Responsibility**: Manage multi-model orchestration and API calls

**Supported Providers**:
- OpenAI (GPT-4, GPT-3.5)
- Anthropic (Claude 3.5, Claude 3)
- Ollama (Local models)
- OpenCode (Dynamic model discovery)
- Firmware.ai
- Requesty.ai
- Z.ai (coding plan support)
- Kimi (coding plan support)
- Custom endpoints

**Interface**:
```go
type APIBridge interface {
    CallModel(provider Provider, model string, prompt string) (*Response, error)
    ValidateModel(provider Provider, model string) error
    NormalizeResponse(rawResponse interface{}) (*Response, error)
    RetryWithBackoff(call func() error, maxRetries int) error
    StreamResponse(provider Provider, model string, prompt string) (<-chan string, error)
    CheckRateLimit(provider Provider) (*RateLimitInfo, error)
    CheckQuota(provider Provider) (*QuotaInfo, error)
}

type Provider interface {
    Name() string
    Authenticate(apiKey string) error
    ListModels() ([]Model, error)
    DiscoverModels() ([]Model, error) // For dynamic discovery (OpenCode)
    Call(model string, prompt string) (*Response, error)
    Stream(model string, prompt string) (<-chan string, error)
    GetRateLimitInfo() (*RateLimitInfo, error)
    GetQuotaInfo() (*QuotaInfo, error)
    SupportsCodingPlan() bool // For Z.ai and Kimi
}

type Response struct {
    Content      string
    TokensInput  int
    TokensOutput int
    Model        string
    Provider     string
    Timestamp    time.Time
    RateLimitRemaining int
    QuotaRemaining     int
}

type RateLimitInfo struct {
    RequestsRemaining int
    RequestsLimit     int
    ResetAt           time.Time
    RetryAfter        time.Duration
}

type QuotaInfo struct {
    TokensRemaining int
    TokensLimit     int
    CostRemaining   float64
    CostLimit       float64
    ResetAt         time.Time
}
```


### 8. Task Executor

**Responsibility**: Execute development phases and tasks with live monitoring

**Interface**:
```go
type TaskExecutor interface {
    ExecutePhase(phase Phase, model Model) error
    ExecuteTask(task Task, model Model) error
    StreamOutput(taskID string) (<-chan TaskUpdate, error)
    PauseExecution() error
    ResumeExecution() error
    SkipTask(taskID string) error
    MarkBlocked(taskID string, reason string) error
    ResolveBlocker(taskID string, resolution string) error
}

type TaskUpdate struct {
    TaskID    string
    Type      UpdateType
    Content   string
    Timestamp time.Time
}

type UpdateType string
const (
    TaskStarted   UpdateType = "started"
    TaskProgress  UpdateType = "progress"
    TaskCompleted UpdateType = "completed"
    TaskError     UpdateType = "error"
    TaskBlocked   UpdateType = "blocked"
)
```

### 9. Git Manager

**Responsibility**: Handle all Git operations and version control

**Interface**:
```go
type GitManager interface {
    InitRepository() error
    IsRepository() bool
    CommitFile(path string, message string, metadata map[string]string) error
    CommitFiles(paths []string, message string, metadata map[string]string) error
    StageChanges(paths []string) error
    CreateTag(name string, message string) error
    ResetToTag(tag string) error
    GetStatus() (*GitStatus, error)
    DetectConflicts() ([]Conflict, error)
}

type GitStatus struct {
    Branch       string
    Staged       []string
    Unstaged     []string
    Untracked    []string
    HasConflicts bool
}
```

### 10. State Store

**Responsibility**: Persist all project state using SQLite

**Schema**:
```sql
-- Projects
CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMP,
    current_stage TEXT,
    current_phase_id TEXT
);

-- Interview data
CREATE TABLE interview_data (
    project_id TEXT PRIMARY KEY,
    data JSON NOT NULL,
    completed_at TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id)
);

-- Architecture
CREATE TABLE architectures (
    project_id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    created_at TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id)
);

-- Phases
CREATE TABLE phases (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    number INTEGER NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id)
);

-- Tasks
CREATE TABLE tasks (
    id TEXT PRIMARY KEY,
    phase_id TEXT NOT NULL,
    number TEXT NOT NULL,
    description TEXT NOT NULL,
    status TEXT NOT NULL,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    FOREIGN KEY (phase_id) REFERENCES phases(id)
);

-- Checkpoints
CREATE TABLE checkpoints (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    name TEXT NOT NULL,
    git_tag TEXT NOT NULL,
    created_at TIMESTAMP,
    metadata JSON,
    FOREIGN KEY (project_id) REFERENCES projects(id)
);

-- Token usage
CREATE TABLE token_usage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL,
    phase_id TEXT,
    task_id TEXT,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    tokens_input INTEGER NOT NULL,
    tokens_output INTEGER NOT NULL,
    cost REAL NOT NULL,
    timestamp TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id)
);

-- Rate limit tracking
CREATE TABLE rate_limits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider TEXT NOT NULL,
    requests_remaining INTEGER NOT NULL,
    requests_limit INTEGER NOT NULL,
    reset_at TIMESTAMP NOT NULL,
    checked_at TIMESTAMP NOT NULL
);

-- Quota tracking
CREATE TABLE quotas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider TEXT NOT NULL,
    tokens_remaining INTEGER,
    tokens_limit INTEGER,
    cost_remaining REAL,
    cost_limit REAL,
    reset_at TIMESTAMP NOT NULL,
    checked_at TIMESTAMP NOT NULL
);

-- Token statistics cache
CREATE TABLE token_stats_cache (
    project_id TEXT PRIMARY KEY,
    total_input INTEGER NOT NULL,
    total_output INTEGER NOT NULL,
    by_provider JSON NOT NULL,
    by_phase JSON NOT NULL,
    last_updated TIMESTAMP NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id)
);

-- Blockers
CREATE TABLE blockers (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL,
    description TEXT NOT NULL,
    resolution TEXT,
    created_at TIMESTAMP,
    resolved_at TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES tasks(id)
);

-- Configuration
CREATE TABLE config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP
);
```

**Interface**:
```go
type StateStore interface {
    // Project operations
    CreateProject(project Project) error
    GetProject(id string) (*Project, error)
    UpdateProjectStage(id string, stage Stage) error
    
    // Interview operations
    SaveInterviewData(projectID string, data InterviewData) error
    GetInterviewData(projectID string) (*InterviewData, error)
    
    // Architecture operations
    SaveArchitecture(projectID string, arch Architecture) error
    GetArchitecture(projectID string) (*Architecture, error)
    
    // Phase operations
    SavePhase(phase Phase) error
    GetPhase(id string) (*Phase, error)
    ListPhases(projectID string) ([]Phase, error)
    UpdatePhaseStatus(id string, status PhaseStatus) error
    
    // Task operations
    SaveTask(task Task) error
    GetTask(id string) (*Task, error)
    UpdateTaskStatus(id string, status TaskStatus) error
    
    // Checkpoint operations
    CreateCheckpoint(checkpoint Checkpoint) error
    ListCheckpoints(projectID string) ([]Checkpoint, error)
    GetCheckpoint(id string) (*Checkpoint, error)
    
    // Token tracking
    RecordTokenUsage(usage TokenUsage) error
    GetTotalCost(projectID string) (float64, error)
    GetTokenStats(projectID string) (*TokenStats, error)
    GetCostStats(projectID string) (*CostStats, error)
    
    // Rate limit tracking
    SaveRateLimit(provider string, info RateLimitInfo) error
    GetRateLimit(provider string) (*RateLimitInfo, error)
    
    // Quota tracking
    SaveQuota(provider string, info QuotaInfo) error
    GetQuota(provider string) (*QuotaInfo, error)
    
    // Blocker operations
    SaveBlocker(blocker Blocker) error
    ResolveBlocker(id string, resolution string) error
    ListActiveBlockers(projectID string) ([]Blocker, error)
    
    // Configuration
    SetConfig(key string, value string) error
    GetConfig(key string) (string, error)
}
```


### 11. Configuration Manager

**Responsibility**: Manage all configuration including API keys and preferences

**Configuration Sources** (in precedence order):
1. Command-line flags
2. Environment variables
3. Config file (~/.geoffrussy/config.yaml)

**Interface**:
```go
type ConfigurationManager interface {
    Load() error
    Save() error
    GetAPIKey(provider string) (string, error)
    SetAPIKey(provider string, key string) error
    ValidateAPIKey(provider string, key string) error
    GetDefaultModel(stage Stage) (string, error)
    SetDefaultModel(stage Stage, model string) error
    GetConfigPath() string
}

type Config struct {
    APIKeys map[string]string
    DefaultModels map[Stage]string
    BudgetLimit float64
    VerboseLogging bool
    ConfigPath string
}
```

### 12. Token Counter & Cost Estimator

**Responsibility**: Track token usage, calculate costs, and provide statistics

**Pricing** (as of 2024, subject to change):
- GPT-4: $0.03/1K input, $0.06/1K output
- GPT-3.5: $0.0015/1K input, $0.002/1K output
- Claude 3.5 Sonnet: $0.003/1K input, $0.015/1K output
- Claude 3 Opus: $0.015/1K input, $0.075/1K output
- Ollama: Free (local)
- Firmware.ai: Variable (check API)
- Requesty.ai: Variable (check API)
- Z.ai: Variable (check API)
- Kimi: Variable (check API)
- OpenCode: Variable (depends on underlying model)

**Interface**:
```go
type TokenCounter interface {
    CountTokens(text string, model string) (int, error)
    EstimateTokens(text string) (int, error)
    GetTotalTokens(projectID string) (*TokenStats, error)
    GetTokensByProvider(projectID string, provider string) (*TokenStats, error)
    GetTokensByPhase(projectID string, phaseID string) (*TokenStats, error)
}

type CostEstimator interface {
    CalculateCost(tokensInput int, tokensOutput int, provider string, model string) (float64, error)
    GetPhaseCost(phaseID string) (float64, error)
    GetTotalCost(projectID string) (float64, error)
    GetCostByProvider(projectID string, provider string) (float64, error)
    CheckBudgetLimit(projectID string) (bool, float64, error)
    SetBudgetLimit(limit float64) error
    GetCostStats(projectID string) (*CostStats, error)
}

type TokenStats struct {
    TotalInput      int
    TotalOutput     int
    TotalCombined   int
    ByModel         map[string]int
    ByPhase         map[string]int
    AveragePerCall  float64
    PeakUsage       int
    PeakUsageTime   time.Time
}

type CostStats struct {
    TotalCost       float64
    ByProvider      map[string]float64
    ByPhase         map[string]float64
    AverageCostPerCall float64
    MostExpensiveCall  *TokenUsage
    CostTrend       []CostDataPoint
}

type CostDataPoint struct {
    Timestamp time.Time
    Cost      float64
    Cumulative float64
}
```

### 13. Live Monitor

**Responsibility**: Display real-time execution progress

**Interface**:
```go
type LiveMonitor interface {
    Start(taskID string) error
    Update(update TaskUpdate) error
    Complete(taskID string, duration time.Duration) error
    Error(taskID string, err error) error
    Pause() error
    Resume() error
}
```


## Data Models

### Core Domain Models

```go
// Project represents a Geoffrussy project
type Project struct {
    ID           string
    Name         string
    CreatedAt    time.Time
    CurrentStage Stage
    CurrentPhase string
}

type Stage string
const (
    StageInit     Stage = "init"
    StageInterview Stage = "interview"
    StageDesign   Stage = "design"
    StagePlan     Stage = "plan"
    StageReview   Stage = "review"
    StageDevelop  Stage = "develop"
    StageComplete Stage = "complete"
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
}

type TechStack struct {
    Backend       TechChoice
    Frontend      TechChoice
    Database      TechChoice
    Cache         TechChoice
    Infrastructure TechChoice
}

type TechChoice struct {
    Language   string
    Framework  string
    Version    string
    Rationale  string
}

type Integration struct {
    Name        string
    Type        string
    Purpose     string
    Required    bool
}

type Scope struct {
    MVPFeatures    []string
    Phase2Features []string
    Timeline       string
    Resources      string
}

type Refinement struct {
    Iteration  int
    Timestamp  time.Time
    Changes    []string
    ApprovedBy string
}

// Architecture represents the system design
type Architecture struct {
    ProjectID         string
    SystemOverview    string
    Components        []Component
    DataFlows         []DataFlow
    TechRationale     map[string]string
    ScalingStrategy   ScalingPlan
    APIContract       APISpec
    DatabaseSchema    Schema
    SecurityApproach  SecurityPlan
    Observability     ObservabilityPlan
    Deployment        DeploymentPlan
    Risks             []Risk
    Assumptions       []string
    Unknowns          []string
    CreatedAt         time.Time
}

type Component struct {
    Name         string
    Type         ComponentType
    Purpose      string
    Technologies []string
    Dependencies []string
}

type ComponentType string
const (
    ComponentFrontend    ComponentType = "frontend"
    ComponentBackend     ComponentType = "backend"
    ComponentDatabase    ComponentType = "database"
    ComponentCache       ComponentType = "cache"
    ComponentQueue       ComponentType = "queue"
    ComponentMonitoring  ComponentType = "monitoring"
)

type DataFlow struct {
    Name        string
    Description string
    Steps       []FlowStep
    Diagram     string
}

type FlowStep struct {
    Order       int
    Component   string
    Action      string
    Description string
}

type Risk struct {
    Name        string
    Probability RiskLevel
    Impact      RiskLevel
    Mitigation  string
}

type RiskLevel string
const (
    RiskLow      RiskLevel = "low"
    RiskMedium   RiskLevel = "medium"
    RiskHigh     RiskLevel = "high"
    RiskCritical RiskLevel = "critical"
)

// Phase represents a development phase
type Phase struct {
    ID              string
    ProjectID       string
    Number          int
    Title           string
    Objective       string
    SuccessCriteria []string
    Dependencies    []string
    Tasks           []Task
    EstimatedTokens int
    EstimatedCost   float64
    Status          PhaseStatus
    CreatedAt       time.Time
    StartedAt       *time.Time
    CompletedAt     *time.Time
}

type PhaseStatus string
const (
    PhaseNotStarted PhaseStatus = "not_started"
    PhaseInProgress PhaseStatus = "in_progress"
    PhaseCompleted  PhaseStatus = "completed"
    PhaseBlocked    PhaseStatus = "blocked"
)

// Task represents a single development task
type Task struct {
    ID                  string
    PhaseID             string
    Number              string
    Description         string
    AcceptanceCriteria  []string
    ImplementationNotes []string
    BlockersEncountered []Blocker
    Status              TaskStatus
    StartedAt           *time.Time
    CompletedAt         *time.Time
}

type TaskStatus string
const (
    TaskNotStarted TaskStatus = "not_started"
    TaskInProgress TaskStatus = "in_progress"
    TaskCompleted  TaskStatus = "completed"
    TaskBlocked    TaskStatus = "blocked"
    TaskSkipped    TaskStatus = "skipped"
)

// Blocker represents an issue preventing progress
type Blocker struct {
    ID          string
    TaskID      string
    Description string
    Resolution  string
    CreatedAt   time.Time
    ResolvedAt  *time.Time
}

// Checkpoint represents a saved state
type Checkpoint struct {
    ID        string
    ProjectID string
    Name      string
    GitTag    string
    Stage     Stage
    PhaseID   string
    Metadata  map[string]string
    CreatedAt time.Time
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

// Model represents an AI model
type Model struct {
    Provider     string
    Name         string
    DisplayName  string
    Capabilities []string
    PriceInput   float64  // per 1K tokens
    PriceOutput  float64  // per 1K tokens
}
```


## Correctness Properties

A property is a characteristic or behavior that should hold true across all valid executions of a system—essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.

### Property 1: Configuration Initialization Idempotence

*For any* project directory, running the init command multiple times should result in the same configuration state without duplication or corruption.

**Validates: Requirements 1.5**

### Property 2: API Provider Integration

*For any* supported Model_Provider (OpenAI, Anthropic, Ollama, custom), when provided with valid credentials, the API_Bridge should successfully authenticate, list available models, and make API calls that return normalized responses.

**Validates: Requirements 1.3, 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7, 6.8**

### Property 3: Interview Data Completeness

*For any* completed interview, the generated JSON file should contain all required fields (problem statement, target users, success metrics, technical stack, integrations, scope, constraints, assumptions, unknowns) with non-empty values for mandatory fields.

**Validates: Requirements 2.9**

### Property 4: State Preservation Round-Trip

*For any* project state (interview answers, architecture, DevPlan, task progress), saving the state then loading it should produce an equivalent state with no data loss.

**Validates: Requirements 2.10, 2.11, 14.1, 14.2, 14.3, 14.4, 14.5, 14.6, 14.7, 14.8, 22.1, 22.2, 22.3, 22.4, 22.5, 22.6, 22.7, 22.8**

### Property 5: Interview Reiteration Preservation

*For any* interview answer that is updated during reiteration, the new answer should replace the old answer in the interview data, and the refinement history should record the change.

**Validates: Requirements 2.12**

### Property 6: Architecture Document Completeness

*For any* generated architecture document, it should contain all required sections (system overview, component breakdown, data flow diagrams, technology rationale, scaling strategy, API contract, database schema, security approach, observability strategy, deployment architecture, risk assessment, assumptions, unknowns).

**Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7, 3.8, 3.9, 3.10, 3.11, 3.12, 3.13**

### Property 7: DevPlan Phase Structure

*For any* generated DevPlan, it should contain 7-10 phases, each phase should have 3-5 tasks, and each phase should include objective, success criteria, dependencies, and estimated token usage.

**Validates: Requirements 4.1, 4.2, 4.3, 4.4, 4.5, 4.7, 4.8, 4.10, 4.11**

### Property 8: Phase Dependency Ordering

*For any* DevPlan, if phase A depends on phase B, then phase B's number should be less than phase A's number, ensuring phases are ordered correctly.

**Validates: Requirements 4.2, 4.6**

### Property 9: Phase Merge Preservation

*For any* two phases that are merged, the resulting phase should contain all tasks from both phases, preserve all dependencies, and maintain task ordering.

**Validates: Requirements 4.14**

### Property 10: Phase Review Categorization

*For any* DevPlan that is reviewed, the review report should categorize all identified issues as critical, warning, or info, and provide specific suggestions for each issue.

**Validates: Requirements 5.1, 5.2, 5.3, 5.4, 5.5, 5.9**

### Property 11: Token Cost Calculation

*For any* API call with known token counts (input and output), the calculated cost should match the expected cost based on the model's pricing (within floating-point precision).

**Validates: Requirements 8.1, 8.2, 8.3, 8.4, 8.5, 8.6**

### Property 12: Git Commit Integrity

*For any* file that is committed to Git, the commit should be created successfully, the file should be in the repository, and the commit message should include appropriate metadata.

**Validates: Requirements 2.14, 3.14, 4.10, 10.1, 10.2, 10.3, 10.4, 10.5, 10.7, 10.8**

### Property 13: Detour Task Integration

*For any* detour that adds new tasks to a phase, the updated DevPlan should include the new tasks, maintain all existing task dependencies, and track the detour in the detours directory.

**Validates: Requirements 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.8**

### Property 14: Blocker Detection Threshold

*For any* task that fails N consecutive times (where N is configurable), the task should be automatically marked as blocked with appropriate context.

**Validates: Requirements 12.1, 12.2, 12.7, 12.8**

### Property 15: Checkpoint Rollback Round-Trip

*For any* checkpoint, rolling back to that checkpoint then checking the project state should restore the exact state that existed when the checkpoint was created (including stage, phase, task progress, and file contents).

**Validates: Requirements 13.1, 13.2, 13.3, 13.4, 13.5, 13.6, 13.7**

### Property 16: Configuration Precedence

*For any* configuration key that is set in multiple sources (file, environment variable, command-line flag), the value from the highest precedence source (flags > env > file) should be used.

**Validates: Requirements 18.1, 18.2, 18.3, 18.4, 18.5**

### Property 17: CLI Argument Validation

*For any* CLI command invoked with invalid arguments, the command should fail with exit code 1 and display a helpful error message describing what was invalid.

**Validates: Requirements 15.3, 15.4, 15.7**

### Property 18: DevPlan Evolution Tracking

*For any* task that is completed, the DevPlan should be updated to reflect the completion, the change should be committed to Git, and the changelog should record the modification with a timestamp.

**Validates: Requirements 19.1, 19.3, 19.4, 19.6, 19.7**

### Property 19: API Retry Exponential Backoff

*For any* API call that fails with a retryable error, the system should retry with exponentially increasing delays (e.g., 1s, 2s, 4s, 8s) up to a maximum number of retries.

**Validates: Requirements 20.1, 20.2**

### Property 20: Progress Calculation Accuracy

*For any* project with N total tasks and M completed tasks, the completion percentage should be (M / N) * 100, rounded to the nearest integer.

**Validates: Requirements 21.1, 21.2, 21.5**

### Property 21: Stage Navigation Preservation

*For any* pipeline stage, navigating back to a previous stage then forward again should preserve all work from the current stage and regenerate dependent artifacts correctly.

**Validates: Requirements 23.1, 23.2, 23.3, 23.4, 23.5**

### Property 22: OpenCode Model Discovery

*For any* OpenCode provider instance, calling DiscoverModels() should return a list of available models that can be used for API calls.

**Validates: Requirements 6.4 (extended)**

### Property 23: Rate Limit Tracking Accuracy

*For any* API call that returns rate limit information, the stored rate limit data should match the API response, and subsequent calls should respect the rate limit.

**Validates: Requirements 6.7 (extended)**

### Property 24: Token Statistics Aggregation

*For any* project with N API calls, the total token count should equal the sum of all individual call token counts, and statistics should be correctly aggregated by provider and phase.

**Validates: Requirements 8.1, 8.2, 8.5 (extended)**

### Property 25: Cost Statistics Accuracy

*For any* project, the total cost should equal the sum of all individual API call costs, and cost breakdowns by provider and phase should sum to the total.

**Validates: Requirements 8.3, 8.4, 8.5 (extended)**

### Property 26: Quota Monitoring

*For any* provider that supports quota checking, the system should track remaining quota and warn when approaching limits.

**Validates: Requirements 6.7 (extended)**

### Property 27: Multi-Provider Support

*For any* supported provider (OpenAI, Anthropic, Ollama, OpenCode, Firmware.ai, Requesty.ai, Z.ai, Kimi), the API_Bridge should successfully authenticate and make API calls.

**Validates: Requirements 6.1, 6.2, 6.3, 6.4 (extended)**


## Error Handling

### Error Categories

1. **User Errors**: Invalid input, missing configuration, invalid commands
   - Response: Display helpful error message, suggest correction, exit with code 1

2. **API Errors**: Rate limits, authentication failures, network timeouts
   - Response: Retry with exponential backoff, fall back to alternative model if available

3. **System Errors**: Database corruption, disk full, permission denied
   - Response: Save state, display error with context, suggest recovery steps

4. **Git Errors**: Merge conflicts, detached HEAD, uncommitted changes
   - Response: Pause execution, display conflict details, request user resolution

### Error Handling Strategies

**Retry with Exponential Backoff**:
```go
func RetryWithBackoff(operation func() error, maxRetries int) error {
    for attempt := 0; attempt < maxRetries; attempt++ {
        err := operation()
        if err == nil {
            return nil
        }
        
        if !isRetryable(err) {
            return err
        }
        
        delay := time.Duration(math.Pow(2, float64(attempt))) * time.Second
        time.Sleep(delay)
    }
    return fmt.Errorf("max retries exceeded")
}
```

**State Preservation on Critical Errors**:
```go
func HandleCriticalError(err error, state *ProjectState) {
    // Save current state
    if saveErr := state.Save(); saveErr != nil {
        log.Fatalf("Failed to save state: %v (original error: %v)", saveErr, err)
    }
    
    // Log error with full context
    log.Errorf("Critical error: %v", err)
    log.Errorf("State saved. You can resume with: geoffrussy resume")
    
    os.Exit(1)
}
```

**Graceful Degradation**:
- If primary model is unavailable, offer alternative models
- If Git operations fail, continue with local state only
- If cost tracking fails, continue without cost data

### Error Messages

All error messages should follow this format:
```
Error: <brief description>

Details: <technical details>

Suggestion: <how to fix>

Example: geoffrussy <command> <args>
```


## Testing Strategy

### Dual Testing Approach

Geoffrussy requires both unit testing and property-based testing for comprehensive coverage:

**Unit Tests**: Verify specific examples, edge cases, and error conditions
- Test specific interview question flows
- Test specific phase generation scenarios
- Test error handling for known failure modes
- Test integration points between components

**Property Tests**: Verify universal properties across all inputs
- Test state preservation with random project states
- Test phase merging with random phase combinations
- Test cost calculation with random token counts
- Test configuration precedence with random config sources

Together, unit tests catch concrete bugs while property tests verify general correctness.

### Property-Based Testing Configuration

**Library**: Use [gopter](https://github.com/leanovate/gopter) for Go property-based testing

**Configuration**:
- Minimum 100 iterations per property test (due to randomization)
- Each property test must reference its design document property
- Tag format: `// Feature: geoffrey-ai-agent, Property N: <property text>`

**Example Property Test**:
```go
func TestProperty4_StatePreservationRoundTrip(t *testing.T) {
    // Feature: geoffrey-ai-agent, Property 4: State Preservation Round-Trip
    properties := gopter.NewProperties(nil)
    
    properties.Property("saving then loading state preserves data", prop.ForAll(
        func(state *ProjectState) bool {
            // Save state
            store := NewStateStore(":memory:")
            err := store.SaveState(state)
            if err != nil {
                return false
            }
            
            // Load state
            loaded, err := store.LoadState(state.ProjectID)
            if err != nil {
                return false
            }
            
            // Compare
            return reflect.DeepEqual(state, loaded)
        },
        genProjectState(),
    ))
    
    properties.TestingRun(t, gopter.ConsoleReporter(false))
}
```

### Unit Testing Strategy

**Test Organization**:
```
geoffrussy/
├── internal/
│   ├── interview/
│   │   ├── engine.go
│   │   └── engine_test.go
│   ├── design/
│   │   ├── generator.go
│   │   └── generator_test.go
│   ├── devplan/
│   │   ├── generator.go
│   │   └── generator_test.go
│   ├── api/
│   │   ├── bridge.go
│   │   └── bridge_test.go
│   └── state/
│       ├── store.go
│       └── store_test.go
└── test/
    ├── integration/
    │   ├── pipeline_test.go
    │   └── git_test.go
    └── properties/
        ├── state_test.go
        ├── devplan_test.go
        └── cost_test.go
```

**Coverage Goals**:
- Unit test coverage: >80% for core logic
- Property test coverage: All 21 correctness properties
- Integration test coverage: All pipeline stages

### Integration Testing

**End-to-End Pipeline Tests**:
1. Init → Interview → Design → DevPlan → Review
2. Resume from each stage
3. Reiteration at each stage
4. Detour during execution
5. Blocker detection and resolution
6. Checkpoint and rollback

**Git Integration Tests**:
- Test commit creation for each artifact
- Test conflict detection
- Test rollback to checkpoints

**API Integration Tests** (with mocking):
- Test each provider (OpenAI, Anthropic, Ollama)
- Test retry logic
- Test error handling

### Test Data Generators

**For Property Tests**:
```go
func genProjectState() gopter.Gen {
    return gopter.CombineGens(
        gen.Identifier(),
        gen.Identifier(),
        genStage(),
        genInterviewData(),
        genArchitecture(),
        genPhases(),
    ).Map(func(vals []interface{}) *ProjectState {
        return &ProjectState{
            ProjectID:    vals[0].(string),
            ProjectName:  vals[1].(string),
            CurrentStage: vals[2].(Stage),
            Interview:    vals[3].(*InterviewData),
            Architecture: vals[4].(*Architecture),
            Phases:       vals[5].([]Phase),
        }
    })
}

func genPhase() gopter.Gen {
    return gopter.CombineGens(
        gen.Identifier(),
        gen.IntRange(0, 10),
        gen.Identifier(),
        gen.SliceOf(genTask()),
    ).Map(func(vals []interface{}) Phase {
        return Phase{
            ID:     vals[0].(string),
            Number: vals[1].(int),
            Title:  vals[2].(string),
            Tasks:  vals[3].([]Task),
        }
    })
}
```

### Continuous Integration

**GitHub Actions Workflow**:
```yaml
name: CI

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run unit tests
        run: go test -v -race -coverprofile=coverage.txt ./...
      
      - name: Run property tests
        run: go test -v -tags=property ./test/properties/...
      
      - name: Run integration tests
        run: go test -v -tags=integration ./test/integration/...
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./coverage.txt
```

### Manual Testing Checklist

Before each release:
- [ ] Test init on fresh directory
- [ ] Test complete pipeline (Interview → Design → DevPlan → Review → Develop)
- [ ] Test pause/resume at each stage
- [ ] Test reiteration at each stage
- [ ] Test with each supported model provider
- [ ] Test checkpoint creation and rollback
- [ ] Test detour during execution
- [ ] Test blocker detection
- [ ] Test on Linux, macOS, and Windows
- [ ] Test with invalid API keys
- [ ] Test with network failures
- [ ] Test with Git conflicts

