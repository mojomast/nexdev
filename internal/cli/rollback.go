package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mojomast/nexdev/internal/checkpoint"
	"github.com/mojomast/nexdev/internal/config"
	"github.com/mojomast/nexdev/internal/git"
	"github.com/mojomast/nexdev/internal/state"
	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rollback to a checkpoint",
	Long: `Rollback project state to a previous checkpoint.
Restores both database state and Git repository.`,
	RunE: runRollback,
}

func runRollback(cmd *cobra.Command, args []string) error {
	fmt.Println("⏪ Rollback to Checkpoint")
	fmt.Println("============================================================")

	// Load config
	cfgMgr := config.NewManager()
	if err := cfgMgr.Load(nil); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	projectID := filepath.Base(cwd)

	// Initialize store (local)
	dbPath := filepath.Join(cwd, ".geoffrussy", "state.db")
	store, err := state.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open state store: %w. Make sure you are in a project directory.", err)
	}
	defer store.Close()

	// Initialize managers
	gitMgr := git.NewManager(cwd)
	dataDir := filepath.Dir(dbPath)
	cpManager := checkpoint.NewManager(store, gitMgr, dataDir)

	// List checkpoints
	checkpoints, err := cpManager.ListCheckpoints(projectID)
	if err != nil {
		return fmt.Errorf("failed to list checkpoints: %w", err)
	}

	if len(checkpoints) == 0 {
		fmt.Println("No checkpoints found.")
		return nil
	}

	fmt.Println("\nAvailable Checkpoints:")
	for i, cp := range checkpoints {
		fmt.Printf("%d. %s (%s)\n   Created: %s\n", i+1, cp.Name, cp.GitTag, cp.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	fmt.Println()

	// Prompt for selection
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Select checkpoint number to rollback to (or 'q' to quit): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "q" || input == "quit" {
		return nil
	}

	index, err := strconv.Atoi(input)
	if err != nil || index < 1 || index > len(checkpoints) {
		return fmt.Errorf("invalid selection")
	}

	targetCP := checkpoints[index-1]

	// Confirm
	fmt.Printf("\n⚠️  WARNING: You are about to rollback to '%s'.\n", targetCP.Name)
	fmt.Println("This will:")
	fmt.Println("  1. Reset your git repository to the checkpoint tag.")
	fmt.Println("  2. Restore the database state to what it was at that time.")
	fmt.Println("  3. Lose any uncommitted changes and commits made after the checkpoint.")
	fmt.Println("\nAre you sure you want to continue? (yes/no): ")

	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm != "yes" {
		fmt.Println("Rollback cancelled.")
		return nil
	}

	fmt.Println("\nPerforming rollback...")
	if err := cpManager.Rollback(targetCP.ID); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	fmt.Println("✅ Rollback complete!")
	return nil
}
