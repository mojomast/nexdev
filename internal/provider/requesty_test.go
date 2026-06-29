package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewRequestyProvider(t *testing.T) {
	provider := NewRequestyProvider()

	if provider == nil {
		t.Fatal("NewRequestyProvider returned nil")
	}
	if provider.Name() != "requesty" {
		t.Errorf("Expected name 'requesty', got '%s'", provider.Name())
	}
	if provider.IsAuthenticated() {
		t.Error("Expected provider to not be authenticated initially")
	}
}

func TestRequestyProvider_Authenticate(t *testing.T) {
	provider := NewRequestyProvider()

	err := provider.Authenticate("test-api-key")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if !provider.IsAuthenticated() {
		t.Error("Expected provider to be authenticated")
	}
}

func TestRequestyProvider_ListModels(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("Expected path '/v1/models', got '%s'", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("Missing or incorrect Authorization header")
		}

		resp := requestyModelsResponse{
			Data: []struct {
				ID      string `json:"id"`
				Object  string `json:"object"`
				Created int64  `json:"created"`
				OwnedBy string `json:"owned_by"`
			}{
				{ID: "requesty-model-1", Object: "model", Created: time.Now().Unix(), OwnedBy: "requesty"},
				{ID: "requesty-model-2", Object: "model", Created: time.Now().Unix(), OwnedBy: "requesty"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewRequestyProvider()
	provider.baseURL = server.URL + "/v1"
	provider.Authenticate("test-key")

	models, err := provider.ListModels()
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}
	if models[0].Name != "requesty-model-1" {
		t.Errorf("Expected model name 'requesty-model-1', got '%s'", models[0].Name)
	}
	if models[0].Provider != "requesty" {
		t.Errorf("Expected provider 'requesty', got '%s'", models[0].Provider)
	}
}

func TestRequestyProvider_ListModels_NotAuthenticated(t *testing.T) {
	provider := NewRequestyProvider()

	_, err := provider.ListModels()
	if err == nil {
		t.Error("Expected error when not authenticated")
	}
}

func TestRequestyProvider_Call(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("Expected path '/v1/chat/completions', got '%s'", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("Missing or incorrect Authorization header")
		}

		var req requestyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.Model != "test-model" {
			t.Errorf("Expected model 'test-model', got '%s'", req.Model)
		}
		if len(req.Messages) != 1 || req.Messages[0].Content != "test prompt" {
			t.Error("Unexpected messages in request")
		}

		resp := requestyResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "test-model",
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
						Content: "test response",
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     15,
				CompletionTokens: 25,
				TotalTokens:      40,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Remaining", "150")
		w.Header().Set("X-Quota-Remaining", "5000")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewRequestyProvider()
	provider.baseURL = server.URL + "/v1"
	provider.Authenticate("test-key")

	response, err := provider.Call(context.TODO(), "test-model", "test prompt")
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	if response.Content != "test response" {
		t.Errorf("Expected content 'test response', got '%s'", response.Content)
	}
	if response.TokensInput != 15 {
		t.Errorf("Expected 15 input tokens, got %d", response.TokensInput)
	}
	if response.TokensOutput != 25 {
		t.Errorf("Expected 25 output tokens, got %d", response.TokensOutput)
	}
	if response.Model != "test-model" {
		t.Errorf("Expected model 'test-model', got '%s'", response.Model)
	}
	if response.Provider != "requesty" {
		t.Errorf("Expected provider 'requesty', got '%s'", response.Provider)
	}
	if response.RateLimitRemaining != 150 {
		t.Errorf("Expected rate limit remaining 150, got %d", response.RateLimitRemaining)
	}
	if response.QuotaRemaining != 5000 {
		t.Errorf("Expected quota remaining 5000, got %d", response.QuotaRemaining)
	}
}

func TestRequestyProvider_Call_NotAuthenticated(t *testing.T) {
	provider := NewRequestyProvider()

	_, err := provider.Call(context.TODO(), "test-model", "test prompt")
	if err == nil {
		t.Error("Expected error when not authenticated")
	}
}

func TestRequestyProvider_Call_ServerError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	provider := NewRequestyProvider()
	provider.baseURL = server.URL + "/v1"
	provider.Authenticate("test-key")
	provider.SetMaxRetries(1) // Reduce retries for faster test

	_, err := provider.Call(context.TODO(), "test-model", "test prompt")
	if err == nil {
		t.Error("Expected error for server error")
	}
}

func TestRequestyProvider_GetRateLimitInfo(t *testing.T) {
	provider := NewRequestyProvider()

	info, err := provider.GetRateLimitInfo()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if info == nil {
		t.Error("Expected non-nil rate limit info")
	}
}

func TestRequestyProvider_GetQuotaInfo(t *testing.T) {
	provider := NewRequestyProvider()

	info, err := provider.GetQuotaInfo()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if info == nil {
		t.Error("Expected non-nil quota info")
	}
}

func TestRequestyProvider_SupportsCodingPlan(t *testing.T) {
	provider := NewRequestyProvider()

	if provider.SupportsCodingPlan() {
		t.Error("Expected false for Requesty.ai provider")
	}
}
