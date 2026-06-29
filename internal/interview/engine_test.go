package interview

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
)

func TestInterviewEngine(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := state.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create test project
	project := &state.Project{
		ID:           "test-project",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInterview,
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	engine := NewEngine(store, nil, "")

	t.Run("GetAllPhases", func(t *testing.T) {
		phases := engine.GetAllPhases()
		if len(phases) != 5 {
			t.Errorf("Expected 5 phases, got %d", len(phases))
		}

		expectedPhases := []Phase{
			PhaseProjectEssence,
			PhaseTechnicalConstraints,
			PhaseIntegrationPoints,
			PhaseScopeDefinition,
			PhaseRefinementValidation,
		}

		for i, expected := range expectedPhases {
			if phases[i] != expected {
				t.Errorf("Phase %d: expected %s, got %s", i, expected, phases[i])
			}
		}
	})

	t.Run("GetPhaseQuestions", func(t *testing.T) {
		questions := engine.GetPhaseQuestions(PhaseProjectEssence)
		if len(questions) == 0 {
			t.Error("Expected questions for Project Essence phase")
		}

		for _, q := range questions {
			if q.ID == "" {
				t.Error("Question ID should not be empty")
			}
			if q.Text == "" {
				t.Error("Question text should not be empty")
			}
			if q.Phase != PhaseProjectEssence {
				t.Errorf("Expected phase %s, got %s", PhaseProjectEssence, q.Phase)
			}
		}
	})

	t.Run("StartInterview", func(t *testing.T) {
		session, err := engine.StartInterview(project.ID)
		if err != nil {
			t.Fatalf("Failed to start interview: %v", err)
		}

		if session.ProjectID != project.ID {
			t.Errorf("Expected project ID %s, got %s", project.ID, session.ProjectID)
		}

		if session.CurrentPhase != PhaseProjectEssence {
			t.Errorf("Expected initial phase %s, got %s", PhaseProjectEssence, session.CurrentPhase)
		}

		if session.CurrentQuestion != 0 {
			t.Errorf("Expected current question 0, got %d", session.CurrentQuestion)
		}

		if session.Completed {
			t.Error("Session should not be completed initially")
		}
	})

	t.Run("GetNextQuestion", func(t *testing.T) {
		session, _ := engine.StartInterview(project.ID)

		question, err := engine.GetNextQuestion(session)
		if err != nil {
			t.Fatalf("Failed to get next question: %v", err)
		}

		if question == nil {
			t.Fatal("Expected a question, got nil")
		}

		if question.Phase != PhaseProjectEssence {
			t.Errorf("Expected phase %s, got %s", PhaseProjectEssence, question.Phase)
		}
	})

	t.Run("RecordAnswer", func(t *testing.T) {
		session, _ := engine.StartInterview(project.ID)
		question, _ := engine.GetNextQuestion(session)

		answerText := "This is a test answer"
		err := engine.RecordAnswer(session, question.ID, answerText)
		if err != nil {
			t.Fatalf("Failed to record answer: %v", err)
		}

		answer, ok := session.Answers[question.ID]
		if !ok {
			t.Fatal("Answer not found in session")
		}

		if answer.Text != answerText {
			t.Errorf("Expected answer text %s, got %s", answerText, answer.Text)
		}

		if session.CurrentQuestion != 1 {
			t.Errorf("Expected current question 1, got %d", session.CurrentQuestion)
		}
	})

	t.Run("CompletePhase", func(t *testing.T) {
		session, _ := engine.StartInterview(project.ID)

		// Answer all questions in first phase
		questions := engine.GetPhaseQuestions(PhaseProjectEssence)
		for _, q := range questions {
			engine.RecordAnswer(session, q.ID, "Test answer for "+q.ID)
		}

		// Get next question should move to next phase
		question, err := engine.GetNextQuestion(session)
		if err != nil {
			t.Fatalf("Failed to get next question: %v", err)
		}

		if question.Phase == PhaseProjectEssence {
			t.Error("Should have moved to next phase")
		}

		if session.CurrentPhase == PhaseProjectEssence {
			t.Error("Session should have moved to next phase")
		}
	})

	t.Run("GenerateSummary", func(t *testing.T) {
		session, _ := engine.StartInterview(project.ID)

		// Add some answers
		session.Answers["pe_1"] = Answer{
			QuestionID: "pe_1",
			Text:       "Solve world hunger",
			Timestamp:  time.Now(),
		}

		summary, err := engine.GenerateSummary(session)
		if err != nil {
			t.Fatalf("Failed to generate summary: %v", err)
		}

		if summary == "" {
			t.Error("Summary should not be empty")
		}

		if len(summary) < 10 {
			t.Error("Summary seems too short")
		}
	})

	t.Run("ExportToJSON", func(t *testing.T) {
		session, _ := engine.StartInterview(project.ID)

		// Add some answers
		session.Answers["pe_1"] = Answer{
			QuestionID: "pe_1",
			Text:       "Test problem",
			Timestamp:  time.Now(),
		}

		jsonStr, err := engine.ExportToJSON(session)
		if err != nil {
			t.Fatalf("Failed to export to JSON: %v", err)
		}

		if jsonStr == "" {
			t.Error("JSON export should not be empty")
		}

		// Verify it's valid JSON
		if jsonStr[0] != '{' {
			t.Error("JSON should start with {")
		}
	})

	t.Run("ProposeDefault", func(t *testing.T) {
		question := Question{
			ID:       "tc_1",
			Phase:    PhaseTechnicalConstraints,
			Text:     "What programming language?",
			Category: "language",
			Required: true,
		}

		defaultVal, err := engine.ProposeDefault(question)
		if err != nil {
			t.Fatalf("Failed to propose default: %v", err)
		}

		if defaultVal == "" {
			t.Error("Expected a default value for tc_1")
		}
	})
}

func TestInterviewEngine_PhaseProgression(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := state.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	project := &state.Project{
		ID:           "test-project",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInterview,
	}
	store.CreateProject(project)

	engine := NewEngine(store, nil, "")
	session, _ := engine.StartInterview(project.ID)

	// Go through all phases
	phases := engine.GetAllPhases()
	for phaseIdx, expectedPhase := range phases {
		t.Run(string(expectedPhase), func(t *testing.T) {
			if session.CurrentPhase != expectedPhase {
				t.Errorf("Expected phase %s, got %s", expectedPhase, session.CurrentPhase)
			}

			questions := engine.GetPhaseQuestions(expectedPhase)
			for range questions {
				question, err := engine.GetNextQuestion(session)
				if err != nil {
					t.Fatalf("Failed to get question: %v", err)
				}

				if question == nil && phaseIdx < len(phases)-1 {
					t.Fatal("Got nil question before interview complete")
				}

				if question != nil {
					engine.RecordAnswer(session, question.ID, "Test answer")
				}
			}

			// Call GetNextQuestion one more time to advance to next phase
			if phaseIdx < len(phases)-1 {
				engine.GetNextQuestion(session)
			}
		})
	}

	// After all phases, should be complete
	question, _ := engine.GetNextQuestion(session)
	if question != nil {
		t.Error("Expected no more questions after all phases")
	}

	if !session.Completed {
		t.Error("Session should be marked as completed")
	}
}

func TestInterviewEngine_SaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := state.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	project := &state.Project{
		ID:           "test-project",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInterview,
	}
	store.CreateProject(project)

	engine := NewEngine(store, nil, "")

	t.Run("SaveSession", func(t *testing.T) {
		session, _ := engine.StartInterview(project.ID)
		session.Answers["pe_1"] = Answer{
			QuestionID: "pe_1",
			Text:       "Test problem statement",
			Timestamp:  time.Now(),
		}

		err := engine.SaveSession(session)
		if err != nil {
			t.Fatalf("Failed to save session: %v", err)
		}
	})

	t.Run("LoadSession", func(t *testing.T) {
		session, err := engine.LoadSession(project.ID)
		if err != nil {
			t.Fatalf("Failed to load session: %v", err)
		}

		if session.ProjectID != project.ID {
			t.Errorf("Expected project ID %s, got %s", project.ID, session.ProjectID)
		}
	})
}

