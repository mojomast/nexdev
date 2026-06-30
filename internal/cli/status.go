package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mojomast/nexdev/internal/blocker"
	"github.com/mojomast/nexdev/internal/state"
	"github.com/mojomast/nexdev/internal/token"
	"github.com/mojomast/nexdev/internal/tui"
	"github.com/spf13/cobra"
)

var (
	statusPhaseFilter  []int
	statusStatusFilter []string
	statusVerbose      bool
	statusTUI          bool
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display project status",
	Long: `Display current project status including stage, phase progress,
blockers, and token usage statistics.`,
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().IntSliceVar(&statusPhaseFilter, "phase", []int{}, "Filter by phase numbers (comma-separated)")
	statusCmd.Flags().StringSliceVar(&statusStatusFilter, "status", []string{}, "Filter by status (not_started, in_progress, completed, blocked)")
	statusCmd.Flags().BoolVarP(&statusVerbose, "verbose", "v", false, "Show detailed information")
	statusCmd.Flags().BoolVar(&statusTUI, "tui", true, "Display status in interactive TUI")
}

func runStatus(cmd *cobra.Command, args []string) error {
	if controlURL != "" || jsonOutput {
		return localRead(cmd, "/status")
	}
	// Get current directory as project root
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Initialize state store
	dbPath := filepath.Join(projectRoot, ".geoffrussy", "state.db")
	store, err := state.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize state store: %w", err)
	}
	defer store.Close()

	// Get project ID (use directory name for now)
	projectID := filepath.Base(projectRoot)

	// Check if project exists
	project, err := store.GetProject(projectID)
	if err != nil {
		fmt.Println("⚠️  No active project found in this directory")
		fmt.Println("   Run 'geoffrussy init' to initialize a new project")
		return nil
	}

	progress, err := store.CalculateProgress(projectID)
	if err != nil {
		return fmt.Errorf("failed to calculate progress: %w", err)
	}

	blockerDetector := blocker.NewDetector(store, nil)
	blockers, _ := blockerDetector.ListActiveBlockers(projectID)

	tokenCounter := token.NewCounter(store)
	tokenStats, _ := tokenCounter.GetTotalTokens(projectID)

	costEstimator := token.NewCostEstimator(store)
	totalCost, _ := costEstimator.GetTotalCost(projectID)

	if statusTUI {
		model := tui.NewStatusModel()
		model.SetProjectInfo(project.Name, string(project.CurrentStage), project.CurrentPhase)
		model.SetPhaseProgress(progress.TotalPhases, progress.CompletedPhases, progress.InProgressPhases, progress.TotalPhases-progress.CompletedPhases-progress.InProgressPhases)

		blockerLines := make([]string, 0, len(blockers))
		for _, b := range blockers {
			blockerLines = append(blockerLines, fmt.Sprintf("Task %s: %s", b.TaskID, b.Description))
		}
		model.SetBlockers(blockerLines)

		if tokenStats != nil {
			model.SetTokenUsage(tokenStats.TotalInput+tokenStats.TotalOutput, totalCost)
		} else {
			model.SetTokenUsage(0, totalCost)
		}

		program := tea.NewProgram(model, tea.WithAltScreen())
		_, err := program.Run()
		return err
	}

	// Display header
	fmt.Println("📊 Project Status")
	fmt.Println("============================================================")
	fmt.Println()

	// Display project info
	fmt.Printf("📁 Project: %s\n", project.Name)
	fmt.Printf("🆔 ID: %s\n", projectID)
	fmt.Printf("📅 Started: %s\n", project.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("🏗️  Current Stage: %s\n", formatStage(project.CurrentStage))
	fmt.Println()

	displayProgressSummary(progress)

	// Display phase-by-phase progress
	fmt.Println("\n📋 Phase Progress")
	fmt.Println("============================================================")

	filter := &state.ProgressFilter{
		PhaseNumbers: statusPhaseFilter,
	}

	// Convert status filter strings to PhaseStatus
	if len(statusStatusFilter) > 0 {
		for _, statusStr := range statusStatusFilter {
			filter.StatusFilter = append(filter.StatusFilter, state.PhaseStatus(statusStr))
		}
	}

	phaseProgress, err := store.GetFilteredProgress(projectID, filter)
	if err != nil {
		return fmt.Errorf("failed to get phase progress: %w", err)
	}

	for _, pp := range phaseProgress {
		displayPhaseProgress(pp, statusVerbose)
	}

	// Display active blockers
	if len(blockers) > 0 {
		fmt.Println("\n🚫 Active Blockers")
		fmt.Println("============================================================")
		for _, b := range blockers {
			fmt.Printf("  ⚠️  Task %s: %s\n", b.TaskID, b.Description)
		}
	}

	// Display token usage and costs
	if statusVerbose {
		fmt.Println("\n💰 Token Usage & Costs")
		fmt.Println("============================================================")

		tokenCounter := token.NewCounter(store)
		stats, err := tokenCounter.GetTotalTokens(projectID)
		if err == nil {
			fmt.Printf("  Total Input Tokens:  %d\n", stats.TotalInput)
			fmt.Printf("  Total Output Tokens: %d\n", stats.TotalOutput)
			totalTokens := stats.TotalInput + stats.TotalOutput
			fmt.Printf("  Total Tokens:        %d\n", totalTokens)

			if len(stats.ByPhase) > 0 {
				fmt.Println("\n  By Phase:")
				for phase, count := range stats.ByPhase {
					fmt.Printf("    Phase %s: %d tokens\n", phase, count)
				}
			}
		}

		fmt.Printf("\n  Total Cost: $%.2f\n", totalCost)
	}

	fmt.Println()
	return nil
}

