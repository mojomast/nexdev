package provider

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	nexerrors "github.com/mojomast/nexdev/internal/errors"
)

const FakeProviderName = "fake"

// FakePromptMatcher selects scripted fake responses by prompt content.
type FakePromptMatcher func(prompt string) bool

// FakeScript is one deterministic scripted interaction. Empty Model and prompt
// matchers act as wildcards. Responses are consumed in order.
type FakeScript struct {
	Name           string
	Model          string
	PromptContains string
	PromptMatch    FakePromptMatcher
	Responses      []FakeResponse
}

// FakeResponse describes a deterministic fake provider call or stream result.
type FakeResponse struct {
	Content            string
	StreamChunks       []string
	Err                error
	Retryable          bool
	TokensInput        int
	TokensOutput       int
	RateLimitRemaining int
	QuotaRemaining     int
	RateLimitInfo      *RateLimitInfo
	QuotaInfo          *QuotaInfo
	Latency            time.Duration
	Timestamp          time.Time
}

// FakeCall records a fake provider invocation for tests and later audit-facing
// assertions. Latency is recorded deterministically; the fake never sleeps.
type FakeCall struct {
	Kind    string
	Model   string
	Prompt  string
	Script  string
	Latency time.Duration
	At      time.Time
}

// FakeProviderOption configures a FakeProvider constructor.
type FakeProviderOption func(*FakeProvider)

// FakeProvider is a deterministic Provider implementation for CI and pipeline
// tests. It is intentionally constructor-only and not registered globally.
type FakeProvider struct {
	mu                 sync.Mutex
	name               string
	authRequired       bool
	authKey            string
	authenticated      bool
	models             []Model
	discoveredModels   []Model
	scripts            []FakeScript
	calls              []FakeCall
	rateLimitInfo      *RateLimitInfo
	quotaInfo          *QuotaInfo
	supportsCodingPlan bool
	now                func() time.Time
}

// NewFakeProvider creates a deterministic fake provider. It is unauthenticated
// only when WithFakeAuthRequired is used.
func NewFakeProvider(opts ...FakeProviderOption) *FakeProvider {
	f := &FakeProvider{
		name:          FakeProviderName,
		authenticated: true,
		models: []Model{{
			Provider:     FakeProviderName,
			Name:         "fake-model",
			DisplayName:  "Fake Model",
			Capabilities: []string{"text", "structured", "stream"},
		}},
		now: func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		opt(f)
	}
	if len(f.discoveredModels) == 0 {
		f.discoveredModels = cloneModels(f.models)
	}
	return f
}

func WithFakeName(name string) FakeProviderOption {
	return func(f *FakeProvider) {
		if name != "" {
			f.name = name
		}
	}
}

func WithFakeAuthRequired(apiKey string) FakeProviderOption {
	return func(f *FakeProvider) {
		f.authRequired = true
		f.authKey = apiKey
		f.authenticated = false
	}
}

func WithFakeModels(models []Model) FakeProviderOption {
	return func(f *FakeProvider) {
		f.models = cloneModels(models)
	}
}

func WithFakeDiscoveredModels(models []Model) FakeProviderOption {
	return func(f *FakeProvider) {
		f.discoveredModels = cloneModels(models)
	}
}

func WithFakeScripts(scripts []FakeScript) FakeProviderOption {
	return func(f *FakeProvider) {
		f.scripts = cloneScripts(scripts)
	}
}

func WithFakeRateLimitInfo(info *RateLimitInfo) FakeProviderOption {
	return func(f *FakeProvider) { f.rateLimitInfo = cloneRateLimitInfo(info) }
}

func WithFakeQuotaInfo(info *QuotaInfo) FakeProviderOption {
	return func(f *FakeProvider) { f.quotaInfo = cloneQuotaInfo(info) }
}

func WithFakeSupportsCodingPlan(supported bool) FakeProviderOption {
	return func(f *FakeProvider) { f.supportsCodingPlan = supported }
}

func WithFakeClock(now func() time.Time) FakeProviderOption {
	return func(f *FakeProvider) {
		if now != nil {
			f.now = now
		}
	}
}

func (f *FakeProvider) Name() string { return f.name }

func (f *FakeProvider) Authenticate(apiKey string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.authRequired {
		f.authenticated = true
		return nil
	}
	if apiKey == "" {
		return fmt.Errorf("fake provider API key cannot be empty")
	}
	if f.authKey != "" && apiKey != f.authKey {
		return fmt.Errorf("fake provider authentication failed")
	}
	f.authenticated = true
	return nil
}

func (f *FakeProvider) IsAuthenticated() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.authenticated
}

func (f *FakeProvider) ListModels() ([]Model, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.requireAuthenticatedLocked(); err != nil {
		return nil, err
	}
	return cloneModels(f.models), nil
}

func (f *FakeProvider) DiscoverModels() ([]Model, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.requireAuthenticatedLocked(); err != nil {
		return nil, err
	}
	return cloneModels(f.discoveredModels), nil
}

