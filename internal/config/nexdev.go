package config

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ProfileDev        = "dev"
	ProfileTrustedLAN = "trusted-lan"
	ProfileCI         = "ci"

	AuthRequiredAuto  = "auto"
	AuthRequiredTrue  = "true"
	AuthRequiredFalse = "false"
)

// NexdevConfig is the typed v0.1 config surface used by new Nexdev code.
// It intentionally coexists with the imported geoffrussy Config for M2 compatibility.
type NexdevConfig struct {
	Version       string              `yaml:"version"`
	Profile       string              `yaml:"profile"`
	Project       ProjectConfig       `yaml:"project"`
	ControlPlane  ControlPlaneConfig  `yaml:"controlplane"`
	Security      SecurityConfig      `yaml:"security"`
	RepoAnalyze   RepoAnalyzeConfig   `yaml:"repo_analyze"`
	Provider      ProviderConfig      `yaml:"provider"`
	Cost          CostConfig          `yaml:"cost"`
	Observability ObservabilityConfig `yaml:"observability"`
	Experimental  ExperimentalConfig  `yaml:"experimental,omitempty"`
}

type ProjectConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	StateDir    string `yaml:"state_dir"`
}

type ControlPlaneConfig struct {
	Enabled          bool     `yaml:"enabled"`
	Bind             string   `yaml:"bind"`
	Port             int      `yaml:"port"`
	AllowRemoteBind  bool     `yaml:"allow_remote_bind"`
	AuthRequired     string   `yaml:"auth_required"`
	TokenEnv         string   `yaml:"token_env"`
	CORSAllowOrigins []string `yaml:"cors_allow_origins"`
}

type SecurityConfig struct {
	RejectSymlinkEscape     bool     `yaml:"reject_symlink_escape"`
	ScrubLogs               bool     `yaml:"scrub_logs"`
	ToolPolicyFile          string   `yaml:"tool_policy_file"`
	CommandExecutionDefault string   `yaml:"command_execution_default"`
	NetworkDefault          string   `yaml:"network_default"`
	SecretEnvAllowlist      []string `yaml:"secret_env_allowlist"`
	MaxPromptBytes          int      `yaml:"max_prompt_bytes"`
}

type RepoAnalyzeConfig struct {
	Enabled         bool     `yaml:"enabled"`
	MaxFileBytes    int      `yaml:"max_file_bytes"`
	MaxContextBytes int      `yaml:"max_context_bytes"`
	IncludeGlobs    []string `yaml:"include_globs"`
	ExcludeGlobs    []string `yaml:"exclude_globs"`
}

type ProviderConfig struct {
	Primary         ProviderSelection            `yaml:"primary"`
	Stages          map[string]ProviderSelection `yaml:"stages"`
	RequestTimeoutS int                          `yaml:"request_timeout_s"`
	MaxRetries      int                          `yaml:"max_retries"`
	RetryBaseMS     int                          `yaml:"retry_base_ms"`
	AllowFallback   bool                         `yaml:"allow_fallback"`
}

type ProviderSelection struct {
	Name      string `yaml:"name,omitempty"`
	Model     string `yaml:"model,omitempty"`
	APIKeyEnv string `yaml:"api_key_env,omitempty"`
}

type CostConfig struct {
	Enabled                 bool    `yaml:"enabled"`
	Currency                string  `yaml:"currency"`
	MaxRunUSD               float64 `yaml:"max_run_usd"`
	MaxStageUSD             float64 `yaml:"max_stage_usd"`
	RequireApprovalAboveUSD float64 `yaml:"require_approval_above_usd"`
	EstimateBeforeHivemind  bool    `yaml:"estimate_before_hivemind"`
	StopOnUnknownPrice      bool    `yaml:"stop_on_unknown_price"`
}

type ObservabilityConfig struct {
	LogLevel string     `yaml:"log_level"`
	JSONLogs bool       `yaml:"json_logs"`
	OTel     OTelConfig `yaml:"otel"`
}

type OTelConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Endpoint    string `yaml:"endpoint"`
	ServiceName string `yaml:"service_name"`
}

type ExperimentalConfig struct {
	AllowUnknownConfig bool `yaml:"allow_unknown_config"`
}

func DefaultNexdevConfig() NexdevConfig {
	return NexdevConfig{
		Version: "0.1",
		Profile: ProfileDev,
		Project: ProjectConfig{StateDir: ".nexdev"},
		ControlPlane: ControlPlaneConfig{
			Enabled:          true,
			Bind:             "127.0.0.1",
			Port:             7432,
			AuthRequired:     AuthRequiredAuto,
			TokenEnv:         "NEXDEV_CONTROL_TOKEN",
			CORSAllowOrigins: []string{"http://127.0.0.1:7432"},
		},
		Security: SecurityConfig{
			RejectSymlinkEscape:     true,
			ScrubLogs:               true,
			ToolPolicyFile:          ".nexdev/tool_policy.yaml",
			CommandExecutionDefault: "deny",
			NetworkDefault:          "deny",
			SecretEnvAllowlist:      []string{},
			MaxPromptBytes:          500000,
		},
		RepoAnalyze: RepoAnalyzeConfig{
			Enabled:         true,
			MaxFileBytes:    200000,
			MaxContextBytes: 500000,
			IncludeGlobs:    []string{},
			ExcludeGlobs:    []string{".git/**", "node_modules/**", "vendor/**", "dist/**", "build/**", ".nexdev/**"},
		},
		Provider: ProviderConfig{
			Primary:         ProviderSelection{Name: "anthropic", Model: "claude-sonnet-4-5", APIKeyEnv: "ANTHROPIC_API_KEY"},
			Stages:          defaultProviderStages(),
			RequestTimeoutS: 120,
			MaxRetries:      3,
			RetryBaseMS:     500,
		},
		Cost:          CostConfig{Enabled: true, Currency: "USD", MaxRunUSD: 25, MaxStageUSD: 8, RequireApprovalAboveUSD: 5, EstimateBeforeHivemind: true},
		Observability: ObservabilityConfig{LogLevel: "info", JSONLogs: false, OTel: OTelConfig{Enabled: false, ServiceName: "nexdev"}},
	}
}

func defaultProviderStages() map[string]ProviderSelection {
	stages := []string{"interview", "complexity", "design", "hivemind_voice", "hivemind_synthesis", "validate", "plan_sketch", "plan_detail", "review", "develop", "verify_repair", "handoff"}
	out := make(map[string]ProviderSelection, len(stages))
	for _, stage := range stages {
		out[stage] = ProviderSelection{}
	}
	return out
}

func LoadNexdevYAML(data []byte) (NexdevConfig, error) {
	cfg := DefaultNexdevConfig()
	if len(bytes.TrimSpace(data)) == 0 {
		applyProviderAPIKeyEnvDefaults(&cfg)
		return cfg, nil
	}
	if err := rejectUnknownTopLevelKeys(data); err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse nexdev config: %w", err)
	}
	applyProviderAPIKeyEnvDefaults(&cfg)
	return cfg, cfg.Validate()
}

func applyProviderAPIKeyEnvDefaults(cfg *NexdevConfig) {
	if cfg.Provider.Primary.APIKeyEnv == "" {
		cfg.Provider.Primary.APIKeyEnv = ProviderAPIKeyEnv(cfg.Provider.Primary.Name)
	}
	for name, selection := range cfg.Provider.Stages {
		if selection.APIKeyEnv == "" {
			selection.APIKeyEnv = ProviderAPIKeyEnv(selection.Name)
			cfg.Provider.Stages[name] = selection
		}
	}
}

