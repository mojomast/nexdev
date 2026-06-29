package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mojomast/nexdev/internal/checkpoint"
	"github.com/mojomast/nexdev/internal/git"
	"github.com/mojomast/nexdev/internal/resume"
	"github.com/mojomast/nexdev/internal/state"
	"github.com/spf13/cobra"
)

var (
	version string
	cfgFile string
	verbose bool
	rootCmd *cobra.Command
)

// Execute runs the root command
func Execute(ver string) error {
	version = ver
	return rootCmd.Execute()
}

func init() {
	rootCmd = &cobra.Command{
		Use:   "geoffrussy",
		Short: "Geoffrussy - AI-powered development orchestration platform",
		Long: `Geoffrussy is a next-generation AI-powered development orchestration platform
that reimagines human-AI collaboration on software projects.

The system prioritizes deep project understanding through a multi-stage iterative
pipeline: Interview → Architecture Design → DevPlan Generation → Phase Review.`,
		Version: version,
		RunE:    runRootWithResumeCheck,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if cmd.Name() != "mcp-server" && cmd.Name() != "version" && cmd.Name() != "__complete" {
				BannerAnimated()
			}
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.geoffrussy/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Add subcommands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(interviewCmd)
	rootCmd.AddCommand(designCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(reviewCmd)
	rootCmd.AddCommand(developCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(quotaCmd)
	rootCmd.AddCommand(checkpointCmd)
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(resumeCmd)
	rootCmd.AddCommand(navigateCmd)
	rootCmd.AddCommand(mcpCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Geoffrussy version %s\n", version)
	},
}

// runRootWithResumeCheck runs when geoffrussy is invoked without a subcommand
// It checks for incomplete work and offers to resume
func runRootWithResumeCheck(cmd *cobra.Command, args []string) error {
	// Determine project ID from current directory
	cwd, err := os.Getwd()
	if err != nil {
		return cmd.Help()
	}
	projectID := filepath.Base(cwd)

	// Initialize state store (project local)
	dbPath := filepath.Join(cwd, ".geoffrussy", "state.db")
	store, err := state.NewStore(dbPath)
	if err != nil {
		// If state store doesn't exist, show help
		return cmd.Help()
	}
	defer store.Close()

	// Try to get project - if it doesn't exist, just show help
	_, err = store.GetProject(projectID)
	if err != nil {
		return cmd.Help()
	}

	// Initialize managers
	gitMgr := git.NewManager(".")
	checkpointMgr := checkpoint.NewManager(store, gitMgr, filepath.Dir(dbPath))
	resumeMgr := resume.NewManager(store, checkpointMgr)

	// Check for incomplete work
	info, err := resumeMgr.DetectIncompleteWork(projectID)
	if err != nil {
		return cmd.Help()
	}

	// If no incomplete work, just show help
	if !info.HasIncompleteWork {
		fmt.Println("✅ Project is complete!")
		fmt.Println()
		return cmd.Help()
	}

	// Show incomplete work detected
	fmt.Println("🔔 Incomplete Work Detected")
	fmt.Println("═══════════════════════════")
	fmt.Println()
	fmt.Println(info.Summary)
	fmt.Println()
	fmt.Println("💡 Tip: Run 'geoffrussy resume' to continue where you left off")
	fmt.Println("     Or run 'geoffrussy status' to see detailed progress")
	fmt.Println()

	return nil
}
