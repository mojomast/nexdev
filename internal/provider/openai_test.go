package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/state"
)

func TestNewOpenAIProvider(t *testing.T) {
	provider := NewOpenAIProvider()

	if provider == nil {
		t.Fatal("NewOpenAIProvider returned nil")
	}

	if provider.Name() != "openai" {
		t.Errorf("Expected provider name 'openai', got '%s'", provider.Name())
	}

	if provider.baseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected base URL 'https://api.openai.com/v1', got '%s'", provider.baseURL)
	}
}

func TestOpenAIProvider_Authenticate(t *testing.T) {
	provider := NewOpenAIProvider()

	tests := []struct {
		name    string
		apiKey  string
		wantErr bool
	}{
		{
			name:    "valid API key",
			apiKey:  "sk-test123",
			wantErr: false,
		},
		{
			name:    "empty API key",
			apiKey:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.Authenticate(tt.apiKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("Authenticate() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && !provider.IsAuthenticated() {
				t.Error("Provider should be authenticated after successful authentication")
			}
		})
	}
}

func TestOpenAIProvider_ListModels(t *testing.T) {
	tests := []struct {
		name           string
		authenticated  bool
		serverResponse string
		statusCode     int
		wantErr        bool
		wantModels     int
	}{
		{
			name:          "not authenticated",
			authenticated: false,
			wantErr:       true,
		},
		{
			name:          "successful list",
			authenticated: true,
			serverResponse: `{
				"data": [
					{"id": "gpt-4", "object": "model", "created": 1234567890, "owned_by": "openai"},
					{"id": "gpt-3.5-turbo", "object": "model", "created": 1234567890, "owned_by": "openai"},
					{"id": "whisper-1", "object": "model", "created": 1234567890, "owned_by": "openai"}
				]
			}`,
			statusCode: http.StatusOK,
			wantErr:    false,
			wantModels: 2, // Only GPT models should be included
		},
		{
			name:           "API error",
			authenticated:  true,
			serverResponse: `{"error": {"message": "Invalid API key"}}`,
			statusCode:     http.StatusUnauthorized,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/models" {
					t.Errorf("Expected path '/v1/models', got '%s'", r.URL.Path)
				}

				if tt.authenticated {
					authHeader := r.Header.Get("Authorization")
					if !strings.HasPrefix(authHeader, "Bearer ") {
						t.Error("Expected Authorization header with Bearer token")
					}
				}

				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			provider := NewOpenAIProvider()
			provider.baseURL = server.URL + "/v1"

			if tt.authenticated {
				provider.Authenticate("sk-test123")
			}

			models, err := provider.ListModels()

			if (err != nil) != tt.wantErr {
				t.Errorf("ListModels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(models) != tt.wantModels {
					t.Errorf("Expected %d models, got %d", tt.wantModels, len(models))
				}

				// Verify pricing is set for known models
				for _, model := range models {
					if strings.HasPrefix(model.Name, "gpt-4") {
						if model.PriceInput != 0.03 || model.PriceOutput != 0.06 {
							t.Errorf("GPT-4 pricing incorrect: input=%f, output=%f", model.PriceInput, model.PriceOutput)
						}
					} else if strings.HasPrefix(model.Name, "gpt-3.5") {
						if model.PriceInput != 0.0015 || model.PriceOutput != 0.002 {
							t.Errorf("GPT-3.5 pricing incorrect: input=%f, output=%f", model.PriceInput, model.PriceOutput)
						}
					}
				}
			}
		})
	}
}

