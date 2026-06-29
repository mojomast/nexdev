package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewOllamaProvider(t *testing.T) {
	provider := NewOllamaProvider("")
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
	if provider.Name() != "ollama" {
		t.Errorf("expected provider name 'ollama', got '%s'", provider.Name())
	}
	if provider.baseURL != "http://localhost:11434" {
		t.Errorf("expected default baseURL 'http://localhost:11434', got '%s'", provider.baseURL)
	}

	// Test with custom URL
	provider2 := NewOllamaProvider("http://custom:8080")
	if provider2.baseURL != "http://custom:8080" {
		t.Errorf("expected baseURL 'http://custom:8080', got '%s'", provider2.baseURL)
	}
}

func TestOllamaProvider_Authenticate(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(ollamaModelsResponse{Models: []struct {
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
			}{}})
		}
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL)

	// Test successful authentication
	err := provider.Authenticate("")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !provider.IsAuthenticated() {
		t.Error("expected provider to be authenticated")
	}
}

func TestOllamaProvider_ListModels(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			response := ollamaModelsResponse{
				Models: []struct {
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
				}{
					{
						Name:       "llama2:latest",
						ModifiedAt: time.Now(),
						Size:       3825819519,
						Digest:     "abc123",
					},
					{
						Name:       "codellama:latest",
						ModifiedAt: time.Now(),
						Size:       3825819519,
						Digest:     "def456",
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL)
	provider.Authenticate("")

	models, err := provider.ListModels()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}

	expectedModels := map[string]bool{
		"llama2:latest":    false,
		"codellama:latest": false,
	}

	for _, model := range models {
		if model.Provider != "ollama" {
			t.Errorf("expected provider 'ollama', got '%s'", model.Provider)
		}
		if _, ok := expectedModels[model.Name]; ok {
			expectedModels[model.Name] = true
		}
		// Ollama is free (local)
		if model.PriceInput != 0.0 || model.PriceOutput != 0.0 {
			t.Errorf("expected zero prices for local Ollama, got input: %f, output: %f",
				model.PriceInput, model.PriceOutput)
		}
	}

	for name, found := range expectedModels {
		if !found {
			t.Errorf("expected model '%s' not found", name)
		}
	}
}

func TestOllamaProvider_Call(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(ollamaModelsResponse{Models: []struct {
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
			}{}})
			return
		}

		if r.URL.Path == "/api/chat" {
			// Verify request headers
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected Content-Type 'application/json', got '%s'", r.Header.Get("Content-Type"))
			}

			// Send mock response
			response := ollamaResponse{
				Model:     "llama2:latest",
				CreatedAt: time.Now(),
				Message: ollamaMessage{
					Role:    "assistant",
					Content: "Hello! How can I help you today?",
				},
				Done:            true,
				PromptEvalCount: 15,
				EvalCount:       25,
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	// Create provider with test server URL
	provider := NewOllamaProvider(server.URL)
	provider.Authenticate("")

	// Make API call
	resp, err := provider.Call(context.TODO(), "llama2:latest", "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify response
	if resp.Content != "Hello! How can I help you today?" {
		t.Errorf("expected content 'Hello! How can I help you today?', got '%s'", resp.Content)
	}
	if resp.TokensInput != 15 {
		t.Errorf("expected 15 input tokens, got %d", resp.TokensInput)
	}
	if resp.TokensOutput != 25 {
		t.Errorf("expected 25 output tokens, got %d", resp.TokensOutput)
	}
	if resp.Model != "llama2:latest" {
		t.Errorf("expected model 'llama2:latest', got '%s'", resp.Model)
	}
	if resp.Provider != "ollama" {
		t.Errorf("expected provider 'ollama', got '%s'", resp.Provider)
	}
}

func TestOllamaProvider_Stream(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(ollamaModelsResponse{Models: []struct {
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
			}{}})
			return
		}

		if r.URL.Path == "/api/chat" {
			// Verify request headers
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected Content-Type 'application/json', got '%s'", r.Header.Get("Content-Type"))
			}

			// Send streaming response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatal("expected http.ResponseWriter to be http.Flusher")
			}

			// Send chunks
			chunks := []ollamaResponse{
				{
					Model:     "llama2:latest",
					CreatedAt: time.Now(),
					Message:   ollamaMessage{Role: "assistant", Content: "Hello"},
					Done:      false,
				},
				{
					Model:     "llama2:latest",
					CreatedAt: time.Now(),
					Message:   ollamaMessage{Role: "assistant", Content: " there"},
					Done:      false,
				},
				{
					Model:     "llama2:latest",
					CreatedAt: time.Now(),
					Message:   ollamaMessage{Role: "assistant", Content: "!"},
					Done:      true,
				},
			}

			encoder := json.NewEncoder(w)
			for _, chunk := range chunks {
				encoder.Encode(chunk)
				flusher.Flush()
				time.Sleep(10 * time.Millisecond)
			}
		}
	}))
	defer server.Close()

	// Create provider with test server URL
	provider := NewOllamaProvider(server.URL)
	provider.Authenticate("")

	// Make streaming call
	ch, err := provider.Stream(context.TODO(), "llama2:latest", "Hello")
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

func TestOllamaProvider_RateLimitsAndQuotas(t *testing.T) {
	provider := NewOllamaProvider("")

	// Ollama doesn't have rate limits or quotas (local)
	rateLimitInfo, err := provider.GetRateLimitInfo()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if rateLimitInfo != nil {
		t.Error("expected nil rate limit info for local Ollama")
	}

	quotaInfo, err := provider.GetQuotaInfo()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if quotaInfo != nil {
		t.Error("expected nil quota info for local Ollama")
	}
}

func TestOllamaProvider_CallError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(ollamaModelsResponse{Models: []struct {
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
			}{}})
			return
		}

		if r.URL.Path == "/api/chat" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "model not found"}`))
		}
	}))
	defer server.Close()

	// Create provider with test server URL
	provider := NewOllamaProvider(server.URL)
	provider.Authenticate("")
	provider.SetMaxRetries(0) // Don't retry for this test

	// Make API call
	_, err := provider.Call(context.TODO(), "nonexistent:latest", "Hello")
	if err == nil {
		t.Error("expected error, got nil")
	}
}
