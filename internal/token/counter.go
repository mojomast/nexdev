package token

import (
	"fmt"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/state"
)

// Counter implements token counting and statistics
type Counter struct {
	store *state.Store
}

// NewCounter creates a new token counter
func NewCounter(store *state.Store) *Counter {
	return &Counter{
		store: store,
	}
}

// CountTokens counts tokens for a specific model
// This is a simplified implementation - in production, you'd use model-specific tokenizers
func (c *Counter) CountTokens(text string, model string) (int, error) {
	if text == "" {
		return 0, nil
	}

	// Simple estimation based on words and characters
	// Real implementation would use tiktoken or similar for accurate counting
	words := len(strings.Fields(text))
	chars := len(text)

	// Rough approximation: ~4 characters per token on average
	// This varies by model and language
	tokens := chars / 4

	// Adjust based on word count (accounts for spaces)
	if words > 0 {
		tokens = (tokens + words) / 2
	}

	// Minimum 1 token for non-empty text
	if tokens == 0 && text != "" {
		tokens = 1
	}

	return tokens, nil
}

// EstimateTokens provides a quick token estimate without model-specific logic
func (c *Counter) EstimateTokens(text string) (int, error) {
	return c.CountTokens(text, "default")
}

// GetTotalTokens returns total token statistics for a project
func (c *Counter) GetTotalTokens(projectID string) (*state.TokenStats, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project ID cannot be empty")
	}

	// Try to get cached stats first
	cachedStats, err := c.store.GetCachedTokenStats(projectID)
	if err == nil {
		// Check if cache is fresh (less than 5 minutes old)
		if time.Since(cachedStats.LastUpdated) < 5*time.Minute {
			return cachedStats, nil
		}
	}

	// Get fresh stats from store
	stats, err := c.store.GetTokenStats(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get token stats: %w", err)
	}

	// Cache the fresh stats
	if err := c.store.CacheTokenStats(projectID, stats); err != nil {
		// Log error but don't fail - caching is optional
		// In production, you'd use a proper logger here
		_ = err
	}

	return stats, nil
}

// GetTokensByProvider returns token statistics for a specific provider
func (c *Counter) GetTokensByProvider(projectID string, provider string) (*state.TokenStats, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project ID cannot be empty")
	}
	if provider == "" {
		return nil, fmt.Errorf("provider cannot be empty")
	}

	// Get all stats and filter by provider
	allStats, err := c.GetTotalTokens(projectID)
	if err != nil {
		return nil, err
	}

	// Create filtered stats
	stats := &state.TokenStats{
		TotalInput:  0,
		TotalOutput: 0,
		ByProvider:  make(map[string]int),
		ByPhase:     allStats.ByPhase, // Keep phase breakdown
		LastUpdated: allStats.LastUpdated,
	}

	// Get tokens for this specific provider
	if tokens, ok := allStats.ByProvider[provider]; ok {
		stats.ByProvider[provider] = tokens
		// Calculate totals based on this provider's usage
		// This is simplified - in production you'd query the database directly
		stats.TotalInput = tokens / 2 // Rough estimate
		stats.TotalOutput = tokens / 2
	}

	return stats, nil
}

// GetTokensByPhase returns token statistics for a specific phase
func (c *Counter) GetTokensByPhase(projectID string, phaseID string) (*state.TokenStats, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project ID cannot be empty")
	}
	if phaseID == "" {
		return nil, fmt.Errorf("phase ID cannot be empty")
	}

	// Get all stats and filter by phase
	allStats, err := c.GetTotalTokens(projectID)
	if err != nil {
		return nil, err
	}

	// Create filtered stats for this phase
	stats := &state.TokenStats{
		TotalInput:  0,
		TotalOutput: 0,
		ByProvider:  allStats.ByProvider, // Keep provider breakdown
		ByPhase:     make(map[string]int),
		LastUpdated: allStats.LastUpdated,
	}

	// Get tokens for this specific phase
	if tokens, ok := allStats.ByPhase[phaseID]; ok {
		stats.ByPhase[phaseID] = tokens
		// Calculate totals based on this phase's usage
		stats.TotalInput = tokens / 2 // Rough estimate
		stats.TotalOutput = tokens / 2
	}

	return stats, nil
}

// RecordUsage records token usage in the database
func (c *Counter) RecordUsage(projectID, phaseID, taskID, provider, model string, tokensInput, tokensOutput int, cost float64) error {
	usage := &state.TokenUsage{
		ProjectID:    projectID,
		PhaseID:      phaseID,
		TaskID:       taskID,
		Provider:     provider,
		Model:        model,
		TokensInput:  tokensInput,
		TokensOutput: tokensOutput,
		Cost:         cost,
		Timestamp:    time.Now(),
	}

	if err := c.store.RecordTokenUsage(usage); err != nil {
		return err
	}

	// Invalidate cache since we have new data
	if err := c.store.InvalidateTokenStatsCache(projectID); err != nil {
		// Log error but don't fail - cache invalidation is optional
		_ = err
	}

	return nil
}
