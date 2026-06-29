package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/mojomast/nexdev/internal/state"
	"github.com/mojomast/nexdev/internal/token"
	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Display token usage and cost statistics",
	Long: `Display detailed token usage and cost statistics broken down
by provider and phase.`,
	RunE: runStats,
}

func runStats(cmd *cobra.Command, args []string) error {
	// Determine project ID from current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	projectID := filepath.Base(cwd)

	// Initialize state store (project local)
	dbPath := filepath.Join(cwd, ".geoffrussy", "state.db")
	store, err := state.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize store: %w", err)
	}
	defer store.Close()

	// Initialize token counter and cost estimator
	counter := token.NewCounter(store)
	costEstimator := token.NewCostEstimator(store)

	// Get token statistics
	tokenStats, err := counter.GetTotalTokens(projectID)
	if err != nil {
		return fmt.Errorf("failed to get token statistics: %w", err)
	}

	// Get cost statistics
	costStats, err := costEstimator.GetCostStats(projectID)
	if err != nil {
		return fmt.Errorf("failed to get cost statistics: %w", err)
	}

	// Get most expensive calls
	expensiveCalls, err := costEstimator.GetMostExpensiveCalls(projectID, 5)
	if err != nil {
		// Non-critical, just log error or ignore
		// fmt.Printf("Warning: failed to get expensive calls: %v\n", err)
	}

	fmt.Println("📊 Token Usage & Cost Statistics")
	fmt.Println("============================================================")

	// Display Overall Totals
	fmt.Printf("Total Cost:   $%.4f\n", costStats.TotalCost)
	fmt.Printf("Total Input:  %d tokens\n", tokenStats.TotalInput)
	fmt.Printf("Total Output: %d tokens\n", tokenStats.TotalOutput)
	fmt.Printf("Grand Total:  %d tokens\n", tokenStats.TotalInput+tokenStats.TotalOutput)
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	// Breakdown by Provider
	if len(tokenStats.ByProvider) > 0 {
		fmt.Println("🔷 Breakdown by Provider")
		fmt.Println("-----------------------")
		fmt.Fprintln(w, "Provider\tTokens\tCost")
		for provider, tokens := range tokenStats.ByProvider {
			cost := costStats.ByProvider[provider]
			fmt.Fprintf(w, "%s\t%d\t$%.4f\n", provider, tokens, cost)
		}
		w.Flush()
		fmt.Println()
	}

	// Breakdown by Phase
	if len(tokenStats.ByPhase) > 0 {
		fmt.Println("🔷 Breakdown by Phase")
		fmt.Println("--------------------")
		fmt.Fprintln(w, "Phase\tTokens\tCost")
		for phase, tokens := range tokenStats.ByPhase {
			cost := costStats.ByPhase[phase]
			fmt.Fprintf(w, "%s\t%d\t$%.4f\n", phase, tokens, cost)
		}
		w.Flush()
		fmt.Println()
	}

	// Top Expensive Calls
	if len(expensiveCalls) > 0 {
		fmt.Println("🔷 Top 5 Most Expensive Calls")
		fmt.Println("-----------------------------")
		fmt.Fprintln(w, "Timestamp\tProvider\tModel\tTokens\tCost")
		for _, call := range expensiveCalls {
			timestamp := call.Timestamp.Format("2006-01-02 15:04:05")
			totalTokens := call.TokensInput + call.TokensOutput
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t$%.4f\n", timestamp, call.Provider, call.Model, totalTokens, call.Cost)
		}
		w.Flush()
		fmt.Println()
	}

	return nil
}
