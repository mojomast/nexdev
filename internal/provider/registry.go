package provider

import (
	"fmt"
	"sort"
)

// ProviderFactory is a function that creates a new Provider
type ProviderFactory func() Provider

// Registry maintains a list of available providers
var Registry = map[string]ProviderFactory{
	"anthropic":    func() Provider { return NewAnthropicProvider() },
	"deepinfra":    func() Provider { return NewDeepInfraProvider() },
	"firmware":     func() Provider { return NewFirmwareProvider() },
	"fireworks":    func() Provider { return NewFireworksProvider() },
	"groq":         func() Provider { return NewGroqProvider() },
	"kimi":         func() Provider { return NewKimiProvider() },
	"mistral":      func() Provider { return NewMistralProvider() },
	"ollama":       func() Provider { return NewOllamaProvider("") }, // Default URL
	"openai":       func() Provider { return NewOpenAIProvider() },
	"openai-codex": func() Provider { return NewOpenAIProviderWithName("openai-codex") },
	"openrouter":   func() Provider { return NewOpenRouterProvider() },
	"opencode":     func() Provider { return NewOpenCodeProvider() },
	"perplexity":   func() Provider { return NewPerplexityProvider() },
	"requesty":     func() Provider { return NewRequestyProvider() },
	"together":     func() Provider { return NewTogetherProvider() },
	"zai":          func() Provider { return NewZAIProvider() },
}

// GetProviderNames returns a list of all registered provider names sorted alphabetically
func GetProviderNames() []string {
	names := make([]string, 0, len(Registry))
	for name := range Registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// CreateProvider creates a new instance of the specified provider
func CreateProvider(name string) (Provider, error) {
	factory, ok := Registry[name]
	if !ok {
		return nil, fmt.Errorf("provider factory not found for: %s", name)
	}
	return factory(), nil
}
