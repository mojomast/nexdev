package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewFirmwareProvider(t *testing.T) {
	provider := NewFirmwareProvider()

	if provider == nil {
		t.Fatal("NewFirmwareProvider returned nil")
	}
	if provider.Name() != "firmware" {
		t.Errorf("Expected name 'firmware', got '%s'", provider.Name())
	}
	if provider.IsAuthenticated() {
		t.Error("Expected provider to not be authenticated initially")
	}
}

func TestFirmwareProvider_Authenticate(t *testing.T) {
	provider := NewFirmwareProvider()

	err := provider.Authenticate("test-api-key")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if !provider.IsAuthenticated() {
		t.Error("Expected provider to be authenticated")
	}
}

func TestFirmwareProvider_ListModels(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("Expected path '/v1/models', got '%s'", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("Missing or incorrect Authorization header")
		}

		resp := firmwareModelsResponse{
			Data: []struct {
				ID      string `json:"id"`
				Object  string `json:"object"`
				Created int64  `json:"created"`
				OwnedBy string `json:"owned_by"`
			}{
				{ID: "firmware-model-1", Object: "model", Created: time.Now().Unix(), OwnedBy: "firmware"},
				{ID: "firmware-model-2", Object: "model", Created: time.Now().Unix(), OwnedBy: "firmware"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewFirmwareProvider()
	provider.baseURL = server.URL + "/v1"
	provider.Authenticate("test-key")

	models, err := provider.ListModels()
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}
	if models[0].Name != "firmware-model-1" {
		t.Errorf("Expected model name 'firmware-model-1', got '%s'", models[0].Name)
	}
	if models[0].Provider != "firmware" {
		t.Errorf("Expected provider 'firmware', got '%s'", models[0].Provider)
	}
}

func TestFirmwareProvider_ListModels_NotAuthenticated(t *testing.T) {
	provider := NewFirmwareProvider()

	_, err := provider.ListModels()
	if err == nil {
		t.Error("Expected error when not authenticated")
	}
}

func TestFirmwareProvider_Call(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("Expected path '/v1/chat/completions', got '%s'", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("Missing or incorrect Authorization header")
		}

		var req firmwareRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.Model != "test-model" {
			t.Errorf("Expected model 'test-model', got '%s'", req.Model)
		}
		if len(req.Messages) != 1 || req.Messages[0].Content != "test prompt" {
			t.Error("Unexpected messages in request")
		}

		resp := firmwareResponse{
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
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Remaining", "100")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewFirmwareProvider()
	provider.baseURL = server.URL + "/v1"
	provider.Authenticate("test-key")

	response, err := provider.Call(context.TODO(), "test-model", "test prompt")
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	if response.Content != "test response" {
		t.Errorf("Expected content 'test response', got '%s'", response.Content)
	}
	if response.TokensInput != 10 {
		t.Errorf("Expected 10 input tokens, got %d", response.TokensInput)
	}
	if response.TokensOutput != 20 {
		t.Errorf("Expected 20 output tokens, got %d", response.TokensOutput)
	}
	if response.Model != "test-model" {
		t.Errorf("Expected model 'test-model', got '%s'", response.Model)
	}
	if response.Provider != "firmware" {
		t.Errorf("Expected provider 'firmware', got '%s'", response.Provider)
	}
	if response.RateLimitRemaining != 100 {
		t.Errorf("Expected rate limit remaining 100, got %d", response.RateLimitRemaining)
	}
}

func TestFirmwareProvider_Call_NotAuthenticated(t *testing.T) {
	provider := NewFirmwareProvider()

	_, err := provider.Call(context.TODO(), "test-model", "test prompt")
	if err == nil {
		t.Error("Expected error when not authenticated")
	}
}

func TestFirmwareProvider_Call_ServerError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	provider := NewFirmwareProvider()
	provider.baseURL = server.URL + "/v1"
	provider.Authenticate("test-key")
	provider.SetMaxRetries(1) // Reduce retries for faster test

	_, err := provider.Call(context.TODO(), "test-model", "test prompt")
	if err == nil {
		t.Error("Expected error for server error")
	}
}

func TestFirmwareProvider_GetRateLimitInfo(t *testing.T) {
	provider := NewFirmwareProvider()

	info, err := provider.GetRateLimitInfo()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if info == nil {
		t.Error("Expected non-nil rate limit info")
	}
}

func TestFirmwareProvider_GetQuotaInfo(t *testing.T) {
	provider := NewFirmwareProvider()

	info, err := provider.GetQuotaInfo()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if info == nil {
		t.Error("Expected non-nil quota info")
	}
}

func TestFirmwareProvider_SupportsCodingPlan(t *testing.T) {
	provider := NewFirmwareProvider()

	if provider.SupportsCodingPlan() {
		t.Error("Expected false for Firmware.ai provider")
	}
}
