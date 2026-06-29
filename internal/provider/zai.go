package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ZAIProvider implements the Provider interface for Z.ai
type ZAIProvider struct {
	*BaseProvider
	baseURL    string
	httpClient *http.Client
}

// NewZAIProvider creates a new Z.ai provider
func NewZAIProvider() *ZAIProvider {
	return &ZAIProvider{
		BaseProvider: NewBaseProvider("zai"),
		baseURL:      "https://api.z.ai/api/paas/v4",
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

// zaiRequest represents a request to Z.ai API
type zaiRequest struct {
	Model          string       `json:"model"`
	Messages       []zaiMessage `json:"messages"`
	Stream         bool         `json:"stream,omitempty"`
	Temperature    float64      `json:"temperature,omitempty"`
	MaxTokens      int          `json:"max_tokens,omitempty"`
	CodingPlan     bool         `json:"coding_plan,omitempty"`
	ProjectContext string       `json:"project_context,omitempty"`
}

type zaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// zaiResponse represents a response from Z.ai API
type zaiResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	CodingPlan *struct {
		Steps []struct {
			Description string   `json:"description"`
			Files       []string `json:"files"`
			Commands    []string `json:"commands"`
		} `json:"steps"`
	} `json:"coding_plan,omitempty"`
}

// zaiStreamChunk represents a streaming response chunk
type zaiStreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// ListModels returns the list of available models from Z.ai
func (z *ZAIProvider) ListModels() ([]Model, error) {
	if !z.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	// Z.ai available models (updated with GLM-4.7 and GLM-4.6V)
	models := []Model{
		{
			Provider:     "z.ai",
			Name:         "glm-4.7",
			DisplayName:  "GLM-4.7",
			Capabilities: []string{"text", "code", "streaming", "coding-plan"},
			PriceInput:   0.0005,
			PriceOutput:  0.0015,
		},
		{
			Provider:     "z.ai",
			Name:         "glm-4.6",
			DisplayName:  "GLM-4.6",
			Capabilities: []string{"text", "code", "streaming", "coding-plan"},
			PriceInput:   0.0006,
			PriceOutput:  0.0022,
		},
		{
			Provider:     "z.ai",
			Name:         "glm-4.6v",
			DisplayName:  "GLM-4.6V (Multimodal)",
			Capabilities: []string{"text", "code", "streaming", "vision"},
			PriceInput:   0.0008,
			PriceOutput:  0.0028,
		},
	}

	return models, nil
}

// Call makes a non-streaming API call to Z.ai
func (z *ZAIProvider) Call(ctx context.Context, model string, prompt string) (*Response, error) {
	if !z.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	startTime := time.Now()
	var response *Response
	err := z.RetryWithBackoff(func() error {
		req := zaiRequest{
			Model: model,
			Messages: []zaiMessage{
				{
					Role:    "user",
					Content: prompt,
				},
			},
			Temperature: 0.7,
			MaxTokens:   8192,
		}

		jsonData, err := json.Marshal(req)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		httpReq, err := http.NewRequest("POST", z.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+z.GetAPIKey())

		resp, err := z.httpClient.Do(httpReq)
		if err != nil {
			return fmt.Errorf("failed to make request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}

		var zaiResp zaiResponse
		if err := json.NewDecoder(resp.Body).Decode(&zaiResp); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		// Extract content
		var content string
		if len(zaiResp.Choices) > 0 {
			content = zaiResp.Choices[0].Message.Content
		}

		// Extract rate limit info from headers
		rateLimitRemaining := 0
		if val := resp.Header.Get("X-RateLimit-Remaining"); val != "" {
			if parsed, err := strconv.Atoi(val); err == nil {
				rateLimitRemaining = parsed
			}
		}

		duration := time.Since(startTime)

		// Log successful API call with metadata
		z.GetLogger().Info("API call completed",
			"provider", "z.ai",
			"model", model,
			"tokens_input", zaiResp.Usage.PromptTokens,
			"tokens_output", zaiResp.Usage.CompletionTokens,
			"tokens_total", zaiResp.Usage.TotalTokens,
			"duration_ms", duration.Milliseconds(),
			"rate_limit_remaining", rateLimitRemaining,
		)

		response = &Response{
			Content:            content,
			TokensInput:        zaiResp.Usage.PromptTokens,
			TokensOutput:       zaiResp.Usage.CompletionTokens,
			Model:              zaiResp.Model,
			Provider:           "z.ai",
			Timestamp:          time.Now(),
			RateLimitRemaining: rateLimitRemaining,
		}

		return nil
	})

	if err != nil {
		z.GetLogger().Error("API call failed",
			"provider", "z.ai",
			"model", model,
			"error", err.Error(),
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
	}

	return response, err
}

// Stream makes a streaming API call to Z.ai
func (z *ZAIProvider) Stream(ctx context.Context, model string, prompt string) (<-chan string, error) {
	if !z.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	req := zaiRequest{
		Model: model,
		Messages: []zaiMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream:      true,
		Temperature: 0.7,
		MaxTokens:   8192,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", z.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+z.GetAPIKey())

	resp, err := z.httpClient.Do(httpReq)
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

			var chunk zaiStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				ch <- chunk.Choices[0].Delta.Content
			}
		}
	}()

	return ch, nil
}

// GetRateLimitInfo returns rate limit information from Z.ai
func (z *ZAIProvider) GetRateLimitInfo() (*RateLimitInfo, error) {
	if !z.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	// Make a minimal request to get rate limit headers
	req := zaiRequest{
		Model: "glm-4.7",
		Messages: []zaiMessage{
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

	httpReq, err := http.NewRequest("POST", z.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+z.GetAPIKey())

	resp, err := z.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Extract rate limit info from headers
	info := &RateLimitInfo{}

	if val := resp.Header.Get("X-RateLimit-Remaining"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			info.RequestsRemaining = parsed
		}
	}

	if val := resp.Header.Get("X-RateLimit-Limit"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			info.RequestsLimit = parsed
		}
	}

	if val := resp.Header.Get("X-RateLimit-Reset"); val != "" {
		if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
			info.ResetAt = time.Unix(parsed, 0)
		}
	}

	if val := resp.Header.Get("Retry-After"); val != "" {
		if seconds, err := strconv.Atoi(val); err == nil {
			info.RetryAfter = time.Duration(seconds) * time.Second
		}
	}

	return info, nil
}

// GetQuotaInfo returns quota information from Z.ai
func (z *ZAIProvider) GetQuotaInfo() (*QuotaInfo, error) {
	if !z.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	// Make a minimal request to get quota headers
	req := zaiRequest{
		Model: "glm-4.7",
		Messages: []zaiMessage{
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

	httpReq, err := http.NewRequest("POST", z.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+z.GetAPIKey())

	resp, err := z.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Extract quota info from headers
	info := &QuotaInfo{}

	if val := resp.Header.Get("X-Quota-Tokens-Remaining"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			info.TokensRemaining = parsed
		}
	}

	if val := resp.Header.Get("X-Quota-Tokens-Limit"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			info.TokensLimit = parsed
		}
	}

	return info, nil
}

// SupportsCodingPlan returns true since Z.ai supports coding plans
func (z *ZAIProvider) SupportsCodingPlan() bool {
	return true
}
