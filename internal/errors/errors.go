package errors

import (
	"fmt"
	"strings"
)

// ErrorCategory represents the category of an error
type ErrorCategory string

const (
	// UserError represents errors caused by invalid user input
	UserError ErrorCategory = "user"

	// APIError represents errors from external API calls
	APIError ErrorCategory = "api"

	// SystemError represents internal system errors
	SystemError ErrorCategory = "system"

	// GitError represents Git operation errors
	GitError ErrorCategory = "git"

	// NetworkError represents network connectivity errors
	NetworkError ErrorCategory = "network"
)

// CategorizedError wraps an error with additional context
type CategorizedError struct {
	Category   ErrorCategory
	Err        error
	Message    string
	Suggestion string
	Retryable  bool
	Fatal      bool
	Context    map[string]interface{}
}

// Error implements the error interface
func (e *CategorizedError) Error() string {
	return e.Message
}

// Unwrap returns the underlying error
func (e *CategorizedError) Unwrap() error {
	return e.Err
}

// NewUserError creates a new user error
func NewUserError(err error, message string, suggestion string) *CategorizedError {
	return &CategorizedError{
		Category:   UserError,
		Err:        err,
		Message:    message,
		Suggestion: suggestion,
		Retryable:  false,
		Fatal:      false,
		Context:    make(map[string]interface{}),
	}
}

// NewAPIError creates a new API error
func NewAPIError(err error, message string, retryable bool) *CategorizedError {
	return &CategorizedError{
		Category:   APIError,
		Err:        err,
		Message:    message,
		Suggestion: "Check your network connection and API key configuration",
		Retryable:  retryable,
		Fatal:      false,
		Context:    make(map[string]interface{}),
	}
}

// NewSystemError creates a new system error
func NewSystemError(err error, message string, fatal bool) *CategorizedError {
	return &CategorizedError{
		Category:   SystemError,
		Err:        err,
		Message:    message,
		Suggestion: "This is an internal error. Please report this issue.",
		Retryable:  false,
		Fatal:      fatal,
		Context:    make(map[string]interface{}),
	}
}

// NewGitError creates a new Git error
func NewGitError(err error, message string, suggestion string) *CategorizedError {
	return &CategorizedError{
		Category:   GitError,
		Err:        err,
		Message:    message,
		Suggestion: suggestion,
		Retryable:  false,
		Fatal:      false,
		Context:    make(map[string]interface{}),
	}
}

// NewNetworkError creates a new network error
func NewNetworkError(err error, message string) *CategorizedError {
	return &CategorizedError{
		Category:   NetworkError,
		Err:        err,
		Message:    message,
		Suggestion: "Check your internet connection and try again",
		Retryable:  true,
		Fatal:      false,
		Context:    make(map[string]interface{}),
	}
}

// WithContext adds context to the error
func (e *CategorizedError) WithContext(key string, value interface{}) *CategorizedError {
	e.Context[key] = value
	return e
}

// WithSuggestion sets a custom suggestion
func (e *CategorizedError) WithSuggestion(suggestion string) *CategorizedError {
	e.Suggestion = suggestion
	return e
}

// Categorize attempts to categorize an error
func Categorize(err error) *CategorizedError {
	if err == nil {
		return nil
	}

	// Check if already categorized
	if catErr, ok := err.(*CategorizedError); ok {
		return catErr
	}

	errMsg := err.Error()
	errMsgLower := strings.ToLower(errMsg)

	// Detect API errors
	if strings.Contains(errMsgLower, "rate limit") ||
		strings.Contains(errMsgLower, "quota exceeded") ||
		strings.Contains(errMsgLower, "api key") ||
		strings.Contains(errMsgLower, "unauthorized") ||
		strings.Contains(errMsgLower, "authentication failed") {
		return NewAPIError(err, errMsg, strings.Contains(errMsgLower, "rate limit"))
	}

	// Detect network errors
	if strings.Contains(errMsgLower, "connection refused") ||
		strings.Contains(errMsgLower, "no route to host") ||
		strings.Contains(errMsgLower, "timeout") ||
		strings.Contains(errMsgLower, "network unreachable") ||
		strings.Contains(errMsgLower, "dial tcp") {
		return NewNetworkError(err, errMsg)
	}

	// Detect Git errors
	if strings.Contains(errMsgLower, "git") ||
		strings.Contains(errMsgLower, "merge conflict") ||
		strings.Contains(errMsgLower, "uncommitted changes") {
		suggestion := "Resolve Git conflicts or commit your changes"
		if strings.Contains(errMsgLower, "not a git repository") {
			suggestion = "Initialize a Git repository with 'git init'"
		}
		return NewGitError(err, errMsg, suggestion)
	}

	// Detect user errors
	if strings.Contains(errMsgLower, "invalid") ||
		strings.Contains(errMsgLower, "missing") ||
		strings.Contains(errMsgLower, "required") ||
		strings.Contains(errMsgLower, "not found") ||
		strings.Contains(errMsgLower, "cannot be empty") {
		return NewUserError(err, errMsg, "Check your input and try again")
	}

	// Detect system errors
	if strings.Contains(errMsgLower, "permission denied") ||
		strings.Contains(errMsgLower, "disk full") ||
		strings.Contains(errMsgLower, "no space left") {
		return NewSystemError(err, errMsg, true)
	}

	// Default to system error
	return NewSystemError(err, errMsg, false)
}

// FormatError formats an error for display to the user
func FormatError(err error) string {
	if err == nil {
		return ""
	}

	catErr := Categorize(err)

	var sb strings.Builder

	// Category icon
	switch catErr.Category {
	case UserError:
		sb.WriteString("❌ User Error\n")
	case APIError:
		sb.WriteString("🔌 API Error\n")
	case SystemError:
		sb.WriteString("⚠️  System Error\n")
	case GitError:
		sb.WriteString("📦 Git Error\n")
	case NetworkError:
		sb.WriteString("🌐 Network Error\n")
	}

	sb.WriteString(strings.Repeat("─", 60))
	sb.WriteString("\n\n")

	// Error message
	sb.WriteString(fmt.Sprintf("Error: %s\n\n", catErr.Message))

	// Suggestion
	if catErr.Suggestion != "" {
		sb.WriteString(fmt.Sprintf("💡 Suggestion: %s\n\n", catErr.Suggestion))
	}

	// Retryable
	if catErr.Retryable {
		sb.WriteString("🔄 This operation can be retried\n")
	}

	// Context (if any)
	if len(catErr.Context) > 0 {
		sb.WriteString("\nContext:\n")
		for key, value := range catErr.Context {
			sb.WriteString(fmt.Sprintf("  %s: %v\n", key, value))
		}
	}

	return sb.String()
}

// IsRetryable checks if an error can be retried
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	catErr := Categorize(err)
	return catErr.Retryable
}

// IsFatal checks if an error is fatal
func IsFatal(err error) bool {
	if err == nil {
		return false
	}

	catErr := Categorize(err)
	return catErr.Fatal
}

// IsOfflineCapable checks if an operation can work offline
func IsOfflineCapable(operation string) bool {
	offlineOps := map[string]bool{
		"status":     true,
		"checkpoint": true,
		"rollback":   true,
		"navigate":   true,
		// Operations that require network
		"interview": false,
		"design":    false,
		"plan":      false,
		"review":    false,
		"develop":   false,
		"quota":     false,
	}

	capable, exists := offlineOps[operation]
	if !exists {
		return false
	}

	return capable
}
