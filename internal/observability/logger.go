package observability

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/mojomast/nexdev/internal/safety"
)

// Format selects the slog handler encoding.
type Format string

const (
	FormatJSON Format = "json"
	FormatText Format = "text"
)

const (
	FieldProjectID = "project_id"
	FieldRunID     = "run_id"
	FieldStage     = "stage"
	FieldTaskID    = "task_id"
	FieldProvider  = "provider"
	FieldModel     = "model"
	FieldEventID   = "event_id"
	FieldRequestID = "request_id"
)

// OpenTelemetry is intentionally disabled in M2. Later M14 work owns SDK setup,
// exporters, trace/metric helpers, and log correlation beyond these field names.
const OpenTelemetryEnabled = false

// Options configures a redacting slog logger.
type Options struct {
	Level  slog.Leveler
	Format Format
}

// NewLogger constructs a slog logger with redaction applied before writes.
func NewLogger(output io.Writer, opts Options) (*slog.Logger, error) {
	if output == nil {
		return nil, fmt.Errorf("observability logger output is nil")
	}
	level := opts.Level
	if level == nil {
		level = slog.LevelInfo
	}

	handlerOpts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	switch opts.Format {
	case "", FormatText:
		handler = slog.NewTextHandler(output, handlerOpts)
	case FormatJSON:
		handler = slog.NewJSONHandler(output, handlerOpts)
	default:
		return nil, fmt.Errorf("unsupported observability log format %q", opts.Format)
	}

	return slog.New(redactingHandler{next: handler}), nil
}

// ParseLevel converts config/CLI level strings into slog levels.
func ParseLevel(level string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug, nil
	case "", "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unsupported observability log level %q", level)
	}
}

func ProjectID(value string) slog.Attr { return slog.String(FieldProjectID, value) }
func RunID(value string) slog.Attr     { return slog.String(FieldRunID, value) }
func Stage(value string) slog.Attr     { return slog.String(FieldStage, value) }
func TaskID(value string) slog.Attr    { return slog.String(FieldTaskID, value) }
func Provider(value string) slog.Attr  { return slog.String(FieldProvider, value) }
func Model(value string) slog.Attr     { return slog.String(FieldModel, value) }
func EventID(value string) slog.Attr   { return slog.String(FieldEventID, value) }
func RequestID(value string) slog.Attr { return slog.String(FieldRequestID, value) }

type redactingHandler struct {
	next slog.Handler
}

func (h redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h redactingHandler) Handle(ctx context.Context, record slog.Record) error {
	redacted := slog.NewRecord(record.Time, record.Level, safety.RedactSecrets(record.Message), record.PC)
	redactedAttrs := make([]slog.Attr, 0, record.NumAttrs())
	record.Attrs(func(attr slog.Attr) bool {
		redactedAttrs = append(redactedAttrs, redactAttr(attr))
		return true
	})
	redacted.AddAttrs(redactedAttrs...)
	return h.next.Handle(ctx, redacted)
}

func (h redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		redacted = append(redacted, redactAttr(attr))
	}
	return redactingHandler{next: h.next.WithAttrs(redacted)}
}

func (h redactingHandler) WithGroup(name string) slog.Handler {
	return redactingHandler{next: h.next.WithGroup(name)}
}

func redactAttr(attr slog.Attr) slog.Attr {
	attr.Value = redactValue(attr.Value)
	return attr
}

func redactValue(value slog.Value) slog.Value {
	if value.Kind() == slog.KindString {
		return slog.StringValue(safety.RedactSecrets(value.String()))
	}
	if value.Kind() == slog.KindGroup {
		attrs := value.Group()
		redacted := make([]slog.Attr, 0, len(attrs))
		for _, attr := range attrs {
			redacted = append(redacted, redactAttr(attr))
		}
		return slog.GroupValue(redacted...)
	}
	return value
}