func TestOpenAIProvider_Call(t *testing.T) {
	tests := []struct {
		name           string
		authenticated  bool
		model          string
		prompt         string
		serverResponse string
		statusCode     int
		headers        map[string]string
		wantErr        bool
		wantContent    string
		wantTokensIn   int
		wantTokensOut  int
	}{
		{
			name:          "not authenticated",
			authenticated: false,
			model:         "gpt-4",
			prompt:        "Hello",
			wantErr:       true,
		},
		{
			name:          "successful call",
			authenticated: true,
			model:         "gpt-4",
			prompt:        "Hello",
			serverResponse: `{
				"id": "chatcmpl-123",
				"object": "chat.completion",
				"created": 1234567890,
				"model": "gpt-4",
				"choices": [{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "Hello! How can I help you?"
					},
					"finish_reason": "stop"
				}],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 20,
					"total_tokens": 30
				}
			}`,
			statusCode: http.StatusOK,
			headers: map[string]string{
				"X-RateLimit-Remaining-Requests": "100",
				"X-RateLimit-Remaining-Tokens":   "50000",
			},
			wantErr:       false,
			wantContent:   "Hello! How can I help you?",
			wantTokensIn:  10,
			wantTokensOut: 20,
		},
		{
			name:          "API error",
			authenticated: true,
			model:         "gpt-4",
			prompt:        "Hello",
			serverResponse: `{
				"error": {
					"message": "Rate limit exceeded",
					"type": "rate_limit_error"
				}
			}`,
			statusCode: http.StatusTooManyRequests,
			wantErr:    true,
		},
		{
			name:          "empty choices",
			authenticated: true,
			model:         "gpt-4",
			prompt:        "Hello",
			serverResponse: `{
				"id": "chatcmpl-123",
				"object": "chat.completion",
				"created": 1234567890,
				"model": "gpt-4",
				"choices": [],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 0,
					"total_tokens": 10
				}
			}`,
			statusCode: http.StatusOK,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/chat/completions" {
					t.Errorf("Expected path '/v1/chat/completions', got '%s'", r.URL.Path)
				}

				if r.Method != http.MethodPost {
					t.Errorf("Expected POST method, got '%s'", r.Method)
				}

				if tt.authenticated {
					authHeader := r.Header.Get("Authorization")
					if !strings.HasPrefix(authHeader, "Bearer ") {
						t.Error("Expected Authorization header with Bearer token")
					}
				}

				// Verify request body
				var reqBody openAIRequest
				if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
					t.Errorf("Failed to decode request body: %v", err)
				}

				if reqBody.Model != tt.model {
					t.Errorf("Expected model '%s', got '%s'", tt.model, reqBody.Model)
				}

				if len(reqBody.Messages) != 1 || reqBody.Messages[0].Content != tt.prompt {
					t.Errorf("Expected prompt '%s', got '%s'", tt.prompt, reqBody.Messages[0].Content)
				}

				// Set response headers
				for key, value := range tt.headers {
					w.Header().Set(key, value)
				}

				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			provider := NewOpenAIProvider()
			provider.baseURL = server.URL + "/v1"

			if tt.authenticated {
				provider.Authenticate("sk-test123")
			}

			resp, err := provider.Call(context.TODO(), tt.model, tt.prompt)

			if (err != nil) != tt.wantErr {
				t.Errorf("Call() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if resp.Content != tt.wantContent {
					t.Errorf("Expected content '%s', got '%s'", tt.wantContent, resp.Content)
				}

				if resp.TokensInput != tt.wantTokensIn {
					t.Errorf("Expected %d input tokens, got %d", tt.wantTokensIn, resp.TokensInput)
				}

				if resp.TokensOutput != tt.wantTokensOut {
					t.Errorf("Expected %d output tokens, got %d", tt.wantTokensOut, resp.TokensOutput)
				}

				if resp.Model != tt.model {
					t.Errorf("Expected model '%s', got '%s'", tt.model, resp.Model)
				}

				if resp.Provider != "openai" {
					t.Errorf("Expected provider 'openai', got '%s'", resp.Provider)
				}

				if resp.RateLimitRemaining != 100 {
					t.Errorf("Expected rate limit remaining 100, got %d", resp.RateLimitRemaining)
				}

				if resp.QuotaRemaining != 50000 {
					t.Errorf("Expected quota remaining 50000, got %d", resp.QuotaRemaining)
				}
			}
		})
	}
}

func TestOpenAIProvider_Stream(t *testing.T) {
	tests := []struct {
		name           string
		authenticated  bool
		model          string
		prompt         string
		serverResponse string
		statusCode     int
		wantErr        bool
		wantChunks     []string
	}{
		{
			name:          "not authenticated",
			authenticated: false,
			model:         "gpt-4",
			prompt:        "Hello",
			wantErr:       true,
		},
		{
			name:          "successful stream",
			authenticated: true,
			model:         "gpt-4",
			prompt:        "Hello",
			serverResponse: `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]

`,
			statusCode: http.StatusOK,
			wantErr:    false,
			wantChunks: []string{"Hello", "!"},
		},
		{
			name:           "API error",
			authenticated:  true,
			model:          "gpt-4",
			prompt:         "Hello",
			serverResponse: `{"error": {"message": "Rate limit exceeded"}}`,
			statusCode:     http.StatusTooManyRequests,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/chat/completions" {
					t.Errorf("Expected path '/v1/chat/completions', got '%s'", r.URL.Path)
				}

				if r.Method != http.MethodPost {
					t.Errorf("Expected POST method, got '%s'", r.Method)
				}

				if tt.authenticated {
					authHeader := r.Header.Get("Authorization")
					if !strings.HasPrefix(authHeader, "Bearer ") {
						t.Error("Expected Authorization header with Bearer token")
					}
				}

				// Verify request body
				var reqBody openAIRequest
				if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
					t.Errorf("Failed to decode request body: %v", err)
				}

				if reqBody.Stream != true {
					t.Error("Expected stream=true in request")
				}

				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			provider := NewOpenAIProvider()
			provider.baseURL = server.URL + "/v1"

			if tt.authenticated {
				provider.Authenticate("sk-test123")
			}

			ch, err := provider.Stream(context.TODO(), tt.model, tt.prompt)

			if (err != nil) != tt.wantErr {
				t.Errorf("Stream() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				var chunks []string
				for chunk := range ch {
					chunks = append(chunks, chunk)
				}

				if len(chunks) != len(tt.wantChunks) {
					t.Errorf("Expected %d chunks, got %d", len(tt.wantChunks), len(chunks))
				}

				for i, chunk := range chunks {
					if i < len(tt.wantChunks) && chunk != tt.wantChunks[i] {
						t.Errorf("Chunk %d: expected '%s', got '%s'", i, tt.wantChunks[i], chunk)
					}
				}
			}
		})
	}
}

