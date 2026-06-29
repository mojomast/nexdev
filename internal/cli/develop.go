package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mojomast/nexdev/internal/blocker"
	"github.com/mojomast/nexdev/internal/config"
	"github.com/mojomast/nexdev/internal/detour"
	"github.com/mojomast/nexdev/internal/devplan"
	"github.com/mojomast/nexdev/internal/executor"
	"github.com/mojomast/nexdev/internal/interview"
	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
	"github.com/spf13/cobra"
)

var (
	developModel   string
	developPhase   string
	stopAfterPhase bool
)

var developCmd = &cobra.Command{
	Use:   "develop",
	Short: "Execute development phases",
	Long: `Execute development phases and tasks with real-time monitoring.
Handles detours and blockers automatically.`,
	RunE: runDevelop,
}

func init() {
	developCmd.Flags().StringVar(&developModel, "model", "", "Model to use for development")
	developCmd.Flags().StringVar(&developPhase, "phase", "", "Specific phase ID to execute")
	developCmd.Flags().BoolVar(&stopAfterPhase, "stop-after-phase", false, "Stop after completing current phase (default: continue to next phase)")
}

func runDevelop(cmd *cobra.Command, args []string) error {
	fmt.Println("🚀 Starting Development Execution...")

	// 1. Load Configuration
	cfgMgr := config.NewManager()
	if err := cfgMgr.Load(nil); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	projectID := filepath.Base(cwd)

	// 2. Initialize Store
	dbPath := filepath.Join(cwd, ".geoffrussy", "state.db")
	store, err := state.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open state store: %w", err)
	}
	defer store.Close()

	project, err := store.GetProject(projectID)
	if err != nil {
		return fmt.Errorf("project not found: %w. Please run 'geoffrussy init' first", err)
	}

	// 3. Initialize Provider
	providerName, modelName, err := getProviderAndModel(cfgMgr, "develop.execute", developModel)
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

	fmt.Printf("📦 Using Provider: %s\n", providerName)
	fmt.Printf("🤖 Using Model: %s\n", modelName)

	// 4. Initialize Components
	interviewEngine := interview.NewEngine(store, prov, modelName)
	// Pass nil cache for now; cache wiring can be added later
	devplanGenerator := devplan.NewGenerator(prov, modelName, nil)

	// We initialize these even if not strictly used by current Executor implementation
	// to ensure all components are ready for the "full implementation" context.
	_ = detour.NewManager(store, interviewEngine, devplanGenerator)
	_ = blocker.NewDetector(store, interviewEngine)

	// 5. Determine Phase
	phaseID := developPhase
	if phaseID == "" {
		if project.CurrentPhase != "" {
			phaseID = project.CurrentPhase
		} else {
			// Find first non-completed phase
			phases, err := store.ListPhases(projectID)
			if err != nil {
				return fmt.Errorf("failed to list phases: %w", err)
			}
			for _, p := range phases {
				if p.Status != state.PhaseCompleted {
					phaseID = p.ID
					break
				}
			}
		}
	}

	if phaseID == "" {
		return fmt.Errorf("no active phase found to execute")
	}

	phase, err := store.GetPhase(phaseID)
	if err != nil {
		return fmt.Errorf("failed to get phase %s: %w", phaseID, err)
	}

	project.CurrentStage = state.StageDevelop
	project.CurrentPhase = phaseID
	if err := store.UpdateProject(project); err != nil {
		return fmt.Errorf("failed to update project stage/phase: %w", err)
	}

	fmt.Printf("📋 Executing Phase: %s (%s)\n", phase.Title, phase.ID)

	// 6. Initialize Executor and Monitor
	exec := executor.NewExecutor(store, prov, modelName)
	mon := executor.NewMonitor(exec, projectID)

	// 7. Start Execution
	// Run execution in a separate goroutine so Monitor can run in main thread
	go func() {
		// Give the monitor a moment to start
		time.Sleep(500 * time.Millisecond)

		if err := exec.ExecuteProject(projectID, phaseID, stopAfterPhase); err != nil {
			// Errors are reported via the update channel usually,
			// but we can also log here if needed or if ExecuteProject returns early
			// We can't easily log to stdout here because the TUI has taken over
		}
		// We might want to close the executor or signal completion here
		// But Monitor handles Ctrl+C/Quit
	}()

	// 8. Run Monitor (Blocking)
	if err := mon.Run(); err != nil {
		return fmt.Errorf("monitor error: %w", err)
	}

	progress, err := store.CalculateProgress(projectID)
	if err == nil && progress.TotalTasks > 0 && progress.CompletedTasks == progress.TotalTasks && progress.BlockedTasks == 0 {
		if err := store.UpdateProjectStage(projectID, state.StageComplete); err == nil {
			fmt.Println("🎉 All tasks completed. Project stage advanced to complete.")
		}
	}

	return nil
}
