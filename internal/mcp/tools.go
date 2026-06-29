package mcp

import (
	"context"
	"fmt"
	"sync"
)

// ToolHandler is a function that executes a tool
type ToolHandler func(ctx context.Context, arguments map[string]interface{}) (*CallToolResult, error)

// ToolRegistry manages registered tools
type ToolRegistry struct {
	mu       sync.RWMutex
	tools    map[string]Tool
	handlers map[string]ToolHandler
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools:    make(map[string]Tool),
		handlers: make(map[string]ToolHandler),
	}
}

// RegisterTool registers a new tool with its handler
func (r *ToolRegistry) RegisterTool(tool Tool, handler ToolHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[tool.Name]; exists {
		return fmt.Errorf("tool already registered: %s", tool.Name)
	}

	r.tools[tool.Name] = tool
	r.handlers[tool.Name] = handler
	return nil
}

// ListTools returns all registered tools
func (r *ToolRegistry) ListTools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// CallTool executes a tool with the given arguments
func (r *ToolRegistry) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*CallToolResult, error) {
	r.mu.RLock()
	handler, exists := r.handlers[name]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	return handler(ctx, arguments)
}

// Helper function to create text content
func TextContent(text string) Content {
	return Content{
		Type: "text",
		Text: text,
	}
}

// Helper function to create error content
func ErrorContent(message string) Content {
	return Content{
		Type: "text",
		Text: fmt.Sprintf("Error: %s", message),
	}
}

// Helper function to create a success result
func SuccessResult(text string) *CallToolResult {
	return &CallToolResult{
		Content: []Content{TextContent(text)},
		IsError: false,
	}
}

// Helper function to create an error result
func ErrorResult(message string) *CallToolResult {
	return &CallToolResult{
		Content: []Content{ErrorContent(message)},
		IsError: true,
	}
}

// Common input schema builders

// StringParam creates a string parameter schema
func StringParam(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "string",
		"description": description,
	}
}

// NumberParam creates a number parameter schema
func NumberParam(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "number",
		"description": description,
	}
}

// BooleanParam creates a boolean parameter schema
func BooleanParam(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "boolean",
		"description": description,
	}
}

// ObjectParam creates an object parameter schema
func ObjectParam(description string, properties map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":        "object",
		"description": description,
		"properties":  properties,
	}
}

// ArrayParam creates an array parameter schema
func ArrayParam(description string, items map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":        "array",
		"description": description,
		"items":       items,
	}
}

// CreateInputSchema creates a complete input schema
func CreateInputSchema(properties map[string]interface{}, required []string) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func getArg(args map[string]interface{}, paramName string, required bool) (interface{}, bool, error) {
	value, exists := args[paramName]
	if required && !exists {
		return nil, false, fmt.Errorf("required parameter '%s' is missing", paramName)
	}
	if !exists {
		return nil, false, nil
	}
	if value == nil {
		if required {
			return nil, false, fmt.Errorf("required parameter '%s' is null", paramName)
		}
		return nil, false, nil
	}

	return value, true, nil
}

// ValidateAndGetString validates and extracts a string parameter from arguments
func ValidateAndGetString(args map[string]interface{}, paramName string, required bool) (string, error) {
	value, exists, err := getArg(args, paramName, required)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", nil
	}

	strValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("parameter '%s' must be a string, got %T", paramName, value)
	}
	if strValue == "" && required {
		return "", fmt.Errorf("required parameter '%s' cannot be empty", paramName)
	}
	return strValue, nil
}

// ValidateAndGetBool validates and extracts a boolean parameter from arguments
func ValidateAndGetBool(args map[string]interface{}, paramName string, required bool, defaultValue bool) (bool, error) {
	value, exists, err := getArg(args, paramName, required)
	if err != nil {
		return false, err
	}
	if !exists {
		return defaultValue, nil
	}

	boolValue, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("parameter '%s' must be a boolean, got %T", paramName, value)
	}
	return boolValue, nil
}

// ValidateAndGetInt validates and extracts an integer parameter from arguments
func ValidateAndGetInt(args map[string]interface{}, paramName string, required bool, defaultValue int) (int, error) {
	value, exists, err := getArg(args, paramName, required)
	if err != nil {
		return 0, err
	}
	if !exists {
		return defaultValue, nil
	}

	// Handle both int and float64 (JSON numbers are parsed as float64)
	switch v := value.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case int64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("parameter '%s' must be a number, got %T", paramName, value)
	}
}

// ValidateAndGetArray validates and extracts an array parameter from arguments
func ValidateAndGetArray(args map[string]interface{}, paramName string, required bool) ([]interface{}, error) {
	value, exists, err := getArg(args, paramName, required)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	arrayValue, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("parameter '%s' must be an array, got %T", paramName, value)
	}
	return arrayValue, nil
}