// MockProvider implements the provider.Provider interface for testing
type MockProvider struct {
	responses map[string]string
	callCount int
}

func NewMockProvider() *MockProvider {
	return &MockProvider{
		responses: make(map[string]string),
		callCount: 0,
	}
}

func (m *MockProvider) Name() string {
	return "mock"
}

func (m *MockProvider) Authenticate(apiKey string) error {
	return nil
}

func (m *MockProvider) IsAuthenticated() bool {
	return true
}

func (m *MockProvider) ListModels() ([]provider.Model, error) {
	return []provider.Model{}, nil
}

func (m *MockProvider) DiscoverModels() ([]provider.Model, error) {
	return []provider.Model{}, nil
}

func (m *MockProvider) Call(ctx context.Context, model string, prompt string) (*provider.Response, error) {
	m.callCount++

	// Return predefined responses based on prompt content
	for key, response := range m.responses {
		if key == "" || key == prompt {
			return &provider.Response{
				Content:      response,
				TokensInput:  100,
				TokensOutput: 50,
				Model:        model,
				Provider:     "mock",
			}, nil
		}
	}

	// Default response
	return &provider.Response{
		Content:      "Mock response",
		TokensInput:  100,
		TokensOutput: 50,
		Model:        model,
		Provider:     "mock",
	}, nil
}

