package provider

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	nexerrors "github.com/mojomast/nexdev/internal/errors"
)

type fakeStructuredFixture struct {
	Name string `json:"name"`
}

func TestFakeProviderBasicCallByModelAndPrompt(t *testing.T) {
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	fake := NewFakeProvider(
		WithFakeClock(func() time.Time { return now }),
		WithFakeScripts([]FakeScript{{
			Name:           "interview",
			Model:          "fake-model",
			PromptContains: "interview",
			Responses: []FakeResponse{{
				Content:     `{"name":"ok"}`,
				TokensInput: 7,
				Latency:     25 * time.Millisecond,
			}},
		}}),
	)

	resp, err := fake.Call(context.Background(), "fake-model", "run interview")
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp.Content != `{"name":"ok"}` || resp.Provider != FakeProviderName || resp.Model != "fake-model" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if resp.TokensInput != 7 {
		t.Fatalf("TokensInput = %d, want 7", resp.TokensInput)
	}
	calls := fake.Calls()
	if len(calls) != 1 || calls[0].Script != "interview" || calls[0].Latency != 25*time.Millisecond || !calls[0].At.Equal(now) {
		t.Fatalf("unexpected calls: %#v", calls)
	}
}

func TestFakeProviderStructuredRepairThroughClient(t *testing.T) {
	fake := NewFakeProvider(WithFakeScripts([]FakeScript{{
		Model: "fake-model",
		Responses: []FakeResponse{
			{Content: `{"name":"bad","extra":true}`},
			{Content: `{"name":"fixed"}`},
		},
	}}))
	client := newFakeStructuredClient(fake)

	var got fakeStructuredFixture
	result, err := client.CallStructured(context.Background(), SlotInterview, "return json", &got, StructuredOptions{MaxRepairAttempts: 1})
	if err != nil {
		t.Fatalf("CallStructured failed: %v", err)
	}
	if got.Name != "fixed" || result.Attempts != 2 {
		t.Fatalf("repair mismatch got=%#v result=%#v", got, result)
	}
	calls := fake.Calls()
	if len(calls) != 2 || !strings.Contains(calls[1].Prompt, "unknown field") {
		t.Fatalf("repair prompt was not sent to fake provider: %#v", calls)
	}
}

func TestFakeProviderUnrecoverableInvalidStructuredResponse(t *testing.T) {
	fake := NewFakeProvider(WithFakeScripts([]FakeScript{{
		Responses: []FakeResponse{{Content: `not json`}, {Content: `still not json`}},
	}}))
	client := newFakeStructuredClient(fake)

	var got fakeStructuredFixture
	result, err := client.CallStructured(context.Background(), SlotInterview, "return json", &got, StructuredOptions{MaxRepairAttempts: 1})
	if err == nil {
		t.Fatal("expected structured failure")
	}
	if result == nil || result.Attempts != 2 || len(result.ValidationErrors) != 2 {
		t.Fatalf("unexpected failed result: %#v", result)
	}
	if got.Name != "" {
		t.Fatalf("destination mutated after invalid response: %#v", got)
	}
}

func TestFakeProviderErrorScripting(t *testing.T) {
	retryable := errors.New("rate limit")
	hard := errors.New("hard failure")
	fake := NewFakeProvider(WithFakeScripts([]FakeScript{
		{Name: "retry", PromptContains: "retry", Responses: []FakeResponse{{Err: retryable, Retryable: true}}},
		{Name: "hard", PromptContains: "hard", Responses: []FakeResponse{{Err: hard}}},
	}))

	_, err := fake.Call(context.Background(), "fake-model", "please retry")
	if err == nil || !nexerrors.IsRetryable(err) {
		t.Fatalf("expected retryable error, got %v", err)
	}
	_, err = fake.Call(context.Background(), "fake-model", "please hard fail")
	if err == nil || err.Error() != hard.Error() || nexerrors.IsRetryable(err) {
		t.Fatalf("expected hard error, got %v", err)
	}
}

