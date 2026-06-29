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
	"sync"
	"time"

	"github.com/mojomast/nexdev/internal/state"
)

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	*BaseProvider
	baseURL    string
	httpClient *http.Client

	// store is an optional state store for persisting rate limit / quota info.
	store *state.Store

	// In-memory cache of the most recent rate-limit / quota data extracted
	// from HTTP response headers.  Protected by mu.
	mu            sync.Mutex
	lastRateLimit *RateLimitInfo
	lastQuotaInfo *QuotaInfo
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider() *OpenAIProvider {
	return NewOpenAIProviderWithName("openai")
}

// NewOpenAIProviderWithName creates a new OpenAI-compatible provider with a custom name.
func NewOpenAIProviderWithName(name string) *OpenAIProvider {
	if name == "" {
		name = "openai"
	}

	return &OpenAIProvider{
		BaseProvider: NewBaseProvider(name),
		baseURL:      "https://api.openai.com/v1",
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// SetStore assigns a state store for persisting rate-limit and quota data.
func (o *OpenAIProvider) SetStore(store *state.Store) {
	o.store = store
}

// openAIRequest represents a request to OpenAI API
type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Stream      bool            `json:"stream,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIResponse represents a response from OpenAI API
type openAIResponse struct {
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

// openAIModelsResponse represents the models list response
type openAIModelsResponse struct {
	Data []struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

// openAIStreamChunk represents a streaming response chunk
type openAIStreamChunk struct {
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

// ListModels returns the list of available models from OpenAI
func (o *OpenAIProvider) ListModels() ([]Model, error) {
	if !o.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	req, err := http.NewRequest("GET", o.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+o.GetAPIKey())
	req.Header.Set("Content-Type", "application/json")

	var resp *http.Response
	err = o.RetryWithBackoff(func() error {
		var reqErr error
		resp, reqErr = o.httpClient.Do(req)
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

	var modelsResp openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]Model, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		isOpenAIFamily := o.Name() == "openai" || o.Name() == "openai-codex"
		includeModel := !isOpenAIFamily || strings.Contains(m.ID, "gpt") || strings.Contains(m.ID, "codex") || strings.HasPrefix(m.ID, "o")
		if includeModel {
			model := Model{
				Provider:    o.Name(),
				Name:        m.ID,
				DisplayName: m.ID,
			}

			// Set pricing based on known models
			switch {
			case strings.HasPrefix(m.ID, "gpt-4"):
				model.PriceInput = 0.03  // $0.03 per 1K tokens
				model.PriceOutput = 0.06 // $0.06 per 1K tokens
			case strings.HasPrefix(m.ID, "gpt-3.5"):
				model.PriceInput = 0.0015 // $0.0015 per 1K tokens
				model.PriceOutput = 0.002 // $0.002 per 1K tokens
			default:
				// Default pricing for unknown models
				model.PriceInput = 0.0
				model.PriceOutput = 0.0
			}

			models = append(models, model)
		}
	}

	return models, nil
}

// Call makes a synchronous API call to OpenAI
func (o *OpenAIProvider) Call(ctx context.Context, model string, prompt string) (*Response, error) {
	if !o.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	startTime := time.Now()

	reqBody := openAIRequest{
		Model: model,
		Messages: []openAIMessage{
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

	var resp *http.Response
	err = o.RetryWithBackoff(func() error {
		// Create a new request for each retry attempt
		req, reqErr := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
		if reqErr != nil {
			return fmt.Errorf("failed to create request: %w", reqErr)
		}

		req.Header.Set("Authorization", "Bearer "+o.GetAPIKey())
		req.Header.Set("Content-Type", "application/json")

		var httpErr error
		resp, httpErr = o.httpClient.Do(req)
		if httpErr != nil {
			return httpErr
		}
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			return fmt.Errorf("server error: %d", resp.StatusCode)
		}
		return nil
	})

	if err != nil {
		o.GetLogger().Error("API call failed",
			"provider", o.Name(),
			"model", model,
			"error", err.Error(),
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		o.GetLogger().Error("API error response",
			"provider", o.Name(),
			"model", model,
			"status_code", resp.StatusCode,
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// Extract rate limit info from headers
	rateLimitInfo := ExtractRateLimitInfo(resp)
	rateLimitRemaining := rateLimitInfo.RequestsRemaining

	// Extract quota info from headers
	quotaInfo := ExtractQuotaInfo(resp)
	quotaRemaining := quotaInfo.TokensRemaining

	// Cache in memory
	o.mu.Lock()
	o.lastRateLimit = rateLimitInfo
	o.lastQuotaInfo = quotaInfo
	o.mu.Unlock()

	// Persist to store if available
	o.persistRateLimitAndQuota(rateLimitInfo, quotaInfo)

	var openAIResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	duration := time.Since(startTime)

	// Log successful API call with metadata
	o.GetLogger().Info("API call completed",
		"provider", o.Name(),
		"model", model,
		"tokens_input", openAIResp.Usage.PromptTokens,
		"tokens_output", openAIResp.Usage.CompletionTokens,
		"tokens_total", openAIResp.Usage.TotalTokens,
		"duration_ms", duration.Milliseconds(),
		"rate_limit_remaining", rateLimitRemaining,
		"quota_remaining", quotaRemaining,
	)

	return &Response{
		Content:            openAIResp.Choices[0].Message.Content,
		TokensInput:        openAIResp.Usage.PromptTokens,
		TokensOutput:       openAIResp.Usage.CompletionTokens,
		Model:              model,
		Provider:           o.Name(),
		Timestamp:          time.Now(),
		RateLimitRemaining: rateLimitRemaining,
		QuotaRemaining:     quotaRemaining,
	}, nil
}

// Stream makes a streaming API call to OpenAI
func (o *OpenAIProvider) Stream(ctx context.Context, model string, prompt string) (<-chan string, error) {
	if !o.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	reqBody := openAIRequest{
		Model: model,
		Messages: []openAIMessage{
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

	req, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+o.GetAPIKey())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := o.httpClient.Do(req)
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

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines and comments
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}

			// Parse SSE format: "data: {...}"
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")

				// Check for stream end
				if data == "[DONE]" {
					return
				}

				var chunk openAIStreamChunk
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					ch <- fmt.Sprintf("Error parsing chunk: %v", err)
					continue
				}

				if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
					ch <- chunk.Choices[0].Delta.Content
				}
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- fmt.Sprintf("Error reading stream: %v", err)
		}
	}()

	return ch, nil
}

// GetRateLimitInfo returns rate limit information for OpenAI.
// It returns the most recent data: first from the state store, then from the
// in-memory cache populated by the last Call().
func (o *OpenAIProvider) GetRateLimitInfo() (*RateLimitInfo, error) {
	// Try reading from the persistent store first.
	if o.store != nil {
		stateInfo, err := o.store.GetRateLimit(o.Name())
		if err == nil && stateInfo != nil {
			return &RateLimitInfo{
				RequestsRemaining: derefIntOr(stateInfo.RequestsRemaining, 0),
				RequestsLimit:     derefIntOr(stateInfo.RequestsLimit, 0),
				ResetAt:           derefTimeOr(stateInfo.ResetAt, time.Time{}),
			}, nil
		}
	}

	// Fall back to in-memory cache.
	o.mu.Lock()
	cached := o.lastRateLimit
	o.mu.Unlock()
	if cached != nil {
		return cached, nil
	}

	return &RateLimitInfo{}, nil
}

// GetQuotaInfo returns quota information for OpenAI.
func (o *OpenAIProvider) GetQuotaInfo() (*QuotaInfo, error) {
	// Try reading from the persistent store first.
	if o.store != nil {
		stateInfo, err := o.store.GetQuota(o.Name())
		if err == nil && stateInfo != nil {
			return &QuotaInfo{
				TokensRemaining: derefIntOr(stateInfo.TokensRemaining, 0),
				TokensLimit:     derefIntOr(stateInfo.TokensLimit, 0),
				CostRemaining:   derefFloat64Or(stateInfo.CostRemaining, 0),
				CostLimit:       derefFloat64Or(stateInfo.CostLimit, 0),
				ResetAt:         stateInfo.ResetAt,
			}, nil
		}
	}

	// Fall back to in-memory cache.
	o.mu.Lock()
	cached := o.lastQuotaInfo
	o.mu.Unlock()
	if cached != nil {
		return cached, nil
	}

	return &QuotaInfo{}, nil
}

// persistRateLimitAndQuota saves extracted rate-limit and quota info to the store.
func (o *OpenAIProvider) persistRateLimitAndQuota(rl *RateLimitInfo, qi *QuotaInfo) {
	if o.store == nil {
		return
	}

	now := time.Now()

	if rl != nil {
		remaining := rl.RequestsRemaining
		limit := rl.RequestsLimit
		resetAt := rl.ResetAt
		stateRL := &state.RateLimitInfo{
			Provider:          o.Name(),
			RequestsRemaining: &remaining,
			RequestsLimit:     &limit,
			ResetAt:           &resetAt,
			CheckedAt:         now,
		}
		if err := o.store.SaveRateLimit(o.Name(), stateRL); err != nil {
			o.GetLogger().Error("failed to persist rate limit info",
				"provider", o.Name(),
				"error", err.Error(),
			)
		}
	}

	if qi != nil {
		tokensRemaining := qi.TokensRemaining
		tokensLimit := qi.TokensLimit
		costRemaining := qi.CostRemaining
		costLimit := qi.CostLimit
		stateQI := &state.QuotaInfo{
			Provider:        o.Name(),
			TokensRemaining: &tokensRemaining,
			TokensLimit:     &tokensLimit,
			CostRemaining:   &costRemaining,
			CostLimit:       &costLimit,
			ResetAt:         qi.ResetAt,
			CheckedAt:       now,
		}
		if err := o.store.SaveQuota(o.Name(), stateQI); err != nil {
			o.GetLogger().Error("failed to persist quota info",
				"provider", o.Name(),
				"error", err.Error(),
			)
		}
	}
}

// derefIntOr dereferences an *int, returning fallback if nil.
func derefIntOr(p *int, fallback int) int {
	if p != nil {
		return *p
	}
	return fallback
}

// derefFloat64Or dereferences a *float64, returning fallback if nil.
func derefFloat64Or(p *float64, fallback float64) float64 {
	if p != nil {
		return *p
	}
	return fallback
}

// derefTimeOr dereferences a *time.Time, returning fallback if nil.
func derefTimeOr(p *time.Time, fallback time.Time) time.Time {
	if p != nil {
		return *p
	}
	return fallback
}
