package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/config"
	"github.com/mojomast/nexdev/internal/design"
	"github.com/mojomast/nexdev/internal/state"
)

// DesignHandlers contains handlers for design-related tools
type DesignHandlers struct {
	configManager *config.Manager
}

// NewDesignHandlers creates a new design handlers instance
func NewDesignHandlers(configManager *config.Manager) *DesignHandlers {
	return &DesignHandlers{
		configManager: configManager,
	}
}

// RegisterHandlers registers design tools with the registry
func (h *DesignHandlers) RegisterHandlers(registry *ToolRegistry) error {
	// Legacy stdio MCP is disabled until rebuilt over the M11 control-plane
	// services. These handlers call providers directly and must not be exposed.
	if !LegacyStdioRegistrationEnabled {
		return nil
	}

	tools := []struct {
		tool    Tool
		handler ToolHandler
	}{
		{h.generateDesignTool(), h.handleGenerateDesign},
		{h.regenerateDesignTool(), h.handleRegenerateDesign},
	}

	for _, t := range tools {
		if err := registry.RegisterTool(t.tool, t.handler); err != nil {
			return fmt.Errorf("failed to register tool %s: %w", t.tool.Name, err)
		}
	}

	return nil
}

// Tool definitions

func (h *DesignHandlers) generateDesignTool() Tool {
	return Tool{
		Name:        "generate_design",
		Description: "Generate system architecture from interview requirements",
		InputSchema: CreateInputSchema(
			map[string]interface{}{
				"projectPath": StringParam("Absolute path to the project directory"),
				"model":       StringParam("Model to use for architecture generation"),
				"regenerate":  BooleanParam("Regenerate architecture if one already exists"),
			},
			[]string{"projectPath"},
		),
	}
}

func (h *DesignHandlers) regenerateDesignTool() Tool {
	return Tool{
		Name:        "regenerate_design",
		Description: "Regenerate architecture with optional guidance for modifications",
		InputSchema: CreateInputSchema(
			map[string]interface{}{
				"projectPath":        StringParam("Absolute path to the project directory"),
				"guidance":           StringParam("Specific changes or improvements to make"),
				"preserveComponents": BooleanParam("Preserve existing components where possible"),
			},
			[]string{"projectPath"},
		),
	}
}

// Handler implementations

func (h *DesignHandlers) handleGenerateDesign(ctx context.Context, args map[string]interface{}) (*CallToolResult, error) {
	projectPath, err := ValidateAndGetString(args, "projectPath", true)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := validateProjectPath(projectPath); err != nil {
		return ErrorResult(err.Error()), nil
	}

	model, _ := ValidateAndGetString(args, "model", false)
	regenerate, _ := ValidateAndGetBool(args, "regenerate", false, false)

	store, err := openStateStore(projectPath)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	defer store.Close()

	projectID := getProjectID(projectPath)

	// Check if interview is complete
	interviewData, err := store.GetInterviewData(projectID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to get interview data: %v. Please run run_interview first.", err)), nil
	}

	// Check if architecture already exists
	// Assuming architecture is stored in a file or DB. The prompt says .geoffrussy/architecture.json
	archPath := filepath.Join(projectPath, ".geoffrussy", "architecture.json")
	if _, err := os.Stat(archPath); err == nil && !regenerate {
		return ErrorResult(fmt.Sprintf("Architecture already exists at %s. Use regenerate=true or regenerate_design to overwrite.", archPath)), nil
	}

	prov, modelName, err := initProviderForStage(h.configManager, "design.generate", model)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to initialize provider: %v", err)), nil
	}

	// Pass nil cache for now; caching can be wired later when a CacheStore is available
	generator := design.NewGenerator(prov, modelName, nil)

	arch, err := generator.GenerateArchitecture(interviewData)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to generate architecture: %v", err)), nil
	}

	// Save architecture
	jsonStr, err := generator.ExportJSON(arch)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to export architecture: %v", err)), nil
	}

	if err := writeArchitectureJSON(archPath, jsonStr); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to save architecture file: %v", err)), nil
	}

	mdContent, err := generator.ExportMarkdown(arch)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to export architecture markdown: %v", err)), nil
	}

	stateArch := &state.Architecture{
		ProjectID: projectID,
		Content:   mdContent,
		CreatedAt: time.Now(),
	}

	if err := store.SaveArchitecture(projectID, stateArch); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to save architecture to store: %v", err)), nil
	}

	// Update project stage
	if err := store.UpdateProjectStage(projectID, state.StageDesign); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to update project stage: %v", err)), nil
	}

	summary := fmt.Sprintf("🏗️ Architecture Generation Complete\n\nGenerated comprehensive system architecture including:\n- System Overview\n- %d Components\n- %d Data Flows\n\nArchitecture saved to: .geoffrussy/architecture.json\nView with: project://architecture resource\n\nNext step: Run create_devplan to generate development phases.", len(arch.Components), len(arch.DataFlows))

	return SuccessResult(summary), nil
}

