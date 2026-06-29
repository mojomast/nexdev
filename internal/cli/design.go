package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/config"
	"github.com/mojomast/nexdev/internal/design"
	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
	"github.com/spf13/cobra"
)

var (
	designModel  string
	designRefine string
)

var designCmd = &cobra.Command{
	Use:   "design",
	Short: "Generate or refine architecture design",
	Long: `Generate architecture design from interview data or refine
existing architecture by updating specific sections.`,
	RunE: runDesign,
}

func init() {
	designCmd.Flags().StringVar(&designModel, "model", "", "Model to use for design generation")
	designCmd.Flags().StringVar(&designRefine, "refine", "", "Section to refine (e.g., technology, scaling)")
}

func runDesign(cmd *cobra.Command, args []string) error {
	if designRefine != "" {
		fmt.Printf("🏗️  Refining Architecture Design (Section: %s)...\n", designRefine)
	} else {
		fmt.Println("🏗️  Generating Architecture Design...")
	}
	fmt.Println("════════════════════════════════════════════════════════")
	fmt.Println()

	// 1. Initialize configuration
	cfgMgr := config.NewManager()
	if err := cfgMgr.Load(nil); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// 2. Initialize state store
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	projectID := filepath.Base(cwd)

	// Use same database location as other commands
	dbPath := filepath.Join(cwd, ".geoffrussy", "state.db")
	store, err := state.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open state store: %w", err)
	}
	defer store.Close()

	// 3. Ensure project and interview data exist
	_, err = store.GetProject(projectID)
	if err != nil {
		return fmt.Errorf("project not found. Please run 'geoffrussy init' first: %w", err)
	}

	interviewData, err := store.GetInterviewData(projectID)
	if err != nil {
		return fmt.Errorf("interview data not found. Please run 'geoffrussy interview' first: %w", err)
	}

	// 4. Setup Provider
	stageKey := "design.generate"
	if designRefine != "" {
		stageKey = "design.refine"
	}

	providerName, modelName, err := getProviderAndModel(cfgMgr, stageKey, designModel)
	if err != nil {
		fmt.Println("\n⚠️  Could not automatically select provider and model")
		fmt.Println("   Available options:")
		fmt.Println("   1. Run './geoffrussy config' to set up providers")
		fmt.Println("   2. Run './geoffrussy config --list-providers' to see available models")
		fmt.Println("   3. Use '--model <model-name>' flag to specify a model")
		return fmt.Errorf("failed to get provider and model: %w", err)
	}

	fmt.Printf("📦 Using Provider: %s\n", providerName)
	fmt.Printf("🤖 Using Model: %s\n", modelName)
	fmt.Println()

	bridge := provider.NewBridge()
	if err := setupProvider(bridge, cfgMgr, providerName); err != nil {
		return fmt.Errorf("failed to setup provider: %w", err)
	}

	prov, err := bridge.GetProvider(providerName)
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}
	printProviderUsageSnapshot(providerName, prov)

	// 5. Initialize Generator
	generator := design.NewGenerator(prov, modelName, store)

	if designRefine != "" {
		return handleRefinement(generator, store, projectID, designRefine)
	}

	return handleGeneration(generator, store, interviewData, projectID)
}

func handleGeneration(generator *design.Generator, store *state.Store, interviewData *state.InterviewData, projectID string) error {
	// Check if architecture already exists
	if _, err := loadArchitectureFromDisk(projectID); err == nil {
		fmt.Printf("⚠️  Architecture already exists for project '%s'.\n", projectID)
		fmt.Print("Do you want to overwrite it? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("❌ Operation cancelled.")
			return nil
		}
	}

	fmt.Println("🧠 Analyzing interview data and generating architecture...")
	fmt.Println("   This may take a minute...")

	arch, err := generator.GenerateArchitecture(interviewData)
	if err != nil {
		return fmt.Errorf("failed to generate architecture: %w", err)
	}

	// Save structured data to disk
	if err := saveArchitectureToDisk(projectID, arch); err != nil {
		return fmt.Errorf("failed to save architecture to disk: %w", err)
	}

	// Export markdown and save to store
	mdContent, err := generator.ExportMarkdown(arch)
	if err != nil {
		return fmt.Errorf("failed to export markdown: %w", err)
	}

	stateArch := &state.Architecture{
		ProjectID: projectID,
		Content:   mdContent,
		CreatedAt: time.Now(),
	}

	if err := store.SaveArchitecture(projectID, stateArch); err != nil {
		return fmt.Errorf("failed to save architecture to store: %w", err)
	}

	// Update project stage
	if err := store.UpdateProjectStage(projectID, state.StageDesign); err != nil {
		// Log error but continue
		fmt.Printf("Warning: failed to update project stage: %v\n", err)
	}

	fmt.Println("\n✅ Architecture generated successfully!")
	fmt.Println("   - Saved structured data to .geoffrussy/architecture.json")
	fmt.Println("   - Saved display document to database")
	fmt.Println("\n💡 Next steps:")
	fmt.Println("   Run 'geoffrussy design --refine <section>' to refine specific parts")
	fmt.Println("   Run 'geoffrussy plan' to generate a development plan")

	return nil
}

func handleRefinement(generator *design.Generator, store *state.Store, projectID string, section string) error {
	arch, err := loadArchitectureFromDisk(projectID)
	if err != nil {
		return fmt.Errorf("no architecture found to refine. Run 'geoffrussy design' first: %w", err)
	}

	// Validate section (simple check)
	validSections := generator.ListRefinableSections()
	isValid := false
	for _, s := range validSections {
		if s == section {
			isValid = true
			break
		}
	}

	if !isValid {
		fmt.Printf("⚠️  Invalid section '%s'. Valid sections are:\n", section)
		for _, s := range validSections {
			fmt.Printf("   - %s\n", s)
		}
		return fmt.Errorf("invalid section")
	}

	fmt.Printf("\nRefining section: %s\n", section)
	fmt.Println("Please describe the changes you want to make:")
	fmt.Print("> ")

	reader := bufio.NewReader(os.Stdin)
	request, _ := reader.ReadString('\n')
	request = strings.TrimSpace(request)

	if request == "" {
		fmt.Println("❌ No refinement request provided.")
		return nil
	}

	fmt.Println("\n🧠 Refining architecture...")

	updatedArch, err := generator.RefineArchitecture(arch, section, request)
	if err != nil {
		return fmt.Errorf("failed to refine architecture: %w", err)
	}

	// Save structured data to disk
	if err := saveArchitectureToDisk(projectID, updatedArch); err != nil {
		return fmt.Errorf("failed to save architecture to disk: %w", err)
	}

	// Export markdown and save to store
	mdContent, err := generator.ExportMarkdown(updatedArch)
	if err != nil {
		return fmt.Errorf("failed to export markdown: %w", err)
	}

	stateArch := &state.Architecture{
		ProjectID: projectID,
		Content:   mdContent,
		CreatedAt: time.Now(),
	}

	if err := store.SaveArchitecture(projectID, stateArch); err != nil {
		return fmt.Errorf("failed to save architecture to store: %w", err)
	}

	fmt.Println("\n✅ Architecture refined successfully!")

	return nil
}

func saveArchitectureToDisk(projectID string, arch *design.Architecture) error {
	data, err := json.MarshalIndent(arch, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	path := filepath.Join(".geoffrussy", "architecture.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func loadArchitectureFromDisk(projectID string) (*design.Architecture, error) {
	path := filepath.Join(".geoffrussy", "architecture.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var arch design.Architecture
	if err := json.Unmarshal(data, &arch); err != nil {
		return nil, err
	}

	return &arch, nil
}