func (m *MockProvider) Stream(ctx context.Context, model string, prompt string) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- "Mock stream response"
	close(ch)
	return ch, nil
}

func (m *MockProvider) GetRateLimitInfo() (*provider.RateLimitInfo, error) {
	return nil, nil
}

func (m *MockProvider) GetQuotaInfo() (*provider.QuotaInfo, error) {
	return nil, nil
}

func (m *MockProvider) SupportsCodingPlan() bool {
	return false
}

func (m *MockProvider) SetResponse(prompt string, response string) {
	m.responses[prompt] = response
}

func TestInterviewEngine_LLMPoweredFollowUp(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := state.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	project := &state.Project{
		ID:           "test-project",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInterview,
	}
	store.CreateProject(project)

	mockProvider := NewMockProvider()
	engine := NewEngine(store, mockProvider, "test-model")

	t.Run("GenerateFollowUp", func(t *testing.T) {
		question := Question{
			ID:       "pe_1",
			Phase:    PhaseProjectEssence,
			Text:     "What problem does your project solve?",
			Category: "problem_statement",
			Required: true,
		}

		answer := Answer{
			QuestionID: "pe_1",
			Text:       "We need a better way to manage tasks",
			Timestamp:  time.Now(),
		}

		mockProvider.SetResponse("", "What specific pain points do users experience with current task management tools?")

		followUp, err := engine.GenerateFollowUp(question, answer)
		if err != nil {
			t.Fatalf("Failed to generate follow-up: %v", err)
		}

		if followUp == "" {
			t.Error("Expected a follow-up question")
		}

		if mockProvider.callCount == 0 {
			t.Error("Expected provider to be called")
		}
	})

	t.Run("GenerateFollowUp_Skip", func(t *testing.T) {
		question := Question{
			ID:       "pe_1",
			Phase:    PhaseProjectEssence,
			Text:     "What problem does your project solve?",
			Category: "problem_statement",
			Required: true,
		}

		answer := Answer{
			QuestionID: "pe_1",
			Text:       "Comprehensive answer with all details",
			Timestamp:  time.Now(),
		}

		// Reset call count and set default response to SKIP
		mockProvider.callCount = 0
		mockProvider.responses = make(map[string]string)
		mockProvider.responses[""] = "SKIP" // Set default response

		followUp, err := engine.GenerateFollowUp(question, answer)
		if err != nil {
			t.Fatalf("Failed to generate follow-up: %v", err)
		}

		if followUp != "" {
			t.Errorf("Expected no follow-up when LLM returns SKIP, got: %s", followUp)
		}
	})

	t.Run("ProposeDefaultWithLLM", func(t *testing.T) {
		question := Question{
			ID:       "custom_1",
			Phase:    PhaseTechnicalConstraints,
			Text:     "What caching strategy will you use?",
			Category: "caching",
			Required: false,
		}

		mockProvider.callCount = 0
		mockProvider.SetResponse("", "Redis for session caching and CDN for static assets")

		defaultVal, err := engine.ProposeDefaultWithLLM(question)
		if err != nil {
			t.Fatalf("Failed to propose default: %v", err)
		}

		if defaultVal == "" {
			t.Error("Expected a default value")
		}

		if mockProvider.callCount == 0 {
			t.Error("Expected provider to be called")
		}
	})

	t.Run("AnalyzeAnswer", func(t *testing.T) {
		question := Question{
			ID:       "pe_1",
			Phase:    PhaseProjectEssence,
			Text:     "What problem does your project solve?",
			Category: "problem_statement",
			Required: true,
		}

		answer := Answer{
			QuestionID: "pe_1",
			Text:       "We need a task management system",
			Timestamp:  time.Now(),
		}

		mockProvider.callCount = 0
		mockProvider.SetResponse("", "KEY_POINTS: task management, system\nCOMPLETENESS: partial\nSUGGESTIONS: target users, specific features")

		analysis, err := engine.AnalyzeAnswer(question, answer)
		if err != nil {
			t.Fatalf("Failed to analyze answer: %v", err)
		}

		if analysis == nil {
			t.Fatal("Expected analysis result")
		}

		if analysis.RawAnalysis == "" {
			t.Error("Expected raw analysis to be populated")
		}
	})

	t.Run("AskWithFollowUp", func(t *testing.T) {
		session, _ := engine.StartInterview(project.ID)

		question := Question{
			ID:       "pe_1",
			Phase:    PhaseProjectEssence,
			Text:     "What problem does your project solve?",
			Category: "problem_statement",
			Required: true,
		}

		// Record an answer
		engine.RecordAnswer(session, question.ID, "We need better task management")

		mockProvider.callCount = 0
		mockProvider.SetResponse("", "What features are most important for your users?")

		followUps, err := engine.AskWithFollowUp(session, question, true)
		if err != nil {
			t.Fatalf("Failed to ask with follow-up: %v", err)
		}

		if len(followUps) == 0 {
			t.Error("Expected at least one follow-up question")
		}

		if mockProvider.callCount == 0 {
			t.Error("Expected provider to be called")
		}
	})

	t.Run("AskWithFollowUp_Disabled", func(t *testing.T) {
		session, _ := engine.StartInterview(project.ID)

		question := Question{
			ID:       "pe_2",
			Phase:    PhaseProjectEssence,
			Text:     "Who are the target users?",
			Category: "target_users",
			Required: true,
		}

		engine.RecordAnswer(session, question.ID, "Software developers")

		mockProvider.callCount = 0

		followUps, err := engine.AskWithFollowUp(session, question, false)
		if err != nil {
			t.Fatalf("Failed to ask with follow-up: %v", err)
		}

		if len(followUps) != 0 {
			t.Error("Expected no follow-ups when disabled")
		}

		if mockProvider.callCount != 0 {
			t.Error("Expected provider not to be called when follow-ups disabled")
		}
	})
}

