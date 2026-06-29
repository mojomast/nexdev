package design

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/logging"
	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
)

// CacheStore interface for caching AI responses
type CacheStore interface {
	GetCache(key string) (string, error)
	SetCache(key string, value string, ttl time.Duration) error
}

// Generator generates system architecture from interview data
type Generator struct {
	provider provider.Provider
	model    string
	cache    CacheStore
	logger   *logging.Logger
}

// NewGenerator creates a new design generator
func NewGenerator(provider provider.Provider, model string, cache CacheStore) *Generator {
	return &Generator{
		provider: provider,
		model:    model,
		cache:    cache,
		logger:   logging.NewLogger(slog.LevelInfo, os.Stdout),
	}
}

// Architecture represents the system architecture
type Architecture struct {
	ProjectID        string
	SystemOverview   string
	Components       []Component
	DataFlows        []DataFlow
	TechRationale    map[string]string
	ScalingStrategy  ScalingPlan
	APIContract      APISpec
	DatabaseSchema   Schema
	SecurityApproach SecurityPlan
	Observability    ObservabilityPlan
	Deployment       DeploymentPlan
	Risks            []Risk
	Assumptions      []string
	Unknowns         []string
	CreatedAt        time.Time
}

// Component represents a system component
type Component struct {
	Name         string
	Type         ComponentType
	Purpose      string
	Technologies []string
	Dependencies []string
}

// ComponentType represents the type of component
type ComponentType string

const (
	ComponentFrontend   ComponentType = "frontend"
	ComponentBackend    ComponentType = "backend"
	ComponentDatabase   ComponentType = "database"
	ComponentCache      ComponentType = "cache"
	ComponentQueue      ComponentType = "queue"
	ComponentMonitoring ComponentType = "monitoring"
)

// DataFlow represents a data flow through the system
type DataFlow struct {
	Name        string
	Description string
	Steps       []FlowStep
	Diagram     string
}

// FlowStep represents a step in a data flow
type FlowStep struct {
	Order       int
	Component   string
	Action      string
	Description string
}

// ScalingPlan describes how the system scales
type ScalingPlan struct {
	HorizontalScaling string
	VerticalScaling   string
	Caching           string
	LoadBalancing     string
	DatabaseScaling   string
}

// APISpec describes the API contract
type APISpec struct {
	RESTEndpoints  []Endpoint
	WebSockets     []WebSocketEvent
	Authentication string
}

// Endpoint represents a REST endpoint
type Endpoint struct {
	Method      string
	Path        string
	Description string
	Request     string
	Response    string
}

// WebSocketEvent represents a WebSocket event
type WebSocketEvent struct {
	Name        string
	Direction   string
	Description string
	Payload     string
}

// Schema represents the database schema
type Schema struct {
	Tables        []Table
	Relationships []Relationship
}

// Table represents a database table
type Table struct {
	Name        string
	Description string
	Columns     []Column
}

// Column represents a table column
type Column struct {
	Name        string
	Type        string
	Constraints string
}

// Relationship represents a table relationship
type Relationship struct {
	From string
	To   string
	Type string
}

// SecurityPlan describes the security approach
type SecurityPlan struct {
	Authentication string
	Authorization  string
	Encryption     string
	Audit          string
}

// ObservabilityPlan describes the observability strategy
type ObservabilityPlan struct {
	Logging string
	Metrics string
	Tracing string
}

// DeploymentPlan describes the deployment architecture
type DeploymentPlan struct {
	Development string
	Staging     string
	Production  string
}

// Risk represents a potential risk
type Risk struct {
	Name        string
	Probability RiskLevel
	Impact      RiskLevel
	Mitigation  string
}

