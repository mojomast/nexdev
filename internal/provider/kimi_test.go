package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewKimiProvider(t *testing.T) {
	provider := NewKimiProvider()
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
	if provider.Name() != "kimi" {
		t.Errorf("expected provider name 'kimi', got '%s'", provider.Name())
	}
}

func TestKimiProvider_Authenticate(t *testing.T) {
	provider := NewKimiProvider()

	// Test successful authentication
	err := provider.Authenticate("test-api-key")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !provider.IsAuthenticated() {
		t.Error("expected provider to be authenticated")
	}

	// Test empty API key
	provider2 := NewKimiProvider()
	err = provider2.Authenticate("")
	if err == nil {
		t.Error("expected error with empty API key")
	}
}

func TestKimiProvider_ListModels(t *testing.T) {
	provider := NewKimiProvider()

	// Test without authentication
	_, err := provider.ListModels()
	if err == nil {
		t.Error("expected error when not authenticated")
	}

	// Test with authentication
	provider.Authenticate("test-api-key")
	models, err := provider.ListModels()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(models) == 0 {
		t.Error("expected at least one model")
	}

	// Check for expected models
	expectedModels := map[string]bool{
		"moonshot-v1-8k":   false,
		"moonshot-v1-32k":  false,
		"moonshot-v1-128k": false,
	}

	for _, model := range models {
		if model.Provider != "kimi" {
			t.Errorf("expected provider 'kimi', got '%s'", model.Provider)
		}
		if _, ok := expectedModels[model.Name]; ok {
			expectedModels[model.Name] = true
		}
	}

	for name, found := range expectedModels {
		if !found {
			t.Errorf("expected model '%s' not found", name)
		}
	}
}

