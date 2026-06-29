package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	APIKeys        map[string]string `yaml:"api_keys"`
	APIKeySources  map[string]string `yaml:"api_key_sources,omitempty"`
	DefaultModels  map[string]string `yaml:"default_models"`
	FavoriteModels []string          `yaml:"favorite_models"`
	BudgetLimit    float64           `yaml:"budget_limit"`
	VerboseLogging bool              `yaml:"verbose_logging"`
	MCP            *MCPConfig        `yaml:"mcp,omitempty"`
	ConfigPath     string            `yaml:"-"` // Not serialized
}

// MCPConfig represents MCP server configuration
type MCPConfig struct {
	Enabled    bool   `yaml:"enabled"`
	LogLevel   string `yaml:"log_level"`
	ServerMode string `yaml:"server_mode"` // Currently only "stdio" is supported
}

// Manager handles configuration loading and management
type Manager struct {
	config    *Config
	validator APIKeyValidator
}

// APIKeyValidator is an interface for validating API keys against providers
type APIKeyValidator interface {
	ValidateAPIKey(provider string, key string) error
}

// NewManager creates a new configuration manager
func NewManager() *Manager {
	return &Manager{
		config: &Config{
			APIKeys:        make(map[string]string),
			APIKeySources:  make(map[string]string),
			DefaultModels:  make(map[string]string),
			FavoriteModels: []string{},
		},
		validator: nil, // Will be set when provider system is implemented
	}
}

// SetValidator sets the API key validator
func (m *Manager) SetValidator(validator APIKeyValidator) {
	m.validator = validator
}

// Load loads configuration from multiple sources with precedence:
// 1. Command-line flags (highest priority)
// 2. Environment variables
// 3. Config file (lowest priority)
func (m *Manager) Load(flagConfig *Config) error {
	// Start with default config
	m.config = &Config{
		APIKeys:        make(map[string]string),
		APIKeySources:  make(map[string]string),
		DefaultModels:  make(map[string]string),
		BudgetLimit:    0,
		VerboseLogging: false,
	}

	// Get config file path
	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}
	m.config.ConfigPath = configPath

	// Load from config file (lowest priority)
	if err := m.loadFromFile(configPath); err != nil {
		// If file doesn't exist, that's okay - we'll create it on save
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Load from environment variables (medium priority)
	m.loadFromEnv()

	// Apply command-line flags (highest priority)
	if flagConfig != nil {
		m.applyFlags(flagConfig)
	}

	return nil
}

// loadFromFile loads configuration from a YAML file
func (m *Manager) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var fileConfig Config
	if err := yaml.Unmarshal(data, &fileConfig); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Merge file config into current config
	if fileConfig.APIKeys != nil {
		for k, v := range fileConfig.APIKeys {
			if v != "" {
				m.config.APIKeys[k] = v
				if _, ok := m.config.APIKeySources[k]; !ok {
					m.config.APIKeySources[k] = "plaintext"
				}
			}
		}
	}
	if fileConfig.APIKeySources != nil {
		for k, v := range fileConfig.APIKeySources {
			if v != "" {
				m.config.APIKeySources[k] = v
			}
		}
	}
	if fileConfig.DefaultModels != nil {
		for k, v := range fileConfig.DefaultModels {
			if v != "" {
				m.config.DefaultModels[k] = v
			}
		}
	}
	if fileConfig.FavoriteModels != nil {
		m.config.FavoriteModels = fileConfig.FavoriteModels
	}
	if fileConfig.BudgetLimit > 0 {
		m.config.BudgetLimit = fileConfig.BudgetLimit
	}
	if fileConfig.VerboseLogging {
		m.config.VerboseLogging = fileConfig.VerboseLogging
	}

	return nil
}

// loadFromEnv loads configuration from environment variables
func (m *Manager) loadFromEnv() {
	// API Keys - format: GEOFFRUSSY_API_KEY_<PROVIDER>=<key>
	// Example: GEOFFRUSSY_API_KEY_OPENAI=sk-...
	// We iterate through os.Environ() to find all matching variables
	envVars := os.Environ()
	for _, env := range envVars {
		// Parse environment variable
		parts := splitEnv(env)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// Check for API key
		if len(key) > 22 && key[:22] == "GEOFFRUSSY_API_KEY_" {
			provider := key[22:] // Remove "GEOFFRUSSY_API_KEY_" prefix
			if value != "" {
				m.config.APIKeys[provider] = value
				m.config.APIKeySources[provider] = "env"
			}
		}

		// Check for default model
		if len(key) > 28 && key[:28] == "GEOFFRUSSY_DEFAULT_MODEL_" {
			stage := key[28:] // Remove "GEOFFRUSSY_DEFAULT_MODEL_" prefix
			if value != "" {
				m.config.DefaultModels[stage] = value
			}
		}
	}

	// Budget Limit
	if budgetStr := os.Getenv("GEOFFRUSSY_BUDGET_LIMIT"); budgetStr != "" {
		var budget float64
		if _, err := fmt.Sscanf(budgetStr, "%f", &budget); err == nil && budget > 0 {
			m.config.BudgetLimit = budget
		}
	}

	// Verbose Logging
	if verboseStr := os.Getenv("GEOFFRUSSY_VERBOSE_LOGGING"); verboseStr != "" {
		m.config.VerboseLogging = verboseStr == "true" || verboseStr == "1" || verboseStr == "yes"
	}
}

