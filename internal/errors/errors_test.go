package errors

import (
	"errors"
	"strings"
	"testing"
)

func TestCategorize(t *testing.T) {
	testCases := []struct {
		name              string
		err               error
		expectedCategory  ErrorCategory
		expectedRetryable bool
	}{
		{
			name:              "Rate limit error",
			err:               errors.New("rate limit exceeded"),
			expectedCategory:  APIError,
			expectedRetryable: true,
		},
		{
			name:              "API key error",
			err:               errors.New("invalid API key"),
			expectedCategory:  APIError,
			expectedRetryable: false,
		},
		{
			name:              "Network timeout",
			err:               errors.New("connection timeout"),
			expectedCategory:  NetworkError,
			expectedRetryable: true,
		},
		{
			name:              "Git merge conflict",
			err:               errors.New("merge conflict detected"),
			expectedCategory:  GitError,
			expectedRetryable: false,
		},
		{
			name:              "Invalid input",
			err:               errors.New("invalid argument provided"),
			expectedCategory:  UserError,
			expectedRetryable: false,
		},
		{
			name:              "Permission denied",
			err:               errors.New("permission denied"),
			expectedCategory:  SystemError,
			expectedRetryable: false,
		},
		{
			name:              "Generic error",
			err:               errors.New("something went wrong"),
			expectedCategory:  SystemError,
			expectedRetryable: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			catErr := Categorize(tc.err)

			if catErr == nil {
				t.Fatal("expected categorized error, got nil")
			}

			if catErr.Category != tc.expectedCategory {
				t.Errorf("expected category %s, got %s", tc.expectedCategory, catErr.Category)
			}

			if catErr.Retryable != tc.expectedRetryable {
				t.Errorf("expected retryable=%v, got %v", tc.expectedRetryable, catErr.Retryable)
			}
		})
	}
}

func TestNewUserError(t *testing.T) {
	err := errors.New("base error")
	userErr := NewUserError(err, "Invalid input", "Check your parameters")

	if userErr.Category != UserError {
		t.Errorf("expected category UserError, got %s", userErr.Category)
	}

	if userErr.Retryable {
		t.Error("user errors should not be retryable")
	}

	if userErr.Fatal {
		t.Error("user errors should not be fatal")
	}

	if userErr.Suggestion != "Check your parameters" {
		t.Errorf("expected suggestion, got %s", userErr.Suggestion)
	}
}

func TestNewAPIError(t *testing.T) {
	err := errors.New("API call failed")
	apiErr := NewAPIError(err, "Rate limit exceeded", true)

	if apiErr.Category != APIError {
		t.Errorf("expected category APIError, got %s", apiErr.Category)
	}

	if !apiErr.Retryable {
		t.Error("this API error should be retryable")
	}
}

func TestNewSystemError(t *testing.T) {
	err := errors.New("disk full")
	sysErr := NewSystemError(err, "No space left on device", true)

	if sysErr.Category != SystemError {
		t.Errorf("expected category SystemError, got %s", sysErr.Category)
	}

	if !sysErr.Fatal {
		t.Error("this system error should be fatal")
	}
}

func TestWithContext(t *testing.T) {
	err := errors.New("test error")
	catErr := Categorize(err)

	catErr.WithContext("key1", "value1")
	catErr.WithContext("key2", 42)

	if catErr.Context["key1"] != "value1" {
		t.Error("expected context key1 to be set")
	}

	if catErr.Context["key2"] != 42 {
		t.Error("expected context key2 to be set")
	}
}

func TestFormatError(t *testing.T) {
	err := errors.New("rate limit exceeded")
	formatted := FormatError(err)

	if formatted == "" {
		t.Error("expected formatted error, got empty string")
	}

	// Should contain category indicator
	if !strings.Contains(formatted, "API Error") {
		t.Error("expected formatted error to contain category")
	}

	// Should contain suggestion
	if !strings.Contains(formatted, "Suggestion") {
		t.Error("expected formatted error to contain suggestion")
	}
}

func TestIsRetryable(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "Rate limit error (retryable)",
			err:      errors.New("rate limit exceeded"),
			expected: true,
		},
		{
			name:     "Invalid input (not retryable)",
			err:      errors.New("invalid argument"),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsRetryable(tc.err)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestIsFatal(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "Permission denied (fatal)",
			err:      errors.New("permission denied"),
			expected: true,
		},
		{
			name:     "Network error (not fatal)",
			err:      errors.New("connection timeout"),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsFatal(tc.err)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestIsOfflineCapable(t *testing.T) {
	testCases := []struct {
		operation string
		expected  bool
	}{
		{"status", true},
		{"checkpoint", true},
		{"rollback", true},
		{"navigate", true},
		{"interview", false},
		{"design", false},
		{"develop", false},
		{"unknown", false},
	}

	for _, tc := range testCases {
		t.Run(tc.operation, func(t *testing.T) {
			result := IsOfflineCapable(tc.operation)
			if result != tc.expected {
				t.Errorf("expected %v for %s, got %v", tc.expected, tc.operation, result)
			}
		})
	}
}

func TestCategorizedError_Error(t *testing.T) {
	err := &CategorizedError{
		Message: "test error message",
	}

	if err.Error() != "test error message" {
		t.Errorf("expected 'test error message', got %s", err.Error())
	}
}

func TestCategorizedError_Unwrap(t *testing.T) {
	baseErr := errors.New("base error")
	catErr := &CategorizedError{
		Err: baseErr,
	}

	unwrapped := catErr.Unwrap()
	if unwrapped != baseErr {
		t.Error("expected to unwrap to base error")
	}
}
