package provider

import (
	"context"
	"testing"
	"time"
)

func TestNewBridge(t *testing.T) {
	bridge := NewBridge()
	if bridge == nil {
		t.Fatal("expected non-nil bridge")
	}
	if len(bridge.providers) != 0 {
		t.Errorf("expected empty providers map, got %d providers", len(bridge.providers))
	}
}

func TestBridge_RegisterProvider(t *testing.T) {
	bridge := NewBridge()

	// Test registering a provider
	provider := NewOpenAIProvider()
	err := bridge.RegisterProvider(provider)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check default provider was set
	if bridge.defaultProvider != "openai" {
		t.Errorf("expected default provider 'openai', got '%s'", bridge.defaultProvider)
	}

	// Test registering nil provider
	err = bridge.RegisterProvider(nil)
	if err == nil {
		t.Error("expected error when registering nil provider")
	}
}

func TestBridge_SetDefaultProvider(t *testing.T) {
	bridge := NewBridge()

	// Register providers
	bridge.RegisterProvider(NewOpenAIProvider())
	bridge.RegisterProvider(NewAnthropicProvider())

	// Test setting default provider
	err := bridge.SetDefaultProvider("anthropic")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if bridge.defaultProvider != "anthropic" {
		t.Errorf("expected default provider 'anthropic', got '%s'", bridge.defaultProvider)
	}

	// Test setting non-existent provider
	err = bridge.SetDefaultProvider("nonexistent")
	if err == nil {
		t.Error("expected error when setting non-existent default provider")
	}
}

func TestBridge_GetProvider(t *testing.T) {
	bridge := NewBridge()

	// Register a provider
	openai := NewOpenAIProvider()
	bridge.RegisterProvider(openai)

	// Test getting provider
	provider, err := bridge.GetProvider("openai")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if provider == nil {
		t.Error("expected non-nil provider")
	}
	if provider.Name() != "openai" {
		t.Errorf("expected provider name 'openai', got '%s'", provider.Name())
	}

	// Test getting non-existent provider
	_, err = bridge.GetProvider("nonexistent")
	if err == nil {
		t.Error("expected error when getting non-existent provider")
	}
}

func TestBridge_ListProviders(t *testing.T) {
	bridge := NewBridge()

	// Register providers
	bridge.RegisterProvider(NewOpenAIProvider())
	bridge.RegisterProvider(NewAnthropicProvider())
	bridge.RegisterProvider(NewFirmwareProvider())

	// Test listing providers
	providers := bridge.ListProviders()
	if len(providers) != 3 {
		t.Errorf("expected 3 providers, got %d", len(providers))
	}

	// Check that all providers are in the list
	providerMap := make(map[string]bool)
	for _, name := range providers {
		providerMap[name] = true
	}

	if !providerMap["openai"] {
		t.Error("expected 'openai' in provider list")
	}
	if !providerMap["anthropic"] {
		t.Error("expected 'anthropic' in provider list")
	}
	if !providerMap["firmware"] {
		t.Error("expected 'firmware' in provider list")
	}
}

func TestBridge_ValidateModel(t *testing.T) {
	bridge := NewBridge()

	// Register and authenticate provider
	provider := NewOpenAIProvider()
	provider.Authenticate("test-key")
	bridge.RegisterProvider(provider)

	// Test with non-existent provider
	err := bridge.ValidateModel("nonexistent", "gpt-4")
	if err == nil {
		t.Error("expected error when validating model with non-existent provider")
	}

	// Test with unauthenticated provider
	unauthProvider := NewAnthropicProvider()
	bridge.RegisterProvider(unauthProvider)
	err = bridge.ValidateModel("anthropic", "claude-3-opus")
	if err == nil {
		t.Error("expected error when validating model with unauthenticated provider")
	}
}

func TestBridge_Call(t *testing.T) {
	bridge := NewBridge()

	// Test with non-existent provider
	_, err := bridge.Call(context.TODO(), "nonexistent", "model", "prompt")
	if err == nil {
		t.Error("expected error when calling non-existent provider")
	}

	// Test with unauthenticated provider
	provider := NewOpenAIProvider()
	bridge.RegisterProvider(provider)
	_, err = bridge.Call(context.TODO(), "openai", "gpt-4", "Hello")
	if err == nil {
		t.Error("expected error when calling unauthenticated provider")
	}
}