// RiskLevel represents the level of a risk
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// GenerateArchitecture generates a complete system architecture from interview data
func (g *Generator) GenerateArchitecture(interviewData *state.InterviewData) (*Architecture, error) {
	if g.provider == nil {
		return nil, fmt.Errorf("provider is required for architecture generation")
	}

	// Create the architecture prompt
	prompt := g.buildArchitecturePrompt(interviewData)
	cacheKey := g.generateCacheKey(prompt)

	var content string

	// Check cache
	if g.cache != nil {
		cached, err := g.cache.GetCache(cacheKey)
		if err == nil && cached != "" {
			// Verify cached content is valid
			arch, parseErr := parseArchitectureJSON(cached, interviewData.ProjectID)
			if parseErr == nil {
				g.logger.Info("Using cached architecture design")
				arch.CreatedAt = time.Now()
				return arch, nil
			}
			g.logger.Warn("Cached architecture invalid, regenerating", "error", parseErr)
		}
	}

	// Try to generate and parse architecture with retry logic
	maxRetries := 2
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Call the LLM
		response, err := g.provider.Call(context.TODO(), g.model, prompt)
		if err != nil {
			return nil, fmt.Errorf("failed to generate architecture: %w", err)
		}
		content = response.Content

		// Try to parse the response using JSON parsing
		architecture, err := parseArchitectureJSON(content, interviewData.ProjectID)
		if err == nil {
			// Success! Cache response and return
			if g.cache != nil {
				if err := g.cache.SetCache(cacheKey, content, 7*24*time.Hour); err != nil {
					g.logger.Warn("Failed to cache response", "error", err)
				}
			}
			architecture.CreatedAt = time.Now()
			return architecture, nil
		}

		// Parsing failed, log it and save the error
		g.logger.Warn("Failed to parse architecture JSON",
			"attempt", attempt+1,
			"error", err,
			"raw_content", content,
		)
		lastErr = err

		// If we've exhausted retries, try graceful degradation
		if attempt >= maxRetries {
			// Try parsing with fallback for partial architecture
			fallbackArch, warnings, parseErr := parseArchitectureWithFallback(content, interviewData.ProjectID)
			if parseErr == nil && fallbackArch != nil {
				// Return partial architecture with warnings
				fallbackArch.CreatedAt = time.Now()
				return fallbackArch, fmt.Errorf("using partial architecture due to validation errors: %v", warnings)
			}
			break
		}

		// Create a clarification prompt for retry
		prompt = g.buildClarificationPrompt(content, err)
	}

	// All retries exhausted and fallback failed
	g.logger.Error("Failed to generate valid architecture after retries",
		"error", lastErr,
		"last_content", content,
	)
	return nil, fmt.Errorf("failed to parse architecture after %d attempts: %w. Check logs for raw output", maxRetries+1, lastErr)
}

// generateCacheKey generates a cache key for a prompt
func (g *Generator) generateCacheKey(prompt string) string {
	hash := sha256.Sum256([]byte(prompt + g.model))
	return hex.EncodeToString(hash[:])
}