func (h *DesignHandlers) handleRegenerateDesign(ctx context.Context, args map[string]interface{}) (*CallToolResult, error) {
	projectPath, err := ValidateAndGetString(args, "projectPath", true)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := validateProjectPath(projectPath); err != nil {
		return ErrorResult(err.Error()), nil
	}

	guidance, _ := ValidateAndGetString(args, "guidance", false)
	preserveComponents, _ := ValidateAndGetBool(args, "preserveComponents", false, true)

	// Validate guidance text if provided (max 50KB)
	if guidance != "" {
		if err := validateTextInput("guidance", guidance, 51200); err != nil {
			return ErrorResult(err.Error()), nil
		}
	}

	store, err := openStateStore(projectPath)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	defer store.Close()

	projectID := getProjectID(projectPath)
	interviewData, err := store.GetInterviewData(projectID)
	if err != nil {
		return ErrorResult("Interview data not found"), nil
	}

	prov, modelName, err := initProviderForStage(h.configManager, "design.refine", "")
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to initialize provider: %v", err)), nil
	}

	// Pass nil cache for now; caching can be wired later when a CacheStore is available
	generator := design.NewGenerator(prov, modelName, nil)

	var arch *design.Architecture
	var refinedSections []string

	if preserveComponents && guidance != "" {
		// Incremental refinement: load existing architecture and refine only
		// sections relevant to the guidance instead of full regeneration.
		arch, err = loadExistingArchitecture(projectPath, projectID)
		if err != nil {
			// Fall back to full regeneration if no existing architecture
			return h.fullRegenerate(generator, interviewData, guidance, projectPath, projectID, store)
		}

		// Determine which sections the guidance targets
		sections := identifyTargetSections(guidance, generator.ListRefinableSections())
		if len(sections) == 0 {
			// Guidance doesn't match specific sections; refine system_overview as default
			sections = []string{"system_overview"}
		}

		// Refine each identified section
		for _, section := range sections {
			arch, err = generator.RefineArchitecture(arch, section, guidance)
			if err != nil {
				return ErrorResult(fmt.Sprintf("Failed to refine section '%s': %v", section, err)), nil
			}
			refinedSections = append(refinedSections, section)
		}
	} else {
		// Full regeneration (original behavior)
		result, callErr := h.fullRegenerate(generator, interviewData, guidance, projectPath, projectID, store)
		return result, callErr
	}

	// Save the refined architecture
	archPath := filepath.Join(projectPath, ".geoffrussy", "architecture.json")
	jsonStr, err := generator.ExportJSON(arch)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to export: %v", err)), nil
	}
	if err := writeArchitectureJSON(archPath, jsonStr); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to save: %v", err)), nil
	}

	mdContent, err := generator.ExportMarkdown(arch)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to export markdown: %v", err)), nil
	}

	stateArch := &state.Architecture{
		ProjectID: projectID,
		Content:   mdContent,
		CreatedAt: time.Now(),
	}

	if err := store.SaveArchitecture(projectID, stateArch); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to save architecture to store: %v", err)), nil
	}

	if err := store.UpdateProjectStage(projectID, state.StageDesign); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to update project stage: %v", err)), nil
	}

	summary := fmt.Sprintf("Architecture incrementally refined.\nSections updated: %s\nUnchanged sections were preserved.", strings.Join(refinedSections, ", "))
	return SuccessResult(summary), nil
}

