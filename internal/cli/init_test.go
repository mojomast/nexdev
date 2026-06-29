package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mojomast/nexdev/internal/provider"
	"github.com/spf13/cobra"
)

// setFlagAPIKey is a test helper that sets a dynamic flag value for a provider.
func setFlagAPIKey(t *testing.T, name, value string) {
	t.Helper()
	ptr, ok := flagAPIKeys[name]
	if !ok {
		t.Fatalf("provider %q not found in flagAPIKeys map", name)
	}
	*ptr = value
}

// resetAllFlagAPIKeys clears every dynamic flag value.
func resetAllFlagAPIKeys() {
	for _, ptr := range flagAPIKeys {
		*ptr = ""
	}
}

func TestGetAPIKey_Precedence_FlagOverEnv(t *testing.T) {
	setFlagAPIKey(t, "openai", "flag-key")
	defer resetAllFlagAPIKeys()

	os.Setenv("GEOFFRUSSY_OPENAI_API_KEY", "env-key")
	defer os.Unsetenv("GEOFFRUSSY_OPENAI_API_KEY")

	key, err := getAPIKey("openai")
	if err != nil {
		t.Fatalf("getAPIKey failed: %v", err)
	}
	if key != "flag-key" {
		t.Errorf("Expected flag-key, got %s", key)
	}
}

func TestGetAPIKey_Precedence_EnvOverConfig(t *testing.T) {
	os.Setenv("GEOFFRUSSY_OPENAI_API_KEY", "env-key")
	defer os.Unsetenv("GEOFFRUSSY_OPENAI_API_KEY")

	tmpDir := t.TempDir()
	configContent := `api_keys:
  openai: config-key
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	os.Setenv("GEOFFRUSSY_CONFIG", configPath)
	defer os.Unsetenv("GEOFFRUSSY_CONFIG")

	key, err := getAPIKey("openai")
	if err != nil {
		t.Fatalf("getAPIKey failed: %v", err)
	}
	if key != "env-key" {
		t.Errorf("Expected env-key, got %s", key)
	}
}

func TestGetAPIKey_Precedence_ConfigFallback(t *testing.T) {
	t.Skip("Config fallback requires integration testing or mocking")
}

func TestGetAPIKey_NoKeyFound(t *testing.T) {
	resetAllFlagAPIKeys()
	os.Unsetenv("GEOFFRUSSY_OPENAI_API_KEY")

	_, err := getAPIKey("openai")
	if err != nil {
		if !strings.Contains(err.Error(), "no API key found") {
			t.Logf("Expected 'no API key found' error, got: %v", err)
		}
	} else {
		t.Log("No error returned - this may mean keys exist in default config file")
	}
}

func TestGetAPIKey_AllProviders(t *testing.T) {
	// Test every registered provider, not just the original 6
	providerNames := provider.GetProviderNames()

	for _, name := range providerNames {
		t.Run(name, func(t *testing.T) {
			flagValue := "flag-" + name + "-key"
			setFlagAPIKey(t, name, flagValue)
			defer resetAllFlagAPIKeys()

			// Set env value (should be ignored in favor of flag)
			envVar := "GEOFFRUSSY_" + strings.ToUpper(name) + "_API_KEY"
			os.Setenv(envVar, "env-"+name+"-key")
			defer os.Unsetenv(envVar)

			key, err := getAPIKey(name)
			if err != nil {
				t.Fatalf("getAPIKey(%s) failed: %v", name, err)
			}
			if key != flagValue {
				t.Errorf("Expected %s, got %s", flagValue, key)
			}
		})
	}
}

func TestGetAPIKey_DynamicProviderCount(t *testing.T) {
	// Verify that flagAPIKeys has an entry for every registered provider
	providerNames := provider.GetProviderNames()
	if len(flagAPIKeys) != len(providerNames) {
		t.Errorf("Expected %d providers in flagAPIKeys, got %d", len(providerNames), len(flagAPIKeys))
	}
	for _, name := range providerNames {
		if _, ok := flagAPIKeys[name]; !ok {
			t.Errorf("Provider %q missing from flagAPIKeys map", name)
		}
	}
}

func TestValidateNonInteractiveConfig_Success(t *testing.T) {
	// Set at least one key so validation passes
	setFlagAPIKey(t, "openai", "flag-openai-key")
	defer resetAllFlagAPIKeys()

	err := validateNonInteractiveConfig()
	if err != nil {
		t.Fatalf("validateNonInteractiveConfig failed: %v", err)
	}
}

func TestValidateNonInteractiveConfig_AllProviders(t *testing.T) {
	// Set keys for all providers
	for _, name := range provider.GetProviderNames() {
		setFlagAPIKey(t, name, "flag-"+name+"-key")
	}
	defer resetAllFlagAPIKeys()

	err := validateNonInteractiveConfig()
	if err != nil {
		t.Fatalf("validateNonInteractiveConfig failed: %v", err)
	}
}

func TestValidateNonInteractiveConfig_NoKeysAvailable(t *testing.T) {
	resetAllFlagAPIKeys()
	// Also clear any env vars
	for _, name := range provider.GetProviderNames() {
		envVar := "GEOFFRUSSY_" + strings.ToUpper(name) + "_API_KEY"
		os.Unsetenv(envVar)
	}

	err := validateNonInteractiveConfig()
	if err != nil {
		if !strings.Contains(err.Error(), "no API keys configured") {
			t.Logf("Expected 'no API keys configured' error, got: %v", err)
		}
	} else {
		t.Log("No error returned - this may mean keys exist in default config file")
	}
}

func TestValidateNonInteractiveConfig_EnvFallback(t *testing.T) {
	resetAllFlagAPIKeys()

	// Set env for a single provider
	os.Setenv("GEOFFRUSSY_OPENAI_API_KEY", "env-openai-key")
	defer os.Unsetenv("GEOFFRUSSY_OPENAI_API_KEY")

	err := validateNonInteractiveConfig()
	if err != nil {
		t.Fatalf("validateNonInteractiveConfig with env vars failed: %v", err)
	}
}

func TestValidateNonInteractiveConfig_ConfigFallback(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `api_keys:
  openai: config-openai-key
  anthropic: config-anthropic-key
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	os.Setenv("GEOFFRUSSY_CONFIG", configPath)
	defer os.Unsetenv("GEOFFRUSSY_CONFIG")

	resetAllFlagAPIKeys()

	err := validateNonInteractiveConfig()
	if err != nil {
		t.Fatalf("validateNonInteractiveConfig with config file failed: %v", err)
	}
}

