package controlplane

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/safety"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

type ErrorResponse struct {
	ErrorCode string         `json:"error_code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details"`
	RequestID string         `json:"request_id"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	for key, value := range details {
		details[key] = scrubErrorDetail(value)
	}
	writeJSON(w, status, ErrorResponse{ErrorCode: code, Message: safety.RedactSecrets(message), Details: details, RequestID: requestID(r)})
}

func scrubErrorDetail(value any) any {
	switch typed := value.(type) {
	case string:
		return safety.RedactSecrets(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = scrubErrorDetail(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = scrubErrorDetail(item)
		}
		return out
	default:
		return value
	}
}

func requestID(r *http.Request) string {
	if r == nil {
		return ""
	}
	if id := strings.TrimSpace(r.Header.Get("X-Request-ID")); id != "" {
		return id
	}
	return "req_" + strings.ReplaceAll(time.Now().UTC().Format("20060102T150405.000000000"), ".", "")
}
