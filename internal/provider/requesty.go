package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// RequestyProvider implements the Provider interface for Requesty.ai
type RequestyProvider struct {
	*BaseProvider
	baseURL    string
	httpClient *http.Client
}

// NewRequestyProvider creates a new Requesty.ai provider
func NewRequestyProvider() *RequestyProvider {
	return &RequestyProvider{
		BaseProvider: NewBaseProvider("requesty"),
		baseURL:      "https://router.requesty.ai/v1",
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

// requestyRequest represents a request to Requesty.ai API
type requestyRequest struct {
	Model       string            `json:"model"`
	Messages    []requestyMessage `json:"messages"`
	Stream      bool              `json:"stream,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
}

type requestyMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// requestyResponse represents a response from Requesty.ai API
type requestyResponse struct {
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
}

// requestyModelsResponse represents the models list response
type requestyModelsResponse struct {
	Data []struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

// ListModels returns the list of available models from Requesty.ai
func (r *RequestyProvider) ListModels() ([]Model, error) {
	if !r.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	req, err := http.NewRequest("GET", r.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+r.GetAPIKey())
	req.Header.Set("Content-Type", "application/json")

	var resp *http.Response
	err = r.RetryWithBackoff(func() error {
		var reqErr error
		resp, reqErr = r.httpClient.Do(req)
		if reqErr != nil {
			return reqErr
		}
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			return fmt.Errorf("server error: %d", resp.StatusCode)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var modelsResp requestyModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]Model, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		models = append(models, Model{
			Provider:    r.Name(),
			Name:        m.ID,
			DisplayName: m.ID,
			// Pricing would need to be configured based on actual Requesty.ai pricing
			PriceInput:  0.0,
			PriceOutput: 0.0,
		})
	}

	return models, nil
}

// Call makes a synchronous API call to Requesty.ai
func (r *RequestyProvider) Call(ctx context.Context, model string, prompt string) (*Response, error) {
	if !r.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	startTime := time.Now()

	reqBody := requestyRequest{
		Model: model,
		Messages: []requestyMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", r.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+r.GetAPIKey())
	req.Header.Set("Content-Type", "application/json")

	var resp *http.Response
	err = r.RetryWithBackoff(func() error {
		var reqErr error
		resp, reqErr = r.httpClient.Do(req)
		if reqErr != nil {
			return reqErr
		}
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			return fmt.Errorf("server error: %d", resp.StatusCode)
		}
		return nil
	})

	if err != nil {
		r.GetLogger().Error("API call failed",
			"provider", r.Name(),
			"model", model,
			"error", err.Error(),
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		r.GetLogger().Error("API error response",
			"provider", r.Name(),
			"model", model,
			"status_code", resp.StatusCode,
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// Extract rate limit info from headers
	rateLimitRemaining := 0
	if val := resp.Header.Get("X-RateLimit-Remaining"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			rateLimitRemaining = parsed
		}
	}

	quotaRemaining := 0
	if val := resp.Header.Get("X-Quota-Remaining"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			quotaRemaining = parsed
		}
	}

	var requestyResp requestyResponse
	if err := json.NewDecoder(resp.Body).Decode(&requestyResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(requestyResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	duration := time.Since(startTime)

	// Log successful API call with metadata
	r.GetLogger().Info("API call completed",
		"provider", r.Name(),
		"model", model,
		"tokens_input", requestyResp.Usage.PromptTokens,
		"tokens_output", requestyResp.Usage.CompletionTokens,
		"tokens_total", requestyResp.Usage.TotalTokens,
		"duration_ms", duration.Milliseconds(),
		"rate_limit_remaining", rateLimitRemaining,
		"quota_remaining", quotaRemaining,
	)

	return &Response{
		Content:            requestyResp.Choices[0].Message.Content,
		TokensInput:        requestyResp.Usage.PromptTokens,
		TokensOutput:       requestyResp.Usage.CompletionTokens,
		Model:              model,
		Provider:           r.Name(),
		Timestamp:          time.Now(),
		RateLimitRemaining: rateLimitRemaining,
		QuotaRemaining:     quotaRemaining,
	}, nil
}

// Stream makes a streaming API call to Requesty.ai
func (r *RequestyProvider) Stream(ctx context.Context, model string, prompt string) (<-chan string, error) {
	if !r.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	reqBody := requestyRequest{
		Model: model,
		Messages: []requestyMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream: true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", r.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+r.GetAPIKey())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan string, 10)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)
		for {
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}

			if err := decoder.Decode(&chunk); err != nil {
				if err != io.EOF {
					ch <- fmt.Sprintf("Error: %v", err)
				}
				return
			}

			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				ch <- chunk.Choices[0].Delta.Content
			}
		}
	}()

	return ch, nil
}

// GetRateLimitInfo returns rate limit information for Requesty.ai
func (r *RequestyProvider) GetRateLimitInfo() (*RateLimitInfo, error) {
	// Requesty.ai rate limits would be extracted from response headers
	// This is a placeholder implementation
	return &RateLimitInfo{
		RequestsRemaining: 0,
		RequestsLimit:     0,
		ResetAt:           time.Time{},
		RetryAfter:        0,
	}, nil
}

// GetQuotaInfo returns quota information for Requesty.ai
func (r *RequestyProvider) GetQuotaInfo() (*QuotaInfo, error) {
	// Requesty.ai quota information would be extracted from API
	// This is a placeholder implementation
	return &QuotaInfo{
		TokensRemaining: 0,
		TokensLimit:     0,
		CostRemaining:   0,
		CostLimit:       0,
		ResetAt:         time.Time{},
	}, nil
}
