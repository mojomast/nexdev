package observability

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestLoggerRedactsMessageAndAttrs(t *testing.T) {
	var buf bytes.Buffer
	logger, err := NewLogger(&buf, Options{Level: slog.LevelInfo, Format: FormatJSON})
	if err != nil {
		t.Fatalf("NewLogger returned error: %v", err)
	}

	logger.Info("starting with api_key=sk-1234567890abcdef", "token", "Bearer abc.def-ghi_jkl")

	output := buf.String()
	if strings.Contains(output, "sk-1234567890abcdef") || strings.Contains(output, "abc.def-ghi_jkl") {
		t.Fatalf("log output leaked secret: %s", output)
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Fatalf("log output did not include redaction marker: %s", output)
	}
}

func TestLoggerRedactsWithAttrsAndGroups(t *testing.T) {
	var buf bytes.Buffer
	logger, err := NewLogger(&buf, Options{Level: slog.LevelInfo, Format: FormatJSON})
	if err != nil {
		t.Fatalf("NewLogger returned error: %v", err)
	}

	logger.With("provider", "anthropic", "auth", slog.GroupValue(slog.String("header", "Bearer abc.def-ghi"))).Info("provider call")

	output := buf.String()
	if strings.Contains(output, "abc.def-ghi") {
		t.Fatalf("log output leaked grouped secret: %s", output)
	}
	if !strings.Contains(output, "Bearer [REDACTED]") {
		t.Fatalf("log output did not redact grouped attr: %s", output)
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger, err := NewLogger(&buf, Options{Level: slog.LevelWarn, Format: FormatJSON})
	if err != nil {
		t.Fatalf("NewLogger returned error: %v", err)
	}

	logger.Info("info message")
	logger.Warn("warn message")

	output := buf.String()
	if strings.Contains(output, "info message") {
		t.Fatalf("info log was not filtered: %s", output)
	}
	if !strings.Contains(output, "warn message") {
		t.Fatalf("warn log was filtered unexpectedly: %s", output)
	}
}

func TestLoggerJSONModeCanBeConstructed(t *testing.T) {
	var buf bytes.Buffer
	logger, err := NewLogger(&buf, Options{Level: slog.LevelInfo, Format: FormatJSON})
	if err != nil {
		t.Fatalf("NewLogger returned error: %v", err)
	}

	logger.Info("json message", ProjectID("proj_123"))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("json log did not unmarshal: %v\n%s", err, buf.String())
	}
	if entry[FieldProjectID] != "proj_123" {
		t.Fatalf("project_id attr mismatch: %#v", entry)
	}
}

func TestLoggerTextModeCanBeConstructed(t *testing.T) {
	var buf bytes.Buffer
	logger, err := NewLogger(&buf, Options{Level: slog.LevelInfo, Format: FormatText})
	if err != nil {
		t.Fatalf("NewLogger returned error: %v", err)
	}

	logger.Info("text message", RunID("run_123"))

	output := buf.String()
	if !strings.Contains(output, "text message") || !strings.Contains(output, `run_id=run_123`) {
		t.Fatalf("text log output missing expected content: %s", output)
	}
}

func TestFieldHelpersUseExpectedKeys(t *testing.T) {
	tests := []struct {
		attr slog.Attr
		key  string
	}{
		{ProjectID("value"), "project_id"},
		{RunID("value"), "run_id"},
		{Stage("value"), "stage"},
		{TaskID("value"), "task_id"},
		{Provider("value"), "provider"},
		{Model("value"), "model"},
		{EventID("value"), "event_id"},
		{RequestID("value"), "request_id"},
	}

	for _, tt := range tests {
		if tt.attr.Key != tt.key {
			t.Fatalf("helper key mismatch: got %q want %q", tt.attr.Key, tt.key)
		}
		if tt.attr.Value.String() != "value" {
			t.Fatalf("helper value mismatch for %s: %q", tt.key, tt.attr.Value.String())
		}
	}
}

func TestParseLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"":        slog.LevelInfo,
		"info":    slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
	}

	for input, want := range tests {
		got, err := ParseLevel(input)
		if err != nil {
			t.Fatalf("ParseLevel(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("ParseLevel(%q) = %v, want %v", input, got, want)
		}
	}
}
