package token

import (
	"fmt"
	"time"

	"github.com/mojomast/nexdev/internal/state"
)

// CostEstimator implements cost calculation and tracking
type CostEstimator struct {
	store        *state.Store
	budgetLimit  float64
	warningLevel float64 // Percentage of budget to trigger warning (e.g., 0.8 for 80%)
}

// NewCostEstimator creates a new cost estimator
func NewCostEstimator(store *state.Store) *CostEstimator {
	return &CostEstimator{
		store:        store,
		budgetLimit:  0, // 0 means no limit
		warningLevel: 0.8,
	}
}

// SetBudgetLimit sets the budget limit for the project
func (c *CostEstimator) SetBudgetLimit(limit float64) {
	c.budgetLimit = limit
}

// SetWarningLevel sets the warning threshold as a percentage (0.0 to 1.0)
func (c *CostEstimator) SetWarningLevel(level float64) {
	if level > 0 && level < 1 {
		c.warningLevel = level
	}
}

// CalculateCost calculates the cost for a given token usage
func (c *CostEstimator) CalculateCost(tokensInput, tokensOutput int, priceInput, priceOutput float64) float64 {
	// Prices are per 1K tokens
	inputCost := (float64(tokensInput) / 1000.0) * priceInput
	outputCost := (float64(tokensOutput) / 1000.0) * priceOutput
	return inputCost + outputCost
}

// GetTotalCost returns the total cost for a project
func (c *CostEstimator) GetTotalCost(projectID string) (float64, error) {
	if projectID == "" {
		return 0, fmt.Errorf("project ID cannot be empty")
	}

	stats, err := c.store.GetCostStats(projectID)
	if err != nil {
		return 0, fmt.Errorf("failed to get cost stats: %w", err)
	}

	return stats.TotalCost, nil
}

// GetCostByProvider returns cost statistics for a specific provider
func (c *CostEstimator) GetCostByProvider(projectID, provider string) (float64, error) {
	if projectID == "" {
		return 0, fmt.Errorf("project ID cannot be empty")
	}
	if provider == "" {
		return 0, fmt.Errorf("provider cannot be empty")
	}

	stats, err := c.store.GetCostStats(projectID)
	if err != nil {
		return 0, fmt.Errorf("failed to get cost stats: %w", err)
	}

	if cost, ok := stats.ByProvider[provider]; ok {
		return cost, nil
	}

	return 0, nil
}

// GetCostByPhase returns cost statistics for a specific phase
func (c *CostEstimator) GetCostByPhase(projectID, phaseID string) (float64, error) {
	if projectID == "" {
		return 0, fmt.Errorf("project ID cannot be empty")
	}
	if phaseID == "" {
		return 0, fmt.Errorf("phase ID cannot be empty")
	}

	stats, err := c.store.GetCostStats(projectID)
	if err != nil {
		return 0, fmt.Errorf("failed to get cost stats: %w", err)
	}

	if cost, ok := stats.ByPhase[phaseID]; ok {
		return cost, nil
	}

	return 0, nil
}

// GetCostStats returns detailed cost statistics
func (c *CostEstimator) GetCostStats(projectID string) (*state.CostStats, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project ID cannot be empty")
	}

	return c.store.GetCostStats(projectID)
}

// CheckBudget checks if the current cost is within budget limits
// Returns warning message if approaching limit, error if exceeded
func (c *CostEstimator) CheckBudget(projectID string) (warning string, err error) {
	if c.budgetLimit <= 0 {
		return "", nil // No budget limit set
	}

	totalCost, err := c.GetTotalCost(projectID)
	if err != nil {
		return "", fmt.Errorf("failed to check budget: %w", err)
	}

	if totalCost >= c.budgetLimit {
		return "", fmt.Errorf("budget limit exceeded: $%.2f / $%.2f", totalCost, c.budgetLimit)
	}

	warningThreshold := c.budgetLimit * c.warningLevel
	if totalCost >= warningThreshold {
		percentage := (totalCost / c.budgetLimit) * 100
		return fmt.Sprintf("approaching budget limit: $%.2f / $%.2f (%.1f%%)", totalCost, c.budgetLimit, percentage), nil
	}

	return "", nil
}

// EstimateDevPlanCost estimates the total cost for a DevPlan
func (c *CostEstimator) EstimateDevPlanCost(phases []PhaseEstimate) float64 {
	totalCost := 0.0
	for _, phase := range phases {
		totalCost += phase.EstimatedCost
	}
	return totalCost
}

// PhaseEstimate represents cost estimate for a phase
type PhaseEstimate struct {
	PhaseID       string
	PhaseName     string
	EstimatedCost float64
	TaskCount     int
}

// GetMostExpensiveCalls returns the most expensive API calls
func (c *CostEstimator) GetMostExpensiveCalls(projectID string, limit int) ([]*state.TokenUsage, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project ID cannot be empty")
	}
	if limit <= 0 {
		limit = 10
	}

	return c.store.GetMostExpensiveCalls(projectID, limit)
}

// GetCostTrends returns cost trends over time
func (c *CostEstimator) GetCostTrends(projectID string, startTime, endTime time.Time) ([]CostTrend, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project ID cannot be empty")
	}

	usage, err := c.store.GetTokenUsageByTimeRange(projectID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get cost trends: %w", err)
	}

	// Group by day and calculate daily costs
	trendMap := make(map[string]float64)
	for _, u := range usage {
		day := u.Timestamp.Format("2006-01-02")
		trendMap[day] += u.Cost
	}

	// Convert to slice
	trends := make([]CostTrend, 0, len(trendMap))
	for day, cost := range trendMap {
		t, _ := time.Parse("2006-01-02", day)
		trends = append(trends, CostTrend{
			Date: t,
			Cost: cost,
		})
	}

	return trends, nil
}

// CostTrend represents cost for a specific time period
type CostTrend struct {
	Date time.Time
	Cost float64
}
