package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/safety"
)

const (
	RealProviderGateEnv     = "NEXDEV_RUN_REAL_PROVIDER_TESTS"
	RealProviderNameEnv     = "NEXDEV_REAL_PROVIDER"
	RealProviderModelEnv    = "NEXDEV_REAL_PROVIDER_MODEL"
	RealProviderAPIKeyEnv   = "NEXDEV_REAL_PROVIDER_API_KEY_ENV"
	RealProviderMaxUSDEnv   = "NEXDEV_REAL_PROVIDER_MAX_USD"
	RealProviderTimeoutEnv  = "NEXDEV_REAL_PROVIDER_TIMEOUT_S"
	defaultRealSmokeTimeout = 15 * time.Second
	maxRealSmokeUSD         = 0.25
)

var ErrRealProviderSmokeSkipped = errors.New("real provider smoke skipped")

type RealProviderSmokeConfig struct {
	Provider  string
	Model     string
	APIKeyEnv string
	MaxUSD    float64
	Timeout   time.Duration
}

type RealProviderSmokeResult struct {
	Provider     string          `json:"provider"`
	Model        string          `json:"model"`
	StructuredOK bool            `json:"structured_ok"`
	Attempts     int             `json:"attempts"`
	Usage        StructuredUsage `json:"usage"`
	EstimatedUSD float64         `json:"estimated_usd,omitempty"`
}

type realSmokeFixture struct {
	OK string `json:"ok"`
}

func RealProviderSmokeConfigFromEnv() (RealProviderSmokeConfig, error) {
	return realProviderSmokeConfigFromLookup(os.LookupEnv)
}

func realProviderSmokeConfigFromLookup(lookup func(string) (string, bool)) (RealProviderSmokeConfig, error) {
	if value, ok := lookup(RealProviderGateEnv); !ok || value != "1" {
		return RealProviderSmokeConfig{}, fmt.Errorf("%w: set %s=1", ErrRealProviderSmokeSkipped, RealProviderGateEnv)
	}
	providerName, ok := lookup(RealProviderNameEnv)
	providerName = strings.TrimSpace(providerName)
	if !ok || providerName == "" {
		return RealProviderSmokeConfig{}, fmt.Errorf("%w: %s is required", ErrRealProviderSmokeSkipped, RealProviderNameEnv)
	}
	model, ok := lookup(RealProviderModelEnv)
	model = strings.TrimSpace(model)
	if !ok || model == "" {
		return RealProviderSmokeConfig{}, fmt.Errorf("%w: %s is required", ErrRealProviderSmokeSkipped, RealProviderModelEnv)
	}
	apiKeyEnv, ok := lookup(RealProviderAPIKeyEnv)
	apiKeyEnv = strings.TrimSpace(apiKeyEnv)
	if !ok || apiKeyEnv == "" {
		apiKeyEnv = defaultAPIKeyEnv(providerName)
	}
	if apiKeyEnv == "" {
		return RealProviderSmokeConfig{}, fmt.Errorf("%w: %s is required for provider %q", ErrRealProviderSmokeSkipped, RealProviderAPIKeyEnv, providerName)
	}
	if key, ok := lookup(apiKeyEnv); !ok || strings.TrimSpace(key) == "" {
		return RealProviderSmokeConfig{}, fmt.Errorf("%w: credential env %s is not set", ErrRealProviderSmokeSkipped, apiKeyEnv)
	}
	capText, ok := lookup(RealProviderMaxUSDEnv)
	if !ok || strings.TrimSpace(capText) == "" {
		return RealProviderSmokeConfig{}, fmt.Errorf("%w: %s is required", ErrRealProviderSmokeSkipped, RealProviderMaxUSDEnv)
	}
	maxUSD, err := strconv.ParseFloat(strings.TrimSpace(capText), 64)
	if err != nil || maxUSD <= 0 || maxUSD > maxRealSmokeUSD {
		return RealProviderSmokeConfig{}, fmt.Errorf("%w: %s must be > 0 and <= %.2f", ErrRealProviderSmokeSkipped, RealProviderMaxUSDEnv, maxRealSmokeUSD)
	}
	timeout := defaultRealSmokeTimeout
	if text, ok := lookup(RealProviderTimeoutEnv); ok && strings.TrimSpace(text) != "" {
		seconds, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || seconds <= 0 || seconds > 30 {
			return RealProviderSmokeConfig{}, fmt.Errorf("%w: %s must be between 1 and 30 seconds", ErrRealProviderSmokeSkipped, RealProviderTimeoutEnv)
		}
		timeout = time.Duration(seconds) * time.Second
	}
	return RealProviderSmokeConfig{Provider: providerName, Model: model, APIKeyEnv: apiKeyEnv, MaxUSD: maxUSD, Timeout: timeout}, nil
}

