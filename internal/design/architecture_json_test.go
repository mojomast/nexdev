package design

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateArchitectureJSON_ValidArchitecture(t *testing.T) {
	arch := &ArchitectureJSON{
		SystemOverview: "A comprehensive system for managing tasks",
		Components: []ComponentJSON{
			{
				Name:         "API Server",
				Type:         "backend",
				Purpose:      "Handle HTTP requests",
				Technologies: []string{"Go", "Gin"},
				Dependencies: []string{"Database"},
			},
		},
		SecurityApproach: SecurityPlanJSON{
			Authentication: "JWT tokens",
			Authorization:  "RBAC",
			Encryption:     "TLS 1.3",
			Audit:          "Structured logging",
		},
		Observability: ObservabilityPlanJSON{
			Logging: "Structured logs with slog",
			Metrics: "Prometheus",
			Tracing: "OpenTelemetry",
		},
		Deployment: DeploymentPlanJSON{
			Development: "Docker Compose",
			Staging:     "Kubernetes on GKE",
			Production:  "Kubernetes on GKE with HA",
		},
	}

	err := ValidateArchitectureJSON(arch)
	assert.NoError(t, err)
}

func TestValidateArchitectureJSON_MissingSystemOverview(t *testing.T) {
	arch := &ArchitectureJSON{
		SystemOverview: "",
		Components: []ComponentJSON{
			{
				Name:    "API Server",
				Type:    "backend",
				Purpose: "Handle HTTP requests",
			},
		},
		SecurityApproach: SecurityPlanJSON{
			Authentication: "JWT",
			Authorization:  "RBAC",
			Encryption:     "TLS",
			Audit:          "Logs",
		},
		Observability: ObservabilityPlanJSON{
			Logging: "slog",
			Metrics: "Prometheus",
			Tracing: "OpenTelemetry",
		},
		Deployment: DeploymentPlanJSON{
			Development: "Docker",
			Staging:     "K8s",
			Production:  "K8s",
		},
	}

	err := ValidateArchitectureJSON(arch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "system_overview is required")
}

func TestValidateArchitectureJSON_EmptyComponents(t *testing.T) {
	arch := &ArchitectureJSON{
		SystemOverview: "A system",
		Components:     []ComponentJSON{},
		SecurityApproach: SecurityPlanJSON{
			Authentication: "JWT",
			Authorization:  "RBAC",
			Encryption:     "TLS",
			Audit:          "Logs",
		},
		Observability: ObservabilityPlanJSON{
			Logging: "slog",
			Metrics: "Prometheus",
			Tracing: "OpenTelemetry",
		},
		Deployment: DeploymentPlanJSON{
			Development: "Docker",
			Staging:     "K8s",
			Production:  "K8s",
		},
	}

	err := ValidateArchitectureJSON(arch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "components array is required and must not be empty")
}

func TestValidateArchitectureJSON_ComponentMissingName(t *testing.T) {
	arch := &ArchitectureJSON{
		SystemOverview: "A system",
		Components: []ComponentJSON{
			{
				Name:    "",
				Type:    "backend",
				Purpose: "Handle requests",
			},
		},
		SecurityApproach: SecurityPlanJSON{
			Authentication: "JWT",
			Authorization:  "RBAC",
			Encryption:     "TLS",
			Audit:          "Logs",
		},
		Observability: ObservabilityPlanJSON{
			Logging: "slog",
			Metrics: "Prometheus",
			Tracing: "OpenTelemetry",
		},
		Deployment: DeploymentPlanJSON{
			Development: "Docker",
			Staging:     "K8s",
			Production:  "K8s",
		},
	}

	err := ValidateArchitectureJSON(arch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "component[0].name is required")
}

func TestValidateArchitectureJSON_ComponentMissingType(t *testing.T) {
	arch := &ArchitectureJSON{
		SystemOverview: "A system",
		Components: []ComponentJSON{
			{
				Name:    "API Server",
				Type:    "",
				Purpose: "Handle requests",
			},
		},
		SecurityApproach: SecurityPlanJSON{
			Authentication: "JWT",
			Authorization:  "RBAC",
			Encryption:     "TLS",
			Audit:          "Logs",
		},
		Observability: ObservabilityPlanJSON{
			Logging: "slog",
			Metrics: "Prometheus",
			Tracing: "OpenTelemetry",
		},
		Deployment: DeploymentPlanJSON{
			Development: "Docker",
			Staging:     "K8s",
			Production:  "K8s",
		},
	}

	err := ValidateArchitectureJSON(arch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "component[0].type is required")
}

func TestValidateArchitectureJSON_ComponentMissingPurpose(t *testing.T) {
	arch := &ArchitectureJSON{
		SystemOverview: "A system",
		Components: []ComponentJSON{
			{
				Name:    "API Server",
				Type:    "backend",
				Purpose: "",
			},
		},
		SecurityApproach: SecurityPlanJSON{
			Authentication: "JWT",
			Authorization:  "RBAC",
			Encryption:     "TLS",
			Audit:          "Logs",
		},
		Observability: ObservabilityPlanJSON{
			Logging: "slog",
			Metrics: "Prometheus",
			Tracing: "OpenTelemetry",
		},
		Deployment: DeploymentPlanJSON{
			Development: "Docker",
			Staging:     "K8s",
			Production:  "K8s",
		},
	}

	err := ValidateArchitectureJSON(arch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "component[0].purpose is required")
}

