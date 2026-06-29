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

const systemPrompt = `You are an expert project interviewer helping gather requirements for software development. Your role is to have a natural conversation with the user to understand their project deeply.

Your goals are to gather information in these key areas:
1. PROJECT ESSENCE: Problem being solved, target users, success metrics, value proposition
2. TECHNICAL CONSTRAINTS: Programming language preferences, performance requirements, expected scale, compliance needs
3. INTEGRATION POINTS: External APIs, database needs, authentication methods, existing codebase integration
4. SCOPE DEFINITION: MVP features, timeline, resources, feature prioritization

INTERVIEW GUIDELINES:
- Be conversational and friendly, not interrogative
- Ask one question at a time
- Build on previous answers to dig deeper
- If the user provides partial information, ask clarifying questions
- Summarize what you've learned periodically to confirm understanding
- Adapt your questions based on the project type (web app, CLI tool, API, etc.)
- Don't ask about things the user has already covered
- When you have enough information in an area, naturally transition to the next

SPECIAL COMMANDS the user might use:
- "skip" - Move to the next topic
- "done" - Finish the interview early
- "summary" - Show current understanding
- "back" - Go back to a previous topic

RESPONSE FORMAT:
- Keep responses concise (2-4 sentences typically)
- Ask clear, specific questions
- When you have gathered sufficient information for all areas, provide a brief summary and ask if they're ready to proceed to the design phase.

START by greeting the user and asking about their project idea in a natural way.`

type ChatMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type ChatSession struct {
	ProjectID     string                 `json:"project_id"`
	Messages      []ChatMessage          `json:"messages"`
	StartedAt     time.Time              `json:"started_at"`
	LastUpdateAt  time.Time              `json:"last_updated_at"`
	Completed     bool                   `json:"completed"`
	ExtractedData map[string]interface{} `json:"extracted_data,omitempty"`
}

type ChatEngine struct {
	store    *state.Store
	provider provider.Provider
	model    string
}

func NewChatEngine(store *state.Store, prov provider.Provider, model string) *ChatEngine {
	return &ChatEngine{
		store:    store,
		provider: prov,
		model:    model,
	}
}

func (e *ChatEngine) StartChatSession(projectID string) *ChatSession {
	return &ChatSession{
		ProjectID:     projectID,
		Messages:      []ChatMessage{},
		StartedAt:     time.Now(),
		LastUpdateAt:  time.Now(),
		Completed:     false,
		ExtractedData: make(map[string]interface{}),
	}
}

func (e *ChatEngine) buildConversationPrompt(session *ChatSession, userMessage string) string {
	var sb strings.Builder

	sb.WriteString(systemPrompt)
	sb.WriteString("\n\n=== CONVERSATION HISTORY ===\n")

	for _, msg := range session.Messages {
		switch msg.Role {
		case "user":
			sb.WriteString(fmt.Sprintf("User: %s\n", msg.Content))
		case "assistant":
			sb.WriteString(fmt.Sprintf("Assistant: %s\n", msg.Content))
		}
	}

	if userMessage != "" {
		sb.WriteString(fmt.Sprintf("\nUser: %s\n", userMessage))
	}

	sb.WriteString("\n=== INSTRUCTIONS ===\n")
	sb.WriteString("Based on the conversation, respond naturally. If this is the first message, greet the user and ask about their project. Otherwise, acknowledge their response and ask the next relevant question.\n")

	return sb.String()
}

func (e *ChatEngine) SendMessage(session *ChatSession, userMessage string) (string, error) {
	if e.provider == nil {
		return "", fmt.Errorf("no provider configured")
	}

	lowerMsg := strings.ToLower(strings.TrimSpace(userMessage))

	switch lowerMsg {
	case "done", "finish", "complete":
		session.Completed = true
		summary, _ := e.generateFinalSummary(session)
		return summary, nil
	case "summary":
		summary, _ := e.generateCurrentSummary(session)
		return summary, nil
	}

	session.Messages = append(session.Messages, ChatMessage{
		Role:      "user",
		Content:   userMessage,
		Timestamp: time.Now(),
	})

	prompt := e.buildConversationPrompt(session, "")

	response, err := e.provider.Call(context.TODO(), e.model, prompt)
	if err != nil {
		return "", fmt.Errorf("LLM call failed: %w", err)
	}

	assistantMessage := response.Content

	session.Messages = append(session.Messages, ChatMessage{
		Role:      "assistant",
		Content:   assistantMessage,
		Timestamp: time.Now(),
	})
	session.LastUpdateAt = time.Now()

	if e.detectCompletion(session) {
		session.Completed = true
		summary, _ := e.generateFinalSummary(session)
		assistantMessage += "\n\n" + summary
	}

	return assistantMessage, nil
}

