package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mojomast/nexdev/internal/checkpoint"
	"github.com/mojomast/nexdev/internal/config"
	"github.com/mojomast/nexdev/internal/git"
	"github.com/mojomast/nexdev/internal/state"
	"github.com/spf13/cobra"
)

var (
	checkpointName     string
	checkpointList     bool
	checkpointRollback string
)

var checkpointCmd = &cobra.Command{
	Use:   "checkpoint",
	Short: "Create, list, or rollback checkpoints",
	Long: `Create a new checkpoint, list existing checkpoints, or rollback to a previous checkpoint.
Checkpoints save the current state for potential rollback.`,
	RunE: runCheckpoint,
}

func init() {
	checkpointCmd.Flags().StringVarP(&checkpointName, "name", "n", "", "Checkpoint name")
	checkpointCmd.Flags().BoolVarP(&checkpointList, "list", "l", false, "List all checkpoints")
	checkpointCmd.Flags().StringVarP(&checkpointRollback, "rollback", "r", "", "Rollback to checkpoint (by name)")
}

func runCheckpoint(cmd *cobra.Command, args []string) error {
	cfgMgr := config.NewManager()
	if err := cfgMgr.Load(nil); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	projectID := filepath.Base(cwd)

	// Use of same database location as init command
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

	// Initialize managers
	gitMgr := git.NewManager(cwd)
	dataDir := filepath.Dir(dbPath)
	cpManager := checkpoint.NewManager(store, gitMgr, dataDir)

	if checkpointRollback != "" {
		return rollbackToCheckpoint(cpManager, store, projectID, checkpointRollback)
	}

	if checkpointList {
		return listCheckpoints(store, projectID)
	}

	return createCheckpoint(cpManager, gitMgr, projectID, checkpointName)
}

func createCheckpoint(cpManager *checkpoint.Manager, gitMgr *git.Manager, projectID, name string) error {
	fmt.Println("💾 Creating Checkpoint")
	fmt.Println("═════════════════════════════════════════════")

	if name == "" {
		name = fmt.Sprintf("checkpoint-%s", time.Now().Format("20060102-150405"))
	}

	isRepo, err := gitMgr.IsRepository()
	if err != nil {
		return fmt.Errorf("failed to check git repository: %w", err)
	}

	if !isRepo {
		return fmt.Errorf("not in a git repository. Checkpoints require git to track state")
	}

	hasChanges, err := gitMgr.HasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("failed to check for uncommitted changes: %w", err)
	}

	if hasChanges {
		fmt.Println("📝 Staging current changes...")
		if err := gitMgr.CommitAll(fmt.Sprintf("geoffrussy checkpoint: %s", name), map[string]string{
			"type":       "checkpoint",
			"project_id": projectID,
			"created_at": time.Now().Format(time.RFC3339),
		}); err != nil {
			return fmt.Errorf("failed to commit changes: %w", err)
		}
	}

	metadata := map[string]string{
		"created_at": time.Now().Format(time.RFC3339),
		"project_id": projectID,
	}

	cp, err := cpManager.CreateCheckpoint(projectID, name, metadata)
	if err != nil {
		return fmt.Errorf("failed to create checkpoint: %w", err)
	}

	fmt.Printf("\n✅ Checkpoint created successfully!\n")
	fmt.Printf("   Name: %s\n", cp.Name)
	fmt.Printf("   Git Tag: %s\n", cp.GitTag)
	fmt.Printf("   Created: %s\n", cp.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println("\n💡 Tip: Use 'geoffrussy checkpoint --rollback=<name>' to restore this checkpoint")

	return nil
}

func listCheckpoints(store *state.Store, projectID string) error {
	fmt.Println("📋 Checkpoints")
	fmt.Println("═════════════════════════════════════════════")

	checkpoints, err := store.ListCheckpoints(projectID)
	if err != nil {
		return fmt.Errorf("failed to list checkpoints: %w", err)
	}

	if len(checkpoints) == 0 {
		fmt.Println("\n⚠️  No checkpoints found.")
		fmt.Println("💡 Tip: Use 'geoffrussy checkpoint --name=<name>' to create a checkpoint")
		return nil
	}

	fmt.Printf("\nFound %d checkpoint(s)\n\n", len(checkpoints))
	for i, cp := range checkpoints {
		fmt.Printf("%d. %s\n", i+1, cp.Name)
		fmt.Printf("   Git Tag: %s\n", cp.GitTag)
		fmt.Printf("   Created: %s\n", cp.CreatedAt.Format("2006-01-02 15:04:05"))
		if len(cp.Metadata) > 0 {
			fmt.Printf("   Metadata: %d key(s)\n", len(cp.Metadata))
		}
		fmt.Println()
	}

	fmt.Println("💡 Tip: Use 'geoffrussy checkpoint --rollback=<name>' to restore a checkpoint")
	return nil
}

func rollbackToCheckpoint(cpManager *checkpoint.Manager, store *state.Store, projectID, checkpointName string) error {
	fmt.Printf("🔄 Rolling Back to Checkpoint: %s\n", checkpointName)
	fmt.Println("═════════════════════════════════════════════")

	// Find checkpoint by name
	var targetCP *state.Checkpoint
	checkpoints, err := store.ListCheckpoints(projectID)
	if err != nil {
		return fmt.Errorf("failed to list checkpoints: %w", err)
	}

	for _, cp := range checkpoints {
		if cp.Name == checkpointName {
			targetCP = cp
			break
		}
	}

	// Try using generateCheckpointID for backward compatibility
	if targetCP == nil {
		cp, err := store.GetCheckpoint(generateCheckpointID(projectID, checkpointName))
		if err == nil {
			targetCP = cp
		}
	}

	if targetCP == nil {
		return fmt.Errorf("checkpoint not found: %s", checkpointName)
	}

	fmt.Printf("\n⚠️  Warning: This will reset your working directory to checkpoint '%s'\n", targetCP.Name)
	fmt.Printf("   Git Tag: %s\n", targetCP.GitTag)
	fmt.Printf("   Created: %s\n\n", targetCP.CreatedAt.Format("2006-01-02 15:04:05"))

	fmt.Println("Note: The following will be lost:")
	fmt.Println("  - Uncommitted changes")
	fmt.Println("  - Commits made after this checkpoint")
	fmt.Println()
	fmt.Println("The following will be preserved:")
	fmt.Println("  - State database (checkpointed state will be restored)")
	fmt.Println("  - Checkpoint history")
	fmt.Println()

	if err := cpManager.Rollback(targetCP.ID); err != nil {
		return fmt.Errorf("failed to rollback: %w", err)
	}

	fmt.Printf("✅ Successfully rolled back to checkpoint: %s\n", checkpointName)
	return nil
}

func generateCheckpointID(projectID, name string) string {
	return fmt.Sprintf("%s-%s", projectID, name)
}
