package reviewer

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/devplan"
	"github.com/mojomast/nexdev/internal/provider"
)

// Reviewer reviews and validates development plan phases
type Reviewer struct {
	provider provider.Provider
	model    string
}

// NewReviewer creates a new phase reviewer
func NewReviewer(provider provider.Provider, model string) *Reviewer {
	return &Reviewer{
		provider: provider,
		model:    model,
	}
}

// ReviewReport contains the results of a phase review
type ReviewReport struct {
	Timestamp         time.Time
	TotalPhases       int
	IssuesFound       int
	SeverityBreakdown map[Severity]int
	PhaseReviews      []PhaseReview
	CrossPhaseIssues  []Issue
	Summary           string
}

// PhaseReview contains the review results for a single phase
type PhaseReview struct {
	PhaseID string
	Status  ReviewStatus
	Issues  []Issue
}

// ReviewStatus represents the review status of a phase
type ReviewStatus string

const (
	ReviewPassed  ReviewStatus = "passed"
	ReviewWarning ReviewStatus = "warning"
	ReviewFailed  ReviewStatus = "failed"
)

// Issue represents a problem found during review
type Issue struct {
	Type        IssueType
	Severity    Severity
	Description string
	Suggestion  string
}

// IssueType represents the type of issue
type IssueType string

const (
	IssueClarity      IssueType = "clarity"
	IssueCompleteness IssueType = "completeness"
	IssueDependencies IssueType = "dependencies"
	IssueScope        IssueType = "scope"
	IssueRisks        IssueType = "risks"
	IssueFeasibility  IssueType = "feasibility"
	IssueTesting      IssueType = "testing"
	IssueIntegration  IssueType = "integration"
)

// Severity represents the severity of an issue
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

// ReviewPhase reviews a single phase
func (r *Reviewer) ReviewPhase(phase *devplan.Phase) (*PhaseReview, error) {
	review := &PhaseReview{
		PhaseID: phase.ID,
		Status:  ReviewPassed,
		Issues:  []Issue{},
	}

	// Check clarity
	if phase.Objective == "" {
		review.Issues = append(review.Issues, Issue{
			Type:        IssueClarity,
			Severity:    SeverityCritical,
			Description: "Phase objective is missing",
			Suggestion:  "Add a clear, concise objective for this phase",
		})
	}

	// Check completeness
	if len(phase.SuccessCriteria) == 0 {
		review.Issues = append(review.Issues, Issue{
			Type:        IssueCompleteness,
			Severity:    SeverityCritical,
			Description: "No success criteria defined",
			Suggestion:  "Add measurable success criteria",
		})
	}

	if len(phase.Tasks) == 0 {
		review.Issues = append(review.Issues, Issue{
			Type:        IssueCompleteness,
			Severity:    SeverityCritical,
			Description: "No tasks defined",
			Suggestion:  "Add 3-5 actionable tasks",
		})
	}

	// Check scope
	if len(phase.Tasks) > 10 {
		review.Issues = append(review.Issues, Issue{
			Type:        IssueScope,
			Severity:    SeverityWarning,
			Description: fmt.Sprintf("Phase has %d tasks, which may be too many", len(phase.Tasks)),
			Suggestion:  "Consider splitting this phase into smaller phases",
		})
	}

	// Check tasks
	for i, task := range phase.Tasks {
		if task.Description == "" {
			review.Issues = append(review.Issues, Issue{
				Type:        IssueClarity,
				Severity:    SeverityCritical,
				Description: fmt.Sprintf("Task %d has no description", i+1),
				Suggestion:  "Add a clear description for each task",
			})
		}

		if len(task.AcceptanceCriteria) == 0 {
			review.Issues = append(review.Issues, Issue{
				Type:        IssueCompleteness,
				Severity:    SeverityWarning,
				Description: fmt.Sprintf("Task %d has no acceptance criteria", i+1),
				Suggestion:  "Add acceptance criteria to make the task verifiable",
			})
		}
	}

	// Check testing
	hasTestingTask := false
	for _, task := range phase.Tasks {
		if strings.Contains(strings.ToLower(task.Description), "test") {
			hasTestingTask = true
			break
		}
	}

	if !hasTestingTask && phase.Number > 0 {
		review.Issues = append(review.Issues, Issue{
			Type:        IssueTesting,
			Severity:    SeverityInfo,
			Description: "No explicit testing tasks found",
			Suggestion:  "Consider adding testing tasks to verify functionality",
		})
	}

	// Determine overall status
	for _, issue := range review.Issues {
		if issue.Severity == SeverityCritical {
			review.Status = ReviewFailed
			break
		} else if issue.Severity == SeverityWarning && review.Status != ReviewFailed {
			review.Status = ReviewWarning
		}
	}

	return review, nil
}

