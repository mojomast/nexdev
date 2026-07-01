package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestOpenRouterAuthenticate(t *testing.T) {
	provider := NewOpenRouterProvider()
	if err := provider.Authenticate(""); err == nil {
		t.Fatal("expected empty key error")
	}
	if provider.IsAuthenticated() {
		t.Fatal("provider authenticated with empty key")
	}
	if err := provider.Authenticate("test-key"); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if !provider.IsAuthenticated() {
		t.Fatal("provider not authenticated")
	}
}

func TestOpenRouterListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/models" {
			t.Fatalf("request = %s %s, want GET /models", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		writeOpenRouterModels(t, w)
	}))
	defer server.Close()

	provider := NewOpenRouterProvider()
	provider.baseURL = server.URL
	provider.Authenticate("test-key")

	models, err := provider.ListModels()
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("len(models) = %d, want 2", len(models))
	}
	if models[0].Provider != "openrouter" || models[0].Name != "openai/gpt-4o-mini" || models[0].DisplayName != "GPT-4o mini" {
		t.Fatalf("models[0] = %#v", models[0])
	}
	if models[0].PriceInput != 0.001 || models[0].PriceOutput != 0.002 {
		t.Fatalf("prices[0] = %f/%f, want 0.001/0.002", models[0].PriceInput, models[0].PriceOutput)
	}
	if models[1].PriceInput != 0.003 || models[1].PriceOutput != 0.004 {
		t.Fatalf("prices[1] = %f/%f, want 0.003/0.004", models[1].PriceInput, models[1].PriceOutput)
	}

	unauthenticated := NewOpenRouterProvider()
	unauthenticated.baseURL = server.URL
	if _, err := unauthenticated.ListModels(); err == nil {
		t.Fatal("expected unauthenticated ListModels error")
	}
}

func TestOpenRouterListModels_BadPricing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{
				"id":   "bad/pricing",
				"name": "Bad Pricing",
				"pricing": map[string]string{
					"prompt":     "bad",
					"completion": "bad",
				},
			}},
		})
	}))
	defer server.Close()

	provider := NewOpenRouterProvider()
	provider.baseURL = server.URL
	provider.Authenticate("test-key")

	models, err := provider.ListModels()
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}
	if models[0].PriceInput != 0 || models[0].PriceOutput != 0 {
		t.Fatalf("prices = %f/%f, want 0/0", models[0].PriceInput, models[0].PriceOutput)
	}
}

func TestOpenRouterCall(t *testing.T) {
	var referer string
	var title string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			t.Fatalf("request = %s %s, want POST /chat/completions", r.Method, r.URL.Path)
		}
		referer = r.Header.Get("HTTP-Referer")
		title = r.Header.Get("X-Title")
		var req openRouterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "openai/gpt-4o-mini" || req.Stream {
			t.Fatalf("request body = %#v", req)
		}
		writeOpenRouterCompletion(t, w, "hello", 7, 11)
	}))
	defer server.Close()

	provider := NewOpenRouterProvider()
	provider.baseURL = server.URL
	provider.Authenticate("test-key")

	resp, err := provider.Call(context.Background(), "openai/gpt-4o-mini", "say hello")
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if resp.Content != "hello" || resp.TokensInput != 7 || resp.TokensOutput != 11 || resp.Model != "openai/gpt-4o-mini" || resp.Provider != "openrouter" {
		t.Fatalf("response = %#v", resp)
	}
	if referer != openRouterReferer || title != openRouterTitle {
		t.Fatalf("metadata headers = %q/%q", referer, title)
	}
}

func TestOpenRouterCall_Unauthenticated(t *testing.T) {
	provider := NewOpenRouterProvider()
	if _, err := provider.Call(context.Background(), "model", "prompt"); err == nil {
		t.Fatal("expected unauthenticated Call error")
	}
}

func TestOpenRouterCall_ServerError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	provider := NewOpenRouterProvider()
	provider.baseURL = server.URL
	provider.Authenticate("test-key")
	provider.SetMaxRetries(1)
	provider.SetBaseDelay(time.Millisecond)

	_, err := provider.Call(context.Background(), "model", "prompt")
	if err == nil {
		t.Fatal("expected server error")
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestOpenRouterCall_Cost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-OpenRouter-Cost", "0.000042")
		writeOpenRouterCompletion(t, w, "hello", 1, 2)
	}))
	defer server.Close()

	provider := NewOpenRouterProvider()
	provider.baseURL = server.URL
	provider.Authenticate("test-key")

	resp, err := provider.Call(context.Background(), "openai/gpt-4o-mini", "say hello")
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if resp.Cost != 0.000042 {
		t.Fatalf("Cost = %f, want 0.000042", resp.Cost)
	}
}

func TestOpenRouterStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("HTTP-Referer") != openRouterReferer || r.Header.Get("X-Title") != openRouterTitle {
			t.Fatalf("missing OpenRouter metadata headers")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, token := range []string{"one", "two", "three"} {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", token)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider := NewOpenRouterProvider()
	provider.baseURL = server.URL
	provider.Authenticate("test-key")

	ch, err := provider.Stream(context.Background(), "openai/gpt-4o-mini", "say hello")
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	var got []string
	for token := range ch {
		got = append(got, token)
	}
	want := []string{"one", "two", "three"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tokens = %#v, want %#v", got, want)
	}
}

func TestOpenRouterDiscoverModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeOpenRouterModels(t, w)
	}))
	defer server.Close()

	provider := NewOpenRouterProvider()
	provider.baseURL = server.URL
	provider.Authenticate("test-key")

	listed, err := provider.ListModels()
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	discovered, err := provider.DiscoverModels()
	if err != nil {
		t.Fatalf("DiscoverModels() error = %v", err)
	}
	if !reflect.DeepEqual(discovered, listed) {
		t.Fatalf("DiscoverModels() = %#v, want %#v", discovered, listed)
	}
}

func writeOpenRouterModels(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"data": []map[string]any{
			{
				"id":   "openai/gpt-4o-mini",
				"name": "GPT-4o mini",
				"pricing": map[string]string{
					"prompt":     "0.000001",
					"completion": "0.000002",
				},
			},
			{
				"id":   "anthropic/claude-3.5-sonnet",
				"name": "Claude 3.5 Sonnet",
				"pricing": map[string]string{
					"prompt":     "0.000003",
					"completion": "0.000004",
				},
			},
		},
	}); err != nil {
		t.Fatalf("encode models: %v", err)
	}
}

func writeOpenRouterCompletion(t *testing.T, w http.ResponseWriter, content string, promptTokens int, completionTokens int) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"id":      "chatcmpl-test",
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   "openai/gpt-4o-mini",
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]string{
				"role":    "assistant",
				"content": content,
			},
			"finish_reason": "stop",
		}},
		"usage": map[string]int{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":      promptTokens + completionTokens,
		},
	}); err != nil {
		t.Fatalf("encode completion: %v", err)
	}
}
