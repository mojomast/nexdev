package config

import (
	"strings"
	"testing"
)

func TestDefaultNexdevConfig(t *testing.T) {
	cfg := DefaultNexdevConfig()
	if cfg.Profile != ProfileDev {
		t.Fatalf("profile = %q, want %q", cfg.Profile, ProfileDev)
	}
	if cfg.Project.StateDir != ".nexdev" {
		t.Fatalf("state dir = %q", cfg.Project.StateDir)
	}
	if cfg.ControlPlane.Bind != "127.0.0.1" || cfg.ControlPlane.Port != 7432 {
		t.Fatalf("controlplane default = %s:%d", cfg.ControlPlane.Bind, cfg.ControlPlane.Port)
	}
	if cfg.ControlPlane.AuthRequired != AuthRequiredAuto {
		t.Fatalf("auth_required = %q", cfg.ControlPlane.AuthRequired)
	}
	if cfg.Security.CommandExecutionDefault != "deny" || cfg.Security.NetworkDefault != "deny" {
		t.Fatalf("unsafe security defaults: %+v", cfg.Security)
	}
	if cfg.Security.ToolPolicyFile != ".nexdev/tool_policy.yaml" {
		t.Fatalf("tool policy path = %q", cfg.Security.ToolPolicyFile)
	}
	if len(cfg.RepoAnalyze.ExcludeGlobs) == 0 {
		t.Fatal("repo analyze excludes are empty")
	}
	if got := cfg.Provider.Stages["develop"]; got != (ProviderSelection{}) {
		t.Fatalf("develop provider stage = %+v, want empty placeholder", got)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default validation failed: %v", err)
	}
}

func TestNexdevConfigProfileValidation(t *testing.T) {
	cfg := DefaultNexdevConfig()
	cfg.Profile = "prod"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "invalid profile") {
		t.Fatalf("Validate() error = %v, want invalid profile", err)
	}
}

func TestNexdevConfigRejectsRemoteBindWithoutAuth(t *testing.T) {
	cfg := DefaultNexdevConfig()
	cfg.ControlPlane.Bind = "0.0.0.0"
	cfg.ControlPlane.AuthRequired = AuthRequiredFalse
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "requires auth") {
		t.Fatalf("Validate() error = %v, want remote bind auth error", err)
	}
}

func TestNexdevConfigAuthAutoResolution(t *testing.T) {
	cfg := DefaultNexdevConfig()
	if cfg.ResolvedAuthRequired() {
		t.Fatal("loopback dev auth_required:auto resolved to true")
	}
	cfg.ControlPlane.Bind = "0.0.0.0"
	if !cfg.ResolvedAuthRequired() {
		t.Fatal("remote bind auth_required:auto resolved to false")
	}
	cfg.ControlPlane.Bind = "127.0.0.1"
	cfg.Profile = ProfileCI
	if !cfg.ResolvedAuthRequired() {
		t.Fatal("ci auth_required:auto resolved to false")
	}
}

func TestLoadNexdevYAMLRejectsUnknownTopLevelKey(t *testing.T) {
	_, err := LoadNexdevYAML([]byte("profile: dev\nwat: true\n"))
	if err == nil || !strings.Contains(err.Error(), "unknown top-level") {
		t.Fatalf("LoadNexdevYAML() error = %v, want unknown top-level key", err)
	}
}

func TestLoadNexdevYAMLAllowsUnknownWithExperimentalOverride(t *testing.T) {
	cfg, err := LoadNexdevYAML([]byte("experimental:\n  allow_unknown_config: true\nwat: true\n"))
	if err != nil {
		t.Fatalf("LoadNexdevYAML() unexpected error: %v", err)
	}
	if !cfg.Experimental.AllowUnknownConfig {
		t.Fatal("experimental allow_unknown_config not loaded")
	}
}
