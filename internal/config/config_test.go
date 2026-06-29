package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.config == nil {
		t.Fatal("config is nil")
	}
	if m.config.APIKeys == nil {
		t.Fatal("APIKeys map is nil")
	}
	if m.config.DefaultModels == nil {
		t.Fatal("DefaultModels map is nil")
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `api_keys:
  openai: sk-test-key
  anthropic: test-anthropic-key
default_models:
  interview: gpt-4
  design: claude-3
budget_limit: 100.50
verbose_logging: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	m := NewManager()
	if err := m.loadFromFile(configPath); err != nil {
		t.Fatalf("loadFromFile failed: %v", err)
	}

	// Verify loaded values
	if m.config.APIKeys["openai"] != "sk-test-key" {
		t.Errorf("Expected openai key 'sk-test-key', got '%s'", m.config.APIKeys["openai"])
	}
	if m.config.APIKeys["anthropic"] != "test-anthropic-key" {
		t.Errorf("Expected anthropic key 'test-anthropic-key', got '%s'", m.config.APIKeys["anthropic"])
	}
	if m.config.DefaultModels["interview"] != "gpt-4" {
		t.Errorf("Expected interview model 'gpt-4', got '%s'", m.config.DefaultModels["interview"])
	}
	if m.config.DefaultModels["design"] != "claude-3" {
		t.Errorf("Expected design model 'claude-3', got '%s'", m.config.DefaultModels["design"])
	}
	if m.config.BudgetLimit != 100.50 {
		t.Errorf("Expected budget limit 100.50, got %f", m.config.BudgetLimit)
	}
	if !m.config.VerboseLogging {
		t.Error("Expected verbose logging to be true")
	}
}

func TestLoadFromFileNotExist(t *testing.T) {
	m := NewManager()
	err := m.loadFromFile("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
	if !os.IsNotExist(err) {
		t.Errorf("Expected IsNotExist error, got: %v", err)
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Set up environment variables
	os.Setenv("GEOFFRUSSY_API_KEY_OPENAI", "env-openai-key")
	os.Setenv("GEOFFRUSSY_API_KEY_ANTHROPIC", "env-anthropic-key")
	os.Setenv("GEOFFRUSSY_DEFAULT_MODEL_INTERVIEW", "gpt-3.5")
	os.Setenv("GEOFFRUSSY_DEFAULT_MODEL_DESIGN", "claude-2")
	os.Setenv("GEOFFRUSSY_BUDGET_LIMIT", "250.75")
	os.Setenv("GEOFFRUSSY_VERBOSE_LOGGING", "true")
	defer func() {
		os.Unsetenv("GEOFFRUSSY_API_KEY_OPENAI")
		os.Unsetenv("GEOFFRUSSY_API_KEY_ANTHROPIC")
		os.Unsetenv("GEOFFRUSSY_DEFAULT_MODEL_INTERVIEW")
		os.Unsetenv("GEOFFRUSSY_DEFAULT_MODEL_DESIGN")
		os.Unsetenv("GEOFFRUSSY_BUDGET_LIMIT")
		os.Unsetenv("GEOFFRUSSY_VERBOSE_LOGGING")
	}()

	m := NewManager()
	// Use testing-friendly version that works with os.Setenv
	m.loadFromEnvForTesting(
		[]string{"OPENAI", "ANTHROPIC"},
		[]string{"INTERVIEW", "DESIGN"},
	)

	// Verify loaded values
	if m.config.APIKeys["OPENAI"] != "env-openai-key" {
		t.Errorf("Expected OPENAI key 'env-openai-key', got '%s'", m.config.APIKeys["OPENAI"])
	}
	if m.config.APIKeys["ANTHROPIC"] != "env-anthropic-key" {
		t.Errorf("Expected ANTHROPIC key 'env-anthropic-key', got '%s'", m.config.APIKeys["ANTHROPIC"])
	}
	if m.config.DefaultModels["INTERVIEW"] != "gpt-3.5" {
		t.Errorf("Expected INTERVIEW model 'gpt-3.5', got '%s'", m.config.DefaultModels["INTERVIEW"])
	}
	if m.config.DefaultModels["DESIGN"] != "claude-2" {
		t.Errorf("Expected DESIGN model 'claude-2', got '%s'", m.config.DefaultModels["DESIGN"])
	}
	if m.config.BudgetLimit != 250.75 {
		t.Errorf("Expected budget limit 250.75, got %f", m.config.BudgetLimit)
	}
	if !m.config.VerboseLogging {
		t.Error("Expected verbose logging to be true")
	}
}

func TestLoadFromEnvVariousFormats(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"true", "true", true},
		{"1", "1", true},
		{"yes", "yes", true},
		{"false", "false", false},
		{"0", "0", false},
		{"no", "no", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("GEOFFRUSSY_VERBOSE_LOGGING", tt.envValue)
			defer os.Unsetenv("GEOFFRUSSY_VERBOSE_LOGGING")

			m := NewManager()
			m.loadFromEnv()

			if m.config.VerboseLogging != tt.expected {
				t.Errorf("For value '%s', expected %v, got %v", tt.envValue, tt.expected, m.config.VerboseLogging)
			}
		})
	}
}

func TestApplyFlags(t *testing.T) {
	m := NewManager()
	m.config.APIKeys["openai"] = "original-key"
	m.config.DefaultModels["interview"] = "original-model"
	m.config.BudgetLimit = 100.0
	m.config.VerboseLogging = false

	flagConfig := &Config{
		APIKeys: map[string]string{
			"openai":    "flag-key",
			"anthropic": "flag-anthropic-key",
		},
		DefaultModels: map[string]string{
			"interview": "flag-model",
			"design":    "flag-design-model",
		},
		BudgetLimit:    200.0,
		VerboseLogging: true,
	}

	m.applyFlags(flagConfig)

	// Verify flags override existing values
	if m.config.APIKeys["openai"] != "flag-key" {
		t.Errorf("Expected openai key 'flag-key', got '%s'", m.config.APIKeys["openai"])
	}
	if m.config.APIKeys["anthropic"] != "flag-anthropic-key" {
		t.Errorf("Expected anthropic key 'flag-anthropic-key', got '%s'", m.config.APIKeys["anthropic"])
	}
	if m.config.DefaultModels["interview"] != "flag-model" {
		t.Errorf("Expected interview model 'flag-model', got '%s'", m.config.DefaultModels["interview"])
	}
	if m.config.DefaultModels["design"] != "flag-design-model" {
		t.Errorf("Expected design model 'flag-design-model', got '%s'", m.config.DefaultModels["design"])
	}
	if m.config.BudgetLimit != 200.0 {
		t.Errorf("Expected budget limit 200.0, got %f", m.config.BudgetLimit)
	}
	if !m.config.VerboseLogging {
		t.Error("Expected verbose logging to be true")
	}
}

func TestPrecedenceRules(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `api_keys:
  openai: file-key
default_models:
  interview: file-model
budget_limit: 100.0
verbose_logging: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Set environment variables
	os.Setenv("GEOFFRUSSY_API_KEY_OPENAI", "env-key")
	os.Setenv("GEOFFRUSSY_DEFAULT_MODEL_INTERVIEW", "env-model")
	os.Setenv("GEOFFRUSSY_BUDGET_LIMIT", "200.0")
	defer func() {
		os.Unsetenv("GEOFFRUSSY_API_KEY_OPENAI")
		os.Unsetenv("GEOFFRUSSY_DEFAULT_MODEL_INTERVIEW")
		os.Unsetenv("GEOFFRUSSY_BUDGET_LIMIT")
	}()

	m := NewManager()
	m.config.ConfigPath = configPath

	// Load from file
	if err := m.loadFromFile(configPath); err != nil {
		t.Fatalf("loadFromFile failed: %v", err)
	}

	// Load from env (should override file) - use testing version
	m.loadFromEnvForTesting(
		[]string{"OPENAI"},
		[]string{"INTERVIEW"},
	)

	// Verify env overrides file
	if m.config.APIKeys["OPENAI"] != "env-key" {
		t.Errorf("Expected env key 'env-key', got '%s'", m.config.APIKeys["OPENAI"])
	}
	if m.config.DefaultModels["INTERVIEW"] != "env-model" {
		t.Errorf("Expected env model 'env-model', got '%s'", m.config.DefaultModels["INTERVIEW"])
	}
	if m.config.BudgetLimit != 200.0 {
		t.Errorf("Expected budget limit 200.0, got %f", m.config.BudgetLimit)
	}

	// Apply flags (should override env)
	flagConfig := &Config{
		APIKeys: map[string]string{
			"OPENAI": "flag-key",
		},
		DefaultModels: map[string]string{
			"INTERVIEW": "flag-model",
		},
		BudgetLimit: 300.0,
	}
	m.applyFlags(flagConfig)

	// Verify flags override env
	if m.config.APIKeys["OPENAI"] != "flag-key" {
		t.Errorf("Expected flag key 'flag-key', got '%s'", m.config.APIKeys["OPENAI"])
	}
	if m.config.DefaultModels["INTERVIEW"] != "flag-model" {
		t.Errorf("Expected flag model 'flag-model', got '%s'", m.config.DefaultModels["INTERVIEW"])
	}
	if m.config.BudgetLimit != 300.0 {
		t.Errorf("Expected budget limit 300.0, got %f", m.config.BudgetLimit)
	}
}

