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

// KimiProvider implements the Provider interface for Kimi (Moonshot AI)
type KimiProvider struct {
	*BaseProvider
	baseURL    string
	httpClient *http.Client
}

// NewKimiProvider creates a new Kimi provider
func NewKimiProvider() *KimiProvider {
	return &KimiProvider{
		BaseProvider: NewBaseProvider("kimi"),
		baseURL:      "https://api.moonshot.cn/v1",
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// kimiRequest represents a request to Kimi API
type kimiRequest struct {
	Model        string        `json:"model"`
	Messages     []kimiMessage `json:"messages"`
	Stream       bool          `json:"stream,omitempty"`
	Temperature  float64       `json:"temperature,omitempty"`
	MaxTokens    int           `json:"max_tokens,omitempty"`
	CodingPlan   bool          `json:"coding_plan,omitempty"`
	ProjectFiles []string      `json:"project_files,omitempty"`
}

type kimiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// kimiResponse represents a response from Kimi API
type kimiResponse struct {
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
		Tasks []struct {
			Description string   `json:"description"`
			Files       []string `json:"files"`
			Actions     []string `json:"actions"`
		} `json:"tasks"`
	} `json:"coding_plan,omitempty"`
}

// kimiStreamChunk represents a streaming response chunk
type kimiStreamChunk struct {
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

// ListModels returns the list of available models from Kimi
func (k *KimiProvider) ListModels() ([]Model, error) {
	if !k.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	// Kimi has a limited set of known models
	models := []Model{
		{
			Provider:     "kimi",
			Name:         "moonshot-v1-8k",
			DisplayName:  "Moonshot v1 8K",
			Capabilities: []string{"text", "code", "streaming", "coding-plan"},
			PriceInput:   0.012, // 12 CNY per 1M tokens (approx $1.7)
			PriceOutput:  0.012,
		},
		{
			Provider:     "kimi",
			Name:         "moonshot-v1-32k",
			DisplayName:  "Moonshot v1 32K",
			Capabilities: []string{"text", "code", "streaming", "coding-plan"},
			PriceInput:   0.024, // 24 CNY per 1M tokens (approx $3.4)
			PriceOutput:  0.024,
		},
		{
			Provider:     "kimi",
			Name:         "moonshot-v1-128k",
			DisplayName:  "Moonshot v1 128K",
			Capabilities: []string{"text", "code", "streaming", "coding-plan"},
			PriceInput:   0.060, // 60 CNY per 1M tokens (approx $8.5)
			PriceOutput:  0.060,
		},
	}

	return models, nil
}

// Call makes a non-streaming API call to Kimi
func (k *KimiProvider) Call(ctx context.Context, model string, prompt string) (*Response, error) {
	if !k.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	startTime := time.Now()
	var response *Response
	err := k.RetryWithBackoff(func() error {
		req := kimiRequest{
			Model: model,
			Messages: []kimiMessage{
				{
					Role:    "user",
					Content: prompt,
				},
			},
			Temperature: 0.7,
			MaxTokens:   4096,
		}

		jsonData, err := json.Marshal(req)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		httpReq, err := http.NewRequest("POST", k.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+k.GetAPIKey())

		resp, err := k.httpClient.Do(httpReq)
		if err != nil {
			return fmt.Errorf("failed to make request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}

		var kimiResp kimiResponse
		if err := json.NewDecoder(resp.Body).Decode(&kimiResp); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		// Extract content
		var content string
		if len(kimiResp.Choices) > 0 {
			content = kimiResp.Choices[0].Message.Content
		}

		// Extract rate limit info from headers
		rateLimitInfo := ExtractRateLimitInfo(resp)
		rateLimitRemaining := rateLimitInfo.RequestsRemaining

		duration := time.Since(startTime)

		// Log successful API call with metadata
		k.GetLogger().Info("API call completed",
			"provider", "kimi",
			"model", model,
			"tokens_input", kimiResp.Usage.PromptTokens,
			"tokens_output", kimiResp.Usage.CompletionTokens,
			"tokens_total", kimiResp.Usage.TotalTokens,
			"duration_ms", duration.Milliseconds(),
			"rate_limit_remaining", rateLimitRemaining,
		)

		response = &Response{
			Content:            content,
			TokensInput:        kimiResp.Usage.PromptTokens,
			TokensOutput:       kimiResp.Usage.CompletionTokens,
			Model:              kimiResp.Model,
			Provider:           "kimi",
			Timestamp:          time.Now(),
			RateLimitRemaining: rateLimitRemaining,
		}

		return nil
	})

	if err != nil {
		k.GetLogger().Error("API call failed",
			"provider", "kimi",
			"model", model,
			"error", err.Error(),
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
	}

	return response, err
}

// Stream makes a streaming API call to Kimi
func (k *KimiProvider) Stream(ctx context.Context, model string, prompt string) (<-chan string, error) {
	if !k.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	req := kimiRequest{
		Model: model,
		Messages: []kimiMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream:      true,
		Temperature: 0.7,
		MaxTokens:   4096,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", k.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+k.GetAPIKey())

	resp, err := k.httpClient.Do(httpReq)
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

			var chunk kimiStreamChunk
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

// GetRateLimitInfo returns rate limit information from Kimi
func (k *KimiProvider) GetRateLimitInfo() (*RateLimitInfo, error) {
	if !k.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	// Make a minimal request to get rate limit headers
	req := kimiRequest{
		Model: "moonshot-v1-8k",
		Messages: []kimiMessage{
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

	httpReq, err := http.NewRequest("POST", k.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+k.GetAPIKey())

	resp, err := k.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Extract rate limit info from headers
	info := ExtractRateLimitInfo(resp)
	return info, nil
}

// GetQuotaInfo returns quota information from Kimi
func (k *KimiProvider) GetQuotaInfo() (*QuotaInfo, error) {
	if !k.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	// Make a minimal request to get quota headers
	req := kimiRequest{
		Model: "moonshot-v1-8k",
		Messages: []kimiMessage{
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

	httpReq, err := http.NewRequest("POST", k.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+k.GetAPIKey())

	resp, err := k.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Extract quota info from headers
	info := ExtractQuotaInfo(resp)
	return info, nil
}

// SupportsCodingPlan returns true since Kimi supports coding plans
func (k *KimiProvider) SupportsCodingPlan() bool {
	return true
}
