package design

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mojomast/nexdev/internal/common"
)

// ArchitectureJSON represents the JSON schema for architecture parsing from LLM responses
type ArchitectureJSON struct {
	SystemOverview   string                `json:"system_overview"`
	Components       []ComponentJSON       `json:"components"`
	DataFlows        []DataFlowJSON        `json:"data_flows"`
	TechRationale    map[string]string     `json:"tech_rationale"`
	ScalingStrategy  ScalingPlanJSON       `json:"scaling_strategy"`
	APIContract      APISpecJSON           `json:"api_contract"`
	DatabaseSchema   SchemaJSON            `json:"database_schema"`
	SecurityApproach SecurityPlanJSON      `json:"security_approach"`
	Observability    ObservabilityPlanJSON `json:"observability"`
	Deployment       DeploymentPlanJSON    `json:"deployment"`
	Risks            []RiskJSON            `json:"risks"`
	Assumptions      []string              `json:"assumptions"`
	Unknowns         []string              `json:"unknowns"`
}

// ComponentJSON represents a system component in JSON format
type ComponentJSON struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Purpose      string   `json:"purpose"`
	Technologies []string `json:"technologies"`
	Dependencies []string `json:"dependencies"`
}

// DataFlowJSON represents a data flow in JSON format
type DataFlowJSON struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Steps       []FlowStepJSON `json:"steps"`
	Diagram     string         `json:"diagram,omitempty"`
}

// FlowStepJSON represents a step in a data flow in JSON format
type FlowStepJSON struct {
	Order       int    `json:"order"`
	Component   string `json:"component"`
	Action      string `json:"action"`
	Description string `json:"description"`
}

// ScalingPlanJSON describes how the system scales in JSON format
type ScalingPlanJSON struct {
	HorizontalScaling string `json:"horizontal_scaling"`
	VerticalScaling   string `json:"vertical_scaling"`
	Caching           string `json:"caching"`
	LoadBalancing     string `json:"load_balancing"`
	DatabaseScaling   string `json:"database_scaling"`
}

// APISpecJSON describes the API contract in JSON format
type APISpecJSON struct {
	RESTEndpoints  []EndpointJSON       `json:"rest_endpoints"`
	WebSockets     []WebSocketEventJSON `json:"websockets,omitempty"`
	Authentication string               `json:"authentication"`
}

// EndpointJSON represents a REST endpoint in JSON format
type EndpointJSON struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Description string `json:"description"`
	Request     string `json:"request,omitempty"`
	Response    string `json:"response,omitempty"`
}

// WebSocketEventJSON represents a WebSocket event in JSON format
type WebSocketEventJSON struct {
	Name        string `json:"name"`
	Direction   string `json:"direction"`
	Description string `json:"description"`
	Payload     string `json:"payload,omitempty"`
}

// SchemaJSON represents the database schema in JSON format
type SchemaJSON struct {
	Tables        []TableJSON        `json:"tables"`
	Relationships []RelationshipJSON `json:"relationships,omitempty"`
}

// TableJSON represents a database table in JSON format
type TableJSON struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Columns     []ColumnJSON `json:"columns"`
}

// ColumnJSON represents a table column in JSON format
type ColumnJSON struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Constraints string `json:"constraints,omitempty"`
}

// RelationshipJSON represents a table relationship in JSON format
type RelationshipJSON struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

// SecurityPlanJSON describes the security approach in JSON format
type SecurityPlanJSON struct {
	Authentication string `json:"authentication"`
	Authorization  string `json:"authorization"`
	Encryption     string `json:"encryption"`
	Audit          string `json:"audit"`
}

// ObservabilityPlanJSON describes the observability strategy in JSON format
type ObservabilityPlanJSON struct {
	Logging string `json:"logging"`
	Metrics string `json:"metrics"`
	Tracing string `json:"tracing"`
}

// DeploymentPlanJSON describes the deployment architecture in JSON format
type DeploymentPlanJSON struct {
	Development string `json:"development"`
	Staging     string `json:"staging"`
	Production  string `json:"production"`
}

// RiskJSON represents a potential risk in JSON format
type RiskJSON struct {
	Name        string `json:"name"`
	Probability string `json:"probability"`
	Impact      string `json:"impact"`
	Mitigation  string `json:"mitigation"`
}