func (e *ChatEngine) detectCompletion(session *ChatSession) bool {
	if len(session.Messages) < 10 {
		return false
	}

	conversation := ""
	for _, msg := range session.Messages {
		conversation += msg.Content + " "
	}

	coverage := e.analyzeCoverage(conversation)
	return coverage >= 0.8
}

func (e *ChatEngine) analyzeCoverage(conversation string) float64 {
	keywords := map[string][]string{
		"problem":     {"problem", "solve", "issue", "challenge", "need", "pain"},
		"users":       {"user", "customer", "client", "audience", "people"},
		"success":     {"success", "metric", "goal", "measure", "kpi"},
		"value":       {"value", "benefit", "advantage", "unique", "proposition"},
		"language":    {"language", "stack", "framework", "technology", "tech"},
		"performance": {"performance", "fast", "speed", "latency", "response"},
		"scale":       {"scale", "user", "traffic", "load", "concurrent"},
		"compliance":  {"compliance", "gdpr", "hipaa", "security", "regulation"},
		"api":         {"api", "integration", "service", "external", "third-party"},
		"database":    {"database", "db", "storage", "data", "persist"},
		"auth":        {"auth", "login", "user", "security", "password", "jwt", "oauth"},
		"mvp":         {"mvp", "feature", "version", "first", "initial"},
		"timeline":    {"timeline", "deadline", "schedule", "month", "week"},
	}

	lowerConv := strings.ToLower(conversation)
	covered := 0
	for _, words := range keywords {
		for _, word := range words {
			if strings.Contains(lowerConv, word) {
				covered++
				break
			}
		}
	}

	return float64(covered) / float64(len(keywords))
}

func (e *ChatEngine) generateCurrentSummary(session *ChatSession) (string, error) {
	conversation := ""
	for _, msg := range session.Messages {
		if msg.Role == "user" {
			conversation += "User: " + msg.Content + "\n"
		} else {
			conversation += "Assistant: " + msg.Content + "\n"
		}
	}

	prompt := fmt.Sprintf(`Based on this conversation, provide a brief summary of what you've learned about the project so far. Format as bullet points.

CONVERSATION:
%s

SUMMARY (bullet points):`, conversation)

	response, err := e.provider.Call(context.TODO(), e.model, prompt)
	if err != nil {
		return "Unable to generate summary.", nil
	}

	return "\n📋 **Current Summary:**\n" + response.Content, nil
}

func (e *ChatEngine) generateFinalSummary(session *ChatSession) (string, error) {
	conversation := ""
	for _, msg := range session.Messages {
		if msg.Role == "user" {
			conversation += "User: " + msg.Content + "\n"
		} else {
			conversation += "Assistant: " + msg.Content + "\n"
		}
	}

	prompt := fmt.Sprintf(`Based on this complete conversation, extract the project requirements in a structured format.

CONVERSATION:
%s

Please provide a structured summary in this exact format:
## Project Summary

**Problem Statement:**
[The core problem being solved]

**Target Users:**
[Who will use this software]

**Success Metrics:**
[How success will be measured]

**Technical Stack:**
[Preferred languages, frameworks, databases]

**Key Features (MVP):**
[Essential features for first version]

**Timeline:**
[Expected timeline]

**Additional Notes:**
[Any other important details]

If any section is unclear from the conversation, note it as "Not specified" and suggest what questions should have been asked.`, conversation)

	response, err := e.provider.Call(context.TODO(), e.model, prompt)
	if err != nil {
		return "Interview completed! Unable to generate final summary.", nil
	}

	return "\n\n✅ **Interview Complete!**\n\n" + response.Content, nil
}

