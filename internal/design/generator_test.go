package design

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
)

// MockProvider for testing
type MockProvider struct {
	response string
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
	return &provider.Response{
		Content:      m.response,
		TokensInput:  100,
		TokensOutput: 200,
		Model:        model,
		Provider:     "mock",
	}, nil
}

func (m *MockProvider) Stream(ctx context.Context, model string, prompt string) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- m.response
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

func TestDesignGenerator(t *testing.T) {
	mockResponse := `{
		"system_overview": "This is a task management system with real-time collaboration features",
		"components": [
			{
				"name": "Backend API",
				"type": "backend",
				"purpose": "REST API server for core business logic",
				"technologies": ["Go", "PostgreSQL"],
				"dependencies": ["Database"]
			},
			{
				"name": "Frontend",
				"type": "frontend",
				"purpose": "React web application for user interface",
				"technologies": ["React", "TypeScript"],
				"dependencies": ["Backend API"]
			},
			{
				"name": "Database",
				"type": "database",
				"purpose": "PostgreSQL for data storage",
				"technologies": ["PostgreSQL"],
				"dependencies": []
			}
		],
		"data_flows": [
			{
				"name": "User creates task",
				"description": "Flow for creating a new task",
				"steps": [
					{
						"order": 1,
						"component": "Frontend",
						"action": "Submit task form",
						"description": "User submits task creation form"
					},
					{
						"order": 2,
						"component": "Backend API",
						"action": "Validate and process",
						"description": "API validates and processes the request"
					},
					{
						"order": 3,
						"component": "Database",
						"action": "Store task",
						"description": "Task is stored in database"
					}
				],
				"diagram": "User -> Frontend -> Backend -> Database"
			}
		],
		"tech_rationale": {
			"language": "Go for backend: Performance and concurrency",
			"framework": "React for frontend: Component-based architecture",
			"database": "PostgreSQL: ACID compliance and reliability",
			"infrastructure": "Kubernetes for orchestration and scaling"
		},
		"scaling_strategy": {
			"horizontal_scaling": "Multiple API servers behind load balancer",
			"vertical_scaling": "Increase server resources as needed",
			"caching": "Redis for session and frequently accessed data",
			"load_balancing": "Nginx reverse proxy",
			"database_scaling": "Read replicas for queries"
		},
		"api_contract": {
			"rest_endpoints": [
				{
					"method": "POST",
					"path": "/api/tasks",
					"description": "Create new task",
					"request": "Task object with title and description",
					"response": "Created task with ID"
				},
				{
					"method": "GET",
					"path": "/api/tasks",
					"description": "List all tasks",
					"request": "",
					"response": "Array of tasks"
				},
				{
					"method": "GET",
					"path": "/api/tasks/:id",
					"description": "Get task details",
					"request": "",
					"response": "Task object"
				}
			],
			"websockets": [],
			"authentication": "JWT tokens for API authentication"
		},
		"database_schema": {
			"tables": [
				{
					"name": "tasks",
					"description": "Stores task information",
					"columns": [
						{
							"name": "id",
							"type": "UUID",
							"constraints": "PRIMARY KEY"
						},
						{
							"name": "title",
							"type": "VARCHAR(255)",
							"constraints": "NOT NULL"
						},
						{
							"name": "description",
							"type": "TEXT",
							"constraints": ""
						},
						{
							"name": "status",
							"type": "VARCHAR(50)",
							"constraints": "NOT NULL"
						},
						{
							"name": "created_at",
							"type": "TIMESTAMP",
							"constraints": "NOT NULL DEFAULT NOW()"
						}
					]
				},
				{
					"name": "users",
					"description": "Stores user information",
					"columns": [
						{
							"name": "id",
							"type": "UUID",
							"constraints": "PRIMARY KEY"
						},
						{
							"name": "email",
							"type": "VARCHAR(255)",
							"constraints": "NOT NULL UNIQUE"
						},
						{
							"name": "name",
							"type": "VARCHAR(255)",
							"constraints": "NOT NULL"
						},
						{
							"name": "created_at",
							"type": "TIMESTAMP",
							"constraints": "NOT NULL DEFAULT NOW()"
						}
					]
				}
			],
			"relationships": [
				{
					"from": "tasks",
					"to": "users",
					"type": "many-to-one"
				}
			]
		},
		"security_approach": {
			"authentication": "JWT tokens",
			"authorization": "Role-based access control",
			"encryption": "TLS for transport, bcrypt for passwords",
			"audit": "Log all data modifications"
		},
		"observability": {
			"logging": "Structured JSON logs",
			"metrics": "Prometheus metrics",
			"tracing": "OpenTelemetry distributed tracing"
		},
		"deployment": {
			"development": "Local Docker Compose",
			"staging": "Kubernetes cluster with staging namespace",
			"production": "Kubernetes cluster with production namespace"
		},
		"risks": [
			{
				"name": "Database failure",
				"probability": "high",
				"impact": "high",
				"mitigation": "Implement automated backups"
			},
			{
				"name": "API overload",
				"probability": "medium",
				"impact": "medium",
				"mitigation": "Add rate limiting"
			}
		],
		"assumptions": [
			"Users have modern browsers"
		],
		"unknowns": [
			"Exact peak load requirements"
		]
	}`

	mockProvider := &MockProvider{response: mockResponse}
	generator := NewGenerator(mockProvider, "test-model", nil)

	interviewData := &state.InterviewData{
		ProjectID:        "test-project",
		ProjectName:      "Test Project",
		ProblemStatement: "Need a task management system",
		TargetUsers:      []string{"Developers"},
		SuccessMetrics:   []string{"User engagement"},
		CreatedAt:        time.Now(),
	}

	t.Run("GenerateArchitecture", func(t *testing.T) {
		architecture, err := generator.GenerateArchitecture(interviewData)
		if err != nil {
			t.Fatalf("Failed to generate architecture: %v", err)
		}

		if architecture == nil {
			t.Fatal("Architecture should not be nil")
		}

		if architecture.ProjectID != "test-project" {
			t.Errorf("Expected project ID 'test-project', got '%s'", architecture.ProjectID)
		}

		if architecture.SystemOverview == "" {
			t.Error("System overview should not be empty")
		}
	})

	t.Run("ExportMarkdown", func(t *testing.T) {
		architecture := &Architecture{
			ProjectID:      "test-project",
			SystemOverview: "Test system",
			Components: []Component{
				{
					Name:         "Backend",
					Type:         ComponentBackend,
					Purpose:      "API server",
					Technologies: []string{"Go"},
					Dependencies: []string{"Database"},
				},
			},
			SecurityApproach: SecurityPlan{
				Authentication: "JWT",
				Authorization:  "RBAC",
				Encryption:     "TLS",
				Audit:          "Logs",
			},
			Risks: []Risk{
				{
					Name:        "Data loss",
					Probability: RiskMedium,
					Impact:      RiskHigh,
					Mitigation:  "Backups",
				},
			},
			Assumptions: []string{"Users have internet"},
			Unknowns:    []string{"Peak load"},
			CreatedAt:   time.Now(),
		}

		markdown, err := generator.ExportMarkdown(architecture)
		if err != nil {
			t.Fatalf("Failed to export markdown: %v", err)
		}

		if markdown == "" {
			t.Fatal("Markdown should not be empty")
		}

		// Check for key sections
		if !contains(markdown, "System Architecture") {
			t.Error("Markdown should contain 'System Architecture'")
		}

		if !contains(markdown, "Backend") {
			t.Error("Markdown should contain component name")
		}

		if !contains(markdown, "Security Approach") {
			t.Error("Markdown should contain security section")
		}

		if !contains(markdown, "Risks") {
			t.Error("Markdown should contain risks section")
		}
	})

	t.Run("ExportJSON", func(t *testing.T) {
		architecture := &Architecture{
			ProjectID:      "test-project",
			SystemOverview: "Test system",
			Components:     []Component{},
			CreatedAt:      time.Now(),
		}

		jsonStr, err := generator.ExportJSON(architecture)
		if err != nil {
			t.Fatalf("Failed to export JSON: %v", err)
		}

		if jsonStr == "" {
			t.Fatal("JSON should not be empty")
		}

		if !contains(jsonStr, "test-project") {
			t.Error("JSON should contain project ID")
		}
	})

	t.Run("GenerateArchitecture_NoProvider", func(t *testing.T) {
		generator := NewGenerator(nil, "test-model", nil)

		_, err := generator.GenerateArchitecture(interviewData)
		if err == nil {
			t.Error("Should error when provider is nil")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsHelper(s, substr)
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDesignGenerator_Reiteration(t *testing.T) {
	mockProvider := &MockProvider{response: "Updated system overview with better details"}
	generator := NewGenerator(mockProvider, "test-model", nil)

	architecture := &Architecture{
		ProjectID:      "test-project",
		SystemOverview: "Original system overview",
		Components: []Component{
			{Name: "Backend", Type: ComponentBackend},
		},
		SecurityApproach: SecurityPlan{
			Authentication: "JWT",
			Authorization:  "RBAC",
		},
		Risks: []Risk{
			{Name: "Test risk", Probability: RiskLow, Impact: RiskLow},
		},
		CreatedAt: time.Now(),
	}

	t.Run("RefineArchitecture", func(t *testing.T) {
		refined, err := generator.RefineArchitecture(architecture, "system_overview", "Make it more detailed")
		if err != nil {
			t.Fatalf("Failed to refine architecture: %v", err)
		}

		if refined == nil {
			t.Fatal("Refined architecture should not be nil")
		}

		if refined.SystemOverview == architecture.SystemOverview {
			t.Error("System overview should have been updated")
		}
	})

	t.Run("ListRefinableSections", func(t *testing.T) {
		sections := generator.ListRefinableSections()

		if len(sections) == 0 {
			t.Error("Should have refinable sections")
		}

		// Check for key sections
		hasSystemOverview := false
		hasSecurity := false
		for _, section := range sections {
			if section == "system_overview" {
				hasSystemOverview = true
			}
			if section == "security" {
				hasSecurity = true
			}
		}

		if !hasSystemOverview {
			t.Error("Should include system_overview in refinable sections")
		}

		if !hasSecurity {
			t.Error("Should include security in refinable sections")
		}
	})

	t.Run("ValidateArchitecture_Valid", func(t *testing.T) {
		isValid, issues := generator.ValidateArchitecture(architecture)

		if !isValid {
			t.Errorf("Architecture should be valid, issues: %v", issues)
		}

		if len(issues) != 0 {
			t.Errorf("Should have no issues, got: %v", issues)
		}
	})

	t.Run("ValidateArchitecture_Invalid", func(t *testing.T) {
		invalidArch := &Architecture{
			ProjectID:      "test-project",
			SystemOverview: "",
			Components:     []Component{},
			SecurityApproach: SecurityPlan{
				Authentication: "",
				Authorization:  "",
			},
			Risks:     []Risk{},
			CreatedAt: time.Now(),
		}

		isValid, issues := generator.ValidateArchitecture(invalidArch)

		if isValid {
			t.Error("Architecture should be invalid")
		}

		if len(issues) == 0 {
			t.Error("Should have validation issues")
		}

		// Check for specific issues
		hasSystemOverviewIssue := false
		hasComponentsIssue := false
		for _, issue := range issues {
			if contains(issue, "System overview") {
				hasSystemOverviewIssue = true
			}
			if contains(issue, "components") {
				hasComponentsIssue = true
			}
		}

		if !hasSystemOverviewIssue {
			t.Error("Should identify missing system overview")
		}

		if !hasComponentsIssue {
			t.Error("Should identify missing components")
		}
	})

	t.Run("RefineArchitecture_NoProvider", func(t *testing.T) {
		generator := NewGenerator(nil, "test-model", nil)

		_, err := generator.RefineArchitecture(architecture, "system_overview", "Update it")
		if err == nil {
			t.Error("Should error when provider is nil")
		}
	})
}

// MockProviderWithRetry allows testing retry logic by returning different responses
type MockProviderWithRetry struct {
	responses []string
	callCount int
}

func (m *MockProviderWithRetry) Name() string {
	return "mock-retry"
}

func (m *MockProviderWithRetry) Authenticate(apiKey string) error {
	return nil
}

func (m *MockProviderWithRetry) IsAuthenticated() bool {
	return true
}

func (m *MockProviderWithRetry) ListModels() ([]provider.Model, error) {
	return []provider.Model{}, nil
}

func (m *MockProviderWithRetry) DiscoverModels() ([]provider.Model, error) {
	return []provider.Model{}, nil
}

func (m *MockProviderWithRetry) Call(ctx context.Context, model string, prompt string) (*provider.Response, error) {
	if m.callCount >= len(m.responses) {
		m.callCount++
		return &provider.Response{
			Content:      m.responses[len(m.responses)-1],
			TokensInput:  100,
			TokensOutput: 200,
			Model:        model,
			Provider:     "mock-retry",
		}, nil
	}

	response := m.responses[m.callCount]
	m.callCount++

	return &provider.Response{
		Content:      response,
		TokensInput:  100,
		TokensOutput: 200,
		Model:        model,
		Provider:     "mock-retry",
	}, nil
}

func (m *MockProviderWithRetry) Stream(ctx context.Context, model string, prompt string) (<-chan string, error) {
	ch := make(chan string, 1)
	if m.callCount < len(m.responses) {
		ch <- m.responses[m.callCount]
		m.callCount++
	}
	close(ch)
	return ch, nil
}

func (m *MockProviderWithRetry) GetRateLimitInfo() (*provider.RateLimitInfo, error) {
	return nil, nil
}

func (m *MockProviderWithRetry) GetQuotaInfo() (*provider.QuotaInfo, error) {
	return nil, nil
}

func (m *MockProviderWithRetry) SupportsCodingPlan() bool {
	return false
}

func TestGenerateArchitecture_RetryLogic(t *testing.T) {
	validJSON := `{
		"system_overview": "A task management system with real-time collaboration",
		"components": [
			{
				"name": "Backend API",
				"type": "backend",
				"purpose": "Core business logic and data management",
				"technologies": ["Go", "PostgreSQL"],
				"dependencies": ["Database"]
			}
		],
		"data_flows": [],
		"tech_rationale": {
			"language": "Go for performance",
			"framework": "Standard library",
			"database": "PostgreSQL for reliability",
			"infrastructure": "Kubernetes for orchestration"
		},
		"scaling_strategy": {
			"horizontal_scaling": "Multiple API servers",
			"vertical_scaling": "Increase resources",
			"caching": "Redis for sessions",
			"load_balancing": "Nginx",
			"database_scaling": "Read replicas"
		},
		"api_contract": {
			"rest_endpoints": [
				{
					"method": "POST",
					"path": "/api/tasks",
					"description": "Create a new task",
					"request": "Task object",
					"response": "Created task with ID"
				}
			],
			"websockets": [],
			"authentication": "JWT tokens"
		},
		"database_schema": {
			"tables": [
				{
					"name": "tasks",
					"description": "Stores task information",
					"columns": [
						{
							"name": "id",
							"type": "UUID",
							"constraints": "PRIMARY KEY"
						},
						{
							"name": "title",
							"type": "VARCHAR(255)",
							"constraints": "NOT NULL"
						}
					]
				}
			],
			"relationships": []
		},
		"security_approach": {
			"authentication": "JWT tokens with refresh mechanism",
			"authorization": "Role-based access control (RBAC)",
			"encryption": "TLS 1.3 for transport, AES-256 at rest",
			"audit": "Comprehensive audit logging of all operations"
		},
		"observability": {
			"logging": "Structured JSON logs with log levels",
			"metrics": "Prometheus metrics for monitoring",
			"tracing": "OpenTelemetry distributed tracing"
		},
		"deployment": {
			"development": "Local Docker Compose setup",
			"staging": "Kubernetes cluster with staging namespace",
			"production": "Kubernetes cluster with HA configuration"
		},
		"risks": [
			{
				"name": "Database failure",
				"probability": "medium",
				"impact": "high",
				"mitigation": "Automated backups and failover"
			}
		],
		"assumptions": [
			"Users have modern browsers",
			"Internet connectivity is reliable"
		],
		"unknowns": [
			"Exact peak load requirements",
			"Geographic distribution of users"
		]
	}`

	interviewData := &state.InterviewData{
		ProjectID:        "test-project",
		ProjectName:      "Test Project",
		ProblemStatement: "Need a task management system",
		TargetUsers:      []string{"Developers"},
		SuccessMetrics:   []string{"User engagement"},
		CreatedAt:        time.Now(),
	}

	t.Run("Success on first attempt", func(t *testing.T) {
		mockProvider := &MockProviderWithRetry{
			responses: []string{validJSON},
			callCount: 0,
		}
		generator := NewGenerator(mockProvider, "test-model", nil)

		architecture, err := generator.GenerateArchitecture(interviewData)
		if err != nil {
			t.Fatalf("Expected success on first attempt, got error: %v", err)
		}

		if architecture == nil {
			t.Fatal("Architecture should not be nil")
		}

		if mockProvider.callCount != 1 {
			t.Errorf("Expected 1 call, got %d", mockProvider.callCount)
		}

		if architecture.SystemOverview != "A task management system with real-time collaboration" {
			t.Errorf("Unexpected system overview: %s", architecture.SystemOverview)
		}
	})

	t.Run("Success on second attempt after malformed JSON", func(t *testing.T) {
		malformedJSON := `This is not JSON at all, just plain text`

		mockProvider := &MockProviderWithRetry{
			responses: []string{malformedJSON, validJSON},
			callCount: 0,
		}
		generator := NewGenerator(mockProvider, "test-model", nil)

		architecture, err := generator.GenerateArchitecture(interviewData)
		if err != nil {
			t.Fatalf("Expected success on second attempt, got error: %v", err)
		}

		if architecture == nil {
			t.Fatal("Architecture should not be nil")
		}

		if mockProvider.callCount != 2 {
			t.Errorf("Expected 2 calls, got %d", mockProvider.callCount)
		}
	})

	t.Run("Success on third attempt after two failures", func(t *testing.T) {
		malformedJSON1 := `{"invalid": "json", "missing": "required fields"}`
		malformedJSON2 := `{"system_overview": "", "components": []}`

		mockProvider := &MockProviderWithRetry{
			responses: []string{malformedJSON1, malformedJSON2, validJSON},
			callCount: 0,
		}
		generator := NewGenerator(mockProvider, "test-model", nil)

		architecture, err := generator.GenerateArchitecture(interviewData)
		if err != nil {
			t.Fatalf("Expected success on third attempt, got error: %v", err)
		}

		if architecture == nil {
			t.Fatal("Architecture should not be nil")
		}

		if mockProvider.callCount != 3 {
			t.Errorf("Expected 3 calls, got %d", mockProvider.callCount)
		}
	})

	t.Run("Failure after exhausting retries", func(t *testing.T) {
		malformedJSON := `This is not valid JSON`

		mockProvider := &MockProviderWithRetry{
			responses: []string{malformedJSON, malformedJSON, malformedJSON},
			callCount: 0,
		}
		generator := NewGenerator(mockProvider, "test-model", nil)

		architecture, err := generator.GenerateArchitecture(interviewData)
		if err == nil {
			t.Fatal("Expected error after exhausting retries")
		}

		if architecture != nil {
			t.Error("Architecture should be nil on failure")
		}

		if mockProvider.callCount != 3 {
			t.Errorf("Expected 3 calls (1 initial + 2 retries), got %d", mockProvider.callCount)
		}

		// Check error message mentions attempts
		if !contains(err.Error(), "after 3 attempts") {
			t.Errorf("Error message should mention number of attempts: %v", err)
		}
	})

	t.Run("Success with JSON in markdown code fence on retry", func(t *testing.T) {
		malformedJSON := `Here's the architecture: not valid json`
		jsonInMarkdown := "```json\n" + validJSON + "\n```"

		mockProvider := &MockProviderWithRetry{
			responses: []string{malformedJSON, jsonInMarkdown},
			callCount: 0,
		}
		generator := NewGenerator(mockProvider, "test-model", nil)

		architecture, err := generator.GenerateArchitecture(interviewData)
		if err != nil {
			t.Fatalf("Expected success with markdown code fence, got error: %v", err)
		}

		if architecture == nil {
			t.Fatal("Architecture should not be nil")
		}

		if mockProvider.callCount != 2 {
			t.Errorf("Expected 2 calls, got %d", mockProvider.callCount)
		}
	})
}

func TestBuildClarificationPrompt(t *testing.T) {
	generator := NewGenerator(nil, "test-model", nil)

	t.Run("Creates clarification prompt with error details", func(t *testing.T) {
		previousResponse := "This is not valid JSON"
		parseError := fmt.Errorf("invalid character 'T' looking for beginning of value")

		prompt := generator.buildClarificationPrompt(previousResponse, parseError)

		if prompt == "" {
			t.Fatal("Clarification prompt should not be empty")
		}

		// Check that error is included
		if !contains(prompt, parseError.Error()) {
			t.Error("Prompt should contain the parse error")
		}

		// Check that previous response is included (truncated)
		if !contains(prompt, previousResponse) {
			t.Error("Prompt should contain the previous response")
		}

		// Check for critical requirements
		if !contains(prompt, "CRITICAL REQUIREMENTS") {
			t.Error("Prompt should contain critical requirements section")
		}

		if !contains(prompt, "valid JSON") {
			t.Error("Prompt should mention valid JSON requirement")
		}
	})

	t.Run("Truncates long responses", func(t *testing.T) {
		longResponse := ""
		for i := 0; i < 1000; i++ {
			longResponse += "x"
		}
		parseError := fmt.Errorf("parse error")

		prompt := generator.buildClarificationPrompt(longResponse, parseError)

		// The prompt should contain truncated response (500 chars + "...")
		if contains(prompt, longResponse) {
			t.Error("Long response should be truncated")
		}

		if !contains(prompt, "...") {
			t.Error("Truncated response should end with ...")
		}
	})
}

func TestTruncateString(t *testing.T) {
	t.Run("Does not truncate short strings", func(t *testing.T) {
		short := "Hello, World!"
		result := truncateString(short, 100)

		if result != short {
			t.Errorf("Expected '%s', got '%s'", short, result)
		}
	})

	t.Run("Truncates long strings", func(t *testing.T) {
		long := "This is a very long string that should be truncated"
		result := truncateString(long, 10)

		if len(result) != 13 { // 10 chars + "..."
			t.Errorf("Expected length 13, got %d", len(result))
		}

		if !contains(result, "...") {
			t.Error("Truncated string should end with ...")
		}

		if result != "This is a ..." {
			t.Errorf("Expected 'This is a ...', got '%s'", result)
		}
	})

	t.Run("Handles exact length", func(t *testing.T) {
		exact := "Exactly10!"
		result := truncateString(exact, 10)

		if result != exact {
			t.Errorf("Expected '%s', got '%s'", exact, result)
		}
	})
}
