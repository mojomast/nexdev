package cli

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mojomast/nexdev/internal/config"
	"github.com/mojomast/nexdev/internal/provider"
	"github.com/spf13/cobra"
)

var configListProviders bool
var configSetKey bool
var configSetModel bool
var configProviderHelp string

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Geoffrussy configuration",
	Long: `Manage Geoffrussy configuration including API keys, provider selection,
 and default models for each pipeline stage.`,
	RunE: runConfig,
}

func init() {
	configCmd.Flags().BoolVar(&configListProviders, "list-providers", false, "List available providers and their models")
	configCmd.Flags().BoolVar(&configSetKey, "set-key", false, "Set API key interactively")
	configCmd.Flags().BoolVar(&configSetModel, "set-model", false, "Set default model for a stage")
	configCmd.Flags().StringVar(&configProviderHelp, "provider-help", "", "Show setup instructions for a provider")
}

func runConfig(cmd *cobra.Command, args []string) error {
	if configListProviders {
		return listProvidersAndModels()
	}

	if configSetKey {
		cfgMgr := config.NewManager()
		if err := cfgMgr.Load(nil); err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
		return setAPIKeyInteractive(cfgMgr)
	}

	if configSetModel {
		cfgMgr := config.NewManager()
		if err := cfgMgr.Load(nil); err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
		return setDefaultModelInteractive(cfgMgr)
	}

	if strings.TrimSpace(configProviderHelp) != "" {
		return showProviderHelp(configProviderHelp)
	}

	return showConfigMenu()
}

func showConfigMenu() error {
	cfgMgr := config.NewManager()
	if err := cfgMgr.Load(nil); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	for {
		fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
		fmt.Println("║           Configuration Management                                ║")
		fmt.Println("╚════════════════════════════════════════════════════════════╝")
		fmt.Println()
		cfg := cfgMgr.GetConfig()
		displayCurrentConfig(cfg)
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  1) 🔑 Set/Update API Key")
		fmt.Println("  2) 🤖 Set Default Model for Stage")
		fmt.Println("  3) 📋 List Available Providers & Models")
		fmt.Println("  4) 💰 Set Budget Limit")
		fmt.Println("  5) 🔍 Toggle Verbose Logging")
		fmt.Println("  6) 💾 Save and Exit")
		fmt.Println("  7) ⭐ Manage Favorite Models")
		fmt.Println("  q) Quit (Exit without Saving)")
		fmt.Println()
		fmt.Print("Select option: ")

		reader := bufio.NewReader(os.Stdin)
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			if err := setAPIKeyInteractive(cfgMgr); err != nil {
				fmt.Printf("⚠️  Error: %v\n", err)
			}
		case "2":
			if err := setDefaultModelInteractive(cfgMgr); err != nil {
				fmt.Printf("⚠️  Error: %v\n", err)
			}
		case "3":
			if err := listProvidersAndModels(); err != nil {
				fmt.Printf("⚠️  Error: %v\n", err)
			}
		case "4":
			if err := setBudgetLimitInteractive(cfgMgr); err != nil {
				fmt.Printf("⚠️  Error: %v\n", err)
			}
		case "5":
			cfg := cfgMgr.GetConfig()
			cfg.VerboseLogging = !cfg.VerboseLogging
			if cfg.VerboseLogging {
				fmt.Println("✅ Verbose logging enabled")
			} else {
				fmt.Println("✅ Verbose logging disabled")
			}
		case "6":
			if err := cfgMgr.Save(); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}
			fmt.Println("✅ Configuration saved!")
			return nil
		case "7":
			if err := manageFavoritesInteractive(cfgMgr); err != nil {
				fmt.Printf("⚠️  Error: %v\n", err)
			}
		case "q", "Q":
			fmt.Println("❌ Exiting without saving...")
			return nil
		default:
			fmt.Println("⚠️  Invalid option")
		}
	}
}

