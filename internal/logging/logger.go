package logging

import (
	"io"
	"log/slog"
	"regexp"
	"strings"
)

// Pre-compiled regex patterns for sensitive data detection.
// These are compiled once at package init time for performance,
// since SanitizeSensitive is now called on every log attribute.
var (
	sensitivePatterns = []*regexp.Regexp{
		regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`),                                          // OpenAI style keys
		regexp.MustCompile(`api[_-]?key[_-]?[a-zA-Z0-9]{20,}`),                             // Generic API keys
		regexp.MustCompile(`Bearer\s+[a-zA-Z0-9\-._~+/]+=*`),                               // Bearer tokens
		regexp.MustCompile(`[a-zA-Z0-9]{32,}`),                                             // Long alphanumeric strings (likely keys)
		regexp.MustCompile(`AIza[a-zA-Z0-9\-_]{35}`),                                       // Google API keys
		regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`), // UUIDs
	}
	sensitiveEnvVarPattern = regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password)\s*[:=]\s*[^\s,;]+`)
)

// Logger wraps slog.Logger to provide structured logging throughout the application
type Logger struct {
	slog *slog.Logger
}

// NewLogger creates a new Logger with the specified level and output writer.
// All string attribute values are automatically scrubbed for sensitive data
// (API keys, tokens, passwords) before being written to the output.
func NewLogger(level slog.Level, output io.Writer) *Logger {
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Value.Kind() == slog.KindString {
				a.Value = slog.StringValue(SanitizeSensitive(a.Value.String()))
			}
			return a
		},
	})
	return &Logger{
		slog: slog.New(handler),
	}
}

// Debug logs a debug-level message with optional key-value pairs
func (l *Logger) Debug(msg string, args ...any) {
	l.slog.Debug(msg, args...)
}

// Info logs an info-level message with optional key-value pairs
func (l *Logger) Info(msg string, args ...any) {
	l.slog.Info(msg, args...)
}

// Warn logs a warning-level message with optional key-value pairs
func (l *Logger) Warn(msg string, args ...any) {
	l.slog.Warn(msg, args...)
}

// Error logs an error-level message with optional key-value pairs
func (l *Logger) Error(msg string, args ...any) {
	l.slog.Error(msg, args...)
}

// With returns a new Logger with the given key-value pairs added to all log entries
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		slog: l.slog.With(args...),
	}
}

// SanitizeSensitive redacts sensitive information like API keys from log output.
// It replaces API keys and other sensitive patterns with [REDACTED].
// Uses pre-compiled package-level regexes for performance.
func SanitizeSensitive(value string) string {
	result := value
	for _, re := range sensitivePatterns {
		result = re.ReplaceAllString(result, "[REDACTED]")
	}

	// Also redact common environment variable patterns
	result = sensitiveEnvVarPattern.ReplaceAllStringFunc(result, func(match string) string {
		parts := strings.SplitN(match, ":", 2)
		if len(parts) == 2 {
			return parts[0] + ": [REDACTED]"
		}
		parts = strings.SplitN(match, "=", 2)
		if len(parts) == 2 {
			return parts[0] + "=[REDACTED]"
		}
		return "[REDACTED]"
	})

	return result
}