func TestInterviewEngine_WithoutProvider(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := state.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	project := &state.Project{
		ID:           "test-project",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInterview,
	}
	store.CreateProject(project)

	// Engine without provider
	engine := NewEngine(store, nil, "")

	t.Run("GenerateFollowUp_NoProvider", func(t *testing.T) {
		question := Question{
			ID:       "pe_1",
			Phase:    PhaseProjectEssence,
			Text:     "What problem does your project solve?",
			Category: "problem_statement",
			Required: true,
		}

		answer := Answer{
			QuestionID: "pe_1",
			Text:       "Task management",
			Timestamp:  time.Now(),
		}

		followUp, err := engine.GenerateFollowUp(question, answer)
		if err != nil {
			t.Fatalf("Should not error without provider: %v", err)
		}

		if followUp != "" {
			t.Error("Expected empty follow-up without provider")
		}
	})

	t.Run("ProposeDefault_NoProvider", func(t *testing.T) {
		question := Question{
			ID:       "tc_1",
			Phase:    PhaseTechnicalConstraints,
			Text:     "What programming language?",
			Category: "language",
			Required: true,
		}

		// Should still return static default
		defaultVal, err := engine.ProposeDefault(question)
		if err != nil {
			t.Fatalf("Failed to propose default: %v", err)
		}

		if defaultVal != "Go" {
			t.Errorf("Expected static default 'Go', got '%s'", defaultVal)
		}
	})

	t.Run("AnalyzeAnswer_NoProvider", func(t *testing.T) {
		question := Question{
			ID:       "pe_1",
			Phase:    PhaseProjectEssence,
			Text:     "What problem does your project solve?",
			Category: "problem_statement",
			Required: true,
		}

		answer := Answer{
			QuestionID: "pe_1",
			Text:       "Task management",
			Timestamp:  time.Now(),
		}

		analysis, err := engine.AnalyzeAnswer(question, answer)
		if err != nil {
			t.Fatalf("Should not error without provider: %v", err)
		}

		if analysis == nil {
			t.Fatal("Expected analysis result even without provider")
		}

		if analysis.Completeness != "unknown" {
			t.Error("Expected completeness to be 'unknown' without provider")
		}
	})
}

