package mcp

import (
	"context"
	"fmt"

	"github.com/mojomast/nexdev/internal/config"
	"github.com/mojomast/nexdev/internal/interview"
	"github.com/mojomast/nexdev/internal/state"
)

// InterviewHandlers contains handlers for interview-related tools
type InterviewHandlers struct {
	configManager *config.Manager
}

// NewInterviewHandlers creates a new interview handlers instance
func NewInterviewHandlers(configManager *config.Manager) *InterviewHandlers {
	return &InterviewHandlers{
		configManager: configManager,
	}
}

// RegisterHandlers registers interview tools with the registry
func (h *InterviewHandlers) RegisterHandlers(registry *ToolRegistry) error {
	// Legacy stdio MCP is disabled until rebuilt over the M11 control-plane
	// services. These handlers call providers directly and must not be exposed.
	if !LegacyStdioRegistrationEnabled {
		return nil
	}

	tools := []struct {
		tool    Tool
		handler ToolHandler
	}{
		{h.runInterviewTool(), h.handleRunInterview},
		{h.submitInterviewAnswerTool(), h.handleSubmitInterviewAnswer},
	}

	for _, t := range tools {
		if err := registry.RegisterTool(t.tool, t.handler); err != nil {
			return fmt.Errorf("failed to register tool %s: %w", t.tool.Name, err)
		}
	}

	return nil
}

// Tool definitions

func (h *InterviewHandlers) runInterviewTool() Tool {
	return Tool{
		Name:        "run_interview",
		Description: "Start or resume the project interview to gather requirements through 5 phases",
		InputSchema: CreateInputSchema(
			map[string]interface{}{
				"projectPath": StringParam("Absolute path to the project directory"),
				"model":       StringParam("Model to use for interview (e.g., 'glm-4.7', 'gpt-4')"),
				"resume":      BooleanParam("Resume existing interview if available"),
			},
			[]string{"projectPath"},
		),
	}
}

func (h *InterviewHandlers) submitInterviewAnswerTool() Tool {
	return Tool{
		Name:        "submit_interview_answer",
		Description: "Submit an answer to the current interview question and proceed to next question",
		InputSchema: CreateInputSchema(
			map[string]interface{}{
				"projectPath": StringParam("Absolute path to the project directory"),
				"questionId":  StringParam("ID of the question being answered (from run_interview response)"),
				"answer":      StringParam("The answer to the current question"),
			},
			[]string{"projectPath", "questionId", "answer"},
		),
	}
}

// Handler implementations

func (h *InterviewHandlers) handleRunInterview(ctx context.Context, args map[string]interface{}) (*CallToolResult, error) {
	projectPath, err := ValidateAndGetString(args, "projectPath", true)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := validateProjectPath(projectPath); err != nil {
		return ErrorResult(err.Error()), nil
	}

	model, _ := ValidateAndGetString(args, "model", false)
	resume, _ := ValidateAndGetBool(args, "resume", false, false)

	store, err := openStateStore(projectPath)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	defer store.Close()

	projectID := getProjectID(projectPath)

	prov, modelName, err := initProviderForStage(h.configManager, "interview.run", model)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to initialize provider: %v", err)), nil
	}

	engine := interview.NewEngine(store, prov, modelName)
	var session *interview.InterviewSession

	if resume {
		session, err = engine.ResumeInterview(projectID)
		if err != nil {
			// If resume failed, try start new
			session, err = engine.StartInterview(projectID)
			if err != nil {
				return ErrorResult(fmt.Sprintf("Failed to start/resume interview: %v", err)), nil
			}
		}
	} else {
		session, err = engine.StartInterview(projectID)
		if err != nil {
			return ErrorResult(fmt.Sprintf("Failed to start interview: %v", err)), nil
		}
	}

	// Save the session so it can be retrieved on next call
	if err := engine.SaveSession(session); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to save session: %v", err)), nil
	}

	// Get current question
	question, err := engine.GetNextQuestion(session)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to get question: %v", err)), nil
	}

	if question == nil {
		return h.completeInterview(projectID, store, engine, session)
	}

	return h.formatQuestionResponse(session, question)
}

