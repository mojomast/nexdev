package provider

import "time"

// NewOpenRouterProvider creates an OpenRouter provider (OpenAI-compatible API).
func NewOpenRouterProvider() *OpenAIProvider {
	p := NewOpenAIProviderWithName("openrouter")
	p.baseURL = "https://openrouter.ai/api/v1"
	p.httpClient.Timeout = 120 * time.Second
	return p
}

// NewGroqProvider creates a Groq provider (OpenAI-compatible API).
func NewGroqProvider() *OpenAIProvider {
	p := NewOpenAIProviderWithName("groq")
	p.baseURL = "https://api.groq.com/openai/v1"
	p.httpClient.Timeout = 60 * time.Second
	return p
}

// NewTogetherProvider creates a Together provider (OpenAI-compatible API).
func NewTogetherProvider() *OpenAIProvider {
	p := NewOpenAIProviderWithName("together")
	p.baseURL = "https://api.together.xyz/v1"
	p.httpClient.Timeout = 120 * time.Second
	return p
}

// NewDeepInfraProvider creates a DeepInfra provider (OpenAI-compatible API).
func NewDeepInfraProvider() *OpenAIProvider {
	p := NewOpenAIProviderWithName("deepinfra")
	p.baseURL = "https://api.deepinfra.com/v1/openai"
	p.httpClient.Timeout = 120 * time.Second
	return p
}

// NewFireworksProvider creates a Fireworks provider (OpenAI-compatible API).
func NewFireworksProvider() *OpenAIProvider {
	p := NewOpenAIProviderWithName("fireworks")
	p.baseURL = "https://api.fireworks.ai/inference/v1"
	p.httpClient.Timeout = 120 * time.Second
	return p
}

// NewPerplexityProvider creates a Perplexity provider (OpenAI-compatible API).
func NewPerplexityProvider() *OpenAIProvider {
	p := NewOpenAIProviderWithName("perplexity")
	p.baseURL = "https://api.perplexity.ai"
	p.httpClient.Timeout = 120 * time.Second
	return p
}

// NewMistralProvider creates a Mistral provider (OpenAI-compatible API).
func NewMistralProvider() *OpenAIProvider {
	p := NewOpenAIProviderWithName("mistral")
	p.baseURL = "https://api.mistral.ai/v1"
	p.httpClient.Timeout = 120 * time.Second
	return p
}
