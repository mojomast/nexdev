package provider

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type structuredFixture struct {
	Name string `json:"name"`
}

func TestCallStructuredValidDecode(t *testing.T) {
	client := newStructuredTestClient(&scriptedProvider{responses: []string{`{"name":"ok"}`}})

	var got structuredFixture
	result, err := client.CallStructured(context.Background(), SlotInterview, "return json", &got, StructuredOptions{})
	if err != nil {
		t.Fatalf("CallStructured failed: %v", err)
	}
	if got.Name != "ok" {
		t.Fatalf("decoded name = %q", got.Name)
	}
	if result.RawResponse != `{"name":"ok"}` || result.Attempts != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestCallStructuredRejectsUnknownFields(t *testing.T) {
	client := newStructuredTestClient(&scriptedProvider{responses: []string{`{"name":"ok","extra":true}`}})

	var got structuredFixture
	result, err := client.CallStructured(context.Background(), SlotInterview, "return json", &got, StructuredOptions{MaxRepairAttempts: 0})
	if err == nil {
		t.Fatal("expected unknown field error")
	}
	if result == nil || len(result.ValidationErrors) != 1 || !strings.Contains(result.ValidationErrors[0], "unknown field") {
		t.Fatalf("missing unknown field validation error: %#v", result)
	}
}

func TestCallStructuredRepairSuccess(t *testing.T) {
	provider := &scriptedProvider{responses: []string{`{"name":"ok","extra":true}`, `{"name":"fixed"}`}}
	client := newStructuredTestClient(provider)

	var got structuredFixture
	result, err := client.CallStructured(context.Background(), SlotInterview, "return json", &got, StructuredOptions{MaxRepairAttempts: 1})
	if err != nil {
		t.Fatalf("CallStructured repair failed: %v", err)
	}
	if got.Name != "fixed" || result.Attempts != 2 {
		t.Fatalf("repair mismatch got=%#v result=%#v", got, result)
	}
	if !strings.Contains(provider.prompts[1], "unknown field") {
		t.Fatalf("repair prompt missing validation context: %q", provider.prompts[1])
	}
}

func TestCallStructuredRepairCapFailure(t *testing.T) {
	client := newStructuredTestClient(&scriptedProvider{responses: []string{`not json`, `also not json`}})

	var got structuredFixture
	result, err := client.CallStructured(context.Background(), SlotInterview, "return json", &got, StructuredOptions{MaxRepairAttempts: 1})
	if err == nil {
		t.Fatal("expected capped repair failure")
	}
	if result.Attempts != 2 {
		t.Fatalf("attempts = %d, want 2", result.Attempts)
	}
	if len(result.ValidationErrors) != 2 {
		t.Fatalf("validation error count = %d, want 2", len(result.ValidationErrors))
	}
}

func TestCallStructuredSemanticValidationFailure(t *testing.T) {
	client := newStructuredTestClient(&scriptedProvider{responses: []string{`{"name":"bad"}`}})

	var got structuredFixture
	result, err := client.CallStructured(context.Background(), SlotInterview, "return json", &got, StructuredOptions{
		MaxRepairAttempts: 0,
		Validate: func(candidate any) error {
			if candidate.(*structuredFixture).Name == "bad" {
				return errors.New("name cannot be bad")
			}
			return nil
		},
	})
	if err == nil {
		t.Fatal("expected semantic validation error")
	}
	if got.Name != "" {
		t.Fatalf("destination was updated after failed validation: %#v", got)
	}
	if result == nil || len(result.ValidationErrors) != 1 || result.ValidationErrors[0] != "name cannot be bad" {
		t.Fatalf("unexpected validation errors: %#v", result)
	}
}

func TestCallStructuredUsesResolvedSlotModel(t *testing.T) {
	provider := &scriptedProvider{responses: []string{`{"name":"ok"}`}}
	client := newStructuredTestClient(provider)

	var got structuredFixture
	result, err := client.CallStructured(context.Background(), SlotReview, "return json", &got, StructuredOptions{})
	if err != nil {
		t.Fatalf("CallStructured failed: %v", err)
	}
	if provider.models[0] != "review-model" {
		t.Fatalf("model = %q, want review-model", provider.models[0])
	}
	if result.Provider != "primary" || result.Model != "review-model" {
		t.Fatalf("route metadata mismatch: %#v", result)
	}
}

func TestCallStructuredCapturesUsageMetadata(t *testing.T) {
	provider := &scriptedProvider{responses: []string{`{"name":"ok"}`}, inputTokens: 7, outputTokens: 11}
	client := newStructuredTestClient(provider)

	var got structuredFixture
	result, err := client.CallStructured(context.Background(), SlotInterview, "return json", &got, StructuredOptions{})
	if err != nil {
		t.Fatalf("CallStructured failed: %v", err)
	}
	if result.Usage.PromptTokens != 7 || result.Usage.CompletionTokens != 11 || result.Usage.TotalTokens != 18 {
		t.Fatalf("usage mismatch: %#v", result.Usage)
	}
}

func TestCallStructuredRedactsProviderErrors(t *testing.T) {
	client := newStructuredTestClient(&scriptedProvider{err: errors.New("api_key=sk-ant-supersecretsecret")})

	var got structuredFixture
	_, err := client.CallStructured(context.Background(), SlotInterview, "return json", &got, StructuredOptions{})
	if err == nil {
		t.Fatal("expected provider error")
	}
	if strings.Contains(err.Error(), "sk-ant-supersecretsecret") || !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("provider error was not redacted: %v", err)
	}
}

func newStructuredTestClient(provider Provider) StructuredClient {
	router, err := NewRouterWithRegistry(
		Selection{Provider: "primary", Model: "primary-model"},
		map[Slot]Selection{SlotReview: {Model: "review-model"}},
		testProviderRegistry(),
	)
	if err != nil {
		panic(err)
	}
	return StructuredClient{Router: router, Providers: map[string]Provider{"primary": provider}}
}

type scriptedProvider struct {
	responses    []string
	err          error
	inputTokens  int
	outputTokens int
	prompts      []string
	models       []string
}

func (p *scriptedProvider) Name() string { return "primary" }

func (p *scriptedProvider) Authenticate(apiKey string) error { return nil }

func (p *scriptedProvider) IsAuthenticated() bool { return true }

func (p *scriptedProvider) ListModels() ([]Model, error) { return nil, nil }

func (p *scriptedProvider) DiscoverModels() ([]Model, error) { return nil, nil }

func (p *scriptedProvider) Call(ctx context.Context, model string, prompt string) (*Response, error) {
	p.models = append(p.models, model)
	p.prompts = append(p.prompts, prompt)
	if p.err != nil {
		return nil, p.err
	}
	if len(p.responses) == 0 {
		return nil, errors.New("no scripted response")
	}
	content := p.responses[0]
	p.responses = p.responses[1:]
	return &Response{Content: content, TokensInput: p.inputTokens, TokensOutput: p.outputTokens, Model: model, Provider: p.Name(), Timestamp: time.Now()}, nil
}

func (p *scriptedProvider) Stream(ctx context.Context, model string, prompt string) (<-chan string, error) {
	return nil, nil
}

func (p *scriptedProvider) GetRateLimitInfo() (*RateLimitInfo, error) { return nil, nil }

func (p *scriptedProvider) GetQuotaInfo() (*QuotaInfo, error) { return nil, nil }

func (p *scriptedProvider) SupportsCodingPlan() bool { return false }
