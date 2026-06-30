package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/safety"
)

const DefaultMaxRepairAttempts = 2

// SemanticValidator validates a decoded structured output candidate.
type SemanticValidator func(candidate any) error

// StructuredOptions configures a structured provider call.
type StructuredOptions struct {
	MaxRepairAttempts  int
	AllowUnknownFields bool
	Validate           SemanticValidator
	BuildRepairPrompt  func(originalPrompt, rawResponse string, validationErrors []string, attempt int) string
}

// StructuredUsage contains usage metadata exposed by the imported Provider Response.
type StructuredUsage struct {
	PromptTokens       int
	CompletionTokens   int
	TotalTokens        int
	RateLimitRemaining int
	QuotaRemaining     int
	RateLimitInfo      *RateLimitInfo
	QuotaInfo          *QuotaInfo
}

// StructuredResult reports the complete wrapper outcome for audit/event layers.
type StructuredResult struct {
	RawResponse      string
	Attempts         int
	Provider         string
	Model            string
	ValidationErrors []string
	Usage            StructuredUsage
}

// StructuredCallRecord is the provider-wrapper observability handoff. It omits
// prompts and carries only redacted response/error metadata plus usage totals.
type StructuredCallRecord struct {
	Slot             Slot
	Provider         string
	Model            string
	Attempts         int
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	RawResponse      string
	ValidationErrors []string
	Error            string
	StartedAt        time.Time
	CompletedAt      time.Time
}

type StructuredCallRecorder interface {
	RecordStructuredCall(ctx context.Context, record StructuredCallRecord) error
}

// StructuredClient adapts the imported geoffrussy Provider.Call(ctx, model,
// prompt) interface into Nexdev's structured output contract.
type StructuredClient struct {
	Router    *Router
	Providers map[string]Provider
	Recorder  StructuredCallRecorder
}

// CallStructured resolves the slot through Router, calls the selected imported
// Provider, strict-decodes JSON into dst, optionally validates it, and repairs
// decode/validation failures up to the configured cap.
func (c StructuredClient) CallStructured(ctx context.Context, slot Slot, prompt string, dst any, opts StructuredOptions) (*StructuredResult, error) {
	if dst == nil {
		return nil, fmt.Errorf("structured output destination cannot be nil")
	}
	dstValue := reflect.ValueOf(dst)
	if dstValue.Kind() != reflect.Pointer || dstValue.IsNil() {
		return nil, fmt.Errorf("structured output destination must be a non-nil pointer")
	}

	route, err := c.Router.Resolve(slot)
	if err != nil {
		return nil, err
	}
	provider, ok := c.Providers[route.Provider]
	if !ok || provider == nil {
		return nil, fmt.Errorf("resolved provider %q is not available", route.Provider)
	}

	maxRepairAttempts := opts.MaxRepairAttempts
	if maxRepairAttempts < 0 {
		maxRepairAttempts = DefaultMaxRepairAttempts
	}

	result := &StructuredResult{Provider: route.Provider, Model: route.Model}
	currentPrompt := prompt
	var lastErr error
	startedAt := time.Now().UTC()

	for attempt := 0; attempt <= maxRepairAttempts; attempt++ {
		response, err := provider.Call(ctx, route.Model, currentPrompt)
		result.Attempts++
		if err != nil {
			redacted := safety.RedactSecrets(err.Error())
			wrapped := fmt.Errorf("provider %q structured call failed: %s", route.Provider, redacted)
			c.recordStructuredCall(ctx, slot, result, startedAt, wrapped)
			return result, wrapped
		}
		if response == nil {
			lastErr = fmt.Errorf("provider %q returned nil response", route.Provider)
			result.ValidationErrors = append(result.ValidationErrors, safety.RedactSecrets(lastErr.Error()))
		} else {
			result.RawResponse = response.Content
			result.Usage = usageFromResponse(response)
			if response.Provider != "" {
				result.Provider = safety.RedactSecrets(response.Provider)
			}
			if response.Model != "" {
				result.Model = safety.RedactSecrets(response.Model)
			}

			candidate, err := decodeStructured(response.Content, dst, !opts.AllowUnknownFields)
			if err != nil {
				lastErr = err
				result.ValidationErrors = append(result.ValidationErrors, safety.RedactSecrets(err.Error()))
			} else if opts.Validate != nil {
				if err := opts.Validate(candidate.Interface()); err != nil {
					lastErr = err
					result.ValidationErrors = append(result.ValidationErrors, safety.RedactSecrets(err.Error()))
				} else {
					dstValue.Elem().Set(candidate.Elem())
					result.RawResponse = safety.RedactSecrets(result.RawResponse)
					c.recordStructuredCall(ctx, slot, result, startedAt, nil)
					return result, nil
				}
			} else {
				dstValue.Elem().Set(candidate.Elem())
				result.RawResponse = safety.RedactSecrets(result.RawResponse)
				c.recordStructuredCall(ctx, slot, result, startedAt, nil)
				return result, nil
			}
		}

		if attempt == maxRepairAttempts {
			break
		}
		currentPrompt = buildRepairPrompt(opts, prompt, safety.RedactSecrets(result.RawResponse), result.ValidationErrors, attempt+1)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("structured output validation failed")
	}
	result.RawResponse = safety.RedactSecrets(result.RawResponse)
	wrapped := fmt.Errorf("structured output failed after %d attempt(s): %s", result.Attempts, safety.RedactSecrets(lastErr.Error()))
	c.recordStructuredCall(ctx, slot, result, startedAt, wrapped)
	return result, wrapped
}