func TestSave(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	m := NewManager()
	m.config.ConfigPath = configPath
	m.config.APIKeys["openai"] = "test-key"
	m.config.DefaultModels["interview"] = "gpt-4"
	m.config.BudgetLimit = 150.0
	m.config.VerboseLogging = true

	if err := m.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load the file and verify contents
	m2 := NewManager()
	if err := m2.loadFromFile(configPath); err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if m2.config.APIKeys["openai"] != "test-key" {
		t.Errorf("Expected openai key 'test-key', got '%s'", m2.config.APIKeys["openai"])
	}
	if m2.config.DefaultModels["interview"] != "gpt-4" {
		t.Errorf("Expected interview model 'gpt-4', got '%s'", m2.config.DefaultModels["interview"])
	}
	if m2.config.BudgetLimit != 150.0 {
		t.Errorf("Expected budget limit 150.0, got %f", m2.config.BudgetLimit)
	}
	if !m2.config.VerboseLogging {
		t.Error("Expected verbose logging to be true")
	}
}

func TestGetAPIKey(t *testing.T) {
	m := NewManager()
	m.config.APIKeys["openai"] = "test-key"

	key, err := m.GetAPIKey("openai")
	if err != nil {
		t.Fatalf("GetAPIKey failed: %v", err)
	}
	if key != "test-key" {
		t.Errorf("Expected 'test-key', got '%s'", key)
	}

	// Test missing key
	_, err = m.GetAPIKey("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent provider")
	}
}