func (e *ChatEngine) ExtractInterviewData(session *ChatSession) (*state.InterviewData, error) {
	conversation := ""
	for _, msg := range session.Messages {
		if msg.Role == "user" {
			conversation += "User: " + msg.Content + "\n"
		} else {
			conversation += "Assistant: " + msg.Content + "\n"
		}
	}

	prompt := fmt.Sprintf(`Extract structured data from this interview conversation. Return ONLY valid JSON with these fields:
{
  "problem_statement": "string - the core problem being solved",
  "target_users": ["array of user types"],
  "success_metrics": ["array of metrics"],
  "value_proposition": "string - the unique value",
  "technical_stack": {
    "language": "string",
    "framework": "string",
    "database": "string"
  },
  "integrations": {
    "external_apis": ["array of APIs"],
    "authentication": "string"
  },
  "scope": {
    "mvp_features": ["array of features"],
    "timeline": "string",
    "resources": "string"
  },
  "compliance": ["array of requirements"]
}

CONVERSATION:
%s

JSON:`, conversation)

	response, err := e.provider.Call(context.TODO(), e.model, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to extract data: %w", err)
	}

	content := response.Content
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart != -1 && jsonEnd != -1 && jsonEnd > jsonStart {
		content = content[jsonStart : jsonEnd+1]
	}

	var extracted map[string]interface{}
	if err := json.Unmarshal([]byte(content), &extracted); err != nil {
		return e.buildInterviewDataFromSession(session), nil
	}

	data := &state.InterviewData{
		ProjectID:   session.ProjectID,
		ProjectName: session.ProjectID,
		CreatedAt:   session.StartedAt,
	}

	if ps, ok := extracted["problem_statement"].(string); ok {
		data.ProblemStatement = ps
	}
	if tu, ok := extracted["target_users"].([]interface{}); ok {
		for _, u := range tu {
			if s, ok := u.(string); ok {
				data.TargetUsers = append(data.TargetUsers, s)
			}
		}
	}
	if sm, ok := extracted["success_metrics"].([]interface{}); ok {
		for _, m := range sm {
			if s, ok := m.(string); ok {
				data.SuccessMetrics = append(data.SuccessMetrics, s)
			}
		}
	}

	return data, nil
}

func (e *ChatEngine) buildInterviewDataFromSession(session *ChatSession) *state.InterviewData {
	data := &state.InterviewData{
		ProjectID:      session.ProjectID,
		ProjectName:    session.ProjectID,
		CreatedAt:      session.StartedAt,
		TargetUsers:    []string{},
		SuccessMetrics: []string{},
	}

	conversation := ""
	for _, msg := range session.Messages {
		conversation += msg.Content + " "
	}

	if idx := strings.Index(conversation, "problem"); idx != -1 {
		end := idx + 200
		if end > len(conversation) {
			end = len(conversation)
		}
		data.ProblemStatement = conversation[idx:end]
	}

	return data
}

func (e *ChatEngine) SaveChatSession(session *ChatSession) error {
	sessionData := map[string]interface{}{
		"project_id":     session.ProjectID,
		"messages":       session.Messages,
		"started_at":     session.StartedAt,
		"last_updated":   session.LastUpdateAt,
		"completed":      session.Completed,
		"extracted_data": session.ExtractedData,
	}

	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	projectName := session.ProjectID
	if e.store != nil {
		if project, err := e.store.GetProject(session.ProjectID); err == nil {
			projectName = project.Name
		}
	}

	interviewData := &state.InterviewData{
		ProjectID:   session.ProjectID,
		ProjectName: projectName,
		CreatedAt:   session.StartedAt,
		RawSession:  string(sessionJSON),
	}

	if e.store != nil {
		return e.store.SaveInterviewData(session.ProjectID, interviewData)
	}

	return nil
}

func (e *ChatEngine) LoadChatSession(projectID string) (*ChatSession, error) {
	if e.store == nil {
		return nil, fmt.Errorf("no store configured")
	}

	data, err := e.store.GetInterviewData(projectID)
	if err != nil {
		return nil, err
	}

	if data.RawSession == "" {
		return nil, fmt.Errorf("no chat session found")
	}

	var sessionData map[string]interface{}
	if err := json.Unmarshal([]byte(data.RawSession), &sessionData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	session := &ChatSession{
		ProjectID:     projectID,
		Messages:      []ChatMessage{},
		StartedAt:     data.CreatedAt,
		LastUpdateAt:  time.Now(),
		Completed:     false,
		ExtractedData: make(map[string]interface{}),
	}

	if msgs, ok := sessionData["messages"].([]interface{}); ok {
		for _, m := range msgs {
			if msgMap, ok := m.(map[string]interface{}); ok {
				msg := ChatMessage{
					Timestamp: time.Now(),
				}
				if role, ok := msgMap["role"].(string); ok {
					msg.Role = role
				}
				if content, ok := msgMap["content"].(string); ok {
					msg.Content = content
				}
				session.Messages = append(session.Messages, msg)
			}
		}
	}

	if completed, ok := sessionData["completed"].(bool); ok {
		session.Completed = completed
	}

	return session, nil
}

func (e *ChatEngine) GetGreeting() string {
	if e.provider == nil {
		return "Hello! I'm ready to learn about your project. What would you like to build?"
	}

	prompt := systemPrompt + "\n\nGenerate a friendly, brief greeting (2 sentences max) to start a project interview. Ask the user about their project idea."

	response, err := e.provider.Call(context.TODO(), e.model, prompt)
	if err != nil {
		return "Hello! I'm ready to learn about your project. What would you like to build?"
	}

	return response.Content
}
