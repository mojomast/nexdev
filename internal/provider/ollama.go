package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaProvider implements the Provider interface for Ollama
type OllamaProvider struct {
	*BaseProvider
	baseURL    string
	httpClient *http.Client
}

// NewOllamaProvider creates a new Ollama provider
func NewOllamaProvider(baseURL string) *OllamaProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaProvider{
		BaseProvider: NewBaseProvider("ollama"),
		baseURL:      baseURL,
		httpClient: &http.Client{
			Timeout: 300 * time.Second, // Ollama can be slow for large models
		},
	}
}

// ollamaRequest represents a request to Ollama API
type ollamaRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// ollamaChatRequest represents a chat request to Ollama API
type ollamaChatRequest struct {
	Model    string                 `json:"model"`
	Messages []ollamaMessage        `json:"messages"`
	Stream   bool                   `json:"stream,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ollamaResponse represents a response from Ollama API
type ollamaResponse struct {
	Model              string        `json:"model"`
	CreatedAt          time.Time     `json:"created_at"`
	Response           string        `json:"response"`
	Message            ollamaMessage `json:"message"`
	Done               bool          `json:"done"`
	TotalDuration      int64         `json:"total_duration,omitempty"`
	LoadDuration       int64         `json:"load_duration,omitempty"`
	PromptEvalCount    int           `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64         `json:"prompt_eval_duration,omitempty"`
	EvalCount          int           `json:"eval_count,omitempty"`
	EvalDuration       int64         `json:"eval_duration,omitempty"`
}

// ollamaModelsResponse represents the models list response
type ollamaModelsResponse struct {
	Models []struct {
		Name       string    `json:"name"`
		ModifiedAt time.Time `json:"modified_at"`
		Size       int64     `json:"size"`
		Digest     string    `json:"digest"`
		Details    struct {
			Format            string   `json:"format"`
			Family            string   `json:"family"`
			Families          []string `json:"families"`
			ParameterSize     string   `json:"parameter_size"`
			QuantizationLevel string   `json:"quantization_level"`
		} `json:"details"`
	} `json:"models"`
}

// Authenticate for Ollama is a no-op since it doesn't require API keys
func (o *OllamaProvider) Authenticate(apiKey string) error {
	// Ollama doesn't require authentication, but we check connectivity
	resp, err := o.httpClient.Get(o.baseURL + "/api/tags")
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama at %s: %w", o.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama server returned status %d", resp.StatusCode)
	}

	o.authenticated = true
	return nil
}

// ListModels returns the list of available models from Ollama
func (o *OllamaProvider) ListModels() ([]Model, error) {
	if !o.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	resp, err := o.httpClient.Get(o.baseURL + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]Model, 0, len(ollamaResp.Models))
	for _, m := range ollamaResp.Models {
		model := Model{
			Provider:     "ollama",
			Name:         m.Name,
			DisplayName:  m.Name,
			Capabilities: []string{"text", "streaming"},
			PriceInput:   0.0, // Ollama is free (local)
			PriceOutput:  0.0,
		}
		models = append(models, model)
	}

	return models, nil
}

// Call makes a non-streaming API call to Ollama
func (o *OllamaProvider) Call(ctx context.Context, model string, prompt string) (*Response, error) {
	if !o.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	startTime := time.Now()
	var response *Response
	err := o.RetryWithBackoff(func() error {
		// Use chat endpoint for better compatibility
		req := ollamaChatRequest{
			Model: model,
			Messages: []ollamaMessage{
				{
					Role:    "user",
					Content: prompt,
				},
			},
			Stream: false,
			Options: map[string]interface{}{
				"temperature": 0.7,
			},
		}

		jsonData, err := json.Marshal(req)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/chat", bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := o.httpClient.Do(httpReq)
		if err != nil {
			return fmt.Errorf("failed to make request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}

		var ollamaResp ollamaResponse
		if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		// Extract content from either response or message
		content := ollamaResp.Response
		if content == "" && ollamaResp.Message.Content != "" {
			content = ollamaResp.Message.Content
		}

		duration := time.Since(startTime)

		// Log successful API call with metadata
		o.GetLogger().Info("API call completed",
			"provider", "ollama",
			"model", model,
			"tokens_input", ollamaResp.PromptEvalCount,
			"tokens_output", ollamaResp.EvalCount,
			"tokens_total", ollamaResp.PromptEvalCount+ollamaResp.EvalCount,
			"duration_ms", duration.Milliseconds(),
		)

		response = &Response{
			Content:      content,
			TokensInput:  ollamaResp.PromptEvalCount,
			TokensOutput: ollamaResp.EvalCount,
			Model:        ollamaResp.Model,
			Provider:     "ollama",
			Timestamp:    time.Now(),
		}

		return nil
	})

	if err != nil {
		o.GetLogger().Error("API call failed",
			"provider", "ollama",
			"model", model,
			"error", err.Error(),
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
	}

	return response, err
}

// Stream makes a streaming API call to Ollama
func (o *OllamaProvider) Stream(ctx context.Context, model string, prompt string) (<-chan string, error) {
	if !o.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	// Use chat endpoint for better compatibility
	req := ollamaChatRequest{
		Model: model,
		Messages: []ollamaMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream: true,
		Options: map[string]interface{}{
			"temperature": 0.7,
		},
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(httpReq)
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
			if line == "" {
				continue
			}

			var chunk ollamaResponse
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				continue
			}

			// Send content from either response or message
			content := chunk.Response
			if content == "" && chunk.Message.Content != "" {
				content = chunk.Message.Content
			}

			if content != "" {
				ch <- content
			}

			if chunk.Done {
				break
			}
		}
	}()

	return ch, nil
}

// GetRateLimitInfo returns nil for Ollama (no rate limits)
func (o *OllamaProvider) GetRateLimitInfo() (*RateLimitInfo, error) {
	// Ollama doesn't have rate limits (local deployment)
	return nil, nil
}

// GetQuotaInfo returns nil for Ollama (no quotas)
func (o *OllamaProvider) GetQuotaInfo() (*QuotaInfo, error) {
	// Ollama doesn't have quotas (local deployment)
	return nil, nil
}
