package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mojomast/nexdev/internal/config"
	"github.com/mojomast/nexdev/internal/devplan"
	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/reviewer"
	"github.com/mojomast/nexdev/internal/state"
	"github.com/mojomast/nexdev/internal/tui"
	"github.com/spf13/cobra"
)

var (
	reviewModel string
	reviewApply bool
	reviewTUI   bool
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Review development plan phases",
	Long: `Review development plan phases for clarity, completeness,
 dependencies, scope, and other quality metrics. Optionally apply
 suggested improvements.`,
	RunE: runReview,
}

func init() {
	reviewCmd.Flags().StringVar(&reviewModel, "model", "", "Model to use for review")
	reviewCmd.Flags().BoolVar(&reviewApply, "apply", false, "Apply improvements automatically")
	reviewCmd.Flags().BoolVar(&reviewTUI, "tui", true, "Display review results in interactive TUI")
}

func runReview(cmd *cobra.Command, args []string) error {
	fmt.Println("🔍 Reviewing Development Plan...")
	fmt.Println("═════════════════════════════════════════════")

	cfgMgr := config.NewManager()
	if err := cfgMgr.Load(nil); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	projectID := filepath.Base(cwd)

	// Use the same database location as init command
	dbPath := filepath.Join(cwd, ".geoffrussy", "state.db")
	store, err := state.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open state store: %w", err)
	}
	defer store.Close()

	_, err = store.GetProject(projectID)
	if err != nil {
		return fmt.Errorf("project not found: %w. Please run 'geoffrussy init' first", err)
	}
	if err := store.UpdateProjectStage(projectID, state.StageReview); err != nil {
		return fmt.Errorf("failed to update project stage: %w", err)
	}

	statePhases, err := store.ListPhases(projectID)
	if err != nil {
		return fmt.Errorf("failed to load phases: %w", err)
	}

	if len(statePhases) == 0 {
		fmt.Println("\n⚠️  No phases found. Run 'geoffrussy plan' first to generate phases.")
		return nil
	}

	fmt.Printf("\nFound %d phase(s) to review\n", len(statePhases))

	devplanPhases, err := convertStatePhasesToDevplan(store, statePhases)
	if err != nil {
		return fmt.Errorf("failed to convert phases: %w", err)
	}

	providerName, modelName, err := getProviderAndModel(cfgMgr, "review.phase", reviewModel)
	if err != nil {
		return fmt.Errorf("failed to get provider and model: %w", err)
	}

	bridge := provider.NewBridge()
	if err := setupProvider(bridge, cfgMgr, providerName); err != nil {
		return fmt.Errorf("failed to setup provider: %w", err)
	}

	prov, err := bridge.GetProvider(providerName)
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}
	printProviderUsageSnapshot(providerName, prov)

	rev := reviewer.NewReviewer(prov, modelName)
	report, err := rev.ReviewAllPhases(devplanPhases)
	if err != nil {
		return fmt.Errorf("failed to review phases: %w", err)
	}

	fmt.Printf("\n📊 Review Report\n")
	fmt.Println("═════════════════════════════════════════════")
	fmt.Printf("\n**Generated:** %s\n", report.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("**Total Phases:** %d\n", report.TotalPhases)
	fmt.Printf("**Issues Found:** %d\n\n", report.IssuesFound)

	if reviewTUI && !reviewApply {
		model := tui.NewReviewModel(report)
		program := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := program.Run(); err != nil {
			return fmt.Errorf("failed to run review TUI: %w", err)
		}
		fmt.Println("\n✨ Review complete!")
		return nil
	}

	fmt.Println("## Summary")
	fmt.Println(report.Summary)

	fmt.Println("\n## Severity Breakdown")
	fmt.Printf("- **Critical:** %d\n", report.SeverityBreakdown[reviewer.SeverityCritical])
	fmt.Printf("- **Warning:** %d\n", report.SeverityBreakdown[reviewer.SeverityWarning])
	fmt.Printf("- **Info:** %d\n", report.SeverityBreakdown[reviewer.SeverityInfo])

	if len(report.CrossPhaseIssues) > 0 {
		fmt.Println("\n## Cross-Phase Issues")
		for _, issue := range report.CrossPhaseIssues {
			fmt.Printf("\n### [%s] %s\n", issue.Severity, issue.Type)
			fmt.Printf("**Description:** %s\n", issue.Description)
			fmt.Printf("**Suggestion:** %s\n", issue.Suggestion)
		}
	}

	fmt.Println("\n## Phase Reviews")
	for _, phaseReview := range report.PhaseReviews {
		fmt.Printf("\n### Phase %s - %s\n", phaseReview.PhaseID, phaseReview.Status)
		if len(phaseReview.Issues) == 0 {
			fmt.Println("✅ No issues found.")
		} else {
			for _, issue := range phaseReview.Issues {
				fmt.Printf("\n#### [%s] %s\n", issue.Severity, issue.Type)
				fmt.Printf("**Description:** %s\n", issue.Description)
				fmt.Printf("**Suggestion:** %s\n", issue.Suggestion)
			}
		}
	}

	if reviewApply {
		if report.IssuesFound == 0 {
			fmt.Println("\n✨ No issues to apply. Plan is already clean!")
			return nil
		}

		fmt.Println("\n📝 Applying Improvements...")
		updatedPhases, err := rev.ApplyImprovementsToAll(devplanPhases, report)
		if err != nil {
			return fmt.Errorf("failed to apply improvements: %w", err)
		}

		for i, phase := range updatedPhases {
			statePhase := &state.Phase{
				ID:        phase.ID,
				ProjectID: projectID,
				Number:    phase.Number,
				Title:     phase.Title,
				Content:   formatPhaseContent(&phase),
				Status:    state.PhaseStatus(phase.Status),
				CreatedAt: phase.CreatedAt,
			}
			if err := store.SavePhase(statePhase); err != nil {
				return fmt.Errorf("failed to save updated phase %d: %w", i, err)
			}
		}

		fmt.Printf("✅ Applied improvements to %d phase(s)\n", len(updatedPhases))
	}

	fmt.Println("\n✨ Review complete!")
	return nil
}

