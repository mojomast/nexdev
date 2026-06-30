package contract

type Finding struct {
	ID          string `json:"id,omitempty"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Evidence    string `json:"evidence,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
}

type RepoAnalysis struct {
	Languages        []string `json:"languages"`
	Frameworks       []string `json:"frameworks"`
	PackageManagers  []string `json:"package_managers"`
	TestCommands     []string `json:"test_commands"`
	LintCommands     []string `json:"lint_commands"`
	Entrypoints      []string `json:"entrypoints"`
	ImportantFiles   []string `json:"important_files"`
	ForbiddenPaths   []string `json:"forbidden_paths"`
	RepoInstructions []string `json:"repo_instructions"`
	RiskNotes        []string `json:"risk_notes"`
	Summary          string   `json:"summary"`
}

type InterviewData struct {
	Requirements      []string `json:"requirements"`
	Constraints       []string `json:"constraints"`
	OpenQuestions     []string `json:"open_questions"`
	UserPersonas      []string `json:"user_personas"`
	NonGoals          []string `json:"non_goals"`
	AcceptanceSignals []string `json:"acceptance_signals"`
	RiskTolerance     string   `json:"risk_tolerance"`
	TargetUsers       []string `json:"target_users"`
	RawTranscript     string   `json:"raw_transcript"`
}

type ComplexityProfile struct {
	Score             int      `json:"score"`
	Level             string   `json:"level"`
	RecommendedPhases int      `json:"recommended_phases"`
	RiskFactors       []string `json:"risk_factors"`
	SuggestedVoices   []string `json:"suggested_voices"`
	SuggestedTests    []string `json:"suggested_tests"`
	Rationale         string   `json:"rationale"`
}

type HivemindCritique struct {
	Voice      string    `json:"voice"`
	Findings   []Finding `json:"findings"`
	Severity   string    `json:"severity"`
	Verdict    string    `json:"verdict"`
	Confidence float64   `json:"confidence"`
}

type HivemindSynthesis struct {
	ConsensusFindings []Finding `json:"consensus_findings"`
	RequiredChanges   []string  `json:"required_changes"`
	OptionalChanges   []string  `json:"optional_changes"`
	Disagreements     []string  `json:"disagreements"`
	FinalVerdict      string    `json:"final_verdict"`
}

type ValidationReport struct {
	Ambiguities        []Finding `json:"ambiguities"`
	Conflicts          []Finding `json:"conflicts"`
	MissingPrereqs     []Finding `json:"missing_prereqs"`
	Blockers           []Finding `json:"blockers"`
	HallucinationRisks []Finding `json:"hallucination_risks"`
	Verdict            string    `json:"verdict"`
}

type PhaseSketch struct {
	ID                  string   `json:"id"`
	Number              int      `json:"number"`
	Title               string   `json:"title"`
	Description         string   `json:"description"`
	EstimatedComplexity string   `json:"estimated_complexity"`
	Goals               []string `json:"goals"`
	Risks               []string `json:"risks"`
}

type TaskSpec struct {
	ID                 string   `json:"id"`
	PhaseID            string   `json:"phase_id"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	ExpectedFiles      []string `json:"expected_files"`
	Dependencies       []string `json:"dependencies"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	TestCommands       []string `json:"test_commands"`
	RiskLevel          string   `json:"risk_level"`
	RequiredTools      []string `json:"required_tools"`
	Notes              []string `json:"notes"`
}

type DetourSource string

const (
	DetourSourceBlockerAuto    DetourSource = "blocker_auto"
	DetourSourceOperatorManual DetourSource = "operator_manual"
	DetourSourceReviewReplan   DetourSource = "review_replan"
)

type DetourRequest struct {
	ProjectID     string       `json:"project_id"`
	RunID         string       `json:"run_id"`
	TriggerTaskID string       `json:"trigger_task_id"`
	Reason        string       `json:"reason"`
	Context       string       `json:"context"`
	Source        DetourSource `json:"source"`
}

type DetourResult struct {
	ID           string     `json:"id"`
	NewTasks     []TaskSpec `json:"new_tasks"`
	SplicedAfter string     `json:"spliced_after"`
	IDConflicts  []string   `json:"id_conflicts"`
	Depth        int        `json:"depth"`
}

type VerifyReport struct {
	Passed       bool            `json:"passed"`
	Commands     []CommandResult `json:"commands"`
	ChangedFiles []ChangedFile   `json:"changed_files"`
	Failures     []Finding       `json:"failures"`
	Warnings     []Finding       `json:"warnings"`
}

type CommandResult struct {
	Command      string `json:"command"`
	ExitCode     int    `json:"exit_code"`
	TimedOut     bool   `json:"timed_out"`
	StdoutTail   string `json:"stdout_tail"`
	StderrTail   string `json:"stderr_tail"`
	OutputSHA256 string `json:"output_sha256"`
	StartedAt    string `json:"started_at"`
	CompletedAt  string `json:"completed_at"`
}