func (f *FakeProvider) Call(ctx context.Context, model string, prompt string) (*Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.requireAuthenticatedLocked(); err != nil {
		return nil, err
	}

	response, scriptName, err := f.nextResponseLocked(model, prompt)
	call := FakeCall{Kind: "call", Model: model, Prompt: prompt, Script: scriptName, Latency: response.Latency, At: f.now()}
	f.calls = append(f.calls, call)
	if err != nil {
		return nil, err
	}
	if response.Err != nil {
		return nil, fakeResponseError(response)
	}

	timestamp := response.Timestamp
	if timestamp.IsZero() {
		timestamp = call.At
	}
	return &Response{
		Content:            response.Content,
		TokensInput:        response.TokensInput,
		TokensOutput:       response.TokensOutput,
		Model:              model,
		Provider:           f.name,
		Timestamp:          timestamp.UTC(),
		RateLimitRemaining: response.RateLimitRemaining,
		QuotaRemaining:     response.QuotaRemaining,
		RateLimitInfo:      cloneRateLimitInfo(response.RateLimitInfo),
		QuotaInfo:          cloneQuotaInfo(response.QuotaInfo),
	}, nil
}

func (f *FakeProvider) Stream(ctx context.Context, model string, prompt string) (<-chan string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f.mu.Lock()
	if err := f.requireAuthenticatedLocked(); err != nil {
		f.mu.Unlock()
		return nil, err
	}
	response, scriptName, err := f.nextResponseLocked(model, prompt)
	call := FakeCall{Kind: "stream", Model: model, Prompt: prompt, Script: scriptName, Latency: response.Latency, At: f.now()}
	f.calls = append(f.calls, call)
	f.mu.Unlock()

	if err != nil {
		return nil, err
	}
	if response.Err != nil {
		return nil, fakeResponseError(response)
	}

	out := make(chan string, len(response.StreamChunks))
	for _, chunk := range response.StreamChunks {
		select {
		case <-ctx.Done():
			close(out)
			return out, ctx.Err()
		case out <- chunk:
		}
	}
	close(out)
	return out, nil
}

func (f *FakeProvider) GetRateLimitInfo() (*RateLimitInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneRateLimitInfo(f.rateLimitInfo), nil
}

func (f *FakeProvider) GetQuotaInfo() (*QuotaInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneQuotaInfo(f.quotaInfo), nil
}

func (f *FakeProvider) SupportsCodingPlan() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.supportsCodingPlan
}

func (f *FakeProvider) Calls() []FakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	calls := make([]FakeCall, len(f.calls))
	copy(calls, f.calls)
	return calls
}

func (f *FakeProvider) requireAuthenticatedLocked() error {
	if f.authRequired && !f.authenticated {
		return fmt.Errorf("fake provider is not authenticated")
	}
	return nil
}

func (f *FakeProvider) nextResponseLocked(model, prompt string) (FakeResponse, string, error) {
	for i := range f.scripts {
		script := &f.scripts[i]
		if !script.matches(model, prompt) {
			continue
		}
		if len(script.Responses) == 0 {
			return FakeResponse{}, script.Name, fmt.Errorf("fake provider script %q has no remaining responses", script.Name)
		}
		response := script.Responses[0]
		script.Responses = script.Responses[1:]
		return response, script.Name, nil
	}
	return FakeResponse{}, "", fmt.Errorf("fake provider has no scripted response for model %q", model)
}

func (s FakeScript) matches(model, prompt string) bool {
	if s.Model != "" && s.Model != model {
		return false
	}
	if s.PromptContains != "" && !strings.Contains(prompt, s.PromptContains) {
		return false
	}
	if s.PromptMatch != nil && !s.PromptMatch(prompt) {
		return false
	}
	return true
}

func fakeResponseError(response FakeResponse) error {
	if !response.Retryable {
		return response.Err
	}
	return nexerrors.NewAPIError(response.Err, response.Err.Error(), true)
}

func cloneScripts(scripts []FakeScript) []FakeScript {
	out := make([]FakeScript, len(scripts))
	for i, script := range scripts {
		out[i] = script
		out[i].Responses = make([]FakeResponse, len(script.Responses))
		copy(out[i].Responses, script.Responses)
	}
	return out
}

func cloneModels(models []Model) []Model {
	out := make([]Model, len(models))
	copy(out, models)
	for i := range out {
		out[i].Capabilities = append([]string(nil), out[i].Capabilities...)
	}
	return out
}

func cloneRateLimitInfo(info *RateLimitInfo) *RateLimitInfo {
	if info == nil {
		return nil
	}
	clone := *info
	return &clone
}

func cloneQuotaInfo(info *QuotaInfo) *QuotaInfo {
	if info == nil {
		return nil
	}
	clone := *info
	return &clone
}

var _ Provider = (*FakeProvider)(nil)
