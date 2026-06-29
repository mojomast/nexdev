package provider

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Bridge provides a unified interface to interact with multiple AI providers
type Bridge struct {
	providers       map[string]Provider
	defaultProvider string
	rateLimitCache  map[string]*RateLimitInfo
	quotaCache      map[string]*QuotaInfo
	cacheMutex      sync.RWMutex
	cacheExpiry     time.Duration
}

// NewBridge creates a new API Bridge
func NewBridge() *Bridge {
	return &Bridge{
		providers:      make(map[string]Provider),
		rateLimitCache: make(map[string]*RateLimitInfo),
		quotaCache:     make(map[string]*QuotaInfo),
		cacheExpiry:    5 * time.Minute, // Cache rate limit/quota info for 5 minutes
	}
}

// RegisterProvider registers a provider with the bridge
func (b *Bridge) RegisterProvider(provider Provider) error {
	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}

	name := provider.Name()
	if name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}

	b.providers[name] = provider

	// Set first provider as default if none set
	if b.defaultProvider == "" {
		b.defaultProvider = name
	}

	return nil
}

// SetDefaultProvider sets the default provider to use when none is specified
func (b *Bridge) SetDefaultProvider(name string) error {
	if _, exists := b.providers[name]; !exists {
		return fmt.Errorf("provider '%s' not registered", name)
	}
	b.defaultProvider = name
	return nil
}

// GetProvider returns a provider by name
func (b *Bridge) GetProvider(name string) (Provider, error) {
	provider, exists := b.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider '%s' not registered", name)
	}
	return provider, nil
}

// ListProviders returns all registered provider names
func (b *Bridge) ListProviders() []string {
	names := make([]string, 0, len(b.providers))
	for name := range b.providers {
		names = append(names, name)
	}
	return names
}

// ListModels lists all available models from all providers
func (b *Bridge) ListModels() ([]Model, error) {
	allModels := make([]Model, 0)

	for _, provider := range b.providers {
		if !provider.IsAuthenticated() {
			continue // Skip unauthenticated providers
		}

		providerModels := make([]Model, 0)
		if discovered, err := provider.DiscoverModels(); err == nil {
			providerModels = append(providerModels, discovered...)
		}

		if listed, err := provider.ListModels(); err == nil {
			providerModels = append(providerModels, listed...)
		}

		if len(providerModels) == 0 {
			continue
		}

		modelMap := make(map[string]Model)
		for _, m := range providerModels {
			modelMap[m.Name] = m
		}
		for _, m := range modelMap {
			allModels = append(allModels, m)
		}
	}

	return allModels, nil
}

// ListModelsByProvider lists models from a specific provider
func (b *Bridge) ListModelsByProvider(providerName string) ([]Model, error) {
	provider, err := b.GetProvider(providerName)
	if err != nil {
		return nil, err
	}

	if !provider.IsAuthenticated() {
		return nil, fmt.Errorf("provider '%s' not authenticated", providerName)
	}

	return provider.ListModels()
}

// ValidateModel checks if a model exists and is available
func (b *Bridge) ValidateModel(providerName, modelName string) error {
	provider, err := b.GetProvider(providerName)
	if err != nil {
		return err
	}

	if !provider.IsAuthenticated() {
		return fmt.Errorf("provider '%s' not authenticated", providerName)
	}

	models, err := provider.ListModels()
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	for _, model := range models {
		if model.Name == modelName {
			return nil
		}
	}

	return fmt.Errorf("model '%s' not found in provider '%s'", modelName, providerName)
}

// Call makes a non-streaming API call using the specified provider and model
func (b *Bridge) Call(ctx context.Context, providerName, model, prompt string) (*Response, error) {
	provider, err := b.GetProvider(providerName)
	if err != nil {
		return nil, err
	}

	if !provider.IsAuthenticated() {
		return nil, fmt.Errorf("provider '%s' not authenticated", providerName)
	}

	// Check rate limits before making call
	if err := b.checkRateLimit(providerName); err != nil {
		return nil, err
	}

	// Check quotas before making call
	if err := b.checkQuota(providerName); err != nil {
		return nil, err
	}

	// Make the call
	resp, err := provider.Call(ctx, model, prompt)
	if err != nil {
		return nil, err
	}

	// Update rate limit and quota cache after successful call
	b.updateCacheAfterCall(providerName, provider)

	return resp, nil
}