func formatPhaseContent(phase *devplan.Phase) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Phase %d: %s\n\n", phase.Number, phase.Title)
	fmt.Fprintf(&sb, "## Objective\n\n%s\n\n", phase.Objective)
	if len(phase.SuccessCriteria) > 0 {
		sb.WriteString("## Success Criteria\n\n")
		for _, sc := range phase.SuccessCriteria {
			fmt.Fprintf(&sb, "- %s\n", sc)
		}
		sb.WriteString("\n")
	}
	if len(phase.Tasks) > 0 {
		sb.WriteString("## Tasks\n\n")
		for _, task := range phase.Tasks {
			fmt.Fprintf(&sb, "### %s: %s\n\n", task.Number, task.Description)
			if len(task.AcceptanceCriteria) > 0 {
				sb.WriteString("**Acceptance Criteria:**\n")
				for _, ac := range task.AcceptanceCriteria {
					fmt.Fprintf(&sb, "- %s\n", ac)
				}
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}

func applyImprovements(rev *reviewer.Reviewer, store *state.Store, phases []state.Phase, report *reviewer.ReviewReport) error {
	fmt.Println("📝 Applying improvements...")
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	projectID := filepath.Base(cwd)

	devplanPhases, err := convertStatePhasesToDevplan(store, []*state.Phase{&phases[0]})
	if err != nil {
		return err
	}

	updatedPhases, err := rev.ApplyImprovementsToAll(devplanPhases, report)
	if err != nil {
		return err
	}

	for _, phase := range updatedPhases {
		statePhase := &state.Phase{
			ID:        phase.ID,
			ProjectID: projectID,
			Number:    phase.Number,
			Title:     phase.Title,
			Content:   formatPhaseContent(&phase),
			Status:    state.PhaseStatus(phase.Status),
			CreatedAt: phase.CreatedAt,
		}
		if err := store.SavePhase(statePhase); err != nil {
			return err
		}
	}

	return nil
}