func (c StructuredClient) recordStructuredCall(ctx context.Context, slot Slot, result *StructuredResult, startedAt time.Time, err error) {
	if c.Recorder == nil || result == nil {
		return
	}
	record := StructuredCallRecord{
		Slot:             slot,
		Provider:         safety.RedactSecrets(result.Provider),
		Model:            safety.RedactSecrets(result.Model),
		Attempts:         result.Attempts,
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
		TotalTokens:      result.Usage.TotalTokens,
		RawResponse:      safety.RedactSecrets(result.RawResponse),
		ValidationErrors: append([]string(nil), result.ValidationErrors...),
		StartedAt:        startedAt,
		CompletedAt:      time.Now().UTC(),
	}
	if err != nil {
		record.Error = safety.RedactSecrets(err.Error())
	}
	_ = c.Recorder.RecordStructuredCall(ctx, record)
}

func decodeStructured(raw string, dst any, rejectUnknown bool) (reflect.Value, error) {
	dstType := reflect.TypeOf(dst)
	candidate := reflect.New(dstType.Elem())

	decoder := json.NewDecoder(strings.NewReader(raw))
	if rejectUnknown {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(candidate.Interface()); err != nil {
		return reflect.Value{}, fmt.Errorf("decode structured JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return reflect.Value{}, fmt.Errorf("decode structured JSON: %w", err)
	}

	return candidate, nil
}

func buildRepairPrompt(opts StructuredOptions, originalPrompt, rawResponse string, validationErrors []string, attempt int) string {
	if opts.BuildRepairPrompt != nil {
		return opts.BuildRepairPrompt(originalPrompt, rawResponse, validationErrors, attempt)
	}

	return fmt.Sprintf("%s\n\nThe previous response did not satisfy the required JSON contract. Return only corrected JSON.\nRepair attempt: %d\nValidation errors:\n- %s\nPrevious raw response:\n%s", originalPrompt, attempt, strings.Join(validationErrors, "\n- "), rawResponse)
}

func usageFromResponse(response *Response) StructuredUsage {
	return StructuredUsage{
		PromptTokens:       response.TokensInput,
		CompletionTokens:   response.TokensOutput,
		TotalTokens:        response.TokensInput + response.TokensOutput,
		RateLimitRemaining: response.RateLimitRemaining,
		QuotaRemaining:     response.QuotaRemaining,
		RateLimitInfo:      response.RateLimitInfo,
		QuotaInfo:          response.QuotaInfo,
	}
}
