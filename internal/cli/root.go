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
	version      string
	cfgFile      string
	verbose      bool
	projectDir   string
	stateDir     string
	noTUI        bool
	jsonOutput   bool
	logLevel     string
	profile      string
	controlURL   string
	controlToken string
	rootCmd      *cobra.Command
)

// Execute runs the root command
func Execute(ver string) error {
	version = ver
	return rootCmd.Execute()
}

func init() {
	rootCmd = &cobra.Command{
		Use:   "nexdev",
		Short: "Nexdev - local-first coding harness",
		Long: `Nexdev is a local-first coding harness with a staged pipeline,
durable SQLite state, HTTP/SSE control plane, and MCP-compatible tools.`,
		Version: version,
		RunE:    runRootWithResumeCheck,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if cmd.Name() != "mcp-server" && cmd.Name() != "version" && cmd.Name() != "__complete" && !jsonOutput {
				BannerAnimated()
			}
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&projectDir, "project-dir", "", "project directory (default: current directory)")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./nexdev.yaml)")
	rootCmd.PersistentFlags().StringVar(&stateDir, "state-dir", "", "project state directory (default: .nexdev)")
	rootCmd.PersistentFlags().BoolVar(&noTUI, "no-tui", false, "disable TUI behavior")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "write JSON output")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level: debug, info, warn, error")
	rootCmd.PersistentFlags().StringVar(&profile, "profile", "", "profile: dev, trusted-lan, ci")
	rootCmd.PersistentFlags().StringVar(&controlURL, "control-url", "", "control-plane base URL for remote client mode")
	rootCmd.PersistentFlags().StringVar(&controlToken, "token", "", "control-plane bearer token")
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
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(tuiCmd)
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(eventsCmd)
	rootCmd.AddCommand(detourCmd)
	rootCmd.AddCommand(steerCmd)
	rootCmd.AddCommand(pauseCmd)
	rootCmd.AddCommand(cancelCmd)
	rootCmd.AddCommand(blockersCmd)
	rootCmd.AddCommand(providerCmd)
	rootCmd.AddCommand(artifactsCmd)
	rootCmd.AddCommand(doctorCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Nexdev version %s\n", version)
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