// ValidateArchitectureJSON validates that all required fields are present in the parsed JSON
func ValidateArchitectureJSON(arch *ArchitectureJSON) error {
	var errors []string

	// Validate required fields
	if arch.SystemOverview == "" {
		errors = append(errors, "system_overview is required")
	}

	if len(arch.Components) == 0 {
		errors = append(errors, "components array is required and must not be empty")
	}

	// Validate each component has required fields
	for i, comp := range arch.Components {
		if comp.Name == "" {
			errors = append(errors, fmt.Sprintf("component[%d].name is required", i))
		}
		if comp.Type == "" {
			errors = append(errors, fmt.Sprintf("component[%d].type is required", i))
		}
		if comp.Purpose == "" {
			errors = append(errors, fmt.Sprintf("component[%d].purpose is required", i))
		}
	}

	// Validate security approach has required fields
	if arch.SecurityApproach.Authentication == "" {
		errors = append(errors, "security_approach.authentication is required")
	}
	if arch.SecurityApproach.Authorization == "" {
		errors = append(errors, "security_approach.authorization is required")
	}
	if arch.SecurityApproach.Encryption == "" {
		errors = append(errors, "security_approach.encryption is required")
	}
	if arch.SecurityApproach.Audit == "" {
		errors = append(errors, "security_approach.audit is required")
	}

	// Validate observability has required fields
	if arch.Observability.Logging == "" {
		errors = append(errors, "observability.logging is required")
	}
	if arch.Observability.Metrics == "" {
		errors = append(errors, "observability.metrics is required")
	}
	if arch.Observability.Tracing == "" {
		errors = append(errors, "observability.tracing is required")
	}

	// Validate deployment has required fields
	if arch.Deployment.Development == "" {
		errors = append(errors, "deployment.development is required")
	}
	if arch.Deployment.Staging == "" {
		errors = append(errors, "deployment.staging is required")
	}
	if arch.Deployment.Production == "" {
		errors = append(errors, "deployment.production is required")
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(errors, "; "))
	}

	return nil
}

// ConvertToArchitecture converts ArchitectureJSON to Architecture struct
func ConvertToArchitecture(jsonArch *ArchitectureJSON, projectID string) *Architecture {
	arch := &Architecture{
		ProjectID:      projectID,
		SystemOverview: jsonArch.SystemOverview,
		Components:     make([]Component, len(jsonArch.Components)),
		DataFlows:      make([]DataFlow, len(jsonArch.DataFlows)),
		TechRationale:  jsonArch.TechRationale,
		ScalingStrategy: ScalingPlan{
			HorizontalScaling: jsonArch.ScalingStrategy.HorizontalScaling,
			VerticalScaling:   jsonArch.ScalingStrategy.VerticalScaling,
			Caching:           jsonArch.ScalingStrategy.Caching,
			LoadBalancing:     jsonArch.ScalingStrategy.LoadBalancing,
			DatabaseScaling:   jsonArch.ScalingStrategy.DatabaseScaling,
		},
		APIContract: APISpec{
			RESTEndpoints:  make([]Endpoint, len(jsonArch.APIContract.RESTEndpoints)),
			WebSockets:     make([]WebSocketEvent, len(jsonArch.APIContract.WebSockets)),
			Authentication: jsonArch.APIContract.Authentication,
		},
		DatabaseSchema: Schema{
			Tables:        make([]Table, len(jsonArch.DatabaseSchema.Tables)),
			Relationships: make([]Relationship, len(jsonArch.DatabaseSchema.Relationships)),
		},
		SecurityApproach: SecurityPlan{
			Authentication: jsonArch.SecurityApproach.Authentication,
			Authorization:  jsonArch.SecurityApproach.Authorization,
			Encryption:     jsonArch.SecurityApproach.Encryption,
			Audit:          jsonArch.SecurityApproach.Audit,
		},
		Observability: ObservabilityPlan{
			Logging: jsonArch.Observability.Logging,
			Metrics: jsonArch.Observability.Metrics,
			Tracing: jsonArch.Observability.Tracing,
		},
		Deployment: DeploymentPlan{
			Development: jsonArch.Deployment.Development,
			Staging:     jsonArch.Deployment.Staging,
			Production:  jsonArch.Deployment.Production,
		},
		Risks:       make([]Risk, len(jsonArch.Risks)),
		Assumptions: jsonArch.Assumptions,
		Unknowns:    jsonArch.Unknowns,
	}

	// Convert components
	for i, comp := range jsonArch.Components {
		arch.Components[i] = Component{
			Name:         comp.Name,
			Type:         ComponentType(comp.Type),
			Purpose:      comp.Purpose,
			Technologies: comp.Technologies,
			Dependencies: comp.Dependencies,
		}
	}

	// Convert data flows
	for i, flow := range jsonArch.DataFlows {
		steps := make([]FlowStep, len(flow.Steps))
		for j, step := range flow.Steps {
			steps[j] = FlowStep{
				Order:       step.Order,
				Component:   step.Component,
				Action:      step.Action,
				Description: step.Description,
			}
		}
		arch.DataFlows[i] = DataFlow{
			Name:        flow.Name,
			Description: flow.Description,
			Steps:       steps,
			Diagram:     flow.Diagram,
		}
	}

	// Convert REST endpoints
	for i, endpoint := range jsonArch.APIContract.RESTEndpoints {
		arch.APIContract.RESTEndpoints[i] = Endpoint{
			Method:      endpoint.Method,
			Path:        endpoint.Path,
			Description: endpoint.Description,
			Request:     endpoint.Request,
			Response:    endpoint.Response,
		}
	}

	// Convert WebSocket events
	for i, ws := range jsonArch.APIContract.WebSockets {
		arch.APIContract.WebSockets[i] = WebSocketEvent{
			Name:        ws.Name,
			Direction:   ws.Direction,
			Description: ws.Description,
			Payload:     ws.Payload,
		}
	}

	// Convert database tables
	for i, table := range jsonArch.DatabaseSchema.Tables {
		columns := make([]Column, len(table.Columns))
		for j, col := range table.Columns {
			columns[j] = Column{
				Name:        col.Name,
				Type:        col.Type,
				Constraints: col.Constraints,
			}
		}
		arch.DatabaseSchema.Tables[i] = Table{
			Name:        table.Name,
			Description: table.Description,
			Columns:     columns,
		}
	}

	// Convert relationships
	for i, rel := range jsonArch.DatabaseSchema.Relationships {
		arch.DatabaseSchema.Relationships[i] = Relationship{
			From: rel.From,
			To:   rel.To,
			Type: rel.Type,
		}
	}

	// Convert risks
	for i, risk := range jsonArch.Risks {
		arch.Risks[i] = Risk{
			Name:        risk.Name,
			Probability: RiskLevel(risk.Probability),
			Impact:      RiskLevel(risk.Impact),
			Mitigation:  risk.Mitigation,
		}
	}

	return arch
}