func RunRealProviderSmoke(ctx context.Context, cfg RealProviderSmokeConfig) (*RealProviderSmokeResult, error) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultRealSmokeTimeout
	}
	if cfg.MaxUSD <= 0 || cfg.MaxUSD > maxRealSmokeUSD {
		return nil, fmt.Errorf("real provider smoke spend cap must be > 0 and <= %.2f", maxRealSmokeUSD)
	}
	key := strings.TrimSpace(os.Getenv(cfg.APIKeyEnv))
	if key == "" {
		return nil, fmt.Errorf("credential env %s is not set", cfg.APIKeyEnv)
	}
	instance, err := CreateProvider(cfg.Provider)
	if err != nil {
		return nil, fmt.Errorf("create provider: %s", safety.RedactSecrets(err.Error()))
	}
	if err := instance.Authenticate(key); err != nil {
		return nil, fmt.Errorf("authenticate provider %q: %s", cfg.Provider, safety.RedactSecrets(err.Error()))
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	models, err := instance.ListModels()
	if err != nil {
		return nil, fmt.Errorf("list provider models: %s", safety.RedactSecrets(err.Error()))
	}
	estimatedInputPrice, estimatedOutputPrice := modelPrices(models, cfg.Model)

	router, err := NewRouterWithRegistry(Selection{Provider: cfg.Provider, Model: cfg.Model}, nil, map[string]ProviderFactory{cfg.Provider: func() Provider { return instance }})
	if err != nil {
		return nil, err
	}
	client := StructuredClient{Router: router, Providers: map[string]Provider{cfg.Provider: instance}}
	prompt := "Return exactly this JSON and no prose: {\"ok\":\"nexdev-real-provider-smoke\"}"
	var got realSmokeFixture
	result, err := client.CallStructured(ctx, SlotInterview, prompt, &got, StructuredOptions{MaxRepairAttempts: 0, Validate: func(candidate any) error {
		fixture, ok := candidate.(*realSmokeFixture)
		if !ok || fixture.OK != "nexdev-real-provider-smoke" {
			return fmt.Errorf("unexpected structured smoke response")
		}
		return nil
	}})
	if err != nil {
		return nil, err
	}
	estimatedUSD := estimateUSD(result.Usage, estimatedInputPrice, estimatedOutputPrice)
	if estimatedUSD > cfg.MaxUSD {
		return nil, fmt.Errorf("real provider smoke estimated cost %.6f exceeds cap %.6f", estimatedUSD, cfg.MaxUSD)
	}
	return &RealProviderSmokeResult{Provider: result.Provider, Model: result.Model, StructuredOK: true, Attempts: result.Attempts, Usage: result.Usage, EstimatedUSD: estimatedUSD}, nil
}

func defaultAPIKeyEnv(providerName string) string {
	switch strings.ToLower(providerName) {
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "openai", "openai-codex":
		return "OPENAI_API_KEY"
	case "requesty":
		return "REQUESTY_API_KEY"
	case "zai":
		return "ZAI_API_KEY"
	case "kimi":
		return "KIMI_API_KEY"
	case "ollama":
		return "OLLAMA_API_KEY"
	default:
		return ""
	}
}

func modelPrices(models []Model, model string) (float64, float64) {
	for _, candidate := range models {
		if candidate.Name == model {
			return candidate.PriceInput, candidate.PriceOutput
		}
	}
	return 0, 0
}

func estimateUSD(usage StructuredUsage, inputPerThousand, outputPerThousand float64) float64 {
	if inputPerThousand <= 0 && outputPerThousand <= 0 {
		return 0
	}
	return float64(usage.PromptTokens)/1000*inputPerThousand + float64(usage.CompletionTokens)/1000*outputPerThousand
}