// Stream makes a streaming API call using the specified provider and model
func (b *Bridge) Stream(ctx context.Context, providerName, model, prompt string) (<-chan string, error) {
	provider, err := b.GetProvider(providerName)
	if err != nil {
		return nil, err
	}

	if !provider.IsAuthenticated() {
		return nil, fmt.Errorf("provider '%s' not authenticated", providerName)
	}

	// Check rate limits before making call
	if err := b.checkRateLimit(providerName); err != nil {
		return nil, err
	}

	// Check quotas before making call
	if err := b.checkQuota(providerName); err != nil {
		return nil, err
	}

	// Make the streaming call
	ch, err := provider.Stream(ctx, model, prompt)
	if err != nil {
		return nil, err
	}

	// Update rate limit and quota cache after successful call
	go b.updateCacheAfterCall(providerName, provider)

	return ch, nil
}

// GetRateLimitInfo returns cached or fresh rate limit information
func (b *Bridge) GetRateLimitInfo(providerName string) (*RateLimitInfo, error) {
	provider, err := b.GetProvider(providerName)
	if err != nil {
		return nil, err
	}

	if !provider.IsAuthenticated() {
		return nil, fmt.Errorf("provider '%s' not authenticated", providerName)
	}

	// Check cache first
	b.cacheMutex.RLock()
	cached, exists := b.rateLimitCache[providerName]
	b.cacheMutex.RUnlock()

	if exists && time.Since(cached.ResetAt) < b.cacheExpiry {
		return cached, nil
	}

	// Fetch fresh data
	info, err := provider.GetRateLimitInfo()
	if err != nil {
		return nil, err
	}

	// Update cache
	if info != nil {
		b.cacheMutex.Lock()
		b.rateLimitCache[providerName] = info
		b.cacheMutex.Unlock()
	}

	return info, nil
}

// GetQuotaInfo returns cached or fresh quota information
func (b *Bridge) GetQuotaInfo(providerName string) (*QuotaInfo, error) {
	provider, err := b.GetProvider(providerName)
	if err != nil {
		return nil, err
	}

	if !provider.IsAuthenticated() {
		return nil, fmt.Errorf("provider '%s' not authenticated", providerName)
	}

	// Check cache first
	b.cacheMutex.RLock()
	cached, exists := b.quotaCache[providerName]
	b.cacheMutex.RUnlock()

	if exists && time.Since(cached.ResetAt) < b.cacheExpiry {
		return cached, nil
	}

	// Fetch fresh data
	info, err := provider.GetQuotaInfo()
	if err != nil {
		return nil, err
	}

	// Update cache
	if info != nil {
		b.cacheMutex.Lock()
		b.quotaCache[providerName] = info
		b.cacheMutex.Unlock()
	}

	return info, nil
}

// checkRateLimit checks if we're within rate limits before making a call
// and delays the request if necessary
func (b *Bridge) checkRateLimit(providerName string) error {
	info, err := b.GetRateLimitInfo(providerName)
	if err != nil {
		// If we can't get rate limit info, proceed anyway
		return nil
	}

	if info == nil {
		// Provider doesn't support rate limits
		return nil
	}

	// Check if we have requests remaining
	if info.RequestsRemaining <= 0 {
		// If we have RetryAfter, wait for it
		if info.RetryAfter > 0 {
			// Only wait up to 5 minutes
			if info.RetryAfter <= 5*time.Minute {
				fmt.Printf("Rate limit reached for provider '%s', waiting %v before retry...\n", providerName, info.RetryAfter)
				time.Sleep(info.RetryAfter)
				return nil
			}
			return fmt.Errorf("rate limit exceeded for provider '%s', retry after %v (too long to wait)", providerName, info.RetryAfter)
		}

		// Check if we can wait until reset
		timeUntilReset := time.Until(info.ResetAt)
		if timeUntilReset > 0 && timeUntilReset <= 5*time.Minute {
			fmt.Printf("Rate limit reached for provider '%s', waiting %v until reset...\n", providerName, timeUntilReset)
			time.Sleep(timeUntilReset + time.Second) // Add 1 second buffer
			return nil
		}

		return fmt.Errorf("rate limit exceeded for provider '%s'", providerName)
	}

	// Check if we're approaching the limit (within 10%)
	if info.RequestsLimit > 0 {
		threshold := info.RequestsLimit / 10
		if info.RequestsRemaining < threshold {
			// Log warning but don't block the request
			fmt.Printf("Warning: approaching rate limit for provider '%s' (%d remaining)\n", providerName, info.RequestsRemaining)
		}
	}

	return nil
}