// loadFromEnvForTesting is a test-friendly version that uses os.Getenv for known providers
// This is needed because os.Setenv doesn't update os.Environ() on all platforms
func (m *Manager) loadFromEnvForTesting(providers []string, stages []string) {
	// Load API keys for known providers
	for _, provider := range providers {
		envKey := "GEOFFRUSSY_API_KEY_" + provider
		if value := os.Getenv(envKey); value != "" {
			m.config.APIKeys[provider] = value
			m.config.APIKeySources[provider] = "env"
		}
	}

	// Load default models for known stages
	for _, stage := range stages {
		envKey := "GEOFFRUSSY_DEFAULT_MODEL_" + stage
		if value := os.Getenv(envKey); value != "" {
			m.config.DefaultModels[stage] = value
		}
	}

	// Budget Limit
	if budgetStr := os.Getenv("GEOFFRUSSY_BUDGET_LIMIT"); budgetStr != "" {
		var budget float64
		if _, err := fmt.Sscanf(budgetStr, "%f", &budget); err == nil && budget > 0 {
			m.config.BudgetLimit = budget
		}
	}

	// Verbose Logging
	if verboseStr := os.Getenv("GEOFFRUSSY_VERBOSE_LOGGING"); verboseStr != "" {
		m.config.VerboseLogging = verboseStr == "true" || verboseStr == "1" || verboseStr == "yes"
	}
}

// applyFlags applies command-line flag configuration (highest priority)
func (m *Manager) applyFlags(flagConfig *Config) {
	if flagConfig.APIKeys != nil {
		for k, v := range flagConfig.APIKeys {
			if v != "" {
				m.config.APIKeys[k] = v
				m.config.APIKeySources[k] = "flag"
			}
		}
	}
	if flagConfig.DefaultModels != nil {
		for k, v := range flagConfig.DefaultModels {
			if v != "" {
				m.config.DefaultModels[k] = v
			}
		}
	}
	if flagConfig.BudgetLimit > 0 {
		m.config.BudgetLimit = flagConfig.BudgetLimit
	}
	// For boolean flags, we need to check if it was explicitly set
	// For now, we'll apply it if true
	if flagConfig.VerboseLogging {
		m.config.VerboseLogging = flagConfig.VerboseLogging
	}
}