func TestSetAPIKey(t *testing.T) {
	m := NewManager()

	err := m.SetAPIKey("openai", "new-key")
	if err != nil {
		t.Fatalf("SetAPIKey failed: %v", err)
	}

	// Use GetAPIKey to retrieve the key (it may be in keyring or config)
	key, err := m.GetAPIKey("openai")
	if err != nil {
		t.Fatalf("GetAPIKey failed: %v", err)
	}
	if key != "new-key" {
		t.Errorf("Expected 'new-key', got '%s'", key)
	}

	// Test empty provider
	err = m.SetAPIKey("", "key")
	if err == nil {
		t.Error("Expected error for empty provider")
	}

	// Test empty key
	err = m.SetAPIKey("provider", "")
	if err == nil {
		t.Error("Expected error for empty key")
	}
}

func TestGetDefaultModel(t *testing.T) {
	m := NewManager()
	m.config.DefaultModels["interview"] = "gpt-4"

	model, err := m.GetDefaultModel("interview")
	if err != nil {
		t.Fatalf("GetDefaultModel failed: %v", err)
	}
	if model != "gpt-4" {
		t.Errorf("Expected 'gpt-4', got '%s'", model)
	}

	// Test missing model
	_, err = m.GetDefaultModel("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent stage")
	}
}