// ReviewAllPhases reviews all phases and generates a comprehensive report
func (r *Reviewer) ReviewAllPhases(phases []devplan.Phase) (*ReviewReport, error) {
	report := &ReviewReport{
		Timestamp:         time.Now(),
		TotalPhases:       len(phases),
		IssuesFound:       0,
		SeverityBreakdown: make(map[Severity]int),
		PhaseReviews:      []PhaseReview{},
		CrossPhaseIssues:  []Issue{},
	}

	// Review each phase
	for i := range phases {
		phaseReview, err := r.ReviewPhase(&phases[i])
		if err != nil {
			return nil, fmt.Errorf("failed to review phase %d: %w", i, err)
		}

		report.PhaseReviews = append(report.PhaseReviews, *phaseReview)
		report.IssuesFound += len(phaseReview.Issues)

		// Count severity
		for _, issue := range phaseReview.Issues {
			report.SeverityBreakdown[issue.Severity]++
		}
	}

	// Check for cross-phase issues
	crossPhaseIssues := r.CheckCrossPhaseIssues(phases)
	report.CrossPhaseIssues = crossPhaseIssues
	report.IssuesFound += len(crossPhaseIssues)

	for _, issue := range crossPhaseIssues {
		report.SeverityBreakdown[issue.Severity]++
	}

	// Generate summary
	report.Summary = r.generateSummary(report)

	return report, nil
}

// CheckCrossPhaseIssues checks for issues that span multiple phases
func (r *Reviewer) CheckCrossPhaseIssues(phases []devplan.Phase) []Issue {
	var issues []Issue

	// Check dependency order
	for i, phase := range phases {
		for _, dep := range phase.Dependencies {
			depNum := -1
			fmt.Sscanf(dep, "%d", &depNum)

			if depNum >= i {
				issues = append(issues, Issue{
					Type:        IssueDependencies,
					Severity:    SeverityCritical,
					Description: fmt.Sprintf("Phase %d depends on phase %d which comes after it", i, depNum),
					Suggestion:  "Reorder phases to satisfy dependencies",
				})
			}
		}
	}

	// Check for missing setup phase
	if len(phases) > 0 && !strings.Contains(strings.ToLower(phases[0].Title), "setup") {
		issues = append(issues, Issue{
			Type:        IssueCompleteness,
			Severity:    SeverityWarning,
			Description: "First phase is not a setup/infrastructure phase",
			Suggestion:  "Consider starting with a setup phase",
		})
	}

	// Check for missing testing phase
	hasTestingPhase := false
	for _, phase := range phases {
		if strings.Contains(strings.ToLower(phase.Title), "test") {
			hasTestingPhase = true
			break
		}
	}

	if !hasTestingPhase {
		issues = append(issues, Issue{
			Type:        IssueTesting,
			Severity:    SeverityWarning,
			Description: "No dedicated testing phase found",
			Suggestion:  "Consider adding a testing and validation phase",
		})
	}

	return issues
}

// generateSummary generates a summary of the review
func (r *Reviewer) generateSummary(report *ReviewReport) string {
	if report.IssuesFound == 0 {
		return "All phases passed review with no issues found."
	}

	critical := report.SeverityBreakdown[SeverityCritical]
	warnings := report.SeverityBreakdown[SeverityWarning]
	info := report.SeverityBreakdown[SeverityInfo]

	return fmt.Sprintf("Found %d issues: %d critical, %d warnings, %d info. Review the issues and apply suggested improvements.",
		report.IssuesFound, critical, warnings, info)
}

// ExportMarkdown exports the review report as markdown
func (r *Reviewer) ExportMarkdown(report *ReviewReport) (string, error) {
	var md strings.Builder

	md.WriteString("# Phase Review Report\n\n")
	md.WriteString(fmt.Sprintf("**Generated:** %s\n", report.Timestamp.Format("2006-01-02 15:04:05")))
	md.WriteString(fmt.Sprintf("**Total Phases:** %d\n", report.TotalPhases))
	md.WriteString(fmt.Sprintf("**Issues Found:** %d\n\n", report.IssuesFound))

	md.WriteString("## Summary\n\n")
	md.WriteString(report.Summary + "\n\n")

	md.WriteString("## Severity Breakdown\n\n")
	md.WriteString(fmt.Sprintf("- **Critical:** %d\n", report.SeverityBreakdown[SeverityCritical]))
	md.WriteString(fmt.Sprintf("- **Warning:** %d\n", report.SeverityBreakdown[SeverityWarning]))
	md.WriteString(fmt.Sprintf("- **Info:** %d\n\n", report.SeverityBreakdown[SeverityInfo]))

	if len(report.CrossPhaseIssues) > 0 {
		md.WriteString("## Cross-Phase Issues\n\n")
		for _, issue := range report.CrossPhaseIssues {
			md.WriteString(fmt.Sprintf("### [%s] %s\n\n", issue.Severity, issue.Type))
			md.WriteString(fmt.Sprintf("**Description:** %s\n\n", issue.Description))
			md.WriteString(fmt.Sprintf("**Suggestion:** %s\n\n", issue.Suggestion))
		}
	}

	md.WriteString("## Phase Reviews\n\n")
	for _, phaseReview := range report.PhaseReviews {
		md.WriteString(fmt.Sprintf("### Phase %s - %s\n\n", phaseReview.PhaseID, phaseReview.Status))

		if len(phaseReview.Issues) == 0 {
			md.WriteString("No issues found.\n\n")
		} else {
			for _, issue := range phaseReview.Issues {
				md.WriteString(fmt.Sprintf("#### [%s] %s\n\n", issue.Severity, issue.Type))
				md.WriteString(fmt.Sprintf("**Description:** %s\n\n", issue.Description))
				md.WriteString(fmt.Sprintf("**Suggestion:** %s\n\n", issue.Suggestion))
			}
		}
	}

	return md.String(), nil
}