// parseArchitectureJSON parses an LLM response into an Architecture struct
// It tries direct JSON parsing first, then attempts to extract JSON from markdown code fences
func parseArchitectureJSON(response string, projectID string) (*Architecture, error) {
	var archData ArchitectureJSON

	// Use common parser
	if err := common.ParseJSON(response, &archData); err != nil {
		return nil, fmt.Errorf("failed to parse architecture JSON: %w", err)
	}

	// Validate all required fields are present
	if err := ValidateArchitectureJSON(&archData); err != nil {
		return nil, fmt.Errorf("invalid architecture data: %w", err)
	}

	// Convert JSON structs to Architecture struct
	return ConvertToArchitecture(&archData, projectID), nil
}

func extractJSONFromMarkdown(markdown string) string {
	extracted, err := common.ExtractJSON(markdown)
	if err != nil || !json.Valid([]byte(extracted)) {
		return ""
	}

	return extracted
}

// parseArchitectureWithFallback parses an LLM response with graceful degradation
// If validation fails, returns a partial architecture with available data
func parseArchitectureWithFallback(response string, projectID string) (*Architecture, []string, error) {
	var archData ArchitectureJSON
	var validationErrors []string

	// Use common parser
	if err := common.ParseJSON(response, &archData); err != nil {
		return nil, nil, fmt.Errorf("failed to parse architecture JSON: %w", err)
	}

	// Validate and collect errors without failing immediately
	validationErrors = validateArchitectureJSONPartial(&archData)

	if len(validationErrors) > 0 {
		// Return partial architecture with validation warnings
		arch := ConvertToArchitecture(&archData, projectID)
		return arch, validationErrors, fmt.Errorf("architecture validation failed with %d error(s), returning partial architecture", len(validationErrors))
	}

	// Fully valid architecture
	arch := ConvertToArchitecture(&archData, projectID)
	return arch, nil, nil
}

// validateArchitectureJSONPartial validates architecture and returns all errors without failing
func validateArchitectureJSONPartial(arch *ArchitectureJSON) []string {
	var errors []string

	// Validate system overview
	if arch.SystemOverview == "" {
		errors = append(errors, "system_overview is missing")
	}

	// Validate components
	if len(arch.Components) == 0 {
		errors = append(errors, "components array is empty")
	} else {
		for i, comp := range arch.Components {
			if comp.Name == "" {
				errors = append(errors, fmt.Sprintf("component[%d].name is missing", i))
			}
			if comp.Type == "" {
				errors = append(errors, fmt.Sprintf("component[%d].type is missing", i))
			}
			if comp.Purpose == "" {
				errors = append(errors, fmt.Sprintf("component[%d].purpose is missing", i))
			}
		}
	}

	// Validate security approach
	if arch.SecurityApproach.Authentication == "" {
		errors = append(errors, "security_approach.authentication is missing")
	}
	if arch.SecurityApproach.Authorization == "" {
		errors = append(errors, "security_approach.authorization is missing")
	}

	// Note: We don't validate all fields strictly for partial parsing
	// The goal is to accept whatever valid data we got

	return errors
}
