package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

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

// StructuredClient adapts the imported geoffrussy Provider.Call(ctx, model,
// prompt) interface into Nexdev's structured output contract.
type StructuredClient struct {
	Router    *Router
	Providers map[string]Provider
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

	for attempt := 0; attempt <= maxRepairAttempts; attempt++ {
		response, err := provider.Call(ctx, route.Model, currentPrompt)
		result.Attempts++
		if err != nil {
			redacted := safety.RedactSecrets(err.Error())
			return result, fmt.Errorf("provider %q structured call failed: %s", route.Provider, redacted)
		}
		if response == nil {
			lastErr = fmt.Errorf("provider %q returned nil response", route.Provider)
			result.ValidationErrors = append(result.ValidationErrors, safety.RedactSecrets(lastErr.Error()))
		} else {
			result.RawResponse = response.Content
			result.Usage = usageFromResponse(response)
			if response.Provider != "" {
				result.Provider = response.Provider
			}
			if response.Model != "" {
				result.Model = response.Model
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
					return result, nil
				}
			} else {
				dstValue.Elem().Set(candidate.Elem())
				return result, nil
			}
		}

		if attempt == maxRepairAttempts {
			break
		}
		currentPrompt = buildRepairPrompt(opts, prompt, result.RawResponse, result.ValidationErrors, attempt+1)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("structured output validation failed")
	}
	return result, fmt.Errorf("structured output failed after %d attempt(s): %s", result.Attempts, safety.RedactSecrets(lastErr.Error()))
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