func TestBridge_Stream(t *testing.T) {
	bridge := NewBridge()

	// Test with non-existent provider
	_, err := bridge.Stream(context.TODO(), "nonexistent", "model", "prompt")
	if err == nil {
		t.Error("expected error when streaming from non-existent provider")
	}

	// Test with unauthenticated provider
	provider := NewOpenAIProvider()
	bridge.RegisterProvider(provider)
	_, err = bridge.Stream(context.TODO(), "openai", "gpt-4", "Hello")
	if err == nil {
		t.Error("expected error when streaming from unauthenticated provider")
	}
}

func TestBridge_GetRateLimitInfo(t *testing.T) {
	bridge := NewBridge()

	// Test with non-existent provider
	_, err := bridge.GetRateLimitInfo("nonexistent")
	if err == nil {
		t.Error("expected error when getting rate limit info for non-existent provider")
	}

	// Test with unauthenticated provider
	provider := NewOpenAIProvider()
	bridge.RegisterProvider(provider)
	_, err = bridge.GetRateLimitInfo("openai")
	if err == nil {
		t.Error("expected error when getting rate limit info for unauthenticated provider")
	}
}

func TestBridge_GetQuotaInfo(t *testing.T) {
	bridge := NewBridge()

	// Test with non-existent provider
	_, err := bridge.GetQuotaInfo("nonexistent")
	if err == nil {
		t.Error("expected error when getting quota info for non-existent provider")
	}

	// Test with unauthenticated provider
	provider := NewOpenAIProvider()
	bridge.RegisterProvider(provider)
	_, err = bridge.GetQuotaInfo("openai")
	if err == nil {
		t.Error("expected error when getting quota info for unauthenticated provider")
	}
}

func TestBridge_RateLimitCache(t *testing.T) {
	bridge := NewBridge()
	bridge.cacheExpiry = 100 * time.Millisecond // Short expiry for testing

	// Manually set cache
	bridge.cacheMutex.Lock()
	bridge.rateLimitCache["test-provider"] = &RateLimitInfo{
		RequestsRemaining: 100,
		RequestsLimit:     1000,
		ResetAt:           time.Now(),
	}
	bridge.cacheMutex.Unlock()

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Cache should be expired now
	bridge.cacheMutex.RLock()
	cached := bridge.rateLimitCache["test-provider"]
	bridge.cacheMutex.RUnlock()

	// Even though the cache exists, it should be considered expired
	// when accessed through GetRateLimitInfo
	if cached == nil {
		t.Error("expected cached rate limit info")
	}
}

func TestBridge_QuotaCache(t *testing.T) {
	bridge := NewBridge()
	bridge.cacheExpiry = 100 * time.Millisecond // Short expiry for testing

	// Manually set cache
	bridge.cacheMutex.Lock()
	bridge.quotaCache["test-provider"] = &QuotaInfo{
		TokensRemaining: 50000,
		TokensLimit:     100000,
		ResetAt:         time.Now(),
	}
	bridge.cacheMutex.Unlock()

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Cache should be expired now
	bridge.cacheMutex.RLock()
	cached := bridge.quotaCache["test-provider"]
	bridge.cacheMutex.RUnlock()

	// Even though the cache exists, it should be considered expired
	// when accessed through GetQuotaInfo
	if cached == nil {
		t.Error("expected cached quota info")
	}
}