// fullRegenerate performs a complete architecture regeneration with optional guidance.
func (h *DesignHandlers) fullRegenerate(generator *design.Generator, interviewData *state.InterviewData, guidance, projectPath, projectID string, store *state.Store) (*CallToolResult, error) {
	if guidance != "" {
		interviewData.ProblemStatement += "\n\nAdditional Guidance: " + guidance
	}

	arch, err := generator.GenerateArchitecture(interviewData)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to regenerate: %v", err)), nil
	}

	archPath := filepath.Join(projectPath, ".geoffrussy", "architecture.json")
	jsonStr, err := generator.ExportJSON(arch)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to export: %v", err)), nil
	}
	if err := writeArchitectureJSON(archPath, jsonStr); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to save: %v", err)), nil
	}

	mdContent, err := generator.ExportMarkdown(arch)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to export markdown: %v", err)), nil
	}

	stateArch := &state.Architecture{
		ProjectID: projectID,
		Content:   mdContent,
		CreatedAt: time.Now(),
	}

	if err := store.SaveArchitecture(projectID, stateArch); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to save architecture to store: %v", err)), nil
	}

	if err := store.UpdateProjectStage(projectID, state.StageDesign); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to update project stage: %v", err)), nil
	}

	return SuccessResult("Architecture fully regenerated with guidance."), nil
}

// loadExistingArchitecture loads the architecture from the JSON file on disk.
func loadExistingArchitecture(projectPath, projectID string) (*design.Architecture, error) {
	archPath := filepath.Join(projectPath, ".geoffrussy", "architecture.json")
	data, err := os.ReadFile(archPath)
	if err != nil {
		return nil, fmt.Errorf("no existing architecture found: %w", err)
	}

	var arch design.Architecture
	if err := json.Unmarshal(data, &arch); err != nil {
		return nil, fmt.Errorf("failed to parse existing architecture: %w", err)
	}

	if arch.ProjectID == "" {
		arch.ProjectID = projectID
	}

	return &arch, nil
}

// identifyTargetSections determines which architecture sections are relevant to the guidance.
// It matches keywords in the guidance against known section names.
func identifyTargetSections(guidance string, allSections []string) []string {
	lower := strings.ToLower(guidance)

	// Map of keywords to section names
	keywordMap := map[string]string{
		"overview":      "system_overview",
		"system":        "system_overview",
		"component":     "components",
		"service":       "components",
		"technology":    "technology_rationale",
		"tech":          "technology_rationale",
		"rationale":     "technology_rationale",
		"scale":         "scaling_strategy",
		"scaling":       "scaling_strategy",
		"performance":   "scaling_strategy",
		"cache":         "scaling_strategy",
		"load":          "scaling_strategy",
		"api":           "api_contract",
		"endpoint":      "api_contract",
		"rest":          "api_contract",
		"websocket":     "api_contract",
		"database":      "database_schema",
		"schema":        "database_schema",
		"table":         "database_schema",
		"sql":           "database_schema",
		"security":      "security",
		"auth":          "security",
		"encrypt":       "security",
		"audit":         "security",
		"observability": "observability",
		"logging":       "observability",
		"metric":        "observability",
		"tracing":       "observability",
		"monitor":       "observability",
		"deploy":        "deployment",
		"deployment":    "deployment",
		"staging":       "deployment",
		"production":    "deployment",
		"kubernetes":    "deployment",
		"docker":        "deployment",
		"risk":          "risks",
		"mitigation":    "risks",
	}

	// Also allow exact section name matches
	sectionSet := make(map[string]bool)
	for _, s := range allSections {
		sectionSet[s] = true
	}

	matched := make(map[string]bool)

	// Check for exact section names in guidance
	for _, section := range allSections {
		if strings.Contains(lower, strings.ReplaceAll(section, "_", " ")) || strings.Contains(lower, section) {
			matched[section] = true
		}
	}

	// Check for keyword matches
	for keyword, section := range keywordMap {
		if strings.Contains(lower, keyword) {
			if sectionSet[section] {
				matched[section] = true
			}
		}
	}

	var result []string
	for section := range matched {
		result = append(result, section)
	}
	return result
}

func writeArchitectureJSON(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(content), 0o644)
}
