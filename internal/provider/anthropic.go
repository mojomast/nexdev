package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AnthropicProvider implements the Provider interface for Anthropic
type AnthropicProvider struct {
	*BaseProvider
	baseURL    string
	httpClient *http.Client
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider() *AnthropicProvider {
	return &AnthropicProvider{
		BaseProvider: NewBaseProvider("anthropic"),
		baseURL:      "https://api.anthropic.com/v1",
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// anthropicRequest represents a request to Anthropic API
type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	Stream      bool               `json:"stream,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse represents a response from Anthropic API
type anthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// anthropicStreamChunk represents a streaming response chunk
type anthropicStreamChunk struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
	ContentBlock struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content_block"`
	Message anthropicResponse `json:"message"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// ListModels returns the list of available models from Anthropic
func (a *AnthropicProvider) ListModels() ([]Model, error) {
	if !a.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	// Anthropic doesn't have a models endpoint, so we return known models
	models := []Model{
		{
			Provider:     "anthropic",
			Name:         "claude-3-5-sonnet-20241022",
			DisplayName:  "Claude 3.5 Sonnet (Latest)",
			Capabilities: []string{"text", "vision", "streaming"},
			PriceInput:   3.0,  // $3 per 1M tokens
			PriceOutput:  15.0, // $15 per 1M tokens
		},
		{
			Provider:     "anthropic",
			Name:         "claude-3-5-haiku-20241022",
			DisplayName:  "Claude 3.5 Haiku (Latest)",
			Capabilities: []string{"text", "vision", "streaming"},
			PriceInput:   1.0, // $1 per 1M tokens
			PriceOutput:  5.0, // $5 per 1M tokens
		},
		{
			Provider:     "anthropic",
			Name:         "claude-3-opus-20240229",
			DisplayName:  "Claude 3 Opus",
			Capabilities: []string{"text", "vision", "streaming"},
			PriceInput:   15.0, // $15 per 1M tokens
			PriceOutput:  75.0, // $75 per 1M tokens
		},
		{
			Provider:     "anthropic",
			Name:         "claude-3-sonnet-20240229",
			DisplayName:  "Claude 3 Sonnet",
			Capabilities: []string{"text", "vision", "streaming"},
			PriceInput:   3.0,  // $3 per 1M tokens
			PriceOutput:  15.0, // $15 per 1M tokens
		},
		{
			Provider:     "anthropic",
			Name:         "claude-3-haiku-20240307",
			DisplayName:  "Claude 3 Haiku",
			Capabilities: []string{"text", "vision", "streaming"},
			PriceInput:   0.25, // $0.25 per 1M tokens
			PriceOutput:  1.25, // $1.25 per 1M tokens
		},
	}

	return models, nil
}

// Call makes a non-streaming API call to Anthropic
func (a *AnthropicProvider) Call(ctx context.Context, model string, prompt string) (*Response, error) {
	if !a.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	startTime := time.Now()
	var response *Response
	err := a.RetryWithBackoff(func() error {
		req := anthropicRequest{
			Model: model,
			Messages: []anthropicMessage{
				{
					Role:    "user",
					Content: prompt,
				},
			},
			MaxTokens:   4096,
			Temperature: 0.7,
		}

		jsonData, err := json.Marshal(req)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/messages", bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("x-api-key", a.GetAPIKey())
		httpReq.Header.Set("anthropic-version", "2023-06-01")

		resp, err := a.httpClient.Do(httpReq)
		if err != nil {
			return fmt.Errorf("failed to make request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}

		var anthropicResp anthropicResponse
		if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		// Extract text content
		var content string
		for _, c := range anthropicResp.Content {
			if c.Type == "text" {
				content += c.Text
			}
		}

		// Extract rate limit info from headers
		rateLimitInfo := ExtractRateLimitInfo(resp)
		rateLimitRemaining := rateLimitInfo.RequestsRemaining

		duration := time.Since(startTime)

		// Log successful API call with metadata
		a.GetLogger().Info("API call completed",
			"provider", "anthropic",
			"model", model,
			"tokens_input", anthropicResp.Usage.InputTokens,
			"tokens_output", anthropicResp.Usage.OutputTokens,
			"tokens_total", anthropicResp.Usage.InputTokens+anthropicResp.Usage.OutputTokens,
			"duration_ms", duration.Milliseconds(),
			"rate_limit_remaining", rateLimitRemaining,
		)

		response = &Response{
			Content:            content,
			TokensInput:        anthropicResp.Usage.InputTokens,
			TokensOutput:       anthropicResp.Usage.OutputTokens,
			Model:              anthropicResp.Model,
			Provider:           "anthropic",
			Timestamp:          time.Now(),
			RateLimitRemaining: rateLimitRemaining,
		}

		return nil
	})

	if err != nil {
		a.GetLogger().Error("API call failed",
			"provider", "anthropic",
			"model", model,
			"error", err.Error(),
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
	}

	return response, err
}

// Stream makes a streaming API call to Anthropic
func (a *AnthropicProvider) Stream(ctx context.Context, model string, prompt string) (<-chan string, error) {
	if !a.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	req := anthropicRequest{
		Model: model,
		Messages: []anthropicMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream:      true,
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.GetAPIKey())
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	ch := make(chan string, 100)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var chunk anthropicStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			// Handle different event types
			switch chunk.Type {
			case "content_block_delta":
				if chunk.Delta.Type == "text_delta" {
					ch <- chunk.Delta.Text
				}
			case "content_block_start":
				if chunk.ContentBlock.Type == "text" && chunk.ContentBlock.Text != "" {
					ch <- chunk.ContentBlock.Text
				}
			}
		}
	}()

	return ch, nil
}

// GetRateLimitInfo returns rate limit information from Anthropic
func (a *AnthropicProvider) GetRateLimitInfo() (*RateLimitInfo, error) {
	if !a.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	// Make a minimal request to get rate limit headers
	req := anthropicRequest{
		Model: "claude-3-haiku-20240307",
		Messages: []anthropicMessage{
			{
				Role:    "user",
				Content: "Hi",
			},
		},
		MaxTokens: 10,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", a.baseURL+"/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.GetAPIKey())
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Log the headers for debugging
	if a.GetLogger() != nil {
		a.GetLogger().Info("Rate limit headers received",
			"requests_remaining", resp.Header.Get("anthropic-ratelimit-requests-remaining"),
			"requests_limit", resp.Header.Get("anthropic-ratelimit-requests-limit"),
			"requests_reset", resp.Header.Get("anthropic-ratelimit-requests-reset"),
			"retry_after", resp.Header.Get("retry-after"),
		)
	}

	// Extract rate limit info from headers
	info := ExtractRateLimitInfo(resp)
	return info, nil
}

// GetQuotaInfo returns quota information from Anthropic
func (a *AnthropicProvider) GetQuotaInfo() (*QuotaInfo, error) {
	if !a.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	// Make a minimal request to get quota headers
	req := anthropicRequest{
		Model: "claude-3-haiku-20240307",
		Messages: []anthropicMessage{
			{
				Role:    "user",
				Content: "Hi",
			},
		},
		MaxTokens: 10,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", a.baseURL+"/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.GetAPIKey())
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Extract quota info from headers
	info := ExtractQuotaInfo(resp)

	return info, nil
}
