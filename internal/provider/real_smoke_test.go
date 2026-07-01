package provider

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRealProviderSmokeConfigSkipsByDefault(t *testing.T) {
	_, err := realProviderSmokeConfigFromLookup(func(string) (string, bool) { return "", false })
	if !errors.Is(err, ErrRealProviderSmokeSkipped) {
		t.Fatalf("err = %v, want skip", err)
	}
}

func TestRealProviderSmokeConfigRequiresCredentialAndSpendCap(t *testing.T) {
	env := map[string]string{
		RealProviderGateEnv:   "1",
		RealProviderNameEnv:   "anthropic",
		RealProviderModelEnv:  "claude-3-haiku-20240307",
		RealProviderMaxUSDEnv: "0.01",
	}
	_, err := realProviderSmokeConfigFromLookup(mapLookup(env))
	if !errors.Is(err, ErrRealProviderSmokeSkipped) || !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Fatalf("err = %v, want credential skip", err)
	}

	env["ANTHROPIC_API_KEY"] = "sk-ant-testsecretsecret"
	delete(env, RealProviderMaxUSDEnv)
	_, err = realProviderSmokeConfigFromLookup(mapLookup(env))
	if !errors.Is(err, ErrRealProviderSmokeSkipped) || !strings.Contains(err.Error(), RealProviderMaxUSDEnv) {
		t.Fatalf("err = %v, want spend cap skip", err)
	}
}

func TestRealProviderSmokeConfigRejectsUnsafeCapAndTimeout(t *testing.T) {
	env := map[string]string{
		RealProviderGateEnv:   "1",
		RealProviderNameEnv:   "anthropic",
		RealProviderModelEnv:  "claude-3-haiku-20240307",
		RealProviderMaxUSDEnv: "1.00",
		"ANTHROPIC_API_KEY":   "sk-ant-testsecretsecret",
	}
	_, err := realProviderSmokeConfigFromLookup(mapLookup(env))
	if !errors.Is(err, ErrRealProviderSmokeSkipped) || !strings.Contains(err.Error(), "<= 0.25") {
		t.Fatalf("err = %v, want cap skip", err)
	}

	env[RealProviderMaxUSDEnv] = "0.01"
	env[RealProviderTimeoutEnv] = "31"
	_, err = realProviderSmokeConfigFromLookup(mapLookup(env))
	if !errors.Is(err, ErrRealProviderSmokeSkipped) || !strings.Contains(err.Error(), "between 1 and 30") {
		t.Fatalf("err = %v, want timeout skip", err)
	}
}

func TestOpenRouterSmokeConfigUsesDeepSeekModel(t *testing.T) {
	env := map[string]string{
		OpenRouterSmokeGateEnv: "1",
		RealProviderMaxUSDEnv:  "0.01",
		"OPENROUTER_API_KEY":   "sk-or-testsecretsecret",
	}
	cfg, err := realProviderSmokeConfigFromLookup(mapLookup(env))
	if err != nil {
		t.Fatalf("realProviderSmokeConfigFromLookup() error = %v", err)
	}
	if cfg.Provider != "openrouter" || cfg.Model != "deepseek/deepseek-v4-flash" || cfg.APIKeyEnv != "OPENROUTER_API_KEY" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestRealProviderSmokeRedactsProviderErrors(t *testing.T) {
	client := newStructuredTestClient(&scriptedProvider{err: errors.New("request failed Authorization: Bearer sk-real-secretsecret")})
	var got realSmokeFixture
	_, err := client.CallStructured(context.Background(), SlotInterview, "tiny", &got, StructuredOptions{MaxRepairAttempts: 0})
	if err == nil {
		t.Fatal("expected provider error")
	}
	if strings.Contains(err.Error(), "sk-real-secretsecret") || !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("provider error was not redacted: %v", err)
	}
}

func TestRealProviderSmoke(t *testing.T) {
	cfg, err := RealProviderSmokeConfigFromEnv()
	if errors.Is(err, ErrRealProviderSmokeSkipped) {
		t.Skip(err.Error())
	}
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout+time.Second)
	defer cancel()
	result, err := RunRealProviderSmoke(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !result.StructuredOK || result.Provider == "" || result.Model == "" {
		t.Fatalf("unexpected smoke result: %#v", result)
	}
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