func TestInterviewEngine_StateManagement(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := state.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	project := &state.Project{
		ID:           "test-project",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInterview,
	}
	store.CreateProject(project)

	engine := NewEngine(store, nil, "")

	t.Run("PauseAndResume", func(t *testing.T) {
		// Start interview
		session, err := engine.StartInterview(project.ID)
		if err != nil {
			t.Fatalf("Failed to start interview: %v", err)
		}

		// Answer some questions
		engine.RecordAnswer(session, "pe_1", "Test problem statement")
		engine.RecordAnswer(session, "pe_2", "Test users")

		// Pause the interview
		err = engine.PauseInterview(session)
		if err != nil {
			t.Fatalf("Failed to pause interview: %v", err)
		}

		if !session.Paused {
			t.Error("Session should be marked as paused")
		}

		// Resume the interview
		resumedSession, err := engine.ResumeInterview(project.ID)
		if err != nil {
			t.Fatalf("Failed to resume interview: %v", err)
		}

		if resumedSession.Paused {
			t.Error("Resumed session should not be paused")
		}

		// Check that answers were preserved
		if len(resumedSession.Answers) != 2 {
			t.Errorf("Expected 2 answers, got %d", len(resumedSession.Answers))
		}

		answer, exists := resumedSession.Answers["pe_1"]
		if !exists {
			t.Error("Answer pe_1 should exist after resume")
		}

		if answer.Text != "Test problem statement" {
			t.Errorf("Expected 'Test problem statement', got '%s'", answer.Text)
		}
	})

	t.Run("ReiterateAnswer", func(t *testing.T) {
		session, _ := engine.StartInterview(project.ID)

		// Record initial answer
		engine.RecordAnswer(session, "pe_1", "Initial answer")

		// Reiterate the answer
		err := engine.ReiterateAnswer(session, "pe_1", "Updated answer", "Need to be more specific")
		if err != nil {
			t.Fatalf("Failed to reiterate answer: %v", err)
		}

		// Check that answer was updated
		answer, _ := engine.GetAnswer(session, "pe_1")
		if answer.Text != "Updated answer" {
			t.Errorf("Expected 'Updated answer', got '%s'", answer.Text)
		}

		// Check iteration history
		if len(session.Iterations) != 1 {
			t.Errorf("Expected 1 iteration, got %d", len(session.Iterations))
		}

		iteration := session.Iterations[0]
		if iteration.OldAnswer != "Initial answer" {
			t.Errorf("Expected old answer 'Initial answer', got '%s'", iteration.OldAnswer)
		}

		if iteration.NewAnswer != "Updated answer" {
			t.Errorf("Expected new answer 'Updated answer', got '%s'", iteration.NewAnswer)
		}

		if iteration.Reason != "Need to be more specific" {
			t.Errorf("Expected reason 'Need to be more specific', got '%s'", iteration.Reason)
		}
	})

	t.Run("GetIterationHistory", func(t *testing.T) {
		session, _ := engine.StartInterview(project.ID)

		// Record and reiterate multiple times
		engine.RecordAnswer(session, "pe_1", "Answer v1")
		engine.ReiterateAnswer(session, "pe_1", "Answer v2", "First update")
		engine.ReiterateAnswer(session, "pe_1", "Answer v3", "Second update")

		history := engine.GetIterationHistory(session, "pe_1")
		if len(history) != 2 {
			t.Errorf("Expected 2 iterations, got %d", len(history))
		}

		if history[0].OldAnswer != "Answer v1" {
			t.Errorf("Expected first old answer 'Answer v1', got '%s'", history[0].OldAnswer)
		}

		if history[1].OldAnswer != "Answer v2" {
			t.Errorf("Expected second old answer 'Answer v2', got '%s'", history[1].OldAnswer)
		}
	})

	t.Run("RecordFollowUpAnswer", func(t *testing.T) {
		session, _ := engine.StartInterview(project.ID)

		// Record main answer
		engine.RecordAnswer(session, "pe_1", "Main answer")

		// Record follow-up answers
		err := engine.RecordFollowUpAnswer(session, "pe_1", "Can you elaborate?", "More details here")
		if err != nil {
			t.Fatalf("Failed to record follow-up answer: %v", err)
		}

		err = engine.RecordFollowUpAnswer(session, "pe_1", "What about X?", "X is important")
		if err != nil {
			t.Fatalf("Failed to record second follow-up answer: %v", err)
		}

		// Check follow-up answers
		followUps, exists := session.FollowUpAnswers["pe_1"]
		if !exists {
			t.Fatal("Follow-up answers should exist")
		}

		if len(followUps) != 2 {
			t.Errorf("Expected 2 follow-up answers, got %d", len(followUps))
		}
	})

	t.Run("SaveAndLoadCompleteSession", func(t *testing.T) {
		// Create a session with various data
		session, _ := engine.StartInterview(project.ID)

		engine.RecordAnswer(session, "pe_1", "Problem statement")
		engine.RecordAnswer(session, "tc_1", "Go")

		// Manually set phase and question position
		session.CurrentPhase = PhaseTechnicalConstraints
		expectedQuestion := session.CurrentQuestion // Save the current question count

		engine.ReiterateAnswer(session, "pe_1", "Better problem statement", "More clarity needed")
		engine.RecordFollowUpAnswer(session, "pe_1", "Why?", "Because reasons")

		// Save the session
		err := engine.SaveSession(session)
		if err != nil {
			t.Fatalf("Failed to save session: %v", err)
		}

		// Load the session
		loadedSession, err := engine.LoadSession(project.ID)
		if err != nil {
			t.Fatalf("Failed to load session: %v", err)
		}

		// Verify all data was preserved
		if loadedSession.CurrentPhase != PhaseTechnicalConstraints {
			t.Errorf("Expected phase %s, got %s", PhaseTechnicalConstraints, loadedSession.CurrentPhase)
		}

		if loadedSession.CurrentQuestion != expectedQuestion {
			t.Errorf("Expected current question %d, got %d", expectedQuestion, loadedSession.CurrentQuestion)
		}

		if len(loadedSession.Answers) != 2 {
			t.Errorf("Expected 2 answers, got %d", len(loadedSession.Answers))
		}

		if len(loadedSession.Iterations) != 1 {
			t.Errorf("Expected 1 iteration, got %d", len(loadedSession.Iterations))
		}

		// Check specific answer
		answer, exists := loadedSession.Answers["pe_1"]
		if !exists {
			t.Fatal("Answer pe_1 should exist")
		}

		if answer.Text != "Better problem statement" {
			t.Errorf("Expected 'Better problem statement', got '%s'", answer.Text)
		}
	})

	t.Run("GetAnswer", func(t *testing.T) {
		session, _ := engine.StartInterview(project.ID)
		engine.RecordAnswer(session, "pe_1", "Test answer")

		answer, err := engine.GetAnswer(session, "pe_1")
		if err != nil {
			t.Fatalf("Failed to get answer: %v", err)
		}

		if answer.Text != "Test answer" {
			t.Errorf("Expected 'Test answer', got '%s'", answer.Text)
		}

		// Try to get non-existent answer
		_, err = engine.GetAnswer(session, "nonexistent")
		if err == nil {
			t.Error("Expected error for non-existent answer")
		}
	})

	t.Run("ReiterateNonExistentAnswer", func(t *testing.T) {
		session, _ := engine.StartInterview(project.ID)

		err := engine.ReiterateAnswer(session, "nonexistent", "New answer", "Reason")
		if err == nil {
			t.Error("Expected error when reiterating non-existent answer")
		}
	})
}