func displayCurrentConfig(cfg *config.Config) {
	fmt.Println("Current Configuration:")
	fmt.Println("─────────────────────────────────────────────────────")
	fmt.Println("\n🔑 API Keys:")
	if len(cfg.APIKeys) == 0 {
		if len(cfg.APIKeySources) == 0 {
			fmt.Println("   None configured")
		} else {
			for provider, source := range cfg.APIKeySources {
				fmt.Printf("   %s: (%s)\n", provider, source)
			}
		}
	} else {
		for provider := range cfg.APIKeys {
			masked := maskAPIKey(cfg.APIKeys[provider])
			source := cfg.APIKeySources[provider]
			if source == "" {
				source = "unknown"
			}
			fmt.Printf("   %s: %s (%s)\n", provider, masked, source)
		}
		for provider, source := range cfg.APIKeySources {
			if _, ok := cfg.APIKeys[provider]; ok {
				continue
			}
			fmt.Printf("   %s: (stored in %s)\n", provider, source)
		}
	}

	fmt.Println("\n🤖 Default Models:")
	if len(cfg.DefaultModels) == 0 {
		fmt.Println("   None configured")
	} else {
		for stage, model := range cfg.DefaultModels {
			fmt.Printf("   %s: %s\n", stage, model)
		}
	}

	fmt.Printf("\n💰 Budget Limit: $%.2f\n", cfg.BudgetLimit)
	if cfg.VerboseLogging {
		fmt.Println("🔍 Verbose Logging: ✅ Enabled")
	} else {
		fmt.Println("🔍 Verbose Logging: ❌ Disabled")
	}
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

func listProvidersAndModels() error {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║          Available Providers & Models                         ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	cfgMgr := config.NewManager()
	if err := cfgMgr.Load(nil); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	cfg := cfgMgr.GetConfig()

	providerNames := provider.GetProviderNames()

	for _, name := range providerNames {
		fmt.Printf("\n📦 %s\n", strings.Title(name))
		if tip, ok := providerOnboardingTips[name]; ok {
			fmt.Printf("   Setup: %s\n", tip.KeyFormat)
			if tip.DocsURL != "" {
				fmt.Printf("   Docs: %s\n", tip.DocsURL)
			}
		}

		snapshot, err := loadProviderSnapshot(cfgMgr, cfg, name)
		if err != nil {
			fmt.Printf("   ⚠️  %v\n", err)
			continue
		}

		if snapshot.rate != nil && snapshot.rate.RequestsLimit > 0 {
			fmt.Printf("   Rate Limit: %d/%d remaining\n", snapshot.rate.RequestsRemaining, snapshot.rate.RequestsLimit)
		}
		if snapshot.quota != nil {
			if snapshot.quota.TokensLimit > 0 {
				fmt.Printf("   Token Quota: %d/%d remaining\n", snapshot.quota.TokensRemaining, snapshot.quota.TokensLimit)
			}
			if snapshot.quota.CostLimit > 0 {
				fmt.Printf("   Cost Quota: $%.2f/$%.2f remaining\n", snapshot.quota.CostRemaining, snapshot.quota.CostLimit)
			}
		}

		if len(snapshot.models) == 0 {
			fmt.Println("   ⚠️  No models reported by provider")
			continue
		}

		fmt.Println("   Models:")
		for _, model := range snapshot.models {
			price := ""
			if model.PriceInput > 0 || model.PriceOutput > 0 {
				price = fmt.Sprintf(" [$%.4f in / $%.4f out per 1K]", model.PriceInput, model.PriceOutput)
			}
			fmt.Printf("      • %s%s\n", model.Name, price)
		}
	}

	fmt.Println("\n💡 Tip: Use 'geoffrussy config --set-key' to configure providers")
	return nil
}

func setAPIKeyInteractive(cfgMgr *config.Manager) error {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║              Set API Key                                       ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	providerNames := provider.GetProviderNames()
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Select a provider to configure (or type 'cancel'):")
	for i, name := range providerNames {
		fmt.Printf("  %d) %s\n", i+1, strings.Title(name))
	}
	fmt.Print("\nSelection: ")

	selection, _ := reader.ReadString('\n')
	selection = strings.TrimSpace(selection)

	if selection == "cancel" {
		fmt.Println("❌ Cancelled")
		return nil
	}

	index := 0
	if _, err := fmt.Sscanf(selection, "%d", &index); err != nil || index < 1 || index > len(providerNames) {
		return fmt.Errorf("invalid selection")
	}

	selectedName := providerNames[index-1]
	if tip, ok := providerOnboardingTips[selectedName]; ok {
		fmt.Printf("\n%s Setup\n", tip.Title)
		fmt.Printf("  Key format: %s\n", tip.KeyFormat)
		if tip.Notes != "" {
			fmt.Printf("  Notes: %s\n", tip.Notes)
		}
		if tip.DocsURL != "" {
			fmt.Printf("  Docs: %s\n", tip.DocsURL)
		}
	}

	fmt.Printf("\nEnter API Key for %s (or press Enter to skip): ", strings.Title(selectedName))
	apiKey, _ := reader.ReadString('\n')
	apiKey = strings.TrimSpace(apiKey)

	if apiKey == "" {
		fmt.Println("⏭️  Skipped")
		return nil
	}

	storage, err := cfgMgr.SetAPIKeyWithStorage(selectedName, apiKey)
	if err != nil {
		return fmt.Errorf("failed to set API key: %w", err)
	}

	if err := cfgMgr.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	if storage == "keyring" {
		fmt.Println("✅ API key configured (stored in OS keyring)")
	} else {
		fmt.Println("✅ API key configured (stored in config file; keyring unavailable)")
	}
	return nil
}

func setDefaultModelInteractive(cfgMgr *config.Manager) error {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║            Set Default Model for Stage                        ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	stages := config.KnownStageKeys()

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Select a stage:")
	for i, stage := range stages {
		fmt.Printf("  %d) %s\n", i+1, stage)
	}
	fmt.Print("\nSelection: ")

	selection, _ := reader.ReadString('\n')
	selection = strings.TrimSpace(selection)

	index := 0
	if _, err := fmt.Sscanf(selection, "%d", &index); err != nil || index < 1 || index > len(stages) {
		return fmt.Errorf("invalid selection")
	}

	selectedStage := stages[index-1]

	cfg := cfgMgr.GetConfig()

	fmt.Printf("\nCurrent configured models:\n")
	if len(cfg.DefaultModels) == 0 {
		fmt.Println("   None configured")
	} else {
		for stage, model := range cfg.DefaultModels {
			fmt.Printf("   %s: %s\n", stage, model)
		}
	}

	fmt.Println("\nFetching available models from provider APIs...")

	allModels, snapshots, err := loadAllProviderModels(cfgMgr, cfg)
	if err != nil || len(allModels) == 0 {
		fmt.Println("⚠️  No models found. Configure providers first.")
		return nil
	}

	if len(snapshots) > 0 {
		fmt.Println("\nProvider Usage Status:")
		for providerName, snap := range snapshots {
			line := fmt.Sprintf("   - %s", providerName)
			if snap.rate != nil && snap.rate.RequestsLimit > 0 {
				line += fmt.Sprintf(" | rate %d/%d", snap.rate.RequestsRemaining, snap.rate.RequestsLimit)
			}
			if snap.quota != nil && snap.quota.TokensLimit > 0 {
				line += fmt.Sprintf(" | tokens %d/%d", snap.quota.TokensRemaining, snap.quota.TokensLimit)
			}
			fmt.Println(line)
		}
	}

	// Separate favorites
	var favorites []provider.Model
	var others []provider.Model

	for _, m := range allModels {
		if cfgMgr.IsFavoriteModel(m.Name) {
			favorites = append(favorites, m)
		} else {
			others = append(others, m)
		}
	}

	// Reconstruct sorted list
	sortedModels := append(favorites, others...)

	fmt.Println("\nAvailable Models:")
	fmt.Println("─────────────────────────────────────────────────────")
	for i, m := range sortedModels {
		prefix := "  "
		if cfgMgr.IsFavoriteModel(m.Name) {
			prefix = "⭐ "
		}
		fmt.Printf("  %d) %s%s (%s)\n", i+1, prefix, m.Name, strings.Title(m.Provider))
	}

	fmt.Printf("\nEnter model for %s stage (1-%d): ", selectedStage, len(sortedModels))
	modelInput, _ := reader.ReadString('\n')
	modelInput = strings.TrimSpace(modelInput)

	modelIndex := 0
	if _, err := fmt.Sscanf(modelInput, "%d", &modelIndex); err != nil || modelIndex < 1 || modelIndex > len(sortedModels) {
		return fmt.Errorf("invalid selection. Please enter a number between 1 and %d", len(sortedModels))
	}

	selectedModel := sortedModels[modelIndex-1]

	if err := cfgMgr.SetDefaultModel(selectedStage, selectedModel.Name); err != nil {
		return fmt.Errorf("failed to set default model: %w", err)
	}

	if err := cfgMgr.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Printf("✅ Default model for %s set to %s (%s)\n", selectedStage, selectedModel.Name, strings.Title(selectedModel.Provider))
	return nil
}

func manageFavoritesInteractive(cfgMgr *config.Manager) error {
	for {
		fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
		fmt.Println("║            Manage Favorite Models                             ║")
		fmt.Println("╚════════════════════════════════════════════════════════════╝")
		fmt.Println()

		favorites := cfgMgr.GetFavoriteModels()
		if len(favorites) == 0 {
			fmt.Println("No favorite models configured.")
		} else {
			fmt.Println("Current Favorites:")
			for _, fav := range favorites {
				fmt.Printf("  ⭐ %s\n", fav)
			}
		}

		fmt.Println("\nOptions:")
		fmt.Println("  1) ➕ Add Favorite")
		fmt.Println("  2) ➖ Remove Favorite")
		fmt.Println("  b) Back to Main Menu")
		fmt.Println()
		fmt.Print("Select option: ")

		reader := bufio.NewReader(os.Stdin)
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			if err := addFavoriteInteractive(cfgMgr); err != nil {
				fmt.Printf("⚠️  Error: %v\n", err)
			}
		case "2":
			if err := removeFavoriteInteractive(cfgMgr); err != nil {
				fmt.Printf("⚠️  Error: %v\n", err)
			}
		case "b", "B":
			return nil
		default:
			fmt.Println("⚠️  Invalid option")
		}
	}
}

