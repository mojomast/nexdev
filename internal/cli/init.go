package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/config"
	"github.com/mojomast/nexdev/internal/git"
	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Geoffrussy in the current project",
	Long: `Initialize Geoffrussy in the current project by creating configuration
directory structure and prompting for API keys.`,
	RunE: runInit,
}

// flags for non-interactive configuration
var (
	// flagAPIKeys stores API key flag values keyed by provider name.
	// Populated dynamically from the provider registry at init time.
	flagAPIKeys        = make(map[string]*string)
	flagNonInteractive bool
	flagValidateOnly   bool
)

func init() {
	// Dynamically register API key flags for all providers in the registry
	for _, name := range provider.GetProviderNames() {
		flagName := "api-key-" + name
		desc := name + " API key"
		val := new(string)
		flagAPIKeys[name] = val
		initCmd.Flags().StringVar(val, flagName, "", desc)
	}
	initCmd.Flags().BoolVar(&flagNonInteractive, "non-interactive", false, "Run in non-interactive mode (no prompts)")
	initCmd.Flags().BoolVar(&flagValidateOnly, "validate-only", false, "Only validate configuration without creating project files")
}

// validateConfiguration validates the configuration before running init
func validateConfiguration(cmd *cobra.Command, args []string) error {
	if flagNonInteractive {
		return validateNonInteractiveConfig()
	}
	return nil
}

// validateNonInteractiveConfig validates the non-interactive configuration.
// Only providers whose API keys are supplied (via flag, env, or config) are configured.
func validateNonInteractiveConfig() error {
	// Load existing config
	cfgManager := config.NewManager()
	if err := cfgManager.Load(nil); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	cfg := cfgManager.GetConfig()
	configured := 0

	// Try to resolve an API key for every registered provider.
	// Only providers with a key available are added to the config.
	for _, name := range provider.GetProviderNames() {
		key, err := getAPIKey(name)
		if err != nil {
			continue // provider not configured – skip
		}
		cfg.APIKeys[name] = key
		configured++
	}

	if configured == 0 {
		return fmt.Errorf("no API keys configured. Provide at least one key via --api-key-<provider> flag or GEOFFRUSSY_<PROVIDER>_API_KEY environment variable")
	}

	// Save the configuration
	if err := cfgManager.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	return nil
}

// getAPIKey retrieves the API key for a provider, checking in order: flag, environment variable, config
func getAPIKey(providerName string) (string, error) {
	// Check flag first (dynamically from the flagAPIKeys map)
	if ptr, ok := flagAPIKeys[providerName]; ok && ptr != nil && *ptr != "" {
		return *ptr, nil
	}

	// Check environment variable
	envVar := "GEOFFRUSSY_" + strings.ToUpper(providerName) + "_API_KEY"
	if apiKey := os.Getenv(envVar); apiKey != "" {
		return apiKey, nil
	}

	// Check config file
	cfgManager := config.NewManager()
	if err := cfgManager.Load(nil); err != nil {
		return "", fmt.Errorf("failed to load configuration: %w", err)
	}

	cfg := cfgManager.GetConfig()
	if apiKey, ok := cfg.APIKeys[providerName]; ok {
		return apiKey, nil
	}

	return "", fmt.Errorf("no API key found for %s", providerName)
}

func runInit(cmd *cobra.Command, args []string) error {
	if flagValidateOnly {
		return runValidateOnly()
	}
	if flagNonInteractive {
		return runInitNonInteractive(cmd, args)
	}
	return runInitInteractive(cmd, args)
}

// runValidateOnly checks that at least one provider has a valid API key
// reachable via flag, env, or config — then exits without modifying anything.
func runValidateOnly() error {
	fmt.Println("🔍 Validating configuration (dry-run)...")

	cfgManager := config.NewManager()
	if err := cfgManager.Load(nil); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	configured := 0
	for _, name := range provider.GetProviderNames() {
		key, err := getAPIKey(name)
		if err != nil {
			fmt.Printf("  - %s: not configured\n", name)
			continue
		}
		source := describeKeySource(name, key)
		fmt.Printf("  ✓ %s: configured (%s)\n", name, source)
		configured++
	}

	if configured == 0 {
		return fmt.Errorf("validation failed: no API keys found. Provide at least one key via --api-key-<provider> flag or GEOFFRUSSY_<PROVIDER>_API_KEY environment variable")
	}

	fmt.Printf("\n✓ Validation passed: %d provider(s) configured\n", configured)
	return nil
}

