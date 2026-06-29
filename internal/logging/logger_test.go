package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelInfo, &buf)

	require.NotNil(t, logger)
	require.NotNil(t, logger.slog)
}

func TestLogger_Debug(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelDebug, &buf)

	logger.Debug("debug message", "key", "value")

	output := buf.String()
	assert.Contains(t, output, "debug message")
	assert.Contains(t, output, "key")
	assert.Contains(t, output, "value")
	assert.Contains(t, output, `"level":"DEBUG"`)
}

func TestLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelInfo, &buf)

	logger.Info("info message", "provider", "openai", "tokens", 100)

	output := buf.String()
	assert.Contains(t, output, "info message")
	assert.Contains(t, output, "provider")
	assert.Contains(t, output, "openai")
	assert.Contains(t, output, "tokens")
	assert.Contains(t, output, `"level":"INFO"`)
}

func TestLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelWarn, &buf)

	logger.Warn("warning message", "reason", "rate limit")

	output := buf.String()
	assert.Contains(t, output, "warning message")
	assert.Contains(t, output, "reason")
	assert.Contains(t, output, "rate limit")
	assert.Contains(t, output, `"level":"WARN"`)
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelError, &buf)

	logger.Error("error message", "error", "connection failed", "component", "database")

	output := buf.String()
	assert.Contains(t, output, "error message")
	assert.Contains(t, output, "error")
	assert.Contains(t, output, "connection failed")
	assert.Contains(t, output, "component")
	assert.Contains(t, output, "database")
	assert.Contains(t, output, `"level":"ERROR"`)
}

func TestLogger_With(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelInfo, &buf)

	// Create a contextual logger with component name
	contextLogger := logger.With("component", "task_executor", "task_id", "123")

	contextLogger.Info("task started")
	contextLogger.Info("task completed")

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.Len(t, lines, 2)

	// Both log entries should have the contextual fields
	for _, line := range lines {
		assert.Contains(t, line, "component")
		assert.Contains(t, line, "task_executor")
		assert.Contains(t, line, "task_id")
		assert.Contains(t, line, "123")
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelWarn, &buf)

	// Debug and Info should be filtered out
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	assert.NotContains(t, output, "debug message")
	assert.NotContains(t, output, "info message")
	assert.Contains(t, output, "warn message")
	assert.Contains(t, output, "error message")
}

func TestLogger_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelInfo, &buf)

	logger.Info("test message", "key1", "value1", "key2", 42)

	// Verify output is valid JSON
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "test message", logEntry["msg"])
	assert.Equal(t, "INFO", logEntry["level"])
	assert.Equal(t, "value1", logEntry["key1"])
	assert.Equal(t, float64(42), logEntry["key2"]) // JSON numbers are float64
}

func TestSanitizeSensitive_OpenAIKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "OpenAI API key",
			input:    "Using API key: sk-1234567890abcdefghijklmnopqrstuvwxyz",
			expected: "Using API key: [REDACTED]",
		},
		{
			name:     "Multiple OpenAI keys",
			input:    "Keys: sk-abc123def456ghi789jkl012mno345pqr and sk-xyz987wvu654tsr321qpo098nml765kji",
			expected: "Keys: [REDACTED] and [REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeSensitive(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeSensitive_GenericAPIKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "api_key pattern",
			input:    "Config: api_key_1234567890abcdefghijklmnop",
			expected: "Config: [REDACTED]",
		},
		{
			name:     "api-key pattern",
			input:    "Header: api-key-abcdefghijklmnopqrstuvwxyz123456",
			expected: "Header: [REDACTED]",
		},
		{
			name:     "apikey pattern",
			input:    "Auth: apikey_zyxwvutsrqponmlkjihgfedcba987654321",
			expected: "Auth: [REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeSensitive(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeSensitive_BearerTokens(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Bearer token",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expected: "Authorization: [REDACTED]",
		},
		{
			name:     "Bearer with special chars",
			input:    "Token: Bearer abc-def_ghi.jkl~mno+pqr/stu=",
			expected: "Token: [REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeSensitive(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeSensitive_GoogleAPIKeys(t *testing.T) {
	input := "Google key: AIzaSyAbCdEfGhIjKlMnOpQrStUvWxYz01234567890"
	expected := "Google key: [REDACTED]"

	result := SanitizeSensitive(input)
	assert.Equal(t, expected, result)
}