// Save saves the current configuration to the config file
func (m *Manager) Save() error {
	if m.config.ConfigPath == "" {
		path, err := getConfigPath()
		if err != nil {
			return fmt.Errorf("failed to get config path: %w", err)
		}
		m.config.ConfigPath = path
	}

	// Ensure config directory exists
	configDir := filepath.Dir(m.config.ConfigPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal config to YAML
	data, err := yaml.Marshal(m.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(m.config.ConfigPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *Config {
	return m.config
}

// GetAPIKey returns the API key for a specific provider
func (m *Manager) GetAPIKey(provider string) (string, error) {
	keySource := m.config.APIKeySources[provider]
	if keySource == "keyring" {
		if key, err := getSecret(provider); err == nil && key != "" {
			return key, nil
		}
	}

	key, ok := m.config.APIKeys[provider]
	if !ok || key == "" {
		return "", fmt.Errorf("API key not found for provider: %s", provider)
	}
	return key, nil
}

// SetAPIKey sets the API key for a specific provider
func (m *Manager) SetAPIKey(provider, key string) error {
	_, err := m.SetAPIKeyWithStorage(provider, key)
	return err
}

// SetAPIKeyWithStorage sets the API key and returns storage mode used.
func (m *Manager) SetAPIKeyWithStorage(provider, key string) (string, error) {
	if provider == "" {
		return "", fmt.Errorf("provider cannot be empty")
	}
	if key == "" {
		return "", fmt.Errorf("API key cannot be empty")
	}

	// Validate API key if validator is set
	if m.validator != nil {
		if err := m.validator.ValidateAPIKey(provider, key); err != nil {
			return "", fmt.Errorf("API key validation failed: %w", err)
		}
	}

	if err := setSecret(provider, key); err == nil {
		delete(m.config.APIKeys, provider)
		m.config.APIKeySources[provider] = "keyring"
		return "keyring", nil
	}

	m.config.APIKeys[provider] = key
	m.config.APIKeySources[provider] = "plaintext"
	return "plaintext", nil
}

// ValidateAPIKey validates an API key against a provider
// This is a convenience method that uses the validator if set
func (m *Manager) ValidateAPIKey(provider, key string) error {
	if m.validator == nil {
		return fmt.Errorf("no validator configured")
	}
	return m.validator.ValidateAPIKey(provider, key)
}

// GetDefaultModel returns the default model for a specific stage
func (m *Manager) GetDefaultModel(stage string) (string, error) {
	model, ok := m.config.DefaultModels[stage]
	if !ok || model == "" {
		return "", fmt.Errorf("default model not found for stage: %s", stage)
	}
	return model, nil
}

// ResolveDefaultModel resolves the model for a stage using granular fallback.
func (m *Manager) ResolveDefaultModel(stage string) (string, error) {
	for _, candidate := range getStageCandidates(stage) {
		if model, ok := m.config.DefaultModels[candidate]; ok && model != "" {
			return model, nil
		}
		for configuredStage, configuredModel := range m.config.DefaultModels {
			if strings.EqualFold(configuredStage, candidate) && configuredModel != "" {
				return configuredModel, nil
			}
		}
	}
	return "", fmt.Errorf("default model not found for stage: %s", stage)
}

// SetDefaultModel sets the default model for a specific stage
func (m *Manager) SetDefaultModel(stage, model string) error {
	if stage == "" {
		return fmt.Errorf("stage cannot be empty")
	}
	if model == "" {
		return fmt.Errorf("model cannot be empty")
	}
	m.config.DefaultModels[normalizeStage(stage)] = model
	return nil
}

func (m *Manager) GetAPIKeySource(provider string) string {
	source, ok := m.config.APIKeySources[provider]
	if !ok || source == "" {
		if _, hasPlain := m.config.APIKeys[provider]; hasPlain {
			return "plaintext"
		}
		return "unknown"
	}
	return source
}

// AddFavoriteModel adds a model to the favorites list
func (m *Manager) AddFavoriteModel(model string) error {
	if model == "" {
		return fmt.Errorf("model cannot be empty")
	}

	// Check if already exists
	for _, m := range m.config.FavoriteModels {
		if m == model {
			return nil // Already a favorite
		}
	}

	m.config.FavoriteModels = append(m.config.FavoriteModels, model)
	return nil
}

// RemoveFavoriteModel removes a model from the favorites list
func (m *Manager) RemoveFavoriteModel(model string) error {
	if model == "" {
		return fmt.Errorf("model cannot be empty")
	}

	newFavorites := []string{}
	found := false
	for _, fav := range m.config.FavoriteModels {
		if fav == model {
			found = true
			continue
		}
		newFavorites = append(newFavorites, fav)
	}

	if !found {
		return fmt.Errorf("model not found in favorites: %s", model)
	}

	m.config.FavoriteModels = newFavorites
	return nil
}

// IsFavoriteModel checks if a model is in the favorites list
func (m *Manager) IsFavoriteModel(model string) bool {
	for _, fav := range m.config.FavoriteModels {
		if fav == model {
			return true
		}
	}
	return false
}

// GetFavoriteModels returns the list of favorite models
func (m *Manager) GetFavoriteModels() []string {
	// Return a copy to prevent mutation
	favorites := make([]string, len(m.config.FavoriteModels))
	copy(favorites, m.config.FavoriteModels)
	return favorites
}

// GetConfigPath returns the path to the config file
func (m *Manager) GetConfigPath() string {
	return m.config.ConfigPath
}

// getConfigPath returns the standard config file path for the current OS
func getConfigPath() (string, error) {
	var configDir string

	switch runtime.GOOS {
	case "windows":
		// Windows: %APPDATA%\geoffrussy\config.yaml
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
		configDir = filepath.Join(appData, "geoffrussy")
	case "darwin":
		// macOS: ~/Library/Application Support/geoffrussy/config.yaml
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		configDir = filepath.Join(home, "Library", "Application Support", "geoffrussy")
	default:
		// Linux and others: ~/.config/geoffrussy/config.yaml
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		configDir = filepath.Join(home, ".config", "geoffrussy")
	}

	return filepath.Join(configDir, "config.yaml"), nil
}

// splitEnv splits an environment variable string into key and value
func splitEnv(env string) []string {
	for i := 0; i < len(env); i++ {
		if env[i] == '=' {
			return []string{env[:i], env[i+1:]}
		}
	}
	return []string{env}
}