// buildArchitecturePrompt creates the prompt for architecture generation
func (g *Generator) buildArchitecturePrompt(interviewData *state.InterviewData) string {
	prompt := `You are an expert software architect. Based on the following project requirements, generate a comprehensive system architecture.

CRITICAL OUTPUT RULES:
1. Return ONLY valid JSON matching the schema below.
2. The response MUST BE strict JSON.
3. Do NOT include any text outside the JSON object.
4. Ensure all required fields are present.
5. Make recommendations concrete and implementation-ready.

PROJECT INFORMATION:
Problem Statement: ` + interviewData.ProblemStatement + `
Target Users: ` + strings.Join(interviewData.TargetUsers, ", ") + `
Success Metrics: ` + strings.Join(interviewData.SuccessMetrics, ", ") + `

OUTPUT FORMAT (REQUIRED):
Return ONLY valid JSON matching this exact schema:

{
  "system_overview": "High-level description of the system and key architectural decisions",
  "components": [
    {
      "name": "Component name",
      "type": "frontend|backend|database|cache|queue|monitoring|other",
      "purpose": "What this component does",
      "technologies": ["Technology 1", "Technology 2"],
      "dependencies": ["Component name 1", "Component name 2"]
    }
  ],
  "data_flows": [
    {
      "name": "User journey name",
      "description": "Description of this flow",
      "steps": [
        {
          "order": 1,
          "component": "Component name",
          "action": "Action performed",
          "description": "What happens in this step"
        }
      ],
      "diagram": "Optional ASCII diagram"
    }
  ],
  "tech_rationale": {
    "language": "Why this language was chosen",
    "framework": "Why this framework was chosen",
    "database": "Why this database was chosen",
    "infrastructure": "Why this infrastructure was chosen"
  },
  "scaling_strategy": {
    "horizontal_scaling": "How to scale horizontally",
    "vertical_scaling": "How to scale vertically",
    "caching": "Caching strategy",
    "load_balancing": "Load balancing approach",
    "database_scaling": "Database scaling strategy"
  },
  "api_contract": {
    "rest_endpoints": [
      {
        "method": "GET|POST|PUT|DELETE|PATCH",
        "path": "/api/path",
        "description": "What this endpoint does",
        "request": "Optional request body description",
        "response": "Optional response body description"
      }
    ],
    "websockets": [
      {
        "name": "Event name",
        "direction": "client-to-server|server-to-client|bidirectional",
        "description": "What this event does",
        "payload": "Optional payload description"
      }
    ],
    "authentication": "Authentication method description"
  },
  "database_schema": {
    "tables": [
      {
        "name": "table_name",
        "description": "What this table stores",
        "columns": [
          {
            "name": "column_name",
            "type": "data_type",
            "constraints": "PRIMARY KEY, FOREIGN KEY, NOT NULL, etc."
          }
        ]
      }
    ],
    "relationships": [
      {
        "from": "table1",
        "to": "table2",
        "type": "one-to-one|one-to-many|many-to-many"
      }
    ]
  },
  "security_approach": {
    "authentication": "Authentication method (JWT, OAuth, etc.)",
    "authorization": "Authorization strategy (RBAC, ABAC, etc.)",
    "encryption": "Encryption approach (TLS, at-rest encryption, etc.)",
    "audit": "Audit logging strategy"
  },
  "observability": {
    "logging": "Logging approach and tools",
    "metrics": "Metrics collection strategy",
    "tracing": "Distributed tracing approach"
  },
  "deployment": {
    "development": "Development environment setup",
    "staging": "Staging environment setup",
    "production": "Production environment setup"
  },
  "risks": [
    {
      "name": "Risk name",
      "probability": "low|medium|high|critical",
      "impact": "low|medium|high|critical",
      "mitigation": "How to mitigate this risk"
    }
  ],
  "assumptions": [
    "Assumption 1",
    "Assumption 2"
  ],
  "unknowns": [
    "Unknown 1 that needs clarification",
    "Unknown 2 that needs clarification"
  ]
}

IMPORTANT: 
- Return ONLY the JSON object, no markdown code fences
- Ensure all required fields are present
- Use the exact field names shown in the schema
- Arrays can be empty but must be present
- All string fields must have meaningful content (no empty strings for required fields)`

	return prompt
}

