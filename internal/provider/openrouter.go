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
	"sync"
	"time"
)

const (
	openRouterBaseURL = "https://openrouter.ai/api/v1"
	openRouterReferer = "https://github.com/mojomast/nexdev"
	openRouterTitle   = "nexdev"
)

// OpenRouterProvider implements the Provider interface for OpenRouter.
type OpenRouterProvider struct {
	*BaseProvider
	baseURL    string
	httpClient *http.Client

	mu          sync.Mutex
	lastHeaders http.Header
}

// NewOpenRouterProvider creates a new OpenRouter provider.
func NewOpenRouterProvider() *OpenRouterProvider {
	return &OpenRouterProvider{
		BaseProvider: NewBaseProvider("openrouter"),
		baseURL:      openRouterBaseURL,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

type openRouterRequest struct {
	Model    string              `json:"model"`
	Messages []openRouterMessage `json:"messages"`
	Stream   bool                `json:"stream"`
}

type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterResponse struct {
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

type openRouterModelsResponse struct {
	Data []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Pricing struct {
			Prompt     string `json:"prompt"`
			Completion string `json:"completion"`
		} `json:"pricing"`
	} `json:"data"`
}

type openRouterStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// ListModels returns live OpenRouter model metadata.
func (o *OpenRouterProvider) ListModels() ([]Model, error) {
	if !o.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	var resp *http.Response
	err := o.RetryWithBackoff(func() error {
		req, reqErr := http.NewRequest("GET", o.baseURL+"/models", nil)
		if reqErr != nil {
			return fmt.Errorf("failed to create request: %w", reqErr)
		}
		o.setHeaders(req, false)

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
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	o.storeHeaders(resp.Header)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var modelsResp openRouterModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]Model, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		displayName := m.Name
		if displayName == "" {
			displayName = m.ID
		}
		models = append(models, Model{
			Provider:    o.Name(),
			Name:        m.ID,
			DisplayName: displayName,
			PriceInput:  parseOpenRouterPrice(m.Pricing.Prompt),
			PriceOutput: parseOpenRouterPrice(m.Pricing.Completion),
		})
	}

	return models, nil
}

// DiscoverModels returns the same live model list as ListModels.
func (o *OpenRouterProvider) DiscoverModels() ([]Model, error) {
	return o.ListModels()
}

// Call makes a synchronous API call to OpenRouter.
func (o *OpenRouterProvider) Call(ctx context.Context, model string, prompt string) (*Response, error) {
	if !o.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	startTime := time.Now()
	reqBody := openRouterRequest{
		Model: model,
		Messages: []openRouterMessage{{
			Role:    "user",
			Content: prompt,
		}},
		Stream: false,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var resp *http.Response
	err = o.RetryWithBackoff(func() error {
		req, reqErr := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
		if reqErr != nil {
			return fmt.Errorf("failed to create request: %w", reqErr)
		}
		o.setHeaders(req, false)

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
	o.storeHeaders(resp.Header)

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

	rateLimitInfo := ExtractRateLimitInfo(resp)
	quotaInfo := ExtractQuotaInfo(resp)
	cost := parseOpenRouterCost(resp.Header.Get("X-OpenRouter-Cost"))

	var openRouterResp openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&openRouterResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if len(openRouterResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	duration := time.Since(startTime)
	o.GetLogger().Info("API call completed",
		"provider", o.Name(),
		"model", model,
		"tokens_input", openRouterResp.Usage.PromptTokens,
		"tokens_output", openRouterResp.Usage.CompletionTokens,
		"tokens_total", openRouterResp.Usage.TotalTokens,
		"duration_ms", duration.Milliseconds(),
		"cost", cost,
	)

	return &Response{
		Content:            openRouterResp.Choices[0].Message.Content,
		TokensInput:        openRouterResp.Usage.PromptTokens,
		TokensOutput:       openRouterResp.Usage.CompletionTokens,
		Model:              model,
		Provider:           o.Name(),
		Timestamp:          time.Now(),
		RateLimitRemaining: rateLimitInfo.RequestsRemaining,
		QuotaRemaining:     quotaInfo.TokensRemaining,
		RateLimitInfo:      rateLimitInfo,
		QuotaInfo:          quotaInfo,
		Cost:               cost,
	}, nil
}

// Stream makes a streaming API call to OpenRouter.
func (o *OpenRouterProvider) Stream(ctx context.Context, model string, prompt string) (<-chan string, error) {
	if !o.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	reqBody := openRouterRequest{
		Model: model,
		Messages: []openRouterMessage{{
			Role:    "user",
			Content: prompt,
		}},
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
	o.setHeaders(req, true)

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	o.storeHeaders(resp.Header)

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
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, ":") || !strings.HasPrefix(line, "data:") {
				continue
			}

			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				return
			}

			var chunk openRouterStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if len(chunk.Choices) == 0 || chunk.Choices[0].Delta.Content == "" {
				continue
			}

			select {
			case ch <- chunk.Choices[0].Delta.Content:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// GetRateLimitInfo returns rate limit information from the last OpenRouter response.
func (o *OpenRouterProvider) GetRateLimitInfo() (*RateLimitInfo, error) {
	headers := o.headers()
	if headers == nil {
		return &RateLimitInfo{}, nil
	}
	return ExtractRateLimitInfo(&http.Response{Header: headers}), nil
}

// GetQuotaInfo returns quota information from the last OpenRouter response.
func (o *OpenRouterProvider) GetQuotaInfo() (*QuotaInfo, error) {
	headers := o.headers()
	if headers == nil {
		return &QuotaInfo{}, nil
	}
	return ExtractQuotaInfo(&http.Response{Header: headers}), nil
}

func (o *OpenRouterProvider) setHeaders(req *http.Request, stream bool) {
	req.Header.Set("Authorization", "Bearer "+o.GetAPIKey())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", openRouterReferer)
	req.Header.Set("X-Title", openRouterTitle)
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
}

func (o *OpenRouterProvider) storeHeaders(headers http.Header) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.lastHeaders = headers.Clone()
}

func (o *OpenRouterProvider) headers() http.Header {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.lastHeaders == nil {
		return nil
	}
	return o.lastHeaders.Clone()
}

func parseOpenRouterPrice(value string) float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return parsed * 1000
}

func parseOpenRouterCost(value string) float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return parsed
}