func TestInterviewEngine_StatePreservation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := state.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	project := &state.Project{
		ID:           "test-project",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInterview,
	}
	store.CreateProject(project)

	engine := NewEngine(store, nil, "")

	t.Run("PreserveStateAcrossMultipleSaveLoad", func(t *testing.T) {
		// Create initial session
		session, _ := engine.StartInterview(project.ID)
		engine.RecordAnswer(session, "pe_1", "Answer 1")
		engine.SaveSession(session)

		// Load and add more data
		session, _ = engine.LoadSession(project.ID)
		engine.RecordAnswer(session, "pe_2", "Answer 2")
		engine.SaveSession(session)

		// Load and add even more data
		session, _ = engine.LoadSession(project.ID)
		engine.RecordAnswer(session, "pe_3", "Answer 3")
		engine.SaveSession(session)

		// Final load and verify
		finalSession, _ := engine.LoadSession(project.ID)

		if len(finalSession.Answers) != 3 {
			t.Errorf("Expected 3 answers, got %d", len(finalSession.Answers))
		}

		// Verify all answers are present
		for i := 1; i <= 3; i++ {
			qid := fmt.Sprintf("pe_%d", i)
			if _, exists := finalSession.Answers[qid]; !exists {
				t.Errorf("Answer %s should exist", qid)
			}
		}
	})
}