func TestSanitizeSensitive_UUIDs(t *testing.T) {
	input := "API Key: 550e8400-e29b-41d4-a716-446655440000"
	expected := "API Key: [REDACTED]"

	result := SanitizeSensitive(input)
	assert.Equal(t, expected, result)
}

func TestSanitizeSensitive_EnvironmentVariables(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "API_KEY with colon",
			input:    "API_KEY: sk-1234567890abcdefghij",
			expected: "API_KEY: [REDACTED]",
		},
		{
			name:     "api_key with equals",
			input:    "api_key=sk-abcdefghijklmnopqrst",
			expected: "api_key=[REDACTED]",
		},
		{
			name:     "TOKEN with colon",
			input:    "TOKEN: bearer_token_12345678901234567890",
			expected: "TOKEN: [REDACTED]",
		},
		{
			name:     "secret with equals",
			input:    "secret=my_secret_value_here",
			expected: "secret=[REDACTED]",
		},
		{
			name:     "password with colon",
			input:    "password: my_password_123",
			expected: "password: [REDACTED]",
		},
		{
			name:     "case insensitive",
			input:    "API_KEY: value, Token: value2",
			expected: "API_KEY: [REDACTED], Token: [REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeSensitive(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeSensitive_LongAlphanumeric(t *testing.T) {
	// Long alphanumeric strings (32+ chars) are likely keys
	input := "Key: abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	result := SanitizeSensitive(input)
	assert.Contains(t, result, "[REDACTED]")
	assert.NotContains(t, result, "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG")
}

func TestSanitizeSensitive_NoSensitiveData(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "Normal log message",
			input: "Processing task 123 for user john",
		},
		{
			name:  "Short strings",
			input: "Status: ok, Count: 42",
		},
		{
			name:  "File paths",
			input: "Writing to /home/user/project/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeSensitive(tt.input)
			// Should remain unchanged
			assert.Equal(t, tt.input, result)
		})
	}
}

func TestSanitizeSensitive_MixedContent(t *testing.T) {
	input := "API call to OpenAI with key sk-abc123def456ghi789jkl012mno345pqr completed in 2.3s with 150 tokens"
	result := SanitizeSensitive(input)

	assert.Contains(t, result, "[REDACTED]")
	assert.NotContains(t, result, "sk-abc123def456ghi789jkl012mno345pqr")
	assert.Contains(t, result, "API call to OpenAI")
	assert.Contains(t, result, "completed in 2.3s")
	assert.Contains(t, result, "150 tokens")
}

func TestLogger_APICallLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelInfo, &buf)

	// Simulate API call logging with sensitive data
	apiKey := "sk-1234567890abcdefghijklmnopqrstuvwxyz"
	sanitizedKey := SanitizeSensitive(apiKey)

	logger.Info("API call completed",
		"provider", "openai",
		"model", "gpt-4",
		"api_key", sanitizedKey,
		"tokens_input", 1500,
		"tokens_output", 800,
		"duration_ms", 2340,
	)

	output := buf.String()
	assert.Contains(t, output, "API call completed")
	assert.Contains(t, output, "openai")
	assert.Contains(t, output, "gpt-4")
	assert.Contains(t, output, "[REDACTED]")
	assert.NotContains(t, output, "sk-1234567890abcdefghijklmnopqrstuvwxyz")
	assert.Contains(t, output, "1500")
	assert.Contains(t, output, "800")
}