func TestSetDefaultModel(t *testing.T) {
	m := NewManager()

	err := m.SetDefaultModel("interview", "gpt-4")
	if err != nil {
		t.Fatalf("SetDefaultModel failed: %v", err)
	}

	if m.config.DefaultModels["interview"] != "gpt-4" {
		t.Errorf("Expected 'gpt-4', got '%s'", m.config.DefaultModels["interview"])
	}

	// Test empty stage
	err = m.SetDefaultModel("", "model")
	if err == nil {
		t.Error("Expected error for empty stage")
	}

	// Test empty model
	err = m.SetDefaultModel("stage", "")
	if err == nil {
		t.Error("Expected error for empty model")
	}
}

func TestGetConfigPath(t *testing.T) {
	m := NewManager()
	m.config.ConfigPath = "/test/path/config.yaml"

	path := m.GetConfigPath()
	if path != "/test/path/config.yaml" {
		t.Errorf("Expected '/test/path/config.yaml', got '%s'", path)
	}
}

func TestGetConfigPathByOS(t *testing.T) {
	path, err := getConfigPath()
	if err != nil {
		t.Fatalf("getConfigPath failed: %v", err)
	}

	if path == "" {
		t.Error("Config path is empty")
	}

	// Verify path ends with config.yaml
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("Expected path to end with 'config.yaml', got '%s'", path)
	}

	// Verify path contains geoffrussy
	if !filepath.IsAbs(path) {
		t.Errorf("Expected absolute path, got '%s'", path)
	}
}

