package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mojomast/nexdev/internal/config"
	"github.com/mojomast/nexdev/internal/design"
	"github.com/mojomast/nexdev/internal/devplan"
	"github.com/mojomast/nexdev/internal/state"
)

// PlanHandlers contains handlers for planning-related tools
type PlanHandlers struct {
	configManager *config.Manager
}

// NewPlanHandlers creates a new plan handlers instance
func NewPlanHandlers(configManager *config.Manager) *PlanHandlers {
	return &PlanHandlers{
		configManager: configManager,
	}
}

// RegisterHandlers registers plan tools with the registry
func (h *PlanHandlers) RegisterHandlers(registry *ToolRegistry) error {
	// Legacy stdio MCP is disabled until rebuilt over the M11 control-plane
	// services. These handlers call providers directly and must not be exposed.
	if !LegacyStdioRegistrationEnabled {
		return nil
	}

	tools := []struct {
		tool    Tool
		handler ToolHandler
	}{
		{h.createDevPlanTool(), h.handleCreateDevPlan},
	}

	for _, t := range tools {
		if err := registry.RegisterTool(t.tool, t.handler); err != nil {
			return fmt.Errorf("failed to register tool %s: %w", t.tool.Name, err)
		}
	}

	return nil
}

// Tool definitions

func (h *PlanHandlers) createDevPlanTool() Tool {
	return Tool{
		Name:        "create_devplan",
		Description: "Generate development plan with 7-10 phases and 3-5 tasks each",
		InputSchema: CreateInputSchema(
			map[string]interface{}{
				"projectPath": StringParam("Absolute path to the project directory"),
				"model":       StringParam("Model to use for devplan generation"),
			},
			[]string{"projectPath"},
		),
	}
}

// Handler implementations

func (h *PlanHandlers) handleCreateDevPlan(ctx context.Context, args map[string]interface{}) (*CallToolResult, error) {
	projectPath, err := ValidateAndGetString(args, "projectPath", true)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := validateProjectPath(projectPath); err != nil {
		return ErrorResult(err.Error()), nil
	}

	model, _ := ValidateAndGetString(args, "model", false)

	store, err := openStateStore(projectPath)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	defer store.Close()

	projectID := getProjectID(projectPath)

	// Load architecture from disk (same path used by CLI design flow).
	var designArch design.Architecture
	archPath := filepath.Join(projectPath, ".geoffrussy", "architecture.json")
	archContent, err := os.ReadFile(archPath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Architecture file not found at %s. Please run generate_design first.", archPath)), nil
	}

	if err := json.Unmarshal(archContent, &designArch); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to parse architecture file: %v", err)), nil
	}

	// Get interview data
	interviewData, err := store.GetInterviewData(projectID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to get interview data: %v", err)), nil
	}

	prov, modelName, err := initProviderForStage(h.configManager, "devplan.generate", model)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to initialize provider: %v", err)), nil
	}

	generator := devplan.NewGenerator(prov, modelName, nil)

	phases, err := generator.GeneratePhases(&designArch, interviewData)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to generate phases: %v", err)), nil
	}

	// Save phases and tasks to DB
	// We need to convert devplan.Phase to state.Phase and devplan.Task to state.Task

	// Reset existing progress if any?
	if err := store.ResetProjectProgress(projectID); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to reset project progress: %v", err)), nil
	}

	// Also delete existing phases? ResetProjectProgress just updates status.
	// If we are regenerating plan, we probably want to wipe old phases.
	// `store.ListPhases` then `store.DeletePhase` loop?
	existingPhases, err := store.ListPhases(projectID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to list existing phases: %v", err)), nil
	}
	for _, p := range existingPhases {
		if err := store.DeletePhase(p.ID); err != nil {
			return ErrorResult(fmt.Sprintf("Failed to delete phase %s: %v", p.ID, err)), nil
		}
	}

	totalTasks := 0
	for _, p := range phases {
		// Save phase
		// Generate markdown content for the phase
		content, err := generator.ExportPhaseMarkdown(&p)
		if err != nil {
			return ErrorResult(fmt.Sprintf("Failed to export phase %d: %v", p.Number, err)), nil
		}

		statePhase := &state.Phase{
			ID:        p.ID,
			ProjectID: projectID,
			Number:    p.Number,
			Title:     p.Title,
			Content:   content,
			Status:    state.PhaseNotStarted, // Default
			CreatedAt: time.Now(),
		}

		if err := store.SavePhase(statePhase); err != nil {
			return ErrorResult(fmt.Sprintf("Failed to save phase %d: %v", p.Number, err)), nil
		}

		// Save tasks
		for _, t := range p.Tasks {
			stateTask := &state.Task{
				ID:          t.ID,
				PhaseID:     p.ID,
				Number:      t.Number,
				Description: t.Description,
				Status:      state.TaskNotStarted,
			}
			if err := store.SaveTask(stateTask); err != nil {
				return ErrorResult(fmt.Sprintf("Failed to save task %s: %v", t.ID, err)), nil
			}
			totalTasks++
		}
	}

	// Update project stage
	if err := store.UpdateProjectStage(projectID, state.StagePlan); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to update project stage: %v", err)), nil
	}

	avgTasks := 0.0
	if len(phases) > 0 {
		avgTasks = float64(totalTasks) / float64(len(phases))
	}

	summary := fmt.Sprintf("📋 DevPlan Generation Complete\n\nGenerated development plan with:\n- %d phases\n- %d tasks total\n- Average %.1f tasks per phase\n\nNext step: Run execute_phase to start development.",
		len(phases), totalTasks, avgTasks)

	return SuccessResult(summary), nil
}