func TestValidateArchitectureJSON_MissingSecurityFields(t *testing.T) {
	tests := []struct {
		name          string
		security      SecurityPlanJSON
		expectedError string
	}{
		{
			name: "missing authentication",
			security: SecurityPlanJSON{
				Authentication: "",
				Authorization:  "RBAC",
				Encryption:     "TLS",
				Audit:          "Logs",
			},
			expectedError: "security_approach.authentication is required",
		},
		{
			name: "missing authorization",
			security: SecurityPlanJSON{
				Authentication: "JWT",
				Authorization:  "",
				Encryption:     "TLS",
				Audit:          "Logs",
			},
			expectedError: "security_approach.authorization is required",
		},
		{
			name: "missing encryption",
			security: SecurityPlanJSON{
				Authentication: "JWT",
				Authorization:  "RBAC",
				Encryption:     "",
				Audit:          "Logs",
			},
			expectedError: "security_approach.encryption is required",
		},
		{
			name: "missing audit",
			security: SecurityPlanJSON{
				Authentication: "JWT",
				Authorization:  "RBAC",
				Encryption:     "TLS",
				Audit:          "",
			},
			expectedError: "security_approach.audit is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arch := &ArchitectureJSON{
				SystemOverview: "A system",
				Components: []ComponentJSON{
					{Name: "API", Type: "backend", Purpose: "Handle requests"},
				},
				SecurityApproach: tt.security,
				Observability: ObservabilityPlanJSON{
					Logging: "slog",
					Metrics: "Prometheus",
					Tracing: "OpenTelemetry",
				},
				Deployment: DeploymentPlanJSON{
					Development: "Docker",
					Staging:     "K8s",
					Production:  "K8s",
				},
			}

			err := ValidateArchitectureJSON(arch)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestValidateArchitectureJSON_MissingObservabilityFields(t *testing.T) {
	tests := []struct {
		name          string
		observability ObservabilityPlanJSON
		expectedError string
	}{
		{
			name: "missing logging",
			observability: ObservabilityPlanJSON{
				Logging: "",
				Metrics: "Prometheus",
				Tracing: "OpenTelemetry",
			},
			expectedError: "observability.logging is required",
		},
		{
			name: "missing metrics",
			observability: ObservabilityPlanJSON{
				Logging: "slog",
				Metrics: "",
				Tracing: "OpenTelemetry",
			},
			expectedError: "observability.metrics is required",
		},
		{
			name: "missing tracing",
			observability: ObservabilityPlanJSON{
				Logging: "slog",
				Metrics: "Prometheus",
				Tracing: "",
			},
			expectedError: "observability.tracing is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arch := &ArchitectureJSON{
				SystemOverview: "A system",
				Components: []ComponentJSON{
					{Name: "API", Type: "backend", Purpose: "Handle requests"},
				},
				SecurityApproach: SecurityPlanJSON{
					Authentication: "JWT",
					Authorization:  "RBAC",
					Encryption:     "TLS",
					Audit:          "Logs",
				},
				Observability: tt.observability,
				Deployment: DeploymentPlanJSON{
					Development: "Docker",
					Staging:     "K8s",
					Production:  "K8s",
				},
			}

			err := ValidateArchitectureJSON(arch)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestValidateArchitectureJSON_MissingDeploymentFields(t *testing.T) {
	tests := []struct {
		name          string
		deployment    DeploymentPlanJSON
		expectedError string
	}{
		{
			name: "missing development",
			deployment: DeploymentPlanJSON{
				Development: "",
				Staging:     "K8s",
				Production:  "K8s",
			},
			expectedError: "deployment.development is required",
		},
		{
			name: "missing staging",
			deployment: DeploymentPlanJSON{
				Development: "Docker",
				Staging:     "",
				Production:  "K8s",
			},
			expectedError: "deployment.staging is required",
		},
		{
			name: "missing production",
			deployment: DeploymentPlanJSON{
				Development: "Docker",
				Staging:     "K8s",
				Production:  "",
			},
			expectedError: "deployment.production is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arch := &ArchitectureJSON{
				SystemOverview: "A system",
				Components: []ComponentJSON{
					{Name: "API", Type: "backend", Purpose: "Handle requests"},
				},
				SecurityApproach: SecurityPlanJSON{
					Authentication: "JWT",
					Authorization:  "RBAC",
					Encryption:     "TLS",
					Audit:          "Logs",
				},
				Observability: ObservabilityPlanJSON{
					Logging: "slog",
					Metrics: "Prometheus",
					Tracing: "OpenTelemetry",
				},
				Deployment: tt.deployment,
			}

			err := ValidateArchitectureJSON(arch)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestValidateArchitectureJSON_MultipleErrors(t *testing.T) {
	arch := &ArchitectureJSON{
		SystemOverview: "",
		Components:     []ComponentJSON{},
		SecurityApproach: SecurityPlanJSON{
			Authentication: "",
			Authorization:  "",
			Encryption:     "",
			Audit:          "",
		},
		Observability: ObservabilityPlanJSON{
			Logging: "",
			Metrics: "",
			Tracing: "",
		},
		Deployment: DeploymentPlanJSON{
			Development: "",
			Staging:     "",
			Production:  "",
		},
	}

	err := ValidateArchitectureJSON(arch)
	require.Error(t, err)

	// Should contain multiple error messages
	assert.Contains(t, err.Error(), "system_overview is required")
	assert.Contains(t, err.Error(), "components array is required")
	assert.Contains(t, err.Error(), "security_approach.authentication is required")
	assert.Contains(t, err.Error(), "observability.logging is required")
	assert.Contains(t, err.Error(), "deployment.development is required")
}

func TestConvertToArchitecture_CompleteConversion(t *testing.T) {
	jsonArch := &ArchitectureJSON{
		SystemOverview: "A task management system",
		Components: []ComponentJSON{
			{
				Name:         "API Server",
				Type:         "backend",
				Purpose:      "Handle HTTP requests",
				Technologies: []string{"Go", "Gin"},
				Dependencies: []string{"Database", "Cache"},
			},
			{
				Name:         "Frontend",
				Type:         "frontend",
				Purpose:      "User interface",
				Technologies: []string{"React", "TypeScript"},
				Dependencies: []string{"API Server"},
			},
		},
		DataFlows: []DataFlowJSON{
			{
				Name:        "User Login",
				Description: "User authentication flow",
				Steps: []FlowStepJSON{
					{Order: 1, Component: "Frontend", Action: "Submit credentials", Description: "User enters username and password"},
					{Order: 2, Component: "API Server", Action: "Validate credentials", Description: "Check against database"},
					{Order: 3, Component: "API Server", Action: "Generate token", Description: "Create JWT token"},
				},
				Diagram: "Frontend -> API Server -> Database",
			},
		},
		TechRationale: map[string]string{
			"Go":    "High performance and concurrency",
			"React": "Rich ecosystem and component reusability",
		},
		ScalingStrategy: ScalingPlanJSON{
			HorizontalScaling: "Add more API server instances",
			VerticalScaling:   "Increase CPU and memory",
			Caching:           "Redis for session data",
			LoadBalancing:     "Nginx load balancer",
			DatabaseScaling:   "Read replicas",
		},
		APIContract: APISpecJSON{
			RESTEndpoints: []EndpointJSON{
				{Method: "POST", Path: "/api/login", Description: "User login", Request: "credentials", Response: "token"},
				{Method: "GET", Path: "/api/tasks", Description: "List tasks", Request: "none", Response: "task list"},
			},
			WebSockets: []WebSocketEventJSON{
				{Name: "task.updated", Direction: "server->client", Description: "Task was updated", Payload: "task object"},
			},
			Authentication: "JWT tokens",
		},
		DatabaseSchema: SchemaJSON{
			Tables: []TableJSON{
				{
					Name:        "users",
					Description: "User accounts",
					Columns: []ColumnJSON{
						{Name: "id", Type: "INTEGER", Constraints: "PRIMARY KEY"},
						{Name: "username", Type: "TEXT", Constraints: "UNIQUE NOT NULL"},
						{Name: "password_hash", Type: "TEXT", Constraints: "NOT NULL"},
					},
				},
			},
			Relationships: []RelationshipJSON{
				{From: "tasks", To: "users", Type: "many-to-one"},
			},
		},
		SecurityApproach: SecurityPlanJSON{
			Authentication: "JWT tokens with refresh tokens",
			Authorization:  "RBAC with role hierarchy",
			Encryption:     "TLS 1.3 for transport, AES-256 for data at rest",
			Audit:          "Structured audit logs with retention policy",
		},
		Observability: ObservabilityPlanJSON{
			Logging: "Structured logs with slog, centralized in ELK",
			Metrics: "Prometheus with Grafana dashboards",
			Tracing: "OpenTelemetry with Jaeger backend",
		},
		Deployment: DeploymentPlanJSON{
			Development: "Docker Compose with hot reload",
			Staging:     "Kubernetes on GKE with staging namespace",
			Production:  "Kubernetes on GKE with HA and auto-scaling",
		},
		Risks: []RiskJSON{
			{Name: "Database bottleneck", Probability: "medium", Impact: "high", Mitigation: "Implement caching and read replicas"},
			{Name: "API rate limiting", Probability: "low", Impact: "medium", Mitigation: "Implement rate limiting per user"},
		},
		Assumptions: []string{"Users have modern browsers", "Internet connectivity is reliable"},
		Unknowns:    []string{"Exact user load patterns", "Peak concurrent users"},
	}

	projectID := "test-project-123"
	arch := ConvertToArchitecture(jsonArch, projectID)

	// Verify basic fields
	assert.Equal(t, projectID, arch.ProjectID)
	assert.Equal(t, "A task management system", arch.SystemOverview)

	// Verify components
	require.Len(t, arch.Components, 2)
	assert.Equal(t, "API Server", arch.Components[0].Name)
	assert.Equal(t, ComponentBackend, arch.Components[0].Type)
	assert.Equal(t, "Handle HTTP requests", arch.Components[0].Purpose)
	assert.Equal(t, []string{"Go", "Gin"}, arch.Components[0].Technologies)
	assert.Equal(t, []string{"Database", "Cache"}, arch.Components[0].Dependencies)

	// Verify data flows
	require.Len(t, arch.DataFlows, 1)
	assert.Equal(t, "User Login", arch.DataFlows[0].Name)
	assert.Equal(t, "User authentication flow", arch.DataFlows[0].Description)
	require.Len(t, arch.DataFlows[0].Steps, 3)
	assert.Equal(t, 1, arch.DataFlows[0].Steps[0].Order)
	assert.Equal(t, "Frontend", arch.DataFlows[0].Steps[0].Component)

	// Verify tech rationale
	assert.Equal(t, "High performance and concurrency", arch.TechRationale["Go"])
	assert.Equal(t, "Rich ecosystem and component reusability", arch.TechRationale["React"])

	// Verify scaling strategy
	assert.Equal(t, "Add more API server instances", arch.ScalingStrategy.HorizontalScaling)
	assert.Equal(t, "Redis for session data", arch.ScalingStrategy.Caching)

	// Verify API contract
	require.Len(t, arch.APIContract.RESTEndpoints, 2)
	assert.Equal(t, "POST", arch.APIContract.RESTEndpoints[0].Method)
	assert.Equal(t, "/api/login", arch.APIContract.RESTEndpoints[0].Path)
	require.Len(t, arch.APIContract.WebSockets, 1)
	assert.Equal(t, "task.updated", arch.APIContract.WebSockets[0].Name)

	// Verify database schema
	require.Len(t, arch.DatabaseSchema.Tables, 1)
	assert.Equal(t, "users", arch.DatabaseSchema.Tables[0].Name)
	require.Len(t, arch.DatabaseSchema.Tables[0].Columns, 3)
	assert.Equal(t, "id", arch.DatabaseSchema.Tables[0].Columns[0].Name)
	require.Len(t, arch.DatabaseSchema.Relationships, 1)
	assert.Equal(t, "tasks", arch.DatabaseSchema.Relationships[0].From)

	// Verify security approach
	assert.Equal(t, "JWT tokens with refresh tokens", arch.SecurityApproach.Authentication)
	assert.Equal(t, "RBAC with role hierarchy", arch.SecurityApproach.Authorization)

	// Verify observability
	assert.Contains(t, arch.Observability.Logging, "slog")
	assert.Contains(t, arch.Observability.Metrics, "Prometheus")

	// Verify deployment
	assert.Contains(t, arch.Deployment.Development, "Docker Compose")
	assert.Contains(t, arch.Deployment.Production, "Kubernetes")

	// Verify risks
	require.Len(t, arch.Risks, 2)
	assert.Equal(t, "Database bottleneck", arch.Risks[0].Name)
	assert.Equal(t, RiskMedium, arch.Risks[0].Probability)
	assert.Equal(t, RiskHigh, arch.Risks[0].Impact)

	// Verify assumptions and unknowns
	assert.Equal(t, jsonArch.Assumptions, arch.Assumptions)
	assert.Equal(t, jsonArch.Unknowns, arch.Unknowns)
}

func TestConvertToArchitecture_EmptyOptionalFields(t *testing.T) {
	jsonArch := &ArchitectureJSON{
		SystemOverview: "Minimal system",
		Components: []ComponentJSON{
			{Name: "API", Type: "backend", Purpose: "Handle requests"},
		},
		DataFlows:     []DataFlowJSON{},
		TechRationale: map[string]string{},
		ScalingStrategy: ScalingPlanJSON{
			HorizontalScaling: "Scale horizontally",
			VerticalScaling:   "Scale vertically",
			Caching:           "Use cache",
			LoadBalancing:     "Use LB",
			DatabaseScaling:   "Scale DB",
		},
		APIContract: APISpecJSON{
			RESTEndpoints:  []EndpointJSON{},
			WebSockets:     []WebSocketEventJSON{},
			Authentication: "JWT",
		},
		DatabaseSchema: SchemaJSON{
			Tables:        []TableJSON{},
			Relationships: []RelationshipJSON{},
		},
		SecurityApproach: SecurityPlanJSON{
			Authentication: "JWT",
			Authorization:  "RBAC",
			Encryption:     "TLS",
			Audit:          "Logs",
		},
		Observability: ObservabilityPlanJSON{
			Logging: "slog",
			Metrics: "Prometheus",
			Tracing: "OpenTelemetry",
		},
		Deployment: DeploymentPlanJSON{
			Development: "Docker",
			Staging:     "K8s",
			Production:  "K8s",
		},
		Risks:       []RiskJSON{},
		Assumptions: []string{},
		Unknowns:    []string{},
	}

	arch := ConvertToArchitecture(jsonArch, "test-project")

	assert.NotNil(t, arch)
	assert.Equal(t, "Minimal system", arch.SystemOverview)
	assert.Len(t, arch.Components, 1)
	assert.Len(t, arch.DataFlows, 0)
	assert.Len(t, arch.Risks, 0)
	assert.Len(t, arch.Assumptions, 0)
	assert.Len(t, arch.Unknowns, 0)
}

func TestArchitectureJSON_JSONMarshaling(t *testing.T) {
	arch := &ArchitectureJSON{
		SystemOverview: "Test system",
		Components: []ComponentJSON{
			{
				Name:         "API",
				Type:         "backend",
				Purpose:      "Handle requests",
				Technologies: []string{"Go"},
				Dependencies: []string{"DB"},
			},
		},
		SecurityApproach: SecurityPlanJSON{
			Authentication: "JWT",
			Authorization:  "RBAC",
			Encryption:     "TLS",
			Audit:          "Logs",
		},
		Observability: ObservabilityPlanJSON{
			Logging: "slog",
			Metrics: "Prometheus",
			Tracing: "OpenTelemetry",
		},
		Deployment: DeploymentPlanJSON{
			Development: "Docker",
			Staging:     "K8s",
			Production:  "K8s",
		},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(arch)
	require.NoError(t, err)

	// Unmarshal back
	var unmarshaled ArchitectureJSON
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	// Verify round-trip
	assert.Equal(t, arch.SystemOverview, unmarshaled.SystemOverview)
	assert.Equal(t, arch.Components[0].Name, unmarshaled.Components[0].Name)
	assert.Equal(t, arch.SecurityApproach.Authentication, unmarshaled.SecurityApproach.Authentication)
}

func TestArchitectureJSON_JSONUnmarshalingFromString(t *testing.T) {
	jsonStr := `{
		"system_overview": "A web application",
		"components": [
			{
				"name": "Web Server",
				"type": "backend",
				"purpose": "Serve HTTP requests",
				"technologies": ["Node.js", "Express"],
				"dependencies": ["Database"]
			}
		],
		"data_flows": [],
		"tech_rationale": {},
		"scaling_strategy": {
			"horizontal_scaling": "Add instances",
			"vertical_scaling": "Increase resources",
			"caching": "Redis",
			"load_balancing": "Nginx",
			"database_scaling": "Sharding"
		},
		"api_contract": {
			"rest_endpoints": [],
			"authentication": "OAuth2"
		},
		"database_schema": {
			"tables": []
		},
		"security_approach": {
			"authentication": "OAuth2",
			"authorization": "ACL",
			"encryption": "TLS 1.3",
			"audit": "Audit logs"
		},
		"observability": {
			"logging": "Winston",
			"metrics": "StatsD",
			"tracing": "Zipkin"
		},
		"deployment": {
			"development": "Local",
			"staging": "AWS EC2",
			"production": "AWS ECS"
		},
		"risks": [],
		"assumptions": [],
		"unknowns": []
	}`

	var arch ArchitectureJSON
	err := json.Unmarshal([]byte(jsonStr), &arch)
	require.NoError(t, err)

	assert.Equal(t, "A web application", arch.SystemOverview)
	assert.Len(t, arch.Components, 1)
	assert.Equal(t, "Web Server", arch.Components[0].Name)
	assert.Equal(t, "backend", arch.Components[0].Type)
	assert.Equal(t, []string{"Node.js", "Express"}, arch.Components[0].Technologies)
	assert.Equal(t, "OAuth2", arch.SecurityApproach.Authentication)
	assert.Equal(t, "Redis", arch.ScalingStrategy.Caching)
}

func TestParseArchitectureJSON_DirectJSON(t *testing.T) {
	jsonStr := `{
		"system_overview": "A task management system",
		"components": [
			{
				"name": "API Server",
				"type": "backend",
				"purpose": "Handle HTTP requests",
				"technologies": ["Go", "Gin"],
				"dependencies": ["Database"]
			}
		],
		"data_flows": [],
		"tech_rationale": {},
		"scaling_strategy": {
			"horizontal_scaling": "Add instances",
			"vertical_scaling": "Increase resources",
			"caching": "Redis",
			"load_balancing": "Nginx",
			"database_scaling": "Sharding"
		},
		"api_contract": {
			"rest_endpoints": [],
			"authentication": "JWT"
		},
		"database_schema": {
			"tables": []
		},
		"security_approach": {
			"authentication": "JWT tokens",
			"authorization": "RBAC",
			"encryption": "TLS 1.3",
			"audit": "Structured logging"
		},
		"observability": {
			"logging": "slog",
			"metrics": "Prometheus",
			"tracing": "OpenTelemetry"
		},
		"deployment": {
			"development": "Docker Compose",
			"staging": "Kubernetes",
			"production": "Kubernetes with HA"
		},
		"risks": [],
		"assumptions": [],
		"unknowns": []
	}`

	projectID := "test-project-123"
	arch, err := parseArchitectureJSON(jsonStr, projectID)

	require.NoError(t, err)
	assert.NotNil(t, arch)
	assert.Equal(t, projectID, arch.ProjectID)
	assert.Equal(t, "A task management system", arch.SystemOverview)
	assert.Len(t, arch.Components, 1)
	assert.Equal(t, "API Server", arch.Components[0].Name)
	assert.Equal(t, "JWT tokens", arch.SecurityApproach.Authentication)
}

func TestParseArchitectureJSON_MarkdownCodeFenceJSON(t *testing.T) {
	response := "```json\n" + `{
		"system_overview": "A web application",
		"components": [
			{
				"name": "Web Server",
				"type": "backend",
				"purpose": "Serve requests",
				"technologies": ["Node.js"],
				"dependencies": []
			}
		],
		"data_flows": [],
		"tech_rationale": {},
		"scaling_strategy": {
			"horizontal_scaling": "Scale out",
			"vertical_scaling": "Scale up",
			"caching": "Redis",
			"load_balancing": "Nginx",
			"database_scaling": "Replicas"
		},
		"api_contract": {
			"rest_endpoints": [],
			"authentication": "OAuth2"
		},
		"database_schema": {
			"tables": []
		},
		"security_approach": {
			"authentication": "OAuth2",
			"authorization": "ACL",
			"encryption": "TLS",
			"audit": "Logs"
		},
		"observability": {
			"logging": "Winston",
			"metrics": "StatsD",
			"tracing": "Zipkin"
		},
		"deployment": {
			"development": "Local",
			"staging": "AWS",
			"production": "AWS"
		},
		"risks": [],
		"assumptions": [],
		"unknowns": []
	}` + "\n```"

	projectID := "test-project-456"
	arch, err := parseArchitectureJSON(response, projectID)

	require.NoError(t, err)
	assert.NotNil(t, arch)
	assert.Equal(t, projectID, arch.ProjectID)
	assert.Equal(t, "A web application", arch.SystemOverview)
	assert.Len(t, arch.Components, 1)
	assert.Equal(t, "Web Server", arch.Components[0].Name)
	assert.Equal(t, "OAuth2", arch.SecurityApproach.Authentication)
}

func TestParseArchitectureJSON_GenericCodeFence(t *testing.T) {
	response := "```\n" + `{
		"system_overview": "A mobile app backend",
		"components": [
			{
				"name": "API Gateway",
				"type": "backend",
				"purpose": "Route requests",
				"technologies": ["Kong"],
				"dependencies": []
			}
		],
		"data_flows": [],
		"tech_rationale": {},
		"scaling_strategy": {
			"horizontal_scaling": "Auto-scale",
			"vertical_scaling": "Manual scale",
			"caching": "Memcached",
			"load_balancing": "HAProxy",
			"database_scaling": "Sharding"
		},
		"api_contract": {
			"rest_endpoints": [],
			"authentication": "API Keys"
		},
		"database_schema": {
			"tables": []
		},
		"security_approach": {
			"authentication": "API Keys",
			"authorization": "Scopes",
			"encryption": "TLS 1.3",
			"audit": "CloudWatch"
		},
		"observability": {
			"logging": "CloudWatch",
			"metrics": "CloudWatch",
			"tracing": "X-Ray"
		},
		"deployment": {
			"development": "Docker",
			"staging": "ECS",
			"production": "ECS"
		},
		"risks": [],
		"assumptions": [],
		"unknowns": []
	}` + "\n```"

	projectID := "test-project-789"
	arch, err := parseArchitectureJSON(response, projectID)

	require.NoError(t, err)
	assert.NotNil(t, arch)
	assert.Equal(t, projectID, arch.ProjectID)
	assert.Equal(t, "A mobile app backend", arch.SystemOverview)
	assert.Len(t, arch.Components, 1)
	assert.Equal(t, "API Gateway", arch.Components[0].Name)
}

func TestParseArchitectureJSON_InvalidJSON(t *testing.T) {
	invalidJSON := `{
		"system_overview": "Invalid JSON",
		"components": [
			{
				"name": "API"
				"type": "backend"
			}
		]
	}`

	_, err := parseArchitectureJSON(invalidJSON, "test-project")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestParseArchitectureJSON_MissingRequiredFields(t *testing.T) {
	jsonStr := `{
		"system_overview": "",
		"components": [],
		"data_flows": [],
		"tech_rationale": {},
		"scaling_strategy": {
			"horizontal_scaling": "Scale",
			"vertical_scaling": "Scale",
			"caching": "Cache",
			"load_balancing": "LB",
			"database_scaling": "Scale"
		},
		"api_contract": {
			"rest_endpoints": [],
			"authentication": "JWT"
		},
		"database_schema": {
			"tables": []
		},
		"security_approach": {
			"authentication": "",
			"authorization": "",
			"encryption": "",
			"audit": ""
		},
		"observability": {
			"logging": "",
			"metrics": "",
			"tracing": ""
		},
		"deployment": {
			"development": "",
			"staging": "",
			"production": ""
		},
		"risks": [],
		"assumptions": [],
		"unknowns": []
	}`

	_, err := parseArchitectureJSON(jsonStr, "test-project")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid architecture data")
	assert.Contains(t, err.Error(), "system_overview is required")
}

func TestParseArchitectureJSON_NoJSON(t *testing.T) {
	response := "This is just plain text without any JSON content."

	_, err := parseArchitectureJSON(response, "test-project")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestParseArchitectureJSON_MarkdownWithoutJSON(t *testing.T) {
	response := "```\nThis is not JSON\nJust some text\n```"

	_, err := parseArchitectureJSON(response, "test-project")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestParseArchitectureJSON_ComplexArchitecture(t *testing.T) {
	jsonStr := `{
		"system_overview": "A comprehensive e-commerce platform",
		"components": [
			{
				"name": "API Gateway",
				"type": "backend",
				"purpose": "Route and authenticate requests",
				"technologies": ["Kong", "Lua"],
				"dependencies": ["Auth Service", "Product Service"]
			},
			{
				"name": "Product Service",
				"type": "backend",
				"purpose": "Manage product catalog",
				"technologies": ["Go", "PostgreSQL"],
				"dependencies": ["Database"]
			},
			{
				"name": "Frontend",
				"type": "frontend",
				"purpose": "User interface",
				"technologies": ["React", "TypeScript", "Redux"],
				"dependencies": ["API Gateway"]
			}
		],
		"data_flows": [
			{
				"name": "Product Search",
				"description": "User searches for products",
				"steps": [
					{
						"order": 1,
						"component": "Frontend",
						"action": "Submit search query",
						"description": "User enters search term"
					},
					{
						"order": 2,
						"component": "API Gateway",
						"action": "Route request",
						"description": "Forward to Product Service"
					},
					{
						"order": 3,
						"component": "Product Service",
						"action": "Query database",
						"description": "Search products"
					}
				],
				"diagram": "Frontend -> API Gateway -> Product Service -> Database"
			}
		],
		"tech_rationale": {
			"Go": "High performance and excellent concurrency",
			"React": "Rich ecosystem and component model",
			"PostgreSQL": "ACID compliance and JSON support"
		},
		"scaling_strategy": {
			"horizontal_scaling": "Kubernetes auto-scaling based on CPU and memory",
			"vertical_scaling": "Increase pod resources for database",
			"caching": "Redis for session data and product catalog",
			"load_balancing": "Kubernetes service load balancing",
			"database_scaling": "Read replicas and connection pooling"
		},
		"api_contract": {
			"rest_endpoints": [
				{
					"method": "GET",
					"path": "/api/products",
					"description": "List products",
					"request": "query parameters: page, limit, search",
					"response": "paginated product list"
				},
				{
					"method": "POST",
					"path": "/api/orders",
					"description": "Create order",
					"request": "order details",
					"response": "order confirmation"
				}
			],
			"websockets": [
				{
					"name": "order.status",
					"direction": "server->client",
					"description": "Order status updates",
					"payload": "order status object"
				}
			],
			"authentication": "JWT tokens with refresh tokens"
		},
		"database_schema": {
			"tables": [
				{
					"name": "products",
					"description": "Product catalog",
					"columns": [
						{
							"name": "id",
							"type": "UUID",
							"constraints": "PRIMARY KEY"
						},
						{
							"name": "name",
							"type": "VARCHAR(255)",
							"constraints": "NOT NULL"
						},
						{
							"name": "price",
							"type": "DECIMAL(10,2)",
							"constraints": "NOT NULL"
						}
					]
				},
				{
					"name": "orders",
					"description": "Customer orders",
					"columns": [
						{
							"name": "id",
							"type": "UUID",
							"constraints": "PRIMARY KEY"
						},
						{
							"name": "user_id",
							"type": "UUID",
							"constraints": "NOT NULL REFERENCES users(id)"
						},
						{
							"name": "total",
							"type": "DECIMAL(10,2)",
							"constraints": "NOT NULL"
						}
					]
				}
			],
			"relationships": [
				{
					"from": "orders",
					"to": "users",
					"type": "many-to-one"
				},
				{
					"from": "order_items",
					"to": "products",
					"type": "many-to-one"
				}
			]
		},
		"security_approach": {
			"authentication": "JWT tokens with RS256 signing",
			"authorization": "RBAC with role hierarchy (admin, seller, customer)",
			"encryption": "TLS 1.3 for transport, AES-256-GCM for data at rest",
			"audit": "Structured audit logs with user actions and timestamps"
		},
		"observability": {
			"logging": "Structured logs with slog, centralized in ELK stack",
			"metrics": "Prometheus with Grafana dashboards for business and technical metrics",
			"tracing": "OpenTelemetry with Jaeger for distributed tracing"
		},
		"deployment": {
			"development": "Docker Compose with hot reload and local databases",
			"staging": "Kubernetes on GKE with staging namespace and test data",
			"production": "Kubernetes on GKE with HA, auto-scaling, and multi-zone deployment"
		},
		"risks": [
			{
				"name": "Database bottleneck during peak traffic",
				"probability": "medium",
				"impact": "high",
				"mitigation": "Implement caching, read replicas, and connection pooling"
			},
			{
				"name": "Payment gateway downtime",
				"probability": "low",
				"impact": "critical",
				"mitigation": "Use multiple payment providers with automatic failover"
			}
		],
		"assumptions": [
			"Users have modern browsers with JavaScript enabled",
			"Payment gateway has 99.9% uptime SLA",
			"Average order size is under 20 items"
		],
		"unknowns": [
			"Exact peak traffic patterns during sales events",
			"User behavior on mobile vs desktop",
			"International shipping requirements"
		]
	}`

	projectID := "ecommerce-project"
	arch, err := parseArchitectureJSON(jsonStr, projectID)

	require.NoError(t, err)
	assert.NotNil(t, arch)
	assert.Equal(t, projectID, arch.ProjectID)
	assert.Equal(t, "A comprehensive e-commerce platform", arch.SystemOverview)

	// Verify components
	assert.Len(t, arch.Components, 3)
	assert.Equal(t, "API Gateway", arch.Components[0].Name)
	assert.Equal(t, "Product Service", arch.Components[1].Name)
	assert.Equal(t, "Frontend", arch.Components[2].Name)

	// Verify data flows
	assert.Len(t, arch.DataFlows, 1)
	assert.Equal(t, "Product Search", arch.DataFlows[0].Name)
	assert.Len(t, arch.DataFlows[0].Steps, 3)

	// Verify tech rationale
	assert.Equal(t, "High performance and excellent concurrency", arch.TechRationale["Go"])

	// Verify API contract
	assert.Len(t, arch.APIContract.RESTEndpoints, 2)
	assert.Equal(t, "GET", arch.APIContract.RESTEndpoints[0].Method)
	assert.Equal(t, "/api/products", arch.APIContract.RESTEndpoints[0].Path)
	assert.Len(t, arch.APIContract.WebSockets, 1)

	// Verify database schema
	assert.Len(t, arch.DatabaseSchema.Tables, 2)
	assert.Equal(t, "products", arch.DatabaseSchema.Tables[0].Name)
	assert.Len(t, arch.DatabaseSchema.Tables[0].Columns, 3)
	assert.Len(t, arch.DatabaseSchema.Relationships, 2)

	// Verify risks
	assert.Len(t, arch.Risks, 2)
	assert.Equal(t, "Database bottleneck during peak traffic", arch.Risks[0].Name)
	assert.Equal(t, RiskMedium, arch.Risks[0].Probability)
	assert.Equal(t, RiskHigh, arch.Risks[0].Impact)

	// Verify assumptions and unknowns
	assert.Len(t, arch.Assumptions, 3)
	assert.Len(t, arch.Unknowns, 3)
}

func TestExtractJSONFromMarkdown_JSONCodeFence(t *testing.T) {
	markdown := "```json\n{\"key\": \"value\"}\n```"
	result := extractJSONFromMarkdown(markdown)
	assert.Equal(t, `{"key": "value"}`, result)
}

func TestExtractJSONFromMarkdown_GenericCodeFence(t *testing.T) {
	markdown := "```\n{\"key\": \"value\"}\n```"
	result := extractJSONFromMarkdown(markdown)
	assert.Equal(t, `{"key": "value"}`, result)
}

func TestExtractJSONFromMarkdown_NoCodeFence(t *testing.T) {
	markdown := "Just plain text"
	result := extractJSONFromMarkdown(markdown)
	assert.Equal(t, "", result)
}

func TestExtractJSONFromMarkdown_NonJSONInCodeFence(t *testing.T) {
	markdown := "```\nThis is not JSON\n```"
	result := extractJSONFromMarkdown(markdown)
	assert.Equal(t, "", result)
}

func TestExtractJSONFromMarkdown_MultilineJSON(t *testing.T) {
	markdown := "```json\n{\n  \"key1\": \"value1\",\n  \"key2\": \"value2\"\n}\n```"
	result := extractJSONFromMarkdown(markdown)
	expected := "{\n  \"key1\": \"value1\",\n  \"key2\": \"value2\"\n}"
	assert.Equal(t, expected, result)
}

func TestExtractJSONFromMarkdown_WithSurroundingText(t *testing.T) {
	markdown := "Here is the architecture:\n\n```json\n{\"system\": \"test\"}\n```\n\nThat's it!"
	result := extractJSONFromMarkdown(markdown)
	assert.Equal(t, `{"system": "test"}`, result)
}

func TestParseArchitectureWithFallback_PartialValid(t *testing.T) {
	// Partial architecture - missing some fields but has valid JSON structure
	response := `{
		"system_overview": "Test system",
		"components": [
			{
				"name": "Component 1",
				"type": "service",
				"purpose": "Test purpose"
			}
		]
	}`

	arch, warnings, err := parseArchitectureWithFallback(response, "test-project")

	if err == nil {
		t.Errorf("expected error for partial architecture, got nil")
	}

	if arch == nil {
		t.Fatal("expected partial architecture, got nil")
	}

	if len(warnings) == 0 {
		t.Errorf("expected warnings for partial architecture, got none")
	}

	if arch.SystemOverview == "" {
		t.Error("expected system_overview to be set")
	}

	if len(arch.Components) == 0 {
		t.Error("expected components to be set")
	}
}

func TestParseArchitectureWithFallback_CompletelyInvalid(t *testing.T) {
	// Completely invalid JSON
	response := `not valid json at all`

	arch, warnings, err := parseArchitectureWithFallback(response, "test-project")

	if err == nil {
		t.Errorf("expected error for invalid JSON, got nil")
	}

	if arch != nil {
		t.Error("expected nil architecture for invalid JSON")
	}

	if warnings != nil {
		t.Error("expected nil warnings for invalid JSON")
	}
}

func TestValidateArchitectureJSONPartial_SomeFieldsMissing(t *testing.T) {
	// Architecture with some fields missing
	arch := &ArchitectureJSON{
		Components: []ComponentJSON{
			{Name: "Test Component", Type: "service", Purpose: "Test purpose"},
		},
	}

	errors := validateArchitectureJSONPartial(arch)

	if len(errors) == 0 {
		t.Error("expected validation errors for missing fields")
	}

	// Should have errors for missing required fields
	hasSystemOverviewError := false
	hasSecurityError := false
	for _, err := range errors {
		if strings.Contains(err, "system_overview") {
			hasSystemOverviewError = true
		}
		if strings.Contains(err, "security") {
			hasSecurityError = true
		}
	}

	if !hasSystemOverviewError {
		t.Error("expected error for missing system_overview")
	}

	if !hasSecurityError {
		t.Error("expected error for missing security fields")
	}
}