func TestLogger_ErrorLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelError, &buf)

	logger.Error("Database connection failed",
		"component", "state_store",
		"operation", "connect",
		"error", "connection refused",
		"database", "/path/to/db.sqlite",
	)

	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "Database connection failed", logEntry["msg"])
	assert.Equal(t, "ERROR", logEntry["level"])
	assert.Equal(t, "state_store", logEntry["component"])
	assert.Equal(t, "connect", logEntry["operation"])
	assert.Equal(t, "connection refused", logEntry["error"])
	assert.Equal(t, "/path/to/db.sqlite", logEntry["database"])
}

func TestLogger_ContextualLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelInfo, &buf)

	// Create a contextual logger for a specific component
	taskLogger := logger.With(
		"component", "task_executor",
		"project_id", "proj-123",
		"task_id", "task-456",
	)

	taskLogger.Info("Starting task execution")
	taskLogger.Info("Writing file", "path", "/project/file.go")
	taskLogger.Info("Task completed", "duration_ms", 1234)

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.Len(t, lines, 3)

	// All entries should have the contextual fields
	for _, line := range lines {
		var entry map[string]interface{}
		err := json.Unmarshal([]byte(line), &entry)
		require.NoError(t, err)

		assert.Equal(t, "task_executor", entry["component"])
		assert.Equal(t, "proj-123", entry["project_id"])
		assert.Equal(t, "task-456", entry["task_id"])
	}
}

func TestLogger_AutomaticScrubbing(t *testing.T) {
	t.Run("ScrubsAPIKeyInAttribute", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewLogger(slog.LevelInfo, &buf)

		// Pass a raw API key as an attribute value - should be automatically redacted
		logger.Info("API call",
			"api_key", "sk-1234567890abcdefghijklmnopqrstuvwxyz",
			"provider", "openai",
		)

		output := buf.String()
		assert.Contains(t, output, "[REDACTED]")
		assert.NotContains(t, output, "sk-1234567890abcdefghijklmnopqrstuvwxyz")
		assert.Contains(t, output, "openai")
	})

	t.Run("ScrubsBearerTokenInAttribute", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewLogger(slog.LevelInfo, &buf)

		logger.Info("Request sent",
			"authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test",
		)

		output := buf.String()
		assert.Contains(t, output, "[REDACTED]")
		assert.NotContains(t, output, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9")
	})

	t.Run("ScrubsInMessageText", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewLogger(slog.LevelInfo, &buf)

		// The message itself also goes through ReplaceAttr as slog.KindString
		logger.Info("Using key sk-abcdefghij1234567890klmnopqr for request")

		output := buf.String()
		assert.NotContains(t, output, "sk-abcdefghij1234567890klmnopqr")
		assert.Contains(t, output, "[REDACTED]")
	})

	t.Run("PreservesNonSensitiveAttributes", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewLogger(slog.LevelInfo, &buf)

		logger.Info("Processing task",
			"task_id", "task-123",
			"phase", "develop",
			"tokens", 500,
		)

		output := buf.String()
		assert.Contains(t, output, "task-123")
		assert.Contains(t, output, "develop")
		assert.Contains(t, output, "500")
	})

	t.Run("ScrubsWithContextLogger", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewLogger(slog.LevelInfo, &buf)

		// Even contextual loggers (via With) should scrub
		contextLogger := logger.With("secret_key", "sk-contextkey12345678901234567890abc")
		contextLogger.Info("doing work")

		output := buf.String()
		assert.NotContains(t, output, "sk-contextkey12345678901234567890abc")
		assert.Contains(t, output, "[REDACTED]")
	})

	t.Run("ScrubsEnvVarPatterns", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewLogger(slog.LevelInfo, &buf)

		logger.Info("Config loaded",
			"config_line", "API_KEY=super_secret_value_123",
		)

		output := buf.String()
		assert.NotContains(t, output, "super_secret_value_123")
		assert.Contains(t, output, "[REDACTED]")
	})
}