// describeKeySource returns a short description of where a key came from.
func describeKeySource(name, key string) string {
	if ptr, ok := flagAPIKeys[name]; ok && ptr != nil && *ptr != "" && *ptr == key {
		return "flag"
	}
	envVar := "GEOFFRUSSY_" + strings.ToUpper(name) + "_API_KEY"
	if os.Getenv(envVar) == key {
		return "env"
	}
	return "config"
}

func runInitInteractive(cmd *cobra.Command, args []string) error {
	fmt.Println("🚀 Initializing Geoffrussy...")

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Create configuration directory
	configDir := filepath.Join(os.Getenv("HOME"), ".geoffrussy")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	fmt.Printf("✓ Created configuration directory: %s\n", configDir)

	// Initialize configuration manager and load existing config
	cfgManager := config.NewManager()
	if err := cfgManager.Load(nil); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Check if config already has API keys
	cfg := cfgManager.GetConfig()
	if len(cfg.APIKeys) > 0 {
		fmt.Println("⚠️  Configuration file already exists with API keys")
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Do you want to reconfigure? (y/N): ")
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Skipping configuration...")
		} else {
			if err := promptForAPIKeys(cfgManager); err != nil {
				return err
			}
		}
	} else {
		if err := promptForAPIKeys(cfgManager); err != nil {
			return err
		}
	}

	// Save configuration
	if err := cfgManager.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}
	fmt.Println("✓ Configuration saved")

	// Initialize database
	dbPath := filepath.Join(cwd, ".geoffrussy", "state.db")
	store, err := state.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer store.Close()
	fmt.Printf("✓ Initialized database: %s\n", dbPath)

	// Create or update project in state store
	projectID := filepath.Base(cwd)
	project := &state.Project{
		ID:           projectID,
		Name:         projectID,
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInit,
		CurrentPhase: "",
	}

	// Check if project exists
	existingProject, err := store.GetProject(projectID)
	if err != nil {
		// Project doesn't exist, create it
		if err := store.CreateProject(project); err != nil {
			return fmt.Errorf("failed to create project: %w", err)
		}
		fmt.Printf("✓ Created project: %s\n", projectID)
	} else {
		// Project exists, update it
		existingProject.CurrentStage = state.StageInit
		existingProject.Name = projectID
		if err := store.UpdateProject(existingProject); err != nil {
			return fmt.Errorf("failed to update project: %w", err)
		}
		fmt.Printf("✓ Updated project: %s\n", projectID)
	}

	// Initialize Git repository if needed
	gitManager := git.NewManager(cwd)
	isRepo, err := gitManager.IsRepository()
	if err != nil {
		return fmt.Errorf("failed to check git repository: %w", err)
	}

	if !isRepo {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Initialize Git repository? (Y/n): ")
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response == "" || response == "y" || response == "yes" {
			if err := gitManager.Initialize(); err != nil {
				return fmt.Errorf("failed to initialize git repository: %w", err)
			}
			fmt.Println("✓ Initialized Git repository")
		}
	} else {
		fmt.Println("✓ Git repository already initialized")
	}

	fmt.Println("\n✨ Geoffrussy initialized successfully!")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Run 'geoffrussy interview' to start the project interview")
	fmt.Println("  2. Run 'geoffrussy design' to generate architecture")
	fmt.Println("  3. Run 'geoffrussy plan' to create development plan")
	fmt.Println("  4. Run 'geoffrussy develop' to start implementation")

	return nil
}

