package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mojomast/nexdev/internal/config"
	"github.com/mojomast/nexdev/internal/provider"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the current Geoffrussy configuration",
	Long: `Validate the current Geoffrussy configuration by checking:
- Required API keys are configured
- Default models are set for each stage
- Configuration file is valid`,
	RunE: runValidate,
}

func runValidate(cmd *cobra.Command, args []string) error {
	fmt.Println("🔍 Validating Geoffrussy configuration...")

	// Get configuration
	cfgManager := config.NewManager()
	if err := cfgManager.Load(nil); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Check for required API keys — dynamically discovered from the provider registry
	registeredProviders := provider.GetProviderNames()

	fmt.Println("\nAPI Keys:")
	allValid := true
	configuredCount := 0
	for _, p := range registeredProviders {
		_, err := cfgManager.GetAPIKey(p)
		if err != nil {
			fmt.Printf("  - %s: Not configured\n", p)
		} else {
			source := cfgManager.GetAPIKeySource(p)
			fmt.Printf("  ✓ %s: Configured (%s)\n", p, source)
			configuredCount++
		}
	}
	if configuredCount == 0 {
		fmt.Println("  ✗ No providers configured — at least one API key is required")
		allValid = false
	}

	// Check for default models
	fmt.Println("\nDefault Models:")
	stages := []string{"interview", "design", "plan", "develop"}
	allModelsValid := true
	for _, stage := range stages {
		_, err := cfgManager.ResolveDefaultModel(stage)
		if err != nil {
			fmt.Printf("  ✗ %s: No default model configured\n", stage)
			allModelsValid = false
		} else {
			fmt.Printf("  ✓ %s: Configured\n", stage)
		}
	}

	// Check configuration file
	configPath := cfgManager.GetConfigPath()
	if configPath != "" {
		fileInfo, err := os.Stat(configPath)
		if err != nil {
			fmt.Printf("  ✗ Configuration file not found: %s\n", configPath)
			allValid = false
		} else {
			fmt.Printf("  ✓ Configuration file exists: %s (%d bytes)\n", configPath, fileInfo.Size())
		}
	}

	// Summary
	fmt.Println("\n" + strings.Repeat("=", 40))
	if allValid && allModelsValid {
		fmt.Println("✓ All validations passed!")
		return nil
	} else {
		fmt.Println("✗ Validation failed")
		os.Exit(1)
		return nil
	}
}