func TestSplitEnv(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"KEY=value", []string{"KEY", "value"}},
		{"KEY=value=with=equals", []string{"KEY", "value=with=equals"}},
		{"KEY=", []string{"KEY", ""}},
		{"KEY", []string{"KEY"}},
		{"", []string{""}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitEnv(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d parts, got %d", len(tt.expected), len(result))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("Part %d: expected '%s', got '%s'", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

func TestLoadIntegration(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `api_keys:
  openai: file-key
  anthropic: file-anthropic-key
default_models:
  interview: file-model
budget_limit: 100.0
verbose_logging: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Set environment variables
	os.Setenv("GEOFFRUSSY_API_KEY_OPENAI", "env-key")
	os.Setenv("GEOFFRUSSY_DEFAULT_MODEL_DESIGN", "env-design-model")
	os.Setenv("GEOFFRUSSY_BUDGET_LIMIT", "200.0")
	defer func() {
		os.Unsetenv("GEOFFRUSSY_API_KEY_OPENAI")
		os.Unsetenv("GEOFFRUSSY_DEFAULT_MODEL_DESIGN")
		os.Unsetenv("GEOFFRUSSY_BUDGET_LIMIT")
	}()

	// Create flag config
	flagConfig := &Config{
		APIKeys: map[string]string{
			"OPENAI": "flag-key",
		},
		DefaultModels: map[string]string{
			"PLAN": "flag-plan-model",
		},
		BudgetLimit: 300.0,
	}

	m := NewManager()
	m.config.ConfigPath = configPath

	// Load from file
	if err := m.loadFromFile(configPath); err != nil {
		t.Fatalf("loadFromFile failed: %v", err)
	}

	// Load from env - use testing version
	m.loadFromEnvForTesting(
		[]string{"OPENAI"},
		[]string{"DESIGN"},
	)

	// Apply flags
	m.applyFlags(flagConfig)

	// Verify precedence: flags > env > file
	// OpenAI key: flag should win
	if m.config.APIKeys["OPENAI"] != "flag-key" {
		t.Errorf("Expected flag key 'flag-key', got '%s'", m.config.APIKeys["OPENAI"])
	}

	// Anthropic key: only in file
	if m.config.APIKeys["anthropic"] != "file-anthropic-key" {
		t.Errorf("Expected file key 'file-anthropic-key', got '%s'", m.config.APIKeys["anthropic"])
	}

	// Interview model: only in file
	if m.config.DefaultModels["interview"] != "file-model" {
		t.Errorf("Expected file model 'file-model', got '%s'", m.config.DefaultModels["interview"])
	}

	// Design model: only in env
	if m.config.DefaultModels["DESIGN"] != "env-design-model" {
		t.Errorf("Expected env model 'env-design-model', got '%s'", m.config.DefaultModels["DESIGN"])
	}

	// Plan model: only in flags
	if m.config.DefaultModels["PLAN"] != "flag-plan-model" {
		t.Errorf("Expected flag model 'flag-plan-model', got '%s'", m.config.DefaultModels["PLAN"])
	}

	// Budget: flag should win
	if m.config.BudgetLimit != 300.0 {
		t.Errorf("Expected budget limit 300.0, got %f", m.config.BudgetLimit)
	}
}

// MockValidator is a mock API key validator for testing
type MockValidator struct {
	shouldFail bool
}

func (v *MockValidator) ValidateAPIKey(provider, key string) error {
	if v.shouldFail {
		return fmt.Errorf("validation failed for provider %s", provider)
	}
	return nil
}

func TestSetAPIKeyWithValidation(t *testing.T) {
	m := NewManager()

	// Test without validator (should succeed)
	err := m.SetAPIKey("openai", "test-key")
	if err != nil {
		t.Fatalf("SetAPIKey without validator failed: %v", err)
	}

	// Test with successful validator
	m.SetValidator(&MockValidator{shouldFail: false})
	err = m.SetAPIKey("openai", "valid-key")
	if err != nil {
		t.Fatalf("SetAPIKey with valid key failed: %v", err)
	}

	// Test with failing validator
	m.SetValidator(&MockValidator{shouldFail: true})
	err = m.SetAPIKey("openai", "invalid-key")
	if err == nil {
		t.Error("Expected error for invalid key")
	}
}

func TestValidateAPIKey(t *testing.T) {
	m := NewManager()

	// Test without validator
	err := m.ValidateAPIKey("openai", "test-key")
	if err == nil {
		t.Error("Expected error when no validator is configured")
	}

	// Test with validator
	m.SetValidator(&MockValidator{shouldFail: false})
	err = m.ValidateAPIKey("openai", "valid-key")
	if err != nil {
		t.Fatalf("ValidateAPIKey failed: %v", err)
	}

	// Test with failing validator
	m.SetValidator(&MockValidator{shouldFail: true})
	err = m.ValidateAPIKey("openai", "invalid-key")
	if err == nil {
		t.Error("Expected validation error")
	}
}

func TestFavoriteModels(t *testing.T) {
	m := NewManager()

	// Initial state
	if len(m.GetFavoriteModels()) != 0 {
		t.Error("Expected no favorite models initially")
	}

	// Add favorites
	m.AddFavoriteModel("gpt-4")
	m.AddFavoriteModel("claude-3-opus")

	favorites := m.GetFavoriteModels()
	if len(favorites) != 2 {
		t.Errorf("Expected 2 favorite models, got %d", len(favorites))
	}

	if !m.IsFavoriteModel("gpt-4") {
		t.Error("Expected gpt-4 to be a favorite")
	}

	if !m.IsFavoriteModel("claude-3-opus") {
		t.Error("Expected claude-3-opus to be a favorite")
	}

	// Add duplicate (should be ignored)
	m.AddFavoriteModel("gpt-4")
	if len(m.GetFavoriteModels()) != 2 {
		t.Error("Expected duplicate model not to be added")
	}

	// Remove favorite
	m.RemoveFavoriteModel("gpt-4")
	if len(m.GetFavoriteModels()) != 1 {
		t.Error("Expected 1 favorite model after removal")
	}
	if m.IsFavoriteModel("gpt-4") {
		t.Error("Expected gpt-4 to no longer be a favorite")
	}

	// Remove non-existent (should error)
	err := m.RemoveFavoriteModel("nonexistent")
	if err == nil {
		t.Error("Expected error when removing non-existent model")
	}

	// Test persistence
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	m.config.ConfigPath = configPath

	if err := m.Save(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	m2 := NewManager()
	if err := m2.loadFromFile(configPath); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(m2.GetFavoriteModels()) != 1 {
		t.Errorf("Expected 1 favorite model after load, got %d", len(m2.GetFavoriteModels()))
	}
	if !m2.IsFavoriteModel("claude-3-opus") {
		t.Error("Expected persisted favorite model to be loaded")
	}
}
