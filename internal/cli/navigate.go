package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mojomast/nexdev/internal/git"
	"github.com/mojomast/nexdev/internal/navigation"
	"github.com/mojomast/nexdev/internal/state"
	"github.com/spf13/cobra"
)

var (
	navigateStage     string
	navigateProjectID string
	navigateList      bool
)

var navigateCmd = &cobra.Command{
	Use:   "navigate",
	Short: "Navigate between pipeline stages",
	Long: `Navigate between pipeline stages (interview, design, plan, review, develop).

You can:
  - Go back to a previous stage to reiterate
  - Move forward to the next stage
  - List available navigation options

Examples:
  geoffrussy navigate --stage interview    # Go back to interview stage
  geoffrussy navigate --list               # Show available navigation options`,
	RunE: runNavigate,
}

func init() {
	navigateCmd.Flags().StringVar(&navigateStage, "stage", "", "Target stage to navigate to (interview, design, plan, review, develop)")
	navigateCmd.Flags().StringVar(&navigateProjectID, "project", "", "Project ID (defaults to current directory)")
	navigateCmd.Flags().BoolVar(&navigateList, "list", false, "List available navigation options")
}

func runNavigate(cmd *cobra.Command, args []string) error {
	// Determine project ID
	projectID := navigateProjectID
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	if projectID == "" {
		projectID = filepath.Base(cwd)
	}

	// Initialize state store (project local)
	dbPath := filepath.Join(cwd, ".geoffrussy", "state.db")
	store, err := state.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize state store: %w", err)
	}
	defer store.Close()

	// Initialize git manager
	gitMgr := git.NewManager(".")

	// Initialize navigator
	nav := navigation.NewNavigator(store, gitMgr)

	// If list flag is set, show navigation options
	if navigateList {
		return showNavigationOptions(nav, projectID)
	}

	// If no stage specified, show current status and options
	if navigateStage == "" {
		fmt.Println("ℹ️  No target stage specified.")
		fmt.Println()
		return showNavigationOptions(nav, projectID)
	}

	// Parse target stage
	targetStage, err := parseStage(navigateStage)
	if err != nil {
		return fmt.Errorf("invalid stage: %w", err)
	}

	// Perform navigation
	fmt.Printf("🧭 Navigating to %s stage...\n\n", targetStage)

	result, err := nav.NavigateToStage(projectID, targetStage)
	if err != nil {
		return fmt.Errorf("navigation failed: %w", err)
	}

	// Display result
	fmt.Println("✅ Navigation Complete")
	fmt.Println("──────────────────────")
	fmt.Printf("From: %s\n", result.FromStage)
	fmt.Printf("To:   %s\n", result.ToStage)
	fmt.Println()

	if len(result.PreservedWork) > 0 {
		fmt.Println("📦 Preserved Work:")
		for _, item := range result.PreservedWork {
			fmt.Printf("   • %s\n", item)
		}
		fmt.Println()
	}

	if len(result.RegeneratedArtifacts) > 0 {
		fmt.Println("🔄 Will Need Regeneration (if modified):")
		for _, item := range result.RegeneratedArtifacts {
			fmt.Printf("   • %s\n", item)
		}
		fmt.Println()
	}

	fmt.Println("📌 Next Action:")
	fmt.Printf("   %s\n", result.NextAction)
	fmt.Println()

	return nil
}

func showNavigationOptions(nav *navigation.Navigator, projectID string) error {
	// Get current project
	options, err := nav.GetNavigationOptions(projectID)
	if err != nil {
		return fmt.Errorf("failed to get navigation options: %w", err)
	}

	fmt.Println("📍 Current Stage:", options.CurrentStage)
	fmt.Println()

	if options.NextStage != "" {
		fmt.Println("⏭️  Next Stage:")
		fmt.Printf("   %s\n", options.NextStage)
		fmt.Println()
	}

	if len(options.CanGoBack) > 0 {
		fmt.Println("⏮️  Can Go Back To:")
		for _, stage := range options.CanGoBack {
			fmt.Printf("   • %s\n", stage)
		}
		fmt.Println()
	}

	fmt.Println("💡 Usage:")
	fmt.Println("   geoffrussy navigate --stage <stage-name>")
	fmt.Println()

	return nil
}