// buildClarificationPrompt creates a clarification prompt when JSON parsing fails
func (g *Generator) buildClarificationPrompt(previousResponse string, parseError error) string {
	prompt := `Your previous response could not be parsed as valid JSON. 

PARSING ERROR:
` + parseError.Error() + `

PREVIOUS RESPONSE (first 500 chars):
` + truncateString(previousResponse, 500) + `

CRITICAL REQUIREMENTS:
1. Return ONLY valid JSON - no markdown code fences, no explanatory text
2. The JSON must start with { and end with }
3. All required fields must be present:
   - system_overview (string, non-empty)
   - components (array with at least one component)
   - security_approach (object with authentication, authorization, encryption, audit)
   - observability (object with logging, metrics, tracing)
   - deployment (object with development, staging, production)
4. Use proper JSON syntax: double quotes for strings, no trailing commas
5. Ensure all string values are non-empty for required fields

Please provide the architecture as valid JSON now.`

	return prompt
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// parseArchitectureResponse parses the LLM response into an Architecture struct
func (g *Generator) parseArchitectureResponse(response string, interviewData *state.InterviewData) (*Architecture, error) {
	// This is a simplified parser. In production, you'd want more robust parsing
	architecture := &Architecture{
		SystemOverview:  extractSection(response, "SYSTEM OVERVIEW", "COMPONENTS"),
		Components:      []Component{},
		DataFlows:       []DataFlow{},
		TechRationale:   make(map[string]string),
		ScalingStrategy: ScalingPlan{},
		APIContract:     APISpec{},
		DatabaseSchema:  Schema{},
		SecurityApproach: SecurityPlan{
			Authentication: extractSection(response, "Authentication", "Authorization"),
			Authorization:  extractSection(response, "Authorization", "Encryption"),
			Encryption:     extractSection(response, "Encryption", "Audit"),
			Audit:          extractSection(response, "Audit", "OBSERVABILITY"),
		},
		Observability: ObservabilityPlan{
			Logging: extractSection(response, "Logging", "Metrics"),
			Metrics: extractSection(response, "Metrics", "Tracing"),
			Tracing: extractSection(response, "Tracing", "DEPLOYMENT"),
		},
		Deployment: DeploymentPlan{
			Development: extractSection(response, "Development", "Staging"),
			Staging:     extractSection(response, "Staging", "Production"),
			Production:  extractSection(response, "Production", "RISK"),
		},
		Risks:       []Risk{},
		Assumptions: []string{},
		Unknowns:    []string{},
	}

	// Extract components (simplified)
	componentsSection := extractSection(response, "COMPONENTS", "DATA FLOWS")
	if componentsSection != "" {
		architecture.Components = append(architecture.Components, Component{
			Name:         "Backend API",
			Type:         ComponentBackend,
			Purpose:      "Core business logic",
			Technologies: []string{},
			Dependencies: []string{},
		})
	}

	return architecture, nil
}

// extractSection extracts a section from the response between two markers
func extractSection(text, startMarker, endMarker string) string {
	startIdx := strings.Index(text, startMarker)
	if startIdx == -1 {
		return ""
	}

	endIdx := strings.Index(text[startIdx:], endMarker)
	if endIdx == -1 {
		return strings.TrimSpace(text[startIdx+len(startMarker):])
	}

	return strings.TrimSpace(text[startIdx+len(startMarker) : startIdx+endIdx])
}

// ExportMarkdown exports the architecture as markdown
func (g *Generator) ExportMarkdown(architecture *Architecture) (string, error) {
	var md strings.Builder

	md.WriteString("# System Architecture\n\n")
	md.WriteString(fmt.Sprintf("**Project ID:** %s\n", architecture.ProjectID))
	md.WriteString(fmt.Sprintf("**Generated:** %s\n\n", architecture.CreatedAt.Format("2006-01-02 15:04:05")))

	md.WriteString("## System Overview\n\n")
	md.WriteString(architecture.SystemOverview + "\n\n")

	md.WriteString("## Components\n\n")
	for _, comp := range architecture.Components {
		md.WriteString(fmt.Sprintf("### %s (%s)\n\n", comp.Name, comp.Type))
		md.WriteString(fmt.Sprintf("**Purpose:** %s\n\n", comp.Purpose))
		if len(comp.Technologies) > 0 {
			md.WriteString(fmt.Sprintf("**Technologies:** %s\n\n", strings.Join(comp.Technologies, ", ")))
		}
		if len(comp.Dependencies) > 0 {
			md.WriteString(fmt.Sprintf("**Dependencies:** %s\n\n", strings.Join(comp.Dependencies, ", ")))
		}
	}

	md.WriteString("## Security Approach\n\n")
	md.WriteString(fmt.Sprintf("**Authentication:** %s\n\n", architecture.SecurityApproach.Authentication))
	md.WriteString(fmt.Sprintf("**Authorization:** %s\n\n", architecture.SecurityApproach.Authorization))
	md.WriteString(fmt.Sprintf("**Encryption:** %s\n\n", architecture.SecurityApproach.Encryption))
	md.WriteString(fmt.Sprintf("**Audit:** %s\n\n", architecture.SecurityApproach.Audit))

	md.WriteString("## Risks\n\n")
	for _, risk := range architecture.Risks {
		md.WriteString(fmt.Sprintf("### %s\n\n", risk.Name))
		md.WriteString(fmt.Sprintf("- **Probability:** %s\n", risk.Probability))
		md.WriteString(fmt.Sprintf("- **Impact:** %s\n", risk.Impact))
		md.WriteString(fmt.Sprintf("- **Mitigation:** %s\n\n", risk.Mitigation))
	}

	if len(architecture.Assumptions) > 0 {
		md.WriteString("## Assumptions\n\n")
		for _, assumption := range architecture.Assumptions {
			md.WriteString(fmt.Sprintf("- %s\n", assumption))
		}
		md.WriteString("\n")
	}

	if len(architecture.Unknowns) > 0 {
		md.WriteString("## Unknowns\n\n")
		for _, unknown := range architecture.Unknowns {
			md.WriteString(fmt.Sprintf("- %s\n", unknown))
		}
		md.WriteString("\n")
	}

	return md.String(), nil
}

// ExportJSON exports the architecture as JSON
func (g *Generator) ExportJSON(architecture *Architecture) (string, error) {
	jsonData, err := json.MarshalIndent(architecture, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal architecture: %w", err)
	}
	return string(jsonData), nil
}

// ArchitectureIteration represents a refinement of the architecture
type ArchitectureIteration struct {
	Timestamp time.Time
	Section   string
	OldValue  string
	NewValue  string
	Reason    string
}

// RefineArchitecture refines a specific section of the architecture
func (g *Generator) RefineArchitecture(architecture *Architecture, section string, refinementRequest string) (*Architecture, error) {
	if g.provider == nil {
		return nil, fmt.Errorf("provider is required for architecture refinement")
	}

	prompt := fmt.Sprintf(`You are refining a system architecture. The user wants to modify the following section:

SECTION: %s
CURRENT CONTENT:
%s

REFINEMENT REQUEST:
%s

Please provide the updated content for this section, maintaining consistency with the rest of the architecture.`,
		section, g.getSectionContent(architecture, section), refinementRequest)

	prompt += "\n\nOUTPUT RULES:\n- Return only the revised section content.\n- No preamble, no explanations, no markdown code fences.\n- Keep content specific and implementation-oriented."

	response, err := g.provider.Call(context.TODO(), g.model, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to refine architecture: %w", err)
	}

	// Update the architecture with the refined content
	updatedArch := g.updateArchitectureSection(architecture, section, response.Content)

	return updatedArch, nil
}

// getSectionContent retrieves the content of a specific section
func (g *Generator) getSectionContent(architecture *Architecture, section string) string {
	switch section {
	case "system_overview":
		return architecture.SystemOverview
	case "scaling_strategy":
		return fmt.Sprintf("Horizontal: %s\nVertical: %s\nCaching: %s\nLoad Balancing: %s\nDatabase: %s",
			architecture.ScalingStrategy.HorizontalScaling,
			architecture.ScalingStrategy.VerticalScaling,
			architecture.ScalingStrategy.Caching,
			architecture.ScalingStrategy.LoadBalancing,
			architecture.ScalingStrategy.DatabaseScaling)
	case "security":
		return fmt.Sprintf("Auth: %s\nAuthz: %s\nEncryption: %s\nAudit: %s",
			architecture.SecurityApproach.Authentication,
			architecture.SecurityApproach.Authorization,
			architecture.SecurityApproach.Encryption,
			architecture.SecurityApproach.Audit)
	case "observability":
		return fmt.Sprintf("Logging: %s\nMetrics: %s\nTracing: %s",
			architecture.Observability.Logging,
			architecture.Observability.Metrics,
			architecture.Observability.Tracing)
	case "deployment":
		return fmt.Sprintf("Dev: %s\nStaging: %s\nProd: %s",
			architecture.Deployment.Development,
			architecture.Deployment.Staging,
			architecture.Deployment.Production)
	default:
		return ""
	}
}

// updateArchitectureSection updates a specific section with new content
func (g *Generator) updateArchitectureSection(architecture *Architecture, section string, newContent string) *Architecture {
	updated := *architecture

	switch section {
	case "system_overview":
		updated.SystemOverview = newContent
	case "scaling_strategy":
		// Parse the new content and update scaling strategy
		updated.ScalingStrategy.HorizontalScaling = newContent
	case "security":
		// Parse and update security approach
		updated.SecurityApproach.Authentication = newContent
	case "observability":
		// Parse and update observability
		updated.Observability.Logging = newContent
	case "deployment":
		// Parse and update deployment
		updated.Deployment.Development = newContent
	}

	return &updated
}

// ListRefinableSection returns the sections that can be refined
func (g *Generator) ListRefinableSections() []string {
	return []string{
		"system_overview",
		"components",
		"technology_rationale",
		"scaling_strategy",
		"api_contract",
		"database_schema",
		"security",
		"observability",
		"deployment",
		"risks",
	}
}

// ValidateArchitecture checks if the architecture is complete and consistent
func (g *Generator) ValidateArchitecture(architecture *Architecture) (bool, []string) {
	var issues []string

	if architecture.SystemOverview == "" {
		issues = append(issues, "System overview is missing")
	}

	if len(architecture.Components) == 0 {
		issues = append(issues, "No components defined")
	}

	if architecture.SecurityApproach.Authentication == "" {
		issues = append(issues, "Authentication approach not defined")
	}

	if architecture.SecurityApproach.Authorization == "" {
		issues = append(issues, "Authorization approach not defined")
	}

	if len(architecture.Risks) == 0 {
		issues = append(issues, "No risks identified")
	}

	return len(issues) == 0, issues
}
