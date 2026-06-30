package cli

import (
	"fmt"

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
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if cmd.Name() != "__complete" && !jsonOutput {
				BannerAnimated()
			}
		},
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
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

	// Add only the SPEC.md section 18 Nexdev CLI surface.
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(reviewCmd)
	rootCmd.AddCommand(developCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(resumeCmd)
	rootCmd.AddCommand(navigateCmd)
	rootCmd.AddCommand(runCmd)
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
	rootCmd.SetHelpCommand(&cobra.Command{Use: "__help [command]", Hidden: true})
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Nexdev version %s\n", version)
	},
}