func TestKimiProvider_Call(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got '%s'", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("expected Authorization 'Bearer test-api-key', got '%s'", r.Header.Get("Authorization"))
		}

		// Set rate limit headers
		w.Header().Set("X-Ratelimit-Remaining-Requests", "100")
		w.Header().Set("X-Ratelimit-Limit-Requests", "1000")

		// Send mock response
		response := zaiResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "moonshot-v1-8k",
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					}{
						Role:    "assistant",
						Content: "Hello! How can I help you with coding today?",
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create provider with test server URL
	provider := NewKimiProvider()
	provider.baseURL = server.URL
	provider.Authenticate("test-api-key")

	// Make API call
	resp, err := provider.Call(context.TODO(), "moonshot-v1-8k", "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify response
	if resp.Content != "Hello! How can I help you with coding today?" {
		t.Errorf("expected content 'Hello! How can I help you with coding today?', got '%s'", resp.Content)
	}
	if resp.TokensInput != 10 {
		t.Errorf("expected 10 input tokens, got %d", resp.TokensInput)
	}
	if resp.TokensOutput != 20 {
		t.Errorf("expected 20 output tokens, got %d", resp.TokensOutput)
	}
	if resp.Model != "moonshot-v1-8k" {
		t.Errorf("expected model 'moonshot-v1-8k', got '%s'", resp.Model)
	}
	if resp.Provider != "kimi" {
		t.Errorf("expected provider 'kimi', got '%s'", resp.Provider)
	}
	if resp.RateLimitRemaining != 100 {
		t.Errorf("expected 100 rate limit remaining, got %d", resp.RateLimitRemaining)
	}
}

func TestKimiProvider_Stream(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got '%s'", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("expected Authorization 'Bearer test-api-key', got '%s'", r.Header.Get("Authorization"))
		}

		// Send streaming response
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.ResponseWriter to be http.Flusher")
		}

		// Send chunks
		chunks := []string{
			`data: {"id":"1","object":"chat.completion.chunk","created":1234567890,"model":"moonshot-v1-8k","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}` + "\n\n",
			`data: {"id":"1","object":"chat.completion.chunk","created":1234567890,"model":"moonshot-v1-8k","choices":[{"index":0,"delta":{"content":" there"},"finish_reason":null}]}` + "\n\n",
			`data: {"id":"1","object":"chat.completion.chunk","created":1234567890,"model":"moonshot-v1-8k","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":"stop"}]}` + "\n\n",
			"data: [DONE]\n\n",
		}

		for _, chunk := range chunks {
			w.Write([]byte(chunk))
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	// Create provider with test server URL
	provider := NewKimiProvider()
	provider.baseURL = server.URL
	provider.Authenticate("test-api-key")

	// Make streaming call
	ch, err := provider.Stream(context.TODO(), "moonshot-v1-8k", "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Collect chunks
	var chunks []string
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	// Verify chunks
	expectedChunks := []string{"Hello", " there", "!"}
	if len(chunks) != len(expectedChunks) {
		t.Errorf("expected %d chunks, got %d", len(expectedChunks), len(chunks))
	}

	for i, expected := range expectedChunks {
		if i >= len(chunks) {
			break
		}
		if chunks[i] != expected {
			t.Errorf("chunk %d: expected '%s', got '%s'", i, expected, chunks[i])
		}
	}
}

func TestKimiProvider_GetRateLimitInfo(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set rate limit headers
		w.Header().Set("X-Ratelimit-Remaining-Requests", "100")
		w.Header().Set("X-Ratelimit-Limit-Requests", "1000")
		w.Header().Set("X-Ratelimit-Reset-Requests", "1704067200")
		w.Header().Set("Retry-After", "60")

		// Send minimal response
		response := zaiResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "moonshot-v1-8k-turbo",
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Message: struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					}{
						Role:    "assistant",
						Content: "Hi",
					},
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     1,
				CompletionTokens: 1,
				TotalTokens:      2,
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create provider with test server URL
	provider := NewKimiProvider()
	provider.baseURL = server.URL
	provider.Authenticate("test-api-key")

	// Get rate limit info
	info, err := provider.GetRateLimitInfo()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify info
	if info.RequestsRemaining != 100 {
		t.Errorf("expected 100 requests remaining, got %d", info.RequestsRemaining)
	}
	if info.RequestsLimit != 1000 {
		t.Errorf("expected 1000 requests limit, got %d", info.RequestsLimit)
	}
	if info.RetryAfter != 60*time.Second {
		t.Errorf("expected 60s retry after, got %v", info.RetryAfter)
	}
}

func TestKimiProvider_GetQuotaInfo(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set quota headers
		w.Header().Set("X-Ratelimit-Remaining-Tokens", "50000")
		w.Header().Set("X-Ratelimit-Limit-Tokens", "100000")

		// Send minimal response
		response := zaiResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "moonshot-v1-8k-turbo",
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Message: struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					}{
						Role:    "assistant",
						Content: "Hi",
					},
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     1,
				CompletionTokens: 1,
				TotalTokens:      2,
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create provider with test server URL
	provider := NewKimiProvider()
	provider.baseURL = server.URL
	provider.Authenticate("test-api-key")

	// Get quota info
	info, err := provider.GetQuotaInfo()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify info
	if info.TokensRemaining != 50000 {
		t.Errorf("expected 50000 tokens remaining, got %d", info.TokensRemaining)
	}
	if info.TokensLimit != 100000 {
		t.Errorf("expected 100000 tokens limit, got %d", info.TokensLimit)
	}
}

func TestKimiProvider_SupportsCodingPlan(t *testing.T) {
	provider := NewKimiProvider()
	if !provider.SupportsCodingPlan() {
		t.Error("expected Z.ai to support coding plans")
	}
}

func TestKimiProvider_CallError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	defer server.Close()

	// Create provider with test server URL
	provider := NewKimiProvider()
	provider.baseURL = server.URL
	provider.Authenticate("invalid-key")
	provider.SetMaxRetries(0) // Don't retry for this test

	// Make API call
	_, err := provider.Call(context.TODO(), "moonshot-v1-8k", "Hello")
	if err == nil {
		t.Error("expected error, got nil")
	}
}