func ProviderAPIKeyEnv(providerName string) string {
	switch strings.ToLower(strings.TrimSpace(providerName)) {
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "openai", "openai-codex":
		return "OPENAI_API_KEY"
	case "requesty":
		return "REQUESTY_API_KEY"
	case "openrouter":
		return "OPENROUTER_API_KEY"
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

func (c NexdevConfig) Validate() error {
	if c.Project.StateDir == "" {
		return errors.New("project.state_dir is required")
	}
	switch c.Profile {
	case ProfileDev, ProfileTrustedLAN, ProfileCI:
	default:
		return fmt.Errorf("invalid profile %q", c.Profile)
	}
	if c.ControlPlane.Port < 0 || c.ControlPlane.Port > 65535 {
		return fmt.Errorf("controlplane.port out of range: %d", c.ControlPlane.Port)
	}
	if c.ControlPlane.AuthRequired == "" {
		return errors.New("controlplane.auth_required is required")
	}
	switch c.ControlPlane.AuthRequired {
	case AuthRequiredAuto, AuthRequiredTrue, AuthRequiredFalse:
	default:
		return fmt.Errorf("invalid controlplane.auth_required %q", c.ControlPlane.AuthRequired)
	}
	if !c.resolvedAuthRequired() && !isLoopbackBind(c.ControlPlane.Bind) {
		return errors.New("non-loopback controlplane bind requires auth")
	}
	if c.Security.CommandExecutionDefault != "deny" {
		return errors.New("security.command_execution_default must default to deny")
	}
	if c.Security.NetworkDefault != "deny" {
		return errors.New("security.network_default must default to deny")
	}
	if c.Cost.Enabled && c.Cost.Currency == "" {
		return errors.New("cost.currency is required when cost tracking is enabled")
	}
	if c.Observability.OTel.Enabled && c.Observability.OTel.Endpoint == "" {
		return errors.New("observability.otel.endpoint is required when OTel is enabled")
	}
	return nil
}

func (c NexdevConfig) ResolvedAuthRequired() bool {
	return c.resolvedAuthRequired()
}

func (c NexdevConfig) resolvedAuthRequired() bool {
	if c.ControlPlane.AuthRequired == AuthRequiredTrue {
		return true
	}
	if c.ControlPlane.AuthRequired == AuthRequiredFalse {
		return false
	}
	return !isLoopbackBind(c.ControlPlane.Bind) || c.Profile == ProfileTrustedLAN || c.Profile == ProfileCI
}

func isLoopbackBind(bind string) bool {
	bind = strings.TrimSpace(bind)
	if bind == "" || strings.EqualFold(bind, "localhost") {
		return true
	}
	ip := net.ParseIP(bind)
	return ip != nil && ip.IsLoopback()
}

func rejectUnknownTopLevelKeys(data []byte) error {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse nexdev config: %w", err)
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	known := map[string]bool{
		"version": true, "project": true, "profile": true, "provider": true, "pipeline": true,
		"repo_analyze": true, "complexity": true, "structured_outputs": true, "design": true,
		"hivemind": true, "validate": true, "review": true, "develop": true, "verify": true,
		"detour": true, "controlplane": true, "security": true, "cost": true, "git": true,
		"observability": true, "experimental": true,
	}
	allowUnknown := false
	for i := 0; i+1 < len(root.Content[0].Content); i += 2 {
		key := root.Content[0].Content[i].Value
		if key == "experimental" && experimentalAllowsUnknown(root.Content[0].Content[i+1]) {
			allowUnknown = true
		}
	}
	if allowUnknown {
		return nil
	}
	for i := 0; i+1 < len(root.Content[0].Content); i += 2 {
		key := root.Content[0].Content[i].Value
		if !known[key] {
			return fmt.Errorf("unknown top-level config key %q", key)
		}
	}
	return nil
}

func experimentalAllowsUnknown(n *yaml.Node) bool {
	if n == nil || n.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == "allow_unknown_config" && n.Content[i+1].Value == "true" {
			return true
		}
	}
	return false
}
