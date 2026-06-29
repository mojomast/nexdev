package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewAnthropicProvider(t *testing.T) {
	provider := NewAnthropicProvider()
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
	if provider.Name() != "anthropic" {
		t.Errorf("expected provider name 'anthropic', got '%s'", provider.Name())
	}
}

func TestAnthropicProvider_Authenticate(t *testing.T) {
	provider := NewAnthropicProvider()

	// Test successful authentication
	err := provider.Authenticate("test-api-key")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !provider.IsAuthenticated() {
		t.Error("expected provider to be authenticated")
	}

	// Test empty API key
	provider2 := NewAnthropicProvider()
	err = provider2.Authenticate("")
	if err == nil {
		t.Error("expected error with empty API key")
	}
}

func TestAnthropicProvider_ListModels(t *testing.T) {
	provider := NewAnthropicProvider()

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
		"claude-3-5-sonnet-20241022": false,
		"claude-3-5-haiku-20241022":  false,
		"claude-3-opus-20240229":     false,
		"claude-3-sonnet-20240229":   false,
		"claude-3-haiku-20240307":    false,
	}

	for _, model := range models {
		if model.Provider != "anthropic" {
			t.Errorf("expected provider 'anthropic', got '%s'", model.Provider)
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

func TestAnthropicProvider_Call(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got '%s'", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("x-api-key") != "test-api-key" {
			t.Errorf("expected x-api-key 'test-api-key', got '%s'", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("expected anthropic-version '2023-06-01', got '%s'", r.Header.Get("anthropic-version"))
		}

		// Set rate limit headers
		w.Header().Set("anthropic-ratelimit-requests-remaining", "100")
		w.Header().Set("anthropic-ratelimit-requests-limit", "1000")

		// Send mock response
		response := anthropicResponse{
			ID:   "msg_123",
			Type: "message",
			Role: "assistant",
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				{Type: "text", Text: "Hello! How can I help you?"},
			},
			Model:      "claude-3-haiku-20240307",
			StopReason: "end_turn",
			Usage: struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}{
				InputTokens:  10,
				OutputTokens: 20,
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create provider with test server URL
	provider := NewAnthropicProvider()
	provider.baseURL = server.URL
	provider.Authenticate("test-api-key")

	// Make API call
	resp, err := provider.Call(context.TODO(), "claude-3-haiku-20240307", "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify response
	if resp.Content != "Hello! How can I help you?" {
		t.Errorf("expected content 'Hello! How can I help you?', got '%s'", resp.Content)
	}
	if resp.TokensInput != 10 {
		t.Errorf("expected 10 input tokens, got %d", resp.TokensInput)
	}
	if resp.TokensOutput != 20 {
		t.Errorf("expected 20 output tokens, got %d", resp.TokensOutput)
	}
	if resp.Model != "claude-3-haiku-20240307" {
		t.Errorf("expected model 'claude-3-haiku-20240307', got '%s'", resp.Model)
	}
	if resp.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got '%s'", resp.Provider)
	}
	if resp.RateLimitRemaining != 100 {
		t.Errorf("expected 100 rate limit remaining, got %d", resp.RateLimitRemaining)
	}
}

func TestAnthropicProvider_Stream(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got '%s'", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("x-api-key") != "test-api-key" {
			t.Errorf("expected x-api-key 'test-api-key', got '%s'", r.Header.Get("x-api-key"))
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
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"Hello"}}` + "\n\n",
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" there"}}` + "\n\n",
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"!"}}` + "\n\n",
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
	provider := NewAnthropicProvider()
	provider.baseURL = server.URL
	provider.Authenticate("test-api-key")

	// Make streaming call
	ch, err := provider.Stream(context.TODO(), "claude-3-haiku-20240307", "Hello")
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

func TestAnthropicProvider_GetRateLimitInfo(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set rate limit headers
		w.Header().Set("anthropic-ratelimit-requests-remaining", "100")
		w.Header().Set("anthropic-ratelimit-requests-limit", "1000")
		w.Header().Set("anthropic-ratelimit-requests-reset", "2024-01-01T00:00:00Z")
		w.Header().Set("retry-after", "60")

		// Send minimal response
		response := anthropicResponse{
			ID:   "msg_123",
			Type: "message",
			Role: "assistant",
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				{Type: "text", Text: "Hi"},
			},
			Model: "claude-3-haiku-20240307",
			Usage: struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}{
				InputTokens:  1,
				OutputTokens: 1,
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create provider with test server URL
	provider := NewAnthropicProvider()
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

func TestAnthropicProvider_GetQuotaInfo(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set quota headers
		w.Header().Set("anthropic-ratelimit-tokens-remaining", "50000")
		w.Header().Set("anthropic-ratelimit-tokens-limit", "100000")
		w.Header().Set("anthropic-ratelimit-tokens-reset", "2024-01-01T00:00:00Z")

		// Send minimal response
		response := anthropicResponse{
			ID:   "msg_123",
			Type: "message",
			Role: "assistant",
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				{Type: "text", Text: "Hi"},
			},
			Model: "claude-3-haiku-20240307",
			Usage: struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}{
				InputTokens:  1,
				OutputTokens: 1,
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create provider with test server URL
	provider := NewAnthropicProvider()
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

func TestAnthropicProvider_CallError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"type": "authentication_error", "message": "Invalid API key"}}`))
	}))
	defer server.Close()

	// Create provider with test server URL
	provider := NewAnthropicProvider()
	provider.baseURL = server.URL
	provider.Authenticate("invalid-key")
	provider.SetMaxRetries(0) // Don't retry for this test

	// Make API call
	_, err := provider.Call(context.TODO(), "claude-3-haiku-20240307", "Hello")
	if err == nil {
		t.Error("expected error, got nil")
	}
}
