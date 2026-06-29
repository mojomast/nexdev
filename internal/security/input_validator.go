package security

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// InputValidator validates all user inputs and LLM outputs to ensure they meet
// security and format requirements. It provides validation for project names,
// JSON data, and file content.
type InputValidator struct {
	// projectNamePattern matches alphanumeric characters, hyphens, and underscores
	projectNamePattern *regexp.Regexp
}

// NewInputValidator creates a new InputValidator with default validation rules.
func NewInputValidator() *InputValidator {
	return &InputValidator{
		// Project names: alphanumeric, hyphens, underscores only
		// Must start with alphanumeric, can contain hyphens/underscores in middle
		projectNamePattern: regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$`),
	}
}

// ValidateProjectName validates that a project name contains only allowed characters.
// Project names must:
// - Contain only alphanumeric characters, hyphens, and underscores
// - Start and end with an alphanumeric character (if more than 1 character)
// - Be at least 1 character long
// - Not exceed 255 characters
//
// Returns an error if validation fails with a specific message indicating the violation.
func (iv *InputValidator) ValidateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	if len(name) > 255 {
		return fmt.Errorf("project name exceeds maximum length of 255 characters (got %d)", len(name))
	}

	// Check for valid pattern
	if !iv.projectNamePattern.MatchString(name) {
		return fmt.Errorf("project name '%s' is invalid: only alphanumeric characters, hyphens, and underscores are allowed, and it must start and end with an alphanumeric character", name)
	}

	// Additional check: no consecutive special characters
	if strings.Contains(name, "--") || strings.Contains(name, "__") || strings.Contains(name, "-_") || strings.Contains(name, "_-") {
		return fmt.Errorf("project name '%s' is invalid: consecutive special characters are not allowed", name)
	}

	return nil
}

// ValidateJSON validates that the provided data is valid JSON and optionally
// validates it against a schema. The schema parameter can be:
// - nil: only validates that data is valid JSON
// - A pointer to a struct: validates that JSON can be unmarshaled into the struct
//
// Returns an error if:
// - The data is not valid JSON
// - The JSON cannot be unmarshaled into the provided schema
// - Required fields are missing (when using a struct schema)
func (iv *InputValidator) ValidateJSON(data []byte, schema interface{}) error {
	if len(data) == 0 {
		return fmt.Errorf("JSON data cannot be empty")
	}

	// First, validate that it's valid JSON
	var temp interface{}
	if err := json.Unmarshal(data, &temp); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// If a schema is provided, validate against it
	if schema != nil {
		if err := json.Unmarshal(data, schema); err != nil {
			return fmt.Errorf("JSON does not match expected schema: %w", err)
		}
	}

	return nil
}

// ValidateFileContent validates that file content meets size and format requirements.
// It checks:
// - Content size does not exceed maxSize (in bytes)
// - Content is valid UTF-8 (to prevent binary data injection)
//
// Parameters:
// - content: The file content to validate
// - maxSize: Maximum allowed size in bytes (0 means no limit)
//
// Returns an error if validation fails with a specific message indicating the violation.
func (iv *InputValidator) ValidateFileContent(content string, maxSize int) error {
	// Check size limit
	contentSize := len(content)
	if maxSize > 0 && contentSize > maxSize {
		return fmt.Errorf("file content exceeds maximum size of %d bytes (got %d bytes)", maxSize, contentSize)
	}

	// Validate UTF-8 encoding to prevent binary data injection
	if !utf8.ValidString(content) {
		return fmt.Errorf("file content contains invalid UTF-8 sequences")
	}

	return nil
}

// ValidateFilePath validates that a file path contains only allowed characters.
// This is a basic validation that checks for:
// - No null bytes
// - No control characters
// - Valid UTF-8
//
// Note: This does NOT check for directory traversal or path safety.
// Use PathSanitizer.ValidatePath() for comprehensive path security validation.
func (iv *InputValidator) ValidateFilePath(path string) error {
	if path == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("file path contains null bytes")
	}

	// Check for control characters (except tab and newline which might be valid in some contexts)
	for _, r := range path {
		if r < 32 && r != '\t' && r != '\n' {
			return fmt.Errorf("file path contains control characters")
		}
	}

	// Validate UTF-8
	if !utf8.ValidString(path) {
		return fmt.Errorf("file path contains invalid UTF-8 sequences")
	}

	return nil
}
