package interview

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
)

// Phase represents an interview phase
type Phase string

const (
	PhaseProjectEssence       Phase = "project_essence"
	PhaseTechnicalConstraints Phase = "technical_constraints"
	PhaseIntegrationPoints    Phase = "integration_points"
	PhaseScopeDefinition      Phase = "scope_definition"
	PhaseRefinementValidation Phase = "refinement_validation"
)

// Engine conducts the interactive interview
type Engine struct {
	store    *state.Store
	provider provider.Provider
	model    string
}

// NewEngine creates a new interview engine
func NewEngine(store *state.Store, provider provider.Provider, model string) *Engine {
	return &Engine{
		store:    store,
		provider: provider,
		model:    model,
	}
}

// Question represents an interview question
type Question struct {
	ID       string
	Phase    Phase
	Text     string
	Category string
	Required bool
}

// Answer represents a user's answer
type Answer struct {
	QuestionID string
	Text       string
	Timestamp  time.Time
}

// InterviewSession represents an active interview session
type InterviewSession struct {
	ProjectID       string
	CurrentPhase    Phase
	CurrentQuestion int
	Answers         map[string]Answer
	FollowUpAnswers map[string][]Answer // Stores follow-up Q&A pairs
	StartedAt       time.Time
	LastUpdatedAt   time.Time
	Completed       bool
	Paused          bool
	Iterations      []Iteration // Track reiteration history
}

// Iteration represents a reiteration of answers
type Iteration struct {
	Timestamp  time.Time
	QuestionID string
	OldAnswer  string
	NewAnswer  string
	Reason     string
}

// GetPhaseQuestions returns the questions for a specific phase
func (e *Engine) GetPhaseQuestions(phase Phase) []Question {
	switch phase {
	case PhaseProjectEssence:
		return []Question{
			{ID: "pe_1", Phase: phase, Text: "What problem does your project solve?", Category: "problem_statement", Required: true},
			{ID: "pe_2", Phase: phase, Text: "Who are the target users?", Category: "target_users", Required: true},
			{ID: "pe_3", Phase: phase, Text: "What are the key success metrics?", Category: "success_metrics", Required: true},
			{ID: "pe_4", Phase: phase, Text: "What is the core value proposition?", Category: "value_proposition", Required: true},
		}
	case PhaseTechnicalConstraints:
		return []Question{
			{ID: "tc_1", Phase: phase, Text: "What programming language(s) do you prefer?", Category: "language", Required: true},
			{ID: "tc_2", Phase: phase, Text: "What are the performance requirements?", Category: "performance", Required: true},
			{ID: "tc_3", Phase: phase, Text: "What scale do you expect (users, requests, data)?", Category: "scale", Required: true},
			{ID: "tc_4", Phase: phase, Text: "Are there any compliance requirements (GDPR, HIPAA, etc.)?", Category: "compliance", Required: false},
		}
	case PhaseIntegrationPoints:
		return []Question{
			{ID: "ip_1", Phase: phase, Text: "What external APIs will you integrate with?", Category: "external_apis", Required: false},
			{ID: "ip_2", Phase: phase, Text: "What type of database do you need?", Category: "database", Required: true},
			{ID: "ip_3", Phase: phase, Text: "What authentication method will you use?", Category: "authentication", Required: true},
			{ID: "ip_4", Phase: phase, Text: "Is there an existing codebase to integrate with?", Category: "existing_code", Required: false},
		}
	case PhaseScopeDefinition:
		return []Question{
			{ID: "sd_1", Phase: phase, Text: "What are the MVP features?", Category: "mvp_features", Required: true},
			{ID: "sd_2", Phase: phase, Text: "What is your timeline?", Category: "timeline", Required: true},
			{ID: "sd_3", Phase: phase, Text: "What are your resource constraints?", Category: "resources", Required: false},
			{ID: "sd_4", Phase: phase, Text: "How do you prioritize features?", Category: "prioritization", Required: false},
		}
	case PhaseRefinementValidation:
		return []Question{
			{ID: "rv_1", Phase: phase, Text: "Review the summary. Is everything correct?", Category: "validation", Required: true},
		}
	default:
		return []Question{}
	}
}