func TestOpenAIProvider_RetryWithBackoff(t *testing.T) {
	attempts := 0
	maxAttempts := 3

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < maxAttempts {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": {"message": "Server error"}}`))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"id": "chatcmpl-123",
				"object": "chat.completion",
				"created": 1234567890,
				"model": "gpt-4",
				"choices": [{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "Success after retries"
					},
					"finish_reason": "stop"
				}],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 20,
					"total_tokens": 30
				}
			}`))
		}
	}))
	defer server.Close()

	provider := NewOpenAIProvider()
	provider.baseURL = server.URL + "/v1"
	provider.Authenticate("sk-test123")
	provider.SetBaseDelay(10 * time.Millisecond) // Speed up test

	start := time.Now()
	resp, err := provider.Call(context.TODO(), "gpt-4", "Test retry")
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Call() should succeed after retries, got error: %v", err)
	}

	if resp == nil || resp.Content != "Success after retries" {
		t.Error("Expected successful response after retries")
	}

	if attempts != maxAttempts {
		t.Errorf("Expected %d attempts, got %d", maxAttempts, attempts)
	}

	// Verify exponential backoff occurred (should take at least 10ms + 20ms = 30ms)
	if duration < 30*time.Millisecond {
		t.Errorf("Expected at least 30ms for retries with backoff, got %v", duration)
	}
}

func TestOpenAIProvider_GetRateLimitInfo(t *testing.T) {
	provider := NewOpenAIProvider()
	provider.Authenticate("sk-test123")

	info, err := provider.GetRateLimitInfo()
	if err != nil {
		t.Errorf("GetRateLimitInfo() error = %v", err)
	}

	if info == nil {
		t.Error("Expected non-nil RateLimitInfo")
	}
}

func TestOpenAIProvider_GetQuotaInfo(t *testing.T) {
	provider := NewOpenAIProvider()
	provider.Authenticate("sk-test123")

	info, err := provider.GetQuotaInfo()
	if err != nil {
		t.Errorf("GetQuotaInfo() error = %v", err)
	}

	if info == nil {
		t.Error("Expected non-nil QuotaInfo")
	}
}

func TestOpenAIProvider_SupportsCodingPlan(t *testing.T) {
	provider := NewOpenAIProvider()

	if provider.SupportsCodingPlan() {
		t.Error("OpenAI should not support coding plan")
	}
}

func TestOpenAIProvider_SetStore(t *testing.T) {
	prov := NewOpenAIProvider()
	if prov.store != nil {
		t.Error("Expected store to be nil initially")
	}

	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := state.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	prov.SetStore(store)
	if prov.store == nil {
		t.Error("Expected store to be set")
	}
}

func TestOpenAIProvider_PersistRateLimitAndQuota(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := state.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a test server that returns rate limit and quota headers
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining-Requests", "42")
		w.Header().Set("X-RateLimit-Limit-Requests", "100")
		w.Header().Set("X-RateLimit-Remaining-Tokens", "5000")
		w.Header().Set("X-Ratelimit-Limit-Tokens", "10000")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(openAIResponse{
			ID: "chatcmpl-test",
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
					}{"assistant", "Hello!"},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		})
	}))
	defer server.Close()

	prov := NewOpenAIProvider()
	prov.Authenticate("sk-test123")
	prov.baseURL = server.URL
	prov.SetStore(store)

	// Make a call — should persist rate limit and quota
	resp, err := prov.Call(context.TODO(), "gpt-4", "Hello")
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp.RateLimitRemaining != 42 {
		t.Errorf("Expected RateLimitRemaining 42, got %d", resp.RateLimitRemaining)
	}

	// Verify data was persisted to the store
	rl, err := store.GetRateLimit("openai")
	if err != nil {
		t.Fatalf("GetRateLimit failed: %v", err)
	}
	if rl.RequestsRemaining == nil || *rl.RequestsRemaining != 42 {
		t.Errorf("Expected persisted RequestsRemaining 42, got %v", rl.RequestsRemaining)
	}
	if rl.RequestsLimit == nil || *rl.RequestsLimit != 100 {
		t.Errorf("Expected persisted RequestsLimit 100, got %v", rl.RequestsLimit)
	}

	qi, err := store.GetQuota("openai")
	if err != nil {
		t.Fatalf("GetQuota failed: %v", err)
	}
	if qi.TokensRemaining == nil || *qi.TokensRemaining != 5000 {
		t.Errorf("Expected persisted TokensRemaining 5000, got %v", qi.TokensRemaining)
	}
	if qi.TokensLimit == nil || *qi.TokensLimit != 10000 {
		t.Errorf("Expected persisted TokensLimit 10000, got %v", qi.TokensLimit)
	}
}

func TestOpenAIProvider_GetRateLimitInfo_FromStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := state.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Seed the store with data
	remaining := 77
	limit := 200
	now := time.Now()
	stateRL := &state.RateLimitInfo{
		Provider:          "openai",
		RequestsRemaining: &remaining,
		RequestsLimit:     &limit,
		ResetAt:           &now,
		CheckedAt:         now,
	}
	if err := store.SaveRateLimit("openai", stateRL); err != nil {
		t.Fatalf("SaveRateLimit failed: %v", err)
	}

	prov := NewOpenAIProvider()
	prov.SetStore(store)

	info, err := prov.GetRateLimitInfo()
	if err != nil {
		t.Fatalf("GetRateLimitInfo failed: %v", err)
	}
	if info.RequestsRemaining != 77 {
		t.Errorf("Expected RequestsRemaining 77, got %d", info.RequestsRemaining)
	}
	if info.RequestsLimit != 200 {
		t.Errorf("Expected RequestsLimit 200, got %d", info.RequestsLimit)
	}
}

func TestOpenAIProvider_GetQuotaInfo_FromStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := state.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Seed the store with data
	tokensRemaining := 9000
	tokensLimit := 50000
	costRemaining := 4.50
	costLimit := 10.0
	stateQI := &state.QuotaInfo{
		Provider:        "openai",
		TokensRemaining: &tokensRemaining,
		TokensLimit:     &tokensLimit,
		CostRemaining:   &costRemaining,
		CostLimit:       &costLimit,
		ResetAt:         time.Now(),
		CheckedAt:       time.Now(),
	}
	if err := store.SaveQuota("openai", stateQI); err != nil {
		t.Fatalf("SaveQuota failed: %v", err)
	}

	prov := NewOpenAIProvider()
	prov.SetStore(store)

	info, err := prov.GetQuotaInfo()
	if err != nil {
		t.Fatalf("GetQuotaInfo failed: %v", err)
	}
	if info.TokensRemaining != 9000 {
		t.Errorf("Expected TokensRemaining 9000, got %d", info.TokensRemaining)
	}
	if info.TokensLimit != 50000 {
		t.Errorf("Expected TokensLimit 50000, got %d", info.TokensLimit)
	}
	if info.CostRemaining != 4.50 {
		t.Errorf("Expected CostRemaining 4.50, got %f", info.CostRemaining)
	}
	if info.CostLimit != 10.0 {
		t.Errorf("Expected CostLimit 10.0, got %f", info.CostLimit)
	}
}

func TestOpenAIProvider_GetRateLimitInfo_FallbackToMemory(t *testing.T) {
	prov := NewOpenAIProvider()
	// No store set — should fall back to in-memory cache

	// First call with no cache — should return empty info
	info, err := prov.GetRateLimitInfo()
	if err != nil {
		t.Fatalf("GetRateLimitInfo failed: %v", err)
	}
	if info.RequestsRemaining != 0 || info.RequestsLimit != 0 {
		t.Errorf("Expected zero values for uncached provider, got remaining=%d limit=%d",
			info.RequestsRemaining, info.RequestsLimit)
	}

	// Simulate caching via a Call
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining-Requests", "33")
		w.Header().Set("X-RateLimit-Limit-Requests", "60")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(openAIResponse{
			ID: "chatcmpl-test",
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
					}{"assistant", "Hi"},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{5, 2, 7},
		})
	}))
	defer server.Close()

	prov.Authenticate("sk-test123")
	prov.baseURL = server.URL

	_, err = prov.Call(context.TODO(), "gpt-4", "Hi")
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	// Now the in-memory cache should be populated
	info, err = prov.GetRateLimitInfo()
	if err != nil {
		t.Fatalf("GetRateLimitInfo failed: %v", err)
	}
	if info.RequestsRemaining != 33 {
		t.Errorf("Expected in-memory cached RequestsRemaining 33, got %d", info.RequestsRemaining)
	}
	if info.RequestsLimit != 60 {
		t.Errorf("Expected in-memory cached RequestsLimit 60, got %d", info.RequestsLimit)
	}
}