func TestInterviewEngine_SummaryAndExport(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := state.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	project := &state.Project{
		ID:           "test-project",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInterview,
	}
	store.CreateProject(project)

	engine := NewEngine(store, nil, "")

	t.Run("GenerateEnhancedSummary", func(t *testing.T) {
		session, _ := engine.StartInterview(project.ID)

		// Add various types of data
		engine.RecordAnswer(session, "pe_1", "Build a task management system")
		engine.RecordAnswer(session, "pe_2", "Software developers")
		engine.ReiterateAnswer(session, "pe_1", "Build an AI-powered task management system", "Added AI aspect")
		engine.RecordFollowUpAnswer(session, "pe_1", "What AI features?", "Smart prioritization and scheduling")

		summary, err := engine.GenerateSummary(session)
		if err != nil {
			t.Fatalf("Failed to generate summary: %v", err)
		}

		if summary == "" {
			t.Fatal("Summary should not be empty")
		}

		// Check that summary includes key elements
		if !contains(summary, "Project ID") {
			t.Error("Summary should include project ID")
		}

		if !contains(summary, "AI-powered task management system") {
			t.Error("Summary should include the updated answer")
		}

		if !contains(summary, "Revision history") {
			t.Error("Summary should include revision history")
		}

		if !contains(summary, "Follow-up responses") {
			t.Error("Summary should include follow-up responses")
		}

		if !contains(summary, "Statistics") {
			t.Error("Summary should include statistics section")
		}
	})

	t.Run("ValidateCompleteness_Incomplete", func(t *testing.T) {
		session, _ := engine.StartInterview(project.ID)

		// Answer only some required questions
		engine.RecordAnswer(session, "pe_1", "Problem statement")

		isComplete, missing := engine.ValidateCompleteness(session)

		if isComplete {
			t.Error("Session should not be complete with missing required questions")
		}

		if len(missing) == 0 {
			t.Error("Should have missing questions")
		}
	})

	t.Run("ValidateCompleteness_Complete", func(t *testing.T) {
		session, _ := engine.StartInterview(project.ID)

		// Answer all required questions
		phases := engine.GetAllPhases()
		for _, phase := range phases {
			questions := engine.GetPhaseQuestions(phase)
			for _, q := range questions {
				if q.Required {
					engine.RecordAnswer(session, q.ID, "Test answer for "+q.ID)
				}
			}
		}

		isComplete, missing := engine.ValidateCompleteness(session)

		if !isComplete {
			t.Errorf("Session should be complete, missing: %v", missing)
		}

		if len(missing) != 0 {
			t.Errorf("Should have no missing questions, got: %v", missing)
		}
	})

	t.Run("ExportToJSON_Complete", func(t *testing.T) {
		session, _ := engine.StartInterview(project.ID)

		// Add comprehensive data
		engine.RecordAnswer(session, "pe_1", "Build a task manager")
		engine.RecordAnswer(session, "pe_2", "Developers")
		engine.RecordAnswer(session, "pe_3", "User engagement")
		engine.RecordAnswer(session, "pe_4", "Simplicity")
		engine.RecordAnswer(session, "tc_1", "Go")
		engine.RecordAnswer(session, "tc_2", "Fast")
		engine.RecordAnswer(session, "tc_3", "1000 users")
		engine.RecordAnswer(session, "ip_2", "PostgreSQL")
		engine.RecordAnswer(session, "ip_3", "JWT")
		engine.RecordAnswer(session, "sd_1", "Basic CRUD")
		engine.RecordAnswer(session, "sd_2", "3 months")
		engine.RecordAnswer(session, "rv_1", "Yes")

		engine.ReiterateAnswer(session, "pe_1", "Build an advanced task manager", "More specific")
		engine.RecordFollowUpAnswer(session, "pe_1", "Why advanced?", "Need more features")

		jsonStr, err := engine.ExportToJSON(session)
		if err != nil {
			t.Fatalf("Failed to export to JSON: %v", err)
		}

		if jsonStr == "" {
			t.Fatal("JSON export should not be empty")
		}

		// Parse JSON to verify structure
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// Check required fields
		if _, ok := data["project_id"]; !ok {
			t.Error("JSON should include project_id")
		}

		if _, ok := data["is_complete"]; !ok {
			t.Error("JSON should include is_complete")
		}

		if _, ok := data["problem_statement"]; !ok {
			t.Error("JSON should include problem_statement")
		}

		if _, ok := data["technical_stack"]; !ok {
			t.Error("JSON should include technical_stack")
		}

		if _, ok := data["phases"]; !ok {
			t.Error("JSON should include phases")
		}

		if _, ok := data["metadata"]; !ok {
			t.Error("JSON should include metadata")
		}

		// Check that revisions are included
		phases := data["phases"].(map[string]interface{})
		projectEssence := phases["project_essence"].(map[string]interface{})
		problemData := projectEssence["problem_statement"].(map[string]interface{})

		if _, ok := problemData["revisions"]; !ok {
			t.Error("JSON should include revisions for questions that were reiterated")
		}

		if _, ok := problemData["follow_ups"]; !ok {
			t.Error("JSON should include follow_ups for questions that have them")
		}
	})

	t.Run("ExportToJSON_Incomplete", func(t *testing.T) {
		session, _ := engine.StartInterview(project.ID)

		// Answer only one question
		engine.RecordAnswer(session, "pe_1", "Partial answer")

		jsonStr, err := engine.ExportToJSON(session)
		if err != nil {
			t.Fatalf("Failed to export to JSON: %v", err)
		}

		var data map[string]interface{}
		json.Unmarshal([]byte(jsonStr), &data)

		isComplete := data["is_complete"].(bool)
		if isComplete {
			t.Error("JSON should indicate incomplete interview")
		}

		if _, ok := data["missing_questions"]; !ok {
			t.Error("JSON should include missing_questions for incomplete interview")
		}
	})

	t.Run("ExtractStructuredData", func(t *testing.T) {
		session, _ := engine.StartInterview(project.ID)

		engine.RecordAnswer(session, "pe_1", "Problem")
		engine.RecordAnswer(session, "pe_2", "Users")
		engine.RecordAnswer(session, "tc_1", "Go")
		engine.RecordAnswer(session, "ip_2", "PostgreSQL")
		engine.RecordAnswer(session, "sd_1", "Features")

		extracted := engine.extractStructuredData(session)

		if _, ok := extracted["problem_statement"]; !ok {
			t.Error("Should extract problem_statement")
		}

		if _, ok := extracted["target_users"]; !ok {
			t.Error("Should extract target_users")
		}

		if _, ok := extracted["technical_stack"]; !ok {
			t.Error("Should extract technical_stack")
		}

		if _, ok := extracted["integrations"]; !ok {
			t.Error("Should extract integrations")
		}

		if _, ok := extracted["scope"]; !ok {
			t.Error("Should extract scope")
		}
	})
}