// GetAllPhases returns all interview phases in order
func (e *Engine) GetAllPhases() []Phase {
	return []Phase{
		PhaseProjectEssence,
		PhaseTechnicalConstraints,
		PhaseIntegrationPoints,
		PhaseScopeDefinition,
		PhaseRefinementValidation,
	}
}

// StartInterview starts a new interview session
func (e *Engine) StartInterview(projectID string) (*InterviewSession, error) {
	session := &InterviewSession{
		ProjectID:       projectID,
		CurrentPhase:    PhaseProjectEssence,
		CurrentQuestion: 0,
		Answers:         make(map[string]Answer),
		FollowUpAnswers: make(map[string][]Answer),
		StartedAt:       time.Now(),
		LastUpdatedAt:   time.Now(),
		Completed:       false,
		Paused:          false,
		Iterations:      []Iteration{},
	}

	return session, nil
}

// ResumeInterview resumes a paused interview session
func (e *Engine) ResumeInterview(projectID string) (*InterviewSession, error) {
	session, err := e.LoadSession(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	session.Paused = false
	session.LastUpdatedAt = time.Now()

	return session, nil
}

// PauseInterview pauses the current interview session
func (e *Engine) PauseInterview(session *InterviewSession) error {
	session.Paused = true
	session.LastUpdatedAt = time.Now()

	return e.SaveSession(session)
}

// GetNextQuestion returns the next question in the interview
func (e *Engine) GetNextQuestion(session *InterviewSession) (*Question, error) {
	questions := e.GetPhaseQuestions(session.CurrentPhase)

	if session.CurrentQuestion >= len(questions) {
		// Move to next phase
		phases := e.GetAllPhases()
		currentPhaseIndex := -1
		for i, p := range phases {
			if p == session.CurrentPhase {
				currentPhaseIndex = i
				break
			}
		}

		if currentPhaseIndex == -1 {
			return nil, fmt.Errorf("invalid current phase")
		}

		if currentPhaseIndex >= len(phases)-1 {
			// Interview complete
			session.Completed = true
			return nil, nil
		}

		// Move to next phase
		session.CurrentPhase = phases[currentPhaseIndex+1]
		session.CurrentQuestion = 0
		questions = e.GetPhaseQuestions(session.CurrentPhase)
	}

	if session.CurrentQuestion >= len(questions) {
		return nil, fmt.Errorf("no more questions in phase")
	}

	question := questions[session.CurrentQuestion]
	return &question, nil
}

// RecordAnswer records a user's answer
func (e *Engine) RecordAnswer(session *InterviewSession, questionID string, answerText string) error {
	answer := Answer{
		QuestionID: questionID,
		Text:       answerText,
		Timestamp:  time.Now(),
	}

	session.Answers[questionID] = answer
	session.CurrentQuestion++
	session.LastUpdatedAt = time.Now()

	return nil
}

// RecordFollowUpAnswer records an answer to a follow-up question
func (e *Engine) RecordFollowUpAnswer(session *InterviewSession, questionID string, followUpQuestion string, answerText string) error {
	answer := Answer{
		QuestionID: questionID + "_followup",
		Text:       answerText,
		Timestamp:  time.Now(),
	}

	if session.FollowUpAnswers == nil {
		session.FollowUpAnswers = make(map[string][]Answer)
	}

	session.FollowUpAnswers[questionID] = append(session.FollowUpAnswers[questionID], answer)
	session.LastUpdatedAt = time.Now()

	return nil
}

// ReiterateAnswer allows changing a previous answer
func (e *Engine) ReiterateAnswer(session *InterviewSession, questionID string, newAnswer string, reason string) error {
	oldAnswer, exists := session.Answers[questionID]
	if !exists {
		return fmt.Errorf("no previous answer found for question %s", questionID)
	}

	// Record the iteration
	iteration := Iteration{
		Timestamp:  time.Now(),
		QuestionID: questionID,
		OldAnswer:  oldAnswer.Text,
		NewAnswer:  newAnswer,
		Reason:     reason,
	}

	session.Iterations = append(session.Iterations, iteration)

	// Update the answer
	session.Answers[questionID] = Answer{
		QuestionID: questionID,
		Text:       newAnswer,
		Timestamp:  time.Now(),
	}

	session.LastUpdatedAt = time.Now()

	return nil
}

// GetAnswer retrieves an answer for a specific question
func (e *Engine) GetAnswer(session *InterviewSession, questionID string) (*Answer, error) {
	answer, exists := session.Answers[questionID]
	if !exists {
		return nil, fmt.Errorf("no answer found for question %s", questionID)
	}

	return &answer, nil
}

// GetIterationHistory returns the iteration history for a question
func (e *Engine) GetIterationHistory(session *InterviewSession, questionID string) []Iteration {
	var history []Iteration
	for _, iter := range session.Iterations {
		if iter.QuestionID == questionID {
			history = append(history, iter)
		}
	}
	return history
}

// GenerateFollowUp generates a follow-up question based on the answer
func (e *Engine) GenerateFollowUp(question Question, answer Answer) (string, error) {
	if e.provider == nil {
		return "", nil // No follow-up if no provider
	}

	prompt := fmt.Sprintf(`You are conducting a technical interview to gather project requirements. Based on the question and answer below, generate ONE brief, specific follow-up question to clarify or expand on the answer. 

The follow-up should:
- Be concise (one sentence)
- Dig deeper into technical details or clarify ambiguities
- Help understand the user's needs better
- Be relevant to the original question's category

If the answer is already comprehensive and clear, respond with "SKIP" to indicate no follow-up is needed.

OUTPUT RULES:
- Return exactly one sentence if asking a follow-up.
- Return exactly "SKIP" if no follow-up is needed.
- No prefixes, labels, bullets, or explanations.

Question: %s
Answer: %s

Follow-up question:`, question.Text, answer.Text)

	response, err := e.provider.Call(context.TODO(), e.model, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate follow-up: %w", err)
	}

	// Check if LLM indicated no follow-up needed
	content := response.Content
	if content == "SKIP" || content == "" {
		return "", nil
	}

	return content, nil
}

// AnalyzeAnswer analyzes a user's answer to extract key information
func (e *Engine) AnalyzeAnswer(question Question, answer Answer) (*AnswerAnalysis, error) {
	if e.provider == nil {
		return &AnswerAnalysis{
			KeyPoints:    []string{answer.Text},
			Completeness: "unknown",
			Suggestions:  []string{},
		}, nil
	}

	prompt := fmt.Sprintf(`Analyze this interview answer and provide a structured analysis.

Question: %s
Answer: %s

Provide your analysis in the following format (exact labels):
KEY_POINTS: List 2-3 key points from the answer (comma-separated)
COMPLETENESS: Rate as "complete", "partial", or "incomplete"
SUGGESTIONS: If incomplete, suggest what additional information would be helpful (comma-separated, or "none")

OUTPUT RULES:
- Return exactly these three labeled lines.
- No extra sections.
- Keep each line concise.

Analysis:`, question.Text, answer.Text)

	response, err := e.provider.Call(context.TODO(), e.model, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze answer: %w", err)
	}

	// Parse the response (simplified parsing)
	analysis := &AnswerAnalysis{
		KeyPoints:    []string{answer.Text},
		Completeness: "unknown",
		Suggestions:  []string{},
	}

	// In a production system, you'd parse the structured response more carefully
	// For now, we'll just store the raw analysis
	analysis.RawAnalysis = response.Content

	return analysis, nil
}

// AnswerAnalysis contains analysis of a user's answer
type AnswerAnalysis struct {
	KeyPoints    []string
	Completeness string
	Suggestions  []string
	RawAnalysis  string
}

// GenerateSummary generates a summary of all answers
func (e *Engine) GenerateSummary(session *InterviewSession) (string, error) {
	var sb strings.Builder
	sb.WriteString("# Interview Summary\n\n")
	fmt.Fprintf(&sb, "**Project ID:** %s\n", session.ProjectID)
	fmt.Fprintf(&sb, "**Started:** %s\n", session.StartedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "**Last Updated:** %s\n", session.LastUpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "**Status:** %s\n\n", func() string {
		if session.Completed {
			return "Completed"
		} else if session.Paused {
			return "Paused"
		}
		return "In Progress"
	}())

	phases := e.GetAllPhases()
	for _, phase := range phases {
		questions := e.GetPhaseQuestions(phase)
		fmt.Fprintf(&sb, "## %s\n\n", formatPhaseName(phase))

		hasAnswers := false
		for _, q := range questions {
			if answer, ok := session.Answers[q.ID]; ok {
				hasAnswers = true
				fmt.Fprintf(&sb, "**Q: %s**\n", q.Text)
				fmt.Fprintf(&sb, "A: %s\n\n", answer.Text)

				// Include follow-up answers if any
				if followUps, ok := session.FollowUpAnswers[q.ID]; ok && len(followUps) > 0 {
					sb.WriteString("  *Follow-up responses:*\n")
					for i, fu := range followUps {
						fmt.Fprintf(&sb, "  %d. %s\n", i+1, fu.Text)
					}
					sb.WriteString("\n")
				}

				// Include iteration history if any
				iterations := e.GetIterationHistory(session, q.ID)
				if len(iterations) > 0 {
					sb.WriteString("  *Revision history:*\n")
					for i, iter := range iterations {
						fmt.Fprintf(&sb, "  %d. Changed from \"%s\" to \"%s\" (%s)\n",
							i+1, iter.OldAnswer, iter.NewAnswer, iter.Reason)
					}
					sb.WriteString("\n")
				}
			}
		}

		if !hasAnswers {
			sb.WriteString("*No answers recorded for this phase yet.*\n\n")
		}
	}

	// Add statistics
	sb.WriteString("## Statistics\n\n")
	fmt.Fprintf(&sb, "- Total questions answered: %d\n", len(session.Answers))
	fmt.Fprintf(&sb, "- Total revisions made: %d\n", len(session.Iterations))

	followUpCount := 0
	for _, followUps := range session.FollowUpAnswers {
		followUpCount += len(followUps)
	}
	fmt.Fprintf(&sb, "- Total follow-up responses: %d\n", followUpCount)

	return sb.String(), nil
}

// formatPhaseName converts a phase constant to a readable name
func formatPhaseName(phase Phase) string {
	switch phase {
	case PhaseProjectEssence:
		return "Project Essence"
	case PhaseTechnicalConstraints:
		return "Technical Constraints"
	case PhaseIntegrationPoints:
		return "Integration Points"
	case PhaseScopeDefinition:
		return "Scope Definition"
	case PhaseRefinementValidation:
		return "Refinement & Validation"
	default:
		return string(phase)
	}
}

// ValidateCompleteness checks if all required questions have been answered
func (e *Engine) ValidateCompleteness(session *InterviewSession) (bool, []string) {
	var missingQuestions []string

	phases := e.GetAllPhases()
	for _, phase := range phases {
		questions := e.GetPhaseQuestions(phase)
		for _, q := range questions {
			if q.Required {
				if _, ok := session.Answers[q.ID]; !ok {
					missingQuestions = append(missingQuestions,
						fmt.Sprintf("%s: %s", formatPhaseName(phase), q.Text))
				}
			}
		}
	}

	return len(missingQuestions) == 0, missingQuestions
}

// ExportToJSON exports the interview data to JSON format
func (e *Engine) ExportToJSON(session *InterviewSession) (string, error) {
	// Validate completeness first
	isComplete, missingQuestions := e.ValidateCompleteness(session)

	data := make(map[string]interface{})
	data["project_id"] = session.ProjectID

	// Get actual project name
	projectName := session.ProjectID
	if e.store != nil {
		if project, err := e.store.GetProject(session.ProjectID); err == nil {
			projectName = project.Name
		}
	}
	data["project_name"] = projectName

	data["started_at"] = session.StartedAt
	data["completed_at"] = time.Now()
	data["is_complete"] = isComplete

	if !isComplete {
		data["missing_questions"] = missingQuestions
	}

	// Extract structured data from answers
	extractedData := e.extractStructuredData(session)
	for key, value := range extractedData {
		data[key] = value
	}

	// Organize answers by phase
	phases := e.GetAllPhases()
	phaseAnswers := make(map[string]interface{})

	for _, phase := range phases {
		questions := e.GetPhaseQuestions(phase)
		phaseData := make(map[string]interface{})

		for _, q := range questions {
			if answer, ok := session.Answers[q.ID]; ok {
				answerData := map[string]interface{}{
					"question":  q.Text,
					"answer":    answer.Text,
					"timestamp": answer.Timestamp,
				}

				// Include follow-ups if any
				if followUps, ok := session.FollowUpAnswers[q.ID]; ok && len(followUps) > 0 {
					followUpTexts := make([]string, len(followUps))
					for i, fu := range followUps {
						followUpTexts[i] = fu.Text
					}
					answerData["follow_ups"] = followUpTexts
				}

				// Include iterations if any
				iterations := e.GetIterationHistory(session, q.ID)
				if len(iterations) > 0 {
					iterData := make([]map[string]interface{}, len(iterations))
					for i, iter := range iterations {
						iterData[i] = map[string]interface{}{
							"old_answer": iter.OldAnswer,
							"new_answer": iter.NewAnswer,
							"reason":     iter.Reason,
							"timestamp":  iter.Timestamp,
						}
					}
					answerData["revisions"] = iterData
				}

				phaseData[q.Category] = answerData
			}
		}

		if len(phaseData) > 0 {
			phaseAnswers[string(phase)] = phaseData
		}
	}

	data["phases"] = phaseAnswers

	// Add metadata
	data["metadata"] = map[string]interface{}{
		"total_questions_answered": len(session.Answers),
		"total_revisions":          len(session.Iterations),
		"current_phase":            string(session.CurrentPhase),
		"paused":                   session.Paused,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(jsonData), nil
}

// extractStructuredData extracts key structured data from answers
func (e *Engine) extractStructuredData(session *InterviewSession) map[string]interface{} {
	data := make(map[string]interface{})

	// Extract problem statement
	if answer, ok := session.Answers["pe_1"]; ok {
		data["problem_statement"] = answer.Text
	}

	// Extract target users
	if answer, ok := session.Answers["pe_2"]; ok {
		data["target_users"] = []string{answer.Text}
	}

	// Extract success metrics
	if answer, ok := session.Answers["pe_3"]; ok {
		data["success_metrics"] = []string{answer.Text}
	}

	// Extract value proposition
	if answer, ok := session.Answers["pe_4"]; ok {
		data["value_proposition"] = answer.Text
	}

	// Extract technical stack
	techStack := make(map[string]interface{})
	if answer, ok := session.Answers["tc_1"]; ok {
		techStack["language"] = answer.Text
	}
	if answer, ok := session.Answers["tc_2"]; ok {
		techStack["performance_requirements"] = answer.Text
	}
	if answer, ok := session.Answers["tc_3"]; ok {
		techStack["scale_expectations"] = answer.Text
	}
	if answer, ok := session.Answers["tc_4"]; ok {
		techStack["compliance"] = answer.Text
	}
	if len(techStack) > 0 {
		data["technical_stack"] = techStack
	}

	// Extract integrations
	integrations := make(map[string]interface{})
	if answer, ok := session.Answers["ip_1"]; ok {
		integrations["external_apis"] = answer.Text
	}
	if answer, ok := session.Answers["ip_2"]; ok {
		integrations["database"] = answer.Text
	}
	if answer, ok := session.Answers["ip_3"]; ok {
		integrations["authentication"] = answer.Text
	}
	if answer, ok := session.Answers["ip_4"]; ok {
		integrations["existing_codebase"] = answer.Text
	}
	if len(integrations) > 0 {
		data["integrations"] = integrations
	}

	// Extract scope
	scope := make(map[string]interface{})
	if answer, ok := session.Answers["sd_1"]; ok {
		scope["mvp_features"] = answer.Text
	}
	if answer, ok := session.Answers["sd_2"]; ok {
		scope["timeline"] = answer.Text
	}
	if answer, ok := session.Answers["sd_3"]; ok {
		scope["resources"] = answer.Text
	}
	if answer, ok := session.Answers["sd_4"]; ok {
		scope["prioritization"] = answer.Text
	}
	if len(scope) > 0 {
		data["scope"] = scope
	}

	return data
}

// SaveSession saves the interview session to the state store
func (e *Engine) SaveSession(session *InterviewSession) error {
	// Serialize the entire session to JSON for storage
	sessionData := map[string]interface{}{
		"project_id":       session.ProjectID,
		"current_phase":    string(session.CurrentPhase),
		"current_question": session.CurrentQuestion,
		"answers":          session.Answers,
		"followup_answers": session.FollowUpAnswers,
		"started_at":       session.StartedAt,
		"last_updated_at":  session.LastUpdatedAt,
		"completed":        session.Completed,
		"paused":           session.Paused,
		"iterations":       session.Iterations,
	}

	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Get actual project name
	projectName := session.ProjectID
	if project, err := e.store.GetProject(session.ProjectID); err == nil {
		projectName = project.Name
	}

	// Convert session to InterviewData format for storage
	interviewData := &state.InterviewData{
		ProjectID:   session.ProjectID,
		ProjectName: projectName,
		CreatedAt:   session.StartedAt,
		RawSession:  string(sessionJSON), // Store the full session as JSON
	}

	// Extract key data from answers for easy access
	for qid, answer := range session.Answers {
		switch qid {
		case "pe_1":
			interviewData.ProblemStatement = answer.Text
		case "pe_2":
			// Parse target users (could be comma-separated)
			interviewData.TargetUsers = []string{answer.Text}
		case "pe_3":
			// Parse success metrics
			interviewData.SuccessMetrics = []string{answer.Text}
			// Add more mappings as needed
		}
	}

	return e.store.SaveInterviewData(session.ProjectID, interviewData)
}

// LoadSession loads an interview session from the state store
func (e *Engine) LoadSession(projectID string) (*InterviewSession, error) {
	data, err := e.store.GetInterviewData(projectID)
	if err != nil {
		return nil, err
	}

	// If we have raw session data, deserialize it
	if data.RawSession != "" {
		var sessionData map[string]interface{}
		if err := json.Unmarshal([]byte(data.RawSession), &sessionData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal session: %w", err)
		}

		session := &InterviewSession{
			ProjectID:       projectID,
			CurrentPhase:    Phase(sessionData["current_phase"].(string)),
			CurrentQuestion: int(sessionData["current_question"].(float64)),
			Answers:         make(map[string]Answer),
			FollowUpAnswers: make(map[string][]Answer),
			StartedAt:       data.CreatedAt,
			LastUpdatedAt:   time.Now(),
			Completed:       sessionData["completed"].(bool),
			Paused:          sessionData["paused"].(bool),
			Iterations:      []Iteration{},
		}

		// Reconstruct answers
		if answersData, ok := sessionData["answers"].(map[string]interface{}); ok {
			for qid, answerData := range answersData {
				if answerMap, ok := answerData.(map[string]interface{}); ok {
					session.Answers[qid] = Answer{
						QuestionID: answerMap["QuestionID"].(string),
						Text:       answerMap["Text"].(string),
						Timestamp:  time.Now(), // Simplified
					}
				}
			}
		}

		// Reconstruct iterations
		if iterationsData, ok := sessionData["iterations"].([]interface{}); ok {
			for _, iterData := range iterationsData {
				if iterMap, ok := iterData.(map[string]interface{}); ok {
					session.Iterations = append(session.Iterations, Iteration{
						Timestamp:  time.Now(), // Simplified
						QuestionID: iterMap["QuestionID"].(string),
						OldAnswer:  iterMap["OldAnswer"].(string),
						NewAnswer:  iterMap["NewAnswer"].(string),
						Reason:     iterMap["Reason"].(string),
					})
				}
			}
		}

		return session, nil
	}

	// Fallback: Create session from basic data
	session := &InterviewSession{
		ProjectID:       projectID,
		CurrentPhase:    PhaseProjectEssence,
		CurrentQuestion: 0,
		Answers:         make(map[string]Answer),
		FollowUpAnswers: make(map[string][]Answer),
		StartedAt:       data.CreatedAt,
		LastUpdatedAt:   time.Now(),
		Completed:       false,
		Paused:          false,
		Iterations:      []Iteration{},
	}

	// Reconstruct basic answers from data
	if data.ProblemStatement != "" {
		session.Answers["pe_1"] = Answer{
			QuestionID: "pe_1",
			Text:       data.ProblemStatement,
			Timestamp:  data.CreatedAt,
		}
	}

	return session, nil
}

// ProposeDefault proposes a reasonable default for a question
func (e *Engine) ProposeDefault(question Question) (string, error) {
	// First check static defaults
	defaults := map[string]string{
		"tc_1": "Go",
		"tc_2": "Sub-second response times for API calls",
		"tc_3": "Thousands of users, millions of requests per day",
		"ip_2": "PostgreSQL",
		"ip_3": "JWT-based authentication",
		"sd_2": "3-6 months",
	}

	if defaultVal, ok := defaults[question.ID]; ok {
		return defaultVal, nil
	}

	// If no static default and we have a provider, use LLM to propose one
	if e.provider != nil {
		return e.ProposeDefaultWithLLM(question)
	}

	return "", nil
}

// ProposeDefaultWithLLM uses the LLM to propose a reasonable default
func (e *Engine) ProposeDefaultWithLLM(question Question) (string, error) {
	prompt := fmt.Sprintf(`You are helping a developer set up a new project. For the following question, propose a reasonable, commonly-used default answer. Keep it brief and practical.

OUTPUT RULES:
- Return one short answer only.
- No leading labels (e.g., "Answer:").
- No explanations unless absolutely necessary.

Question: %s
Category: %s

Proposed default answer:`, question.Text, question.Category)

	response, err := e.provider.Call(context.TODO(), e.model, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to propose default: %w", err)
	}

	return response.Content, nil
}

// AskWithFollowUp asks a question and optionally generates follow-up questions
func (e *Engine) AskWithFollowUp(session *InterviewSession, question Question, enableFollowUp bool) ([]string, error) {
	followUps := []string{}

	if !enableFollowUp || e.provider == nil {
		return followUps, nil
	}

	// Get the answer for this question
	answer, ok := session.Answers[question.ID]
	if !ok {
		return followUps, nil
	}

	// Generate follow-up question
	followUp, err := e.GenerateFollowUp(question, answer)
	if err != nil {
		// Don't fail the interview if follow-up generation fails
		return followUps, nil
	}

	if followUp != "" {
		followUps = append(followUps, followUp)
	}

	return followUps, nil
}