func addFavoriteInteractive(cfgMgr *config.Manager) error {
	fmt.Println("\nFetching available models...")
	cfg := cfgMgr.GetConfig()
	allModels, _, err := loadAllProviderModels(cfgMgr, cfg)
	if err != nil || len(allModels) == 0 {
		return fmt.Errorf("no models found. Configure providers first")
	}

	fmt.Println("\nSelect model to favorite:")
	fmt.Println("─────────────────────────────────────────────────────")
	for i, m := range allModels {
		prefix := "   "
		if cfgMgr.IsFavoriteModel(m.Name) {
			prefix = "⭐ "
		}
		fmt.Printf("  %d) %s%s (%s)\n", i+1, prefix, m.Name, strings.Title(m.Provider))
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\nEnter number (1-%d): ", len(allModels))

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	index := 0
	if _, err := fmt.Sscanf(input, "%d", &index); err != nil || index < 1 || index > len(allModels) {
		return fmt.Errorf("invalid selection")
	}

	selected := allModels[index-1]
	if err := cfgMgr.AddFavoriteModel(selected.Name); err != nil {
		return err
	}

	if err := cfgMgr.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✅ Added %s to favorites\n", selected.Name)
	return nil
}

func removeFavoriteInteractive(cfgMgr *config.Manager) error {
	favorites := cfgMgr.GetFavoriteModels()
	if len(favorites) == 0 {
		return fmt.Errorf("no favorites to remove")
	}

	fmt.Println("\nSelect favorite to remove:")
	fmt.Println("─────────────────────────────────────────────────────")
	for i, fav := range favorites {
		fmt.Printf("  %d) %s\n", i+1, fav)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\nEnter number (1-%d): ", len(favorites))

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	index := 0
	if _, err := fmt.Sscanf(input, "%d", &index); err != nil || index < 1 || index > len(favorites) {
		return fmt.Errorf("invalid selection")
	}

	selected := favorites[index-1]
	if err := cfgMgr.RemoveFavoriteModel(selected); err != nil {
		return err
	}

	if err := cfgMgr.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✅ Removed %s from favorites\n", selected)
	return nil
}

func setBudgetLimitInteractive(cfgMgr *config.Manager) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter budget limit in USD (or 0 for unlimited): ")
	limitStr, _ := reader.ReadString('\n')
	limitStr = strings.TrimSpace(limitStr)

	var limit float64
	if _, err := fmt.Sscanf(limitStr, "%f", &limit); err != nil {
		return fmt.Errorf("invalid budget limit: %w", err)
	}

	cfg := cfgMgr.GetConfig()
	cfg.BudgetLimit = limit

	if err := cfgMgr.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	if limit > 0 {
		fmt.Printf("✅ Budget limit set to $%.2f\n", limit)
	} else {
		fmt.Println("✅ Budget limit removed (unlimited)")
	}
	return nil
}

func showProviderHelp(providerName string) error {
	name := strings.ToLower(strings.TrimSpace(providerName))
	tip, ok := providerOnboardingTips[name]
	if !ok {
		return fmt.Errorf("unknown provider '%s'. Run 'geoffrussy config --list-providers' to see supported providers", providerName)
	}

	fmt.Printf("\n%s Setup\n", tip.Title)
	fmt.Println("─────────────────────────────────────────────────────")
	fmt.Printf("Provider:   %s\n", name)
	fmt.Printf("Key format: %s\n", tip.KeyFormat)
	if tip.Notes != "" {
		fmt.Printf("Notes:      %s\n", tip.Notes)
	}
	if tip.DocsURL != "" {
		fmt.Printf("Docs:       %s\n", tip.DocsURL)
	}

	fmt.Println("\nQuick commands:")
	fmt.Printf("  geoffrussy config --set-key\n")
	fmt.Printf("  geoffrussy config --set-model\n")

	return nil
}

type providerSnapshot struct {
	name   string
	models []provider.Model
	rate   *provider.RateLimitInfo
	quota  *provider.QuotaInfo
}

type onboardingTip struct {
	Title     string
	KeyFormat string
	Notes     string
	DocsURL   string
}

var providerOnboardingTips = map[string]onboardingTip{
	"openai": {
		Title:     "OpenAI",
		KeyFormat: "starts with sk-...",
		DocsURL:   "https://platform.openai.com/api-keys",
	},
	"openai-codex": {
		Title:     "OpenAI Codex Business",
		KeyFormat: "OpenAI org credential/token",
		Notes:     "Complete business web login flow first, then paste a usable API credential.",
		DocsURL:   "https://platform.openai.com/docs",
	},
	"anthropic": {
		Title:     "Anthropic",
		KeyFormat: "starts with sk-ant-...",
		DocsURL:   "https://console.anthropic.com/settings/keys",
	},
	"openrouter": {
		Title:     "OpenRouter",
		KeyFormat: "starts with sk-or-...",
		DocsURL:   "https://openrouter.ai/keys",
	},
	"groq": {
		Title:     "Groq",
		KeyFormat: "Groq API key",
		DocsURL:   "https://console.groq.com/keys",
	},
	"together": {
		Title:     "Together",
		KeyFormat: "Together API key",
		DocsURL:   "https://api.together.xyz/settings/api-keys",
	},
	"deepinfra": {
		Title:     "DeepInfra",
		KeyFormat: "DeepInfra API token",
		DocsURL:   "https://deepinfra.com/dash/api_keys",
	},
	"fireworks": {
		Title:     "Fireworks",
		KeyFormat: "Fireworks API key",
		DocsURL:   "https://fireworks.ai/account/api-keys",
	},
	"perplexity": {
		Title:     "Perplexity",
		KeyFormat: "Perplexity API key",
		DocsURL:   "https://docs.perplexity.ai",
	},
	"mistral": {
		Title:     "Mistral",
		KeyFormat: "Mistral API key",
		DocsURL:   "https://console.mistral.ai/api-keys",
	},
	"requesty": {
		Title:     "Requesty",
		KeyFormat: "Requesty API key",
		DocsURL:   "https://router.requesty.ai",
	},
	"kimi": {
		Title:     "Kimi (Moonshot)",
		KeyFormat: "Moonshot API key",
		DocsURL:   "https://platform.moonshot.cn",
	},
	"zai": {
		Title:     "Z.ai",
		KeyFormat: "Z.ai API key",
		DocsURL:   "https://platform.z.ai",
	},
	"firmware": {
		Title:     "Firmware",
		KeyFormat: "Firmware API key",
	},
	"opencode": {
		Title:     "OpenCode",
		KeyFormat: "not required in geoffrussy",
		Notes:     "Authenticate in OpenCode CLI separately; geoffrussy uses local opencode binary.",
	},
	"ollama": {
		Title:     "Ollama",
		KeyFormat: "not required",
		Notes:     "Ensure Ollama is running locally at http://localhost:11434.",
		DocsURL:   "https://ollama.com",
	},
}

func loadProviderSnapshot(cfgMgr *config.Manager, cfg *config.Config, name string) (*providerSnapshot, error) {
	p, err := provider.CreateProvider(name)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize provider: %w", err)
	}

	if name == "ollama" {
		if err := p.Authenticate(""); err != nil {
			return nil, fmt.Errorf("connection failed: %w", err)
		}
	} else {
		key, err := cfgMgr.GetAPIKey(name)
		if err != nil || key == "" {
			if _, exists := cfg.APIKeySources[name]; !exists {
				return nil, fmt.Errorf("not configured")
			}
			return nil, fmt.Errorf("credential configured but unavailable: %v", err)
		}
		if err := p.Authenticate(key); err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
	}

	models := make([]provider.Model, 0)
	if discovered, err := p.DiscoverModels(); err == nil {
		models = append(models, discovered...)
	}
	if listed, err := p.ListModels(); err == nil {
		models = append(models, listed...)
	}

	modelMap := make(map[string]provider.Model)
	for _, m := range models {
		modelMap[m.Name] = m
	}

	merged := make([]provider.Model, 0, len(modelMap))
	for _, m := range modelMap {
		merged = append(merged, m)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Name < merged[j].Name
	})

	rateInfo, _ := p.GetRateLimitInfo()
	quotaInfo, _ := p.GetQuotaInfo()

	return &providerSnapshot{
		name:   name,
		models: merged,
		rate:   rateInfo,
		quota:  quotaInfo,
	}, nil
}

func loadAllProviderModels(cfgMgr *config.Manager, cfg *config.Config) ([]provider.Model, map[string]*providerSnapshot, error) {
	providerNames := provider.GetProviderNames()
	allModels := make([]provider.Model, 0)
	snapshots := make(map[string]*providerSnapshot)

	for _, name := range providerNames {
		snapshot, err := loadProviderSnapshot(cfgMgr, cfg, name)
		if err != nil {
			continue
		}
		snapshots[name] = snapshot
		allModels = append(allModels, snapshot.models...)
	}

	if len(allModels) == 0 {
		return nil, snapshots, fmt.Errorf("no models found")
	}

	sort.Slice(allModels, func(i, j int) bool {
		if allModels[i].Provider == allModels[j].Provider {
			return allModels[i].Name < allModels[j].Name
		}
		return allModels[i].Provider < allModels[j].Provider
	})

	return allModels, snapshots, nil
}