// ExportJSON exports the review report as JSON
func (r *Reviewer) ExportJSON(report *ReviewReport) (string, error) {
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal review report: %w", err)
	}
	return string(jsonData), nil
}

// Improvement represents a suggested improvement
type Improvement struct {
	IssueType   IssueType
	PhaseID     string
	TaskID      string
	Description string
	NewContent  string
}

// GenerateImprovements generates specific improvements from issues
func (r *Reviewer) GenerateImprovements(issues []Issue) ([]Improvement, error) {
	improvements := []Improvement{}

	for _, issue := range issues {
		improvement := Improvement{
			IssueType:   issue.Type,
			Description: issue.Suggestion,
		}

		// Generate specific improvements based on issue type
		switch issue.Type {
		case IssueClarity:
			improvement.NewContent = "Clarified: " + issue.Suggestion
		case IssueCompleteness:
			improvement.NewContent = "Added: " + issue.Suggestion
		case IssueTesting:
			improvement.NewContent = "Testing task: Verify functionality works as expected"
		default:
			improvement.NewContent = issue.Suggestion
		}

		improvements = append(improvements, improvement)
	}

	return improvements, nil
}

// ApplyImprovements applies selected improvements to a phase
func (r *Reviewer) ApplyImprovements(phase *devplan.Phase, improvements []Improvement) (*devplan.Phase, error) {
	if phase == nil {
		return nil, fmt.Errorf("phase cannot be nil")
	}

	updated := *phase

	for _, improvement := range improvements {
		switch improvement.IssueType {
		case IssueClarity:
			if updated.Objective == "" {
				updated.Objective = improvement.NewContent
			}

		case IssueCompleteness:
			if len(updated.SuccessCriteria) == 0 {
				updated.SuccessCriteria = []string{improvement.NewContent}
			}
			if len(updated.Tasks) == 0 {
				updated.Tasks = []devplan.Task{
					{
						ID:                  fmt.Sprintf("task-%d-1", updated.Number),
						Number:              fmt.Sprintf("%d.1", updated.Number),
						Description:         improvement.NewContent,
						AcceptanceCriteria:  []string{"Task completed successfully"},
						ImplementationNotes: []string{},
						Status:              devplan.TaskNotStarted,
					},
				}
			}

		case IssueTesting:
			// Add a testing task
			testTask := devplan.Task{
				ID:                  fmt.Sprintf("task-%d-test", updated.Number),
				Number:              fmt.Sprintf("%d.%d", updated.Number, len(updated.Tasks)+1),
				Description:         improvement.NewContent,
				AcceptanceCriteria:  []string{"All tests pass"},
				ImplementationNotes: []string{"Write comprehensive tests"},
				Status:              devplan.TaskNotStarted,
			}
			updated.Tasks = append(updated.Tasks, testTask)

		case IssueScope:
			// For scope issues, the suggestion is usually to split the phase
			// This would be handled by the SplitPhase method
		}
	}

	return &updated, nil
}

// ApplyImprovementsToAll applies improvements to all phases based on the review report
func (r *Reviewer) ApplyImprovementsToAll(phases []devplan.Phase, report *ReviewReport) ([]devplan.Phase, error) {
	updated := make([]devplan.Phase, len(phases))
	copy(updated, phases)

	for i, phaseReview := range report.PhaseReviews {
		if len(phaseReview.Issues) > 0 {
			improvements, err := r.GenerateImprovements(phaseReview.Issues)
			if err != nil {
				return nil, fmt.Errorf("failed to generate improvements for phase %s: %w", phaseReview.PhaseID, err)
			}

			improvedPhase, err := r.ApplyImprovements(&updated[i], improvements)
			if err != nil {
				return nil, fmt.Errorf("failed to apply improvements to phase %s: %w", phaseReview.PhaseID, err)
			}

			updated[i] = *improvedPhase
		}
	}

	return updated, nil
}

// SelectiveApplyImprovements applies only selected improvements
func (r *Reviewer) SelectiveApplyImprovements(phase *devplan.Phase, improvements []Improvement, selectedIndices []int) (*devplan.Phase, error) {
	if phase == nil {
		return nil, fmt.Errorf("phase cannot be nil")
	}

	selectedImprovements := []Improvement{}
	for _, idx := range selectedIndices {
		if idx >= 0 && idx < len(improvements) {
			selectedImprovements = append(selectedImprovements, improvements[idx])
		}
	}

	return r.ApplyImprovements(phase, selectedImprovements)
}