// checkQuota checks if we're within quota limits before making a call
func (b *Bridge) checkQuota(providerName string) error {
	info, err := b.GetQuotaInfo(providerName)
	if err != nil {
		// If we can't get quota info, proceed anyway
		return nil
	}

	if info == nil {
		// Provider doesn't support quotas
		return nil
	}

	// Check token quota
	if info.TokensRemaining <= 0 {
		return fmt.Errorf("token quota exceeded for provider '%s'", providerName)
	}

	// Check cost quota
	if info.CostRemaining <= 0 {
		return fmt.Errorf("cost quota exceeded for provider '%s'", providerName)
	}

	// Warn if approaching limits (within 10%)
	if info.TokensLimit > 0 {
		threshold := info.TokensLimit / 10
		if info.TokensRemaining < threshold {
			fmt.Printf("Warning: approaching token quota for provider '%s' (%d remaining)\n", providerName, info.TokensRemaining)
		}
	}

	if info.CostLimit > 0 {
		threshold := info.CostLimit * 0.1
		if info.CostRemaining < threshold {
			fmt.Printf("Warning: approaching cost quota for provider '%s' ($%.2f remaining)\n", providerName, info.CostRemaining)
		}
	}

	return nil
}

// updateCacheAfterCall updates rate limit and quota cache after a call
func (b *Bridge) updateCacheAfterCall(providerName string, provider Provider) {
	// Try to update rate limit info
	if info, err := provider.GetRateLimitInfo(); err == nil && info != nil {
		b.cacheMutex.Lock()
		b.rateLimitCache[providerName] = info
		b.cacheMutex.Unlock()
	}

	// Try to update quota info
	if info, err := provider.GetQuotaInfo(); err == nil && info != nil {
		b.cacheMutex.Lock()
		b.quotaCache[providerName] = info
		b.cacheMutex.Unlock()
	}
}

// RefreshRateLimits refreshes rate limit information for all providers
func (b *Bridge) RefreshRateLimits() error {
	for name, provider := range b.providers {
		if !provider.IsAuthenticated() {
			continue
		}

		info, err := provider.GetRateLimitInfo()
		if err != nil {
			continue
		}

		if info != nil {
			b.cacheMutex.Lock()
			b.rateLimitCache[name] = info
			b.cacheMutex.Unlock()
		}
	}

	return nil
}

// RefreshQuotas refreshes quota information for all providers
func (b *Bridge) RefreshQuotas() error {
	for name, provider := range b.providers {
		if !provider.IsAuthenticated() {
			continue
		}

		info, err := provider.GetQuotaInfo()
		if err != nil {
			continue
		}

		if info != nil {
			b.cacheMutex.Lock()
			b.quotaCache[name] = info
			b.cacheMutex.Unlock()
		}
	}

	return nil
}

// GetAllRateLimits returns rate limit information for all providers
func (b *Bridge) GetAllRateLimits() map[string]*RateLimitInfo {
	result := make(map[string]*RateLimitInfo)

	for name := range b.providers {
		info, err := b.GetRateLimitInfo(name)
		if err == nil && info != nil {
			result[name] = info
		}
	}

	return result
}

// GetAllQuotas returns quota information for all providers
func (b *Bridge) GetAllQuotas() map[string]*QuotaInfo {
	result := make(map[string]*QuotaInfo)

	for name := range b.providers {
		info, err := b.GetQuotaInfo(name)
		if err == nil && info != nil {
			result[name] = info
		}
	}

	return result
}

// SupportsCodingPlan checks if a provider supports coding plans
func (b *Bridge) SupportsCodingPlan(providerName string) (bool, error) {
	provider, err := b.GetProvider(providerName)
	if err != nil {
		return false, err
	}

	return provider.SupportsCodingPlan(), nil
}