func TestBridge_RefreshRateLimits(t *testing.T) {
	bridge := NewBridge()

	// Register providers (without authentication, so refresh will skip them)
	bridge.RegisterProvider(NewOpenAIProvider())
	bridge.RegisterProvider(NewAnthropicProvider())

	// Refresh rate limits (should not error even with unauthenticated providers)
	err := bridge.RefreshRateLimits()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBridge_RefreshQuotas(t *testing.T) {
	bridge := NewBridge()

	// Register providers (without authentication, so refresh will skip them)
	bridge.RegisterProvider(NewOpenAIProvider())
	bridge.RegisterProvider(NewAnthropicProvider())

	// Refresh quotas (should not error even with unauthenticated providers)
	err := bridge.RefreshQuotas()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBridge_GetAllRateLimits(t *testing.T) {
	bridge := NewBridge()

	// Register providers
	openai := NewOpenAIProvider()
	openai.Authenticate("test-key")
	bridge.RegisterProvider(openai)

	anthropic := NewAnthropicProvider()
	anthropic.Authenticate("test-key")
	bridge.RegisterProvider(anthropic)

	// Manually set some cached data with future reset time
	bridge.cacheMutex.Lock()
	bridge.rateLimitCache["openai"] = &RateLimitInfo{
		RequestsRemaining: 100,
		RequestsLimit:     1000,
		ResetAt:           time.Now().Add(time.Hour), // Future time so cache doesn't expire
	}
	bridge.cacheMutex.Unlock()

	// Get all rate limits (will use cache for openai)
	limits := bridge.GetAllRateLimits()

	// Should have at least the cached openai limit
	if len(limits) == 0 {
		t.Error("expected at least one rate limit")
	}
}

func TestBridge_GetAllQuotas(t *testing.T) {
	bridge := NewBridge()

	// Register providers
	openai := NewOpenAIProvider()
	openai.Authenticate("test-key")
	bridge.RegisterProvider(openai)

	anthropic := NewAnthropicProvider()
	anthropic.Authenticate("test-key")
	bridge.RegisterProvider(anthropic)

	// Manually set some cached data with future reset time
	bridge.cacheMutex.Lock()
	bridge.quotaCache["openai"] = &QuotaInfo{
		TokensRemaining: 50000,
		TokensLimit:     100000,
		ResetAt:         time.Now().Add(time.Hour), // Future time so cache doesn't expire
	}
	bridge.cacheMutex.Unlock()

	// Get all quotas (will use cache for openai)
	quotas := bridge.GetAllQuotas()

	// Should have at least the cached openai quota
	if len(quotas) == 0 {
		t.Error("expected at least one quota")
	}
}

func TestBridge_SupportsCodingPlan(t *testing.T) {
	bridge := NewBridge()

	// Register providers
	bridge.RegisterProvider(NewZAIProvider())
	bridge.RegisterProvider(NewKimiProvider())
	bridge.RegisterProvider(NewOpenAIProvider())

	// Test Z.ai (should support coding plan)
	supports, err := bridge.SupportsCodingPlan("zai")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !supports {
		t.Error("expected Z.ai to support coding plan")
	}

	// Test Kimi (should support coding plan)
	supports, err = bridge.SupportsCodingPlan("kimi")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !supports {
		t.Error("expected Kimi to support coding plan")
	}

	// Test OpenAI (should not support coding plan)
	supports, err = bridge.SupportsCodingPlan("openai")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if supports {
		t.Error("expected OpenAI not to support coding plan")
	}

	// Test non-existent provider
	_, err = bridge.SupportsCodingPlan("nonexistent")
	if err == nil {
		t.Error("expected error when checking coding plan support for non-existent provider")
	}
}

func TestBridge_ListModels(t *testing.T) {
	bridge := NewBridge()

	// Register and authenticate providers
	openai := NewOpenAIProvider()
	openai.Authenticate("test-key")
	bridge.RegisterProvider(openai)

	anthropic := NewAnthropicProvider()
	anthropic.Authenticate("test-key")
	bridge.RegisterProvider(anthropic)

	// List models from all providers
	models, err := bridge.ListModels()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should have models from both providers
	if len(models) == 0 {
		t.Error("expected at least one model")
	}
}

func TestBridge_ListModelsByProvider(t *testing.T) {
	bridge := NewBridge()

	// Register and authenticate provider
	anthropic := NewAnthropicProvider()
	anthropic.Authenticate("test-key")
	bridge.RegisterProvider(anthropic)

	// List models from specific provider
	models, err := bridge.ListModelsByProvider("anthropic")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(models) == 0 {
		t.Error("expected at least one model")
	}

	// All models should be from anthropic
	for _, model := range models {
		if model.Provider != "anthropic" {
			t.Errorf("expected provider 'anthropic', got '%s'", model.Provider)
		}
	}

	// Test with non-existent provider
	_, err = bridge.ListModelsByProvider("nonexistent")
	if err == nil {
		t.Error("expected error when listing models from non-existent provider")
	}
}