func TestSaveSession_ProjectName(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := state.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create test project with a specific name
	projectName := "My Awesome Project"
	project := &state.Project{
		ID:           "test-project-id",
		Name:         projectName,
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInterview,
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	engine := NewEngine(store, nil, "")

	// Start interview
	session, err := engine.StartInterview(project.ID)
	if err != nil {
		t.Fatalf("Failed to start interview: %v", err)
	}

	// Save session
	if err := engine.SaveSession(session); err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Retrieve InterviewData and check ProjectName
	interviewData, err := store.GetInterviewData(project.ID)
	if err != nil {
		t.Fatalf("Failed to get interview data: %v", err)
	}

	if interviewData.ProjectName != projectName {
		t.Errorf("Expected ProjectName to be %q, got %q", projectName, interviewData.ProjectName)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestInterviewEngine_ProjectNameExport(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := state.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create test project with specific name different from ID
	projectID := "test-project-id"
	projectName := "Test Project Name"
	project := &state.Project{
		ID:           projectID,
		Name:         projectName,
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInterview,
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	engine := NewEngine(store, nil, "")
	session, err := engine.StartInterview(project.ID)
	if err != nil {
		t.Fatalf("Failed to start interview: %v", err)
	}

	// Export to JSON
	jsonStr, err := engine.ExportToJSON(session)
	if err != nil {
		t.Fatalf("Failed to export to JSON: %v", err)
	}

	// Parse JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify project_name
	exportedProjectName, ok := data["project_name"].(string)
	if !ok {
		t.Fatal("project_name missing or not a string")
	}

	if exportedProjectName != projectName {
		t.Errorf("Expected project_name to be '%s', got '%s'", projectName, exportedProjectName)
	}
}

func TestValidateCompleteness_AllPhasesChecked(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := state.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	project := &state.Project{
		ID:           "test-project",
		Name:         "Test Project",
		CreatedAt:    time.Now(),
		CurrentStage: state.StageInterview,
	}
	store.CreateProject(project)

	engine := NewEngine(store, nil, "")
	session, _ := engine.StartInterview(project.ID)

	// We'll intentionally NOT answer required questions from multiple phases
	// PE_1 is required (Project Essence)
	// TC_1 is required (Technical Constraints)

	isComplete, missing := engine.ValidateCompleteness(session)

	if isComplete {
		t.Error("Session should not be complete")
	}

	// We expect at least one question from each phase to be missing
	foundPE := false
	foundTC := false

	for _, msg := range missing {
		if contains(msg, "Project Essence") && contains(msg, "What problem does your project solve") {
			foundPE = true
		}
		if contains(msg, "Technical Constraints") && contains(msg, "What programming language") {
			foundTC = true
		}
	}

	if !foundPE {
		t.Error("Expected missing question from Project Essence")
	}
	if !foundTC {
		t.Error("Expected missing question from Technical Constraints")
	}
}
