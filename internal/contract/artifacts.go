package contract

type ArtifactKind string

const (
	ArtifactKindInterview         ArtifactKind = "interview"
	ArtifactKindRepoAnalysis      ArtifactKind = "repo_analysis"
	ArtifactKindComplexityProfile ArtifactKind = "complexity_profile"
	ArtifactKindDesignDraft       ArtifactKind = "design_draft"
	ArtifactKindDesignReview      ArtifactKind = "design_review"
	ArtifactKindValidatedDesign   ArtifactKind = "validated_design"
	ArtifactKindValidationReport  ArtifactKind = "validation_report"
	ArtifactKindDevplanJSON       ArtifactKind = "devplan_json"
	ArtifactKindDevplanMarkdown   ArtifactKind = "devplan_markdown"
	ArtifactKindPhaseMarkdown     ArtifactKind = "phase_markdown"
	ArtifactKindHandoff           ArtifactKind = "handoff"
	ArtifactKindVerifyReport      ArtifactKind = "verify_report"
	ArtifactKindChangedFiles      ArtifactKind = "changed_files"
	ArtifactKindRunSummary        ArtifactKind = "run_summary"
)

type ArtifactManifest struct {
	ProjectID string         `json:"project_id"`
	RunID     string         `json:"run_id,omitempty"`
	Artifacts []ArtifactItem `json:"artifacts"`
}

type ArtifactItem struct {
	ID        string         `json:"id"`
	ProjectID string         `json:"project_id"`
	RunID     string         `json:"run_id,omitempty"`
	Kind      ArtifactKind   `json:"kind"`
	Path      string         `json:"path"`
	SHA256    string         `json:"sha256,omitempty"`
	Version   int            `json:"version"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
}

type ChangedFile struct {
	Path        string   `json:"path"`
	Status      string   `json:"status"`
	SHA256      string   `json:"sha256"`
	ByteSize    int64    `json:"byte_size"`
	OwningTasks []string `json:"owning_tasks"`
}

type RunSummary struct {
	ProjectID     string          `json:"project_id"`
	RunID         string          `json:"run_id"`
	Status        string          `json:"status"`
	Stages        []StageSummary  `json:"stages"`
	ProviderUsage []ProviderUsage `json:"provider_usage,omitempty"`
	CostEstimate  string          `json:"cost_estimate,omitempty"`
	StartedAt     string          `json:"started_at"`
	CompletedAt   string          `json:"completed_at,omitempty"`
	OpenRisks     []string        `json:"open_risks,omitempty"`
	ChangedFiles  []ChangedFile   `json:"changed_files,omitempty"`
}

type StageSummary struct {
	Stage       string `json:"stage"`
	Status      string `json:"status"`
	StartedAt   string `json:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
}

type ProviderUsage struct {
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	PromptTokens     int    `json:"prompt_tokens,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty"`
	TotalTokens      int    `json:"total_tokens,omitempty"`
	EstimatedUSD     string `json:"estimated_usd,omitempty"`
	RetryCount       int    `json:"retry_count,omitempty"`
}