func runInitNonInteractive(cmd *cobra.Command, args []string) error {
	fmt.Println("🚀 Initializing Geoffrussy (non-interactive)...")

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Create configuration directory
	configDir := filepath.Join(os.Getenv("HOME"), ".geoffrussy")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	fmt.Printf("✓ Created configuration directory: %s\n", configDir)

	// Initialize configuration manager and load existing config
	cfgManager := config.NewManager()
	if err := cfgManager.Load(nil); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// In non-interactive mode, configuration should already be set by validateNonInteractiveConfig
	// Just verify it's set
	cfg := cfgManager.GetConfig()
	if len(cfg.APIKeys) == 0 {
		return fmt.Errorf("no API keys configured. Please set them using environment variables or flags")
	}
	fmt.Println("✓ Configuration loaded")

	// Initialize database
	dbPath := filepath.Join(cwd, ".geoffrussy", "state.db")
	store, err := state.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer store.Close()
	fmt.Printf("✓ Initialized database: %s\n", dbPath)

	// Create or update project in state store
	projectID := filepath.Base(cwd)
	project := &state.Project{
		ID:           projectID,
		Name:         projectID,
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInit,
		CurrentPhase: "",
	}

	// Check if project exists
	existingProject, err := store.GetProject(projectID)
	if err != nil {
		// Project doesn't exist, create it
		if err := store.CreateProject(project); err != nil {
			return fmt.Errorf("failed to create project: %w", err)
		}
		fmt.Printf("✓ Created project: %s\n", projectID)
	} else {
		// Project exists, update it
		existingProject.CurrentStage = state.StageInit
		existingProject.Name = projectID
		if err := store.UpdateProject(existingProject); err != nil {
			return fmt.Errorf("failed to update project: %w", err)
		}
		fmt.Printf("✓ Updated project: %s\n", projectID)
	}

	fmt.Println("\n✨ Geoffrussy initialized successfully!")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Run 'geoffrussy interview' to start the project interview")
	fmt.Println("  2. Run 'geoffrussy design' to generate architecture")
	fmt.Println("  3. Run 'geoffrussy plan' to create development plan")
	fmt.Println("  4. Run 'geoffrussy develop' to start implementation")

	return nil
}

func promptForAPIKeys(cfgManager *config.Manager) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n📝 API Key Configuration")
	fmt.Println("Enter API keys for the providers you want to use (press Enter to skip):")

	for _, name := range provider.GetProviderNames() {
		displayName := providerDisplayName(name)
		fmt.Printf("\n%s API Key: ", displayName)
		apiKey, _ := reader.ReadString('\n')
		apiKey = strings.TrimSpace(apiKey)
		if apiKey != "" {
			cfgManager.SetAPIKey(name, apiKey)
			fmt.Printf("✓ %s API key configured\n", displayName)
		}
	}

	// Prompt for default model
	fmt.Println("\n📦 Available Models:")
	fmt.Println("─────────────────────────────────────────────────────")
	displayConfiguredModels(cfgManager)

	fmt.Print("\nDefault model for interview stage (select from above): ")
	defaultModel, _ := reader.ReadString('\n')
	defaultModel = strings.TrimSpace(defaultModel)
	if defaultModel != "" {
		cfgManager.SetDefaultModel("interview", defaultModel)
		fmt.Printf("✓ Default interview model set to: %s\n", defaultModel)
	}

	return nil
}

// providerDisplayName returns a human-friendly display name for a provider.
func providerDisplayName(name string) string {
	display := map[string]string{
		"anthropic":    "Anthropic",
		"deepinfra":    "DeepInfra",
		"firmware":     "Firmware.ai",
		"fireworks":    "Fireworks",
		"groq":         "Groq",
		"kimi":         "Kimi",
		"mistral":      "Mistral",
		"ollama":       "Ollama (Local)",
		"openai":       "OpenAI",
		"openai-codex": "OpenAI Codex",
		"opencode":     "OpenCode",
		"openrouter":   "OpenRouter",
		"perplexity":   "Perplexity",
		"requesty":     "Requesty.ai",
		"together":     "Together",
		"zai":          "Z.ai",
	}
	if d, ok := display[name]; ok {
		return d
	}
	// Fallback: capitalize first letter
	if len(name) == 0 {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func displayConfiguredModels(cfgMgr *config.Manager) {
	cfg := cfgMgr.GetConfig()

	if len(cfg.APIKeys) == 0 {
		fmt.Println("⚠️  No API keys configured. Run 'geoffrussy config' to add keys.")
		return
	}

	bridge := provider.NewBridge()
	providerNames := provider.GetProviderNames()

	for _, name := range providerNames {
		if err := setupProvider(bridge, cfgMgr, name); err != nil {
			continue
		}
	}

	allModels, err := bridge.ListModels()
	if err != nil || len(allModels) == 0 {
		fmt.Println("⚠️  No models found. Configure providers first.")
		return
	}

	modelsByProvider := make(map[string][]string)
	for _, m := range allModels {
		modelsByProvider[m.Provider] = append(modelsByProvider[m.Provider], m.Name)
	}

	for p := range cfg.APIKeys {
		models, ok := modelsByProvider[p]
		if !ok {
			continue
		}
		displayName := providerDisplayName(p)
		fmt.Printf("\n📦 %s:\n", displayName)
		for _, model := range models {
			fmt.Printf("   • %s\n", model)
		}
	}
}