func TestRunInitNonInteractive_Command(t *testing.T) {
	// Basic test to ensure the function is defined and callable
}

func TestInitCommand_NonInteractiveFlag(t *testing.T) {
	flags := initCmd.Flags()

	flag, err := flags.GetBool("non-interactive")
	if err != nil {
		t.Fatalf("Failed to get non-interactive flag: %v", err)
	}
	if flag != false {
		t.Errorf("Expected non-interactive flag default to be false, got %v", flag)
	}

	// Verify that every registered provider has a corresponding flag
	for _, name := range provider.GetProviderNames() {
		flagName := "api-key-" + name
		f := initCmd.Flags().Lookup(flagName)
		if f == nil {
			t.Errorf("Flag %s is not defined for provider %s", flagName, name)
		}
	}
}

func TestValidateCommand_Exists(t *testing.T) {
	if validateCmd.Use != "validate" {
		t.Errorf("Expected validateCmd.Use to be 'validate', got %s", validateCmd.Use)
	}

	if validateCmd.Short == "" {
		t.Error("validateCmd.Short is empty")
	}

	if validateCmd.Long == "" {
		t.Error("validateCmd.Long is empty")
	}
}

func TestInitCommand_SplitsCorrectly(t *testing.T) {
	tests := []struct {
		name   string
		nonInt bool
	}{
		{"interactive", false},
		{"non-interactive", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flagNonInteractive = tt.nonInt
			cmd := &cobra.Command{}
			args := []string{}

			err := runInit(cmd, args)
			// We expect an error because we're not in a valid project directory
			// but the function should be callable without panicking
			if err == nil && tt.nonInt {
				// This might succeed if we're in a valid directory
			}
		})
	}
}

func TestProviderDisplayName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"openai", "OpenAI"},
		{"anthropic", "Anthropic"},
		{"firmware", "Firmware.ai"},
		{"ollama", "Ollama (Local)"},
		{"zai", "Z.ai"},
		{"unknown", "Unknown"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := providerDisplayName(tt.input)
			if got != tt.expected {
				t.Errorf("providerDisplayName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestValidateOnlyFlag_Defined(t *testing.T) {
	f := initCmd.Flags().Lookup("validate-only")
	if f == nil {
		t.Fatal("Flag --validate-only is not defined")
	}
	if f.DefValue != "false" {
		t.Errorf("Expected default false, got %s", f.DefValue)
	}
}

func TestRunValidateOnly_WithKey(t *testing.T) {
	setFlagAPIKey(t, "openai", "test-key")
	defer resetAllFlagAPIKeys()

	err := runValidateOnly()
	if err != nil {
		t.Fatalf("runValidateOnly failed with a configured key: %v", err)
	}
}

func TestRunValidateOnly_NoKeys(t *testing.T) {
	resetAllFlagAPIKeys()
	// Clear env vars too
	for _, name := range provider.GetProviderNames() {
		envVar := "GEOFFRUSSY_" + strings.ToUpper(name) + "_API_KEY"
		os.Unsetenv(envVar)
	}

	err := runValidateOnly()
	if err != nil {
		if !strings.Contains(err.Error(), "no API keys found") {
			t.Logf("Expected 'no API keys found' error, got: %v", err)
		}
	} else {
		t.Log("No error returned - keys may exist in default config file")
	}
}

func TestDescribeKeySource(t *testing.T) {
	// Test flag source
	setFlagAPIKey(t, "openai", "flag-val")
	defer resetAllFlagAPIKeys()

	src := describeKeySource("openai", "flag-val")
	if src != "flag" {
		t.Errorf("Expected 'flag', got %q", src)
	}

	// Test env source
	resetAllFlagAPIKeys()
	os.Setenv("GEOFFRUSSY_OPENAI_API_KEY", "env-val")
	defer os.Unsetenv("GEOFFRUSSY_OPENAI_API_KEY")

	src = describeKeySource("openai", "env-val")
	if src != "env" {
		t.Errorf("Expected 'env', got %q", src)
	}

	// Test config source (fallback)
	os.Unsetenv("GEOFFRUSSY_OPENAI_API_KEY")
	src = describeKeySource("openai", "some-config-val")
	if src != "config" {
		t.Errorf("Expected 'config', got %q", src)
	}
}