func displayProgressSummary(progress *state.ProgressStats) {
	fmt.Println("📈 Overall Progress")
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("  Completion: %.1f%%\n", progress.CompletionPercentage)
	displayProgressBar(int(progress.CompletionPercentage))
	fmt.Printf("  Tasks: %d/%d completed (%d in progress, %d blocked, %d skipped)\n",
		progress.CompletedTasks,
		progress.TotalTasks,
		progress.InProgressTasks,
		progress.BlockedTasks,
		progress.SkippedTasks,
	)
	fmt.Printf("  Phases: %d/%d completed (%d in progress, %d blocked)\n",
		progress.CompletedPhases,
		progress.TotalPhases,
		progress.InProgressPhases,
		progress.BlockedPhases,
	)

	// Display time tracking
	fmt.Printf("\n  ⏱️  Elapsed Time: %s\n", formatDuration(progress.ElapsedTime))
	if progress.EstimatedRemaining > 0 {
		fmt.Printf("  ⏳ Estimated Remaining: %s\n", formatDuration(progress.EstimatedRemaining))
	}
}

func displayPhaseProgress(progress *state.PhaseProgress, verbose bool) {
	statusIcon := getStatusIcon(progress.Status)
	fmt.Printf("\n%s Phase %d: %s\n", statusIcon, progress.PhaseNumber, progress.PhaseTitle)

	if progress.TotalTasks > 0 {
		fmt.Printf("  Progress: %.0f%% (%d/%d tasks completed)\n",
			progress.Percentage,
			progress.CompletedTasks,
			progress.TotalTasks,
		)

		if verbose {
			if progress.InProgressTasks > 0 {
				fmt.Printf("  🔄 In Progress: %d tasks\n", progress.InProgressTasks)
			}
			if progress.BlockedTasks > 0 {
				fmt.Printf("  🚫 Blocked: %d tasks\n", progress.BlockedTasks)
			}
			if progress.SkippedTasks > 0 {
				fmt.Printf("  ⏭️  Skipped: %d tasks\n", progress.SkippedTasks)
			}
		}
	}
}

func displayProgressBar(percent int) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	barLength := 40
	filled := percent * barLength / 100
	bar := "  [" + strings.Repeat("█", filled) + strings.Repeat("░", barLength-filled) + fmt.Sprintf("] %d%%", percent)
	fmt.Println(bar)
}

func getStatusIcon(status state.PhaseStatus) string {
	switch status {
	case state.PhaseCompleted:
		return "✅"
	case state.PhaseInProgress:
		return "🔄"
	case state.PhaseBlocked:
		return "🚫"
	default:
		return "⬜"
	}
}

func formatStage(stage state.Stage) string {
	switch stage {
	case state.StageInit:
		return "🔧 Initialization"
	case state.StageInterview:
		return "💬 Interview"
	case state.StageDesign:
		return "🎨 Design"
	case state.StagePlan:
		return "📋 Planning"
	case state.StageReview:
		return "🔍 Review"
	case state.StageDevelop:
		return "⚡ Development"
	case state.StageComplete:
		return "🎉 Complete"
	default:
		return string(stage)
	}
}