func TestFakeProviderUsageMetadata(t *testing.T) {
	rl := &RateLimitInfo{RequestsRemaining: 3, RequestsLimit: 10}
	quota := &QuotaInfo{TokensRemaining: 100, TokensLimit: 200}
	fake := NewFakeProvider(WithFakeScripts([]FakeScript{{Responses: []FakeResponse{{
		Content:            `{"name":"ok"}`,
		TokensInput:        5,
		TokensOutput:       8,
		RateLimitRemaining: 3,
		QuotaRemaining:     100,
		RateLimitInfo:      rl,
		QuotaInfo:          quota,
	}}}}))
	client := newFakeStructuredClient(fake)

	var got fakeStructuredFixture
	result, err := client.CallStructured(context.Background(), SlotInterview, "return json", &got, StructuredOptions{})
	if err != nil {
		t.Fatalf("CallStructured failed: %v", err)
	}
	if result.Usage.PromptTokens != 5 || result.Usage.CompletionTokens != 8 || result.Usage.TotalTokens != 13 {
		t.Fatalf("token usage mismatch: %#v", result.Usage)
	}
	if result.Usage.RateLimitRemaining != 3 || result.Usage.QuotaRemaining != 100 {
		t.Fatalf("remaining usage mismatch: %#v", result.Usage)
	}
	if result.Usage.RateLimitInfo == rl || result.Usage.QuotaInfo == quota {
		t.Fatalf("usage metadata pointers were not cloned")
	}
}

func TestFakeProviderStreamingChunks(t *testing.T) {
	fake := NewFakeProvider(WithFakeScripts([]FakeScript{{
		PromptMatch: func(prompt string) bool { return strings.HasPrefix(prompt, "stream") },
		Responses:   []FakeResponse{{StreamChunks: []string{"a", "b", "c"}}},
	}}))

	ch, err := fake.Stream(context.Background(), "fake-model", "stream please")
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}
	var chunks []string
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}
	if !reflect.DeepEqual(chunks, []string{"a", "b", "c"}) {
		t.Fatalf("chunks = %#v", chunks)
	}
	calls := fake.Calls()
	if len(calls) != 1 || calls[0].Kind != "stream" {
		t.Fatalf("stream call not recorded: %#v", calls)
	}
}

func TestFakeProviderModelListingAndDiscovery(t *testing.T) {
	listed := []Model{{Provider: FakeProviderName, Name: "listed"}}
	discovered := []Model{{Provider: FakeProviderName, Name: "discovered", Capabilities: []string{"structured"}}}
	fake := NewFakeProvider(WithFakeModels(listed), WithFakeDiscoveredModels(discovered), WithFakeSupportsCodingPlan(true))

	models, err := fake.ListModels()
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	discoveredModels, err := fake.DiscoverModels()
	if err != nil {
		t.Fatalf("DiscoverModels failed: %v", err)
	}
	if models[0].Name != "listed" || discoveredModels[0].Name != "discovered" || !fake.SupportsCodingPlan() {
		t.Fatalf("model metadata mismatch listed=%#v discovered=%#v", models, discoveredModels)
	}
	discoveredModels[0].Capabilities[0] = "mutated"
	again, err := fake.DiscoverModels()
	if err != nil {
		t.Fatalf("DiscoverModels failed: %v", err)
	}
	if again[0].Capabilities[0] != "structured" {
		t.Fatalf("discovered model slice was not cloned: %#v", again)
	}
}

func TestFakeProviderAuthRequired(t *testing.T) {
	fake := NewFakeProvider(WithFakeAuthRequired("secret"))
	if fake.IsAuthenticated() {
		t.Fatal("fake provider should start unauthenticated when auth is required")
	}
	if _, err := fake.ListModels(); err == nil {
		t.Fatal("expected ListModels auth error")
	}
	if err := fake.Authenticate("wrong"); err == nil {
		t.Fatal("expected wrong key auth error")
	}
	if err := fake.Authenticate("secret"); err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if !fake.IsAuthenticated() {
		t.Fatal("fake provider should be authenticated")
	}
}

func TestFakeProviderNotRegisteredByDefault(t *testing.T) {
	if _, ok := Registry[FakeProviderName]; ok {
		t.Fatal("fake provider must stay disabled by default and out of global registry")
	}
}

func newFakeStructuredClient(fake *FakeProvider) StructuredClient {
	registry := map[string]ProviderFactory{FakeProviderName: func() Provider { return fake }}
	router, err := NewRouterWithRegistry(Selection{Provider: FakeProviderName, Model: "fake-model"}, nil, registry)
	if err != nil {
		panic(err)
	}
	return StructuredClient{Router: router, Providers: map[string]Provider{FakeProviderName: fake}}
}