func (h *InterviewHandlers) handleSubmitInterviewAnswer(ctx context.Context, args map[string]interface{}) (*CallToolResult, error) {
	projectPath, err := ValidateAndGetString(args, "projectPath", true)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := validateProjectPath(projectPath); err != nil {
		return ErrorResult(err.Error()), nil
	}

	questionID, err := ValidateAndGetString(args, "questionId", true)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := validateIdentifier("questionId", questionID); err != nil {
		return ErrorResult(err.Error()), nil
	}

	answer, err := ValidateAndGetString(args, "answer", true)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	// Validate answer text: max 50KB, must be valid UTF-8
	if err := validateTextInput("answer", answer, 51200); err != nil {
		return ErrorResult(err.Error()), nil
	}

	store, err := openStateStore(projectPath)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	defer store.Close()

	projectID := getProjectID(projectPath)

	prov, modelName, _ := initProviderForStage(h.configManager, "interview.followup", "")

	engine := interview.NewEngine(store, prov, modelName)
	session, err := engine.LoadSession(projectID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to load interview session: %v", err)), nil
	}

	if err := engine.RecordAnswer(session, questionID, answer); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to record answer: %v", err)), nil
	}

	if err := engine.SaveSession(session); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to save session: %v", err)), nil
	}

	// Move to next question
	nextQuestion, err := engine.GetNextQuestion(session)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to get next question: %v", err)), nil
	}

	if nextQuestion == nil {
		return h.completeInterview(projectID, store, engine, session)
	}

	return h.formatQuestionResponse(session, nextQuestion, fmt.Sprintf("✅ Answer recorded for %s", questionID))
}

func (h *InterviewHandlers) completeInterview(projectID string, store *state.Store, engine *interview.Engine, session *interview.InterviewSession) (*CallToolResult, error) {
	if err := engine.SaveSession(session); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to save completed session: %v", err)), nil
	}

	if err := store.UpdateProjectStage(projectID, state.StageDesign); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to update project stage: %v", err)), nil
	}

	return h.handleInterviewComplete(engine, session)
}

func (h *InterviewHandlers) handleInterviewComplete(engine *interview.Engine, session *interview.InterviewSession) (*CallToolResult, error) {
	complete, _ := engine.ValidateCompleteness(session)

	summary := "✅ Interview Complete!\n\nAll phases completed. Requirements have been saved.\n\nNext step: Run generate_design to create system architecture."
	if !complete {
		summary = "⚠️ Interview ended but some required questions are missing."
	}

	return &CallToolResult{
		Content: []Content{TextContent(summary)},
		IsError: false,
	}, nil
}

func (h *InterviewHandlers) formatQuestionResponse(session *interview.InterviewSession, question *interview.Question, prefix ...string) (*CallToolResult, error) {
	text := ""
	if len(prefix) > 0 {
		text += prefix[0] + "\n\n"
	}

	text += fmt.Sprintf("Interview Phase: %s\n", formatPhaseName(session.CurrentPhase))
	text += fmt.Sprintf("Question: %s\n", question.Text)
	text += fmt.Sprintf("ID: %s\n\n", question.ID)
	text += "Provide your answer using the submit_interview_answer tool."

	return &CallToolResult{
		Content: []Content{TextContent(text)},
		IsError: false,
	}, nil
}

func formatPhaseName(phase interview.Phase) string {
	switch phase {
	case interview.PhaseProjectEssence:
		return "Project Essence"
	case interview.PhaseTechnicalConstraints:
		return "Technical Constraints"
	case interview.PhaseIntegrationPoints:
		return "Integration Points"
	case interview.PhaseScopeDefinition:
		return "Scope Definition"
	case interview.PhaseRefinementValidation:
		return "Refinement & Validation"
	default:
		return string(phase)
	}
}
