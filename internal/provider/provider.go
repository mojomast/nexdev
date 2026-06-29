package provider

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/errors"
	"github.com/mojomast/nexdev/internal/logging"
)

// Provider is the interface that all AI model providers must implement
type Provider interface {
	Name() string
	Authenticate(apiKey string) error
	IsAuthenticated() bool
	ListModels() ([]Model, error)
	DiscoverModels() ([]Model, error) // For dynamic discovery (OpenCode)
	Call(ctx context.Context, model string, prompt string) (*Response, error)
	Stream(ctx context.Context, model string, prompt string) (<-chan string, error)
	GetRateLimitInfo() (*RateLimitInfo, error)
	GetQuotaInfo() (*QuotaInfo, error)
	SupportsCodingPlan() bool // For Z.ai and Kimi
}

// Response represents a response from an AI model provider
type Response struct {
	Content            string
	TokensInput        int
	TokensOutput       int
	Model              string
	Provider           string
	Timestamp          time.Time
	RateLimitRemaining int
	QuotaRemaining     int
	RateLimitInfo      *RateLimitInfo
	QuotaInfo          *QuotaInfo
}

// RateLimitInfo contains rate limiting information from a provider
type RateLimitInfo struct {
	RequestsRemaining int
	RequestsLimit     int
	ResetAt           time.Time
	RetryAfter        time.Duration
}

// QuotaInfo contains quota information from a provider
type QuotaInfo struct {
	TokensRemaining int
	TokensLimit     int
	CostRemaining   float64
	CostLimit       float64
	ResetAt         time.Time
}

// Model represents an AI model
type Model struct {
	Provider     string
	Name         string
	DisplayName  string
	Capabilities []string
	PriceInput   float64 // per 1K tokens
	PriceOutput  float64 // per 1K tokens
}

// BaseProvider provides common functionality for all providers
type BaseProvider struct {
	name          string
	apiKey        string
	authenticated bool
	maxRetries    int
	baseDelay     time.Duration
	logger        *logging.Logger
}

// NewBaseProvider creates a new base provider
func NewBaseProvider(name string) *BaseProvider {
	return &BaseProvider{
		name:       name,
		maxRetries: 3,
		baseDelay:  time.Second,
		logger:     logging.NewLogger(slog.LevelInfo, os.Stdout),
	}
}

// SetLogger sets a custom logger for the provider
func (b *BaseProvider) SetLogger(logger *logging.Logger) {
	b.logger = logger
}

// GetLogger returns the provider's logger
func (b *BaseProvider) GetLogger() *logging.Logger {
	return b.logger
}

// Name returns the provider name
func (b *BaseProvider) Name() string {
	return b.name
}

// Authenticate stores the API key
func (b *BaseProvider) Authenticate(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}
	b.apiKey = apiKey
	b.authenticated = true
	return nil
}

// IsAuthenticated returns whether the provider is authenticated
func (b *BaseProvider) IsAuthenticated() bool {
	return b.authenticated
}

// GetAPIKey returns the stored API key
func (b *BaseProvider) GetAPIKey() string {
	return b.apiKey
}

// SetMaxRetries sets the maximum number of retries
func (b *BaseProvider) SetMaxRetries(maxRetries int) {
	b.maxRetries = maxRetries
}

// SetBaseDelay sets the base delay for exponential backoff
func (b *BaseProvider) SetBaseDelay(delay time.Duration) {
	b.baseDelay = delay
}

// RetryWithBackoff executes a function with exponential backoff retry logic
func (b *BaseProvider) RetryWithBackoff(fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= b.maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry on last attempt
		if attempt == b.maxRetries {
			break
		}

		// Check if error is retryable
		if !isRetryableError(err) {
			return err
		}

		// Calculate exponential backoff delay
		delay := b.baseDelay * time.Duration(math.Pow(2, float64(attempt)))
		time.Sleep(delay)
	}

	return fmt.Errorf("failed after %d retries: %w", b.maxRetries, lastErr)
}

// DiscoverModels is a default implementation that returns an error
// Providers that support dynamic discovery should override this
func (b *BaseProvider) DiscoverModels() ([]Model, error) {
	return nil, fmt.Errorf("provider %s does not support dynamic model discovery", b.name)
}

// GetRateLimitInfo is a default implementation that returns nil
// Providers that support rate limiting should override this
func (b *BaseProvider) GetRateLimitInfo() (*RateLimitInfo, error) {
	return nil, nil
}

// GetQuotaInfo is a default implementation that returns nil
// Providers that support quotas should override this
func (b *BaseProvider) GetQuotaInfo() (*QuotaInfo, error) {
	return nil, nil
}

// SupportsCodingPlan is a default implementation that returns false
// Providers that support coding plans should override this
func (b *BaseProvider) SupportsCodingPlan() bool {
	return false
}

// ExtractRateLimitInfo extracts rate limit information from HTTP response headers
func ExtractRateLimitInfo(resp *http.Response) *RateLimitInfo {
	info := &RateLimitInfo{}

	// Extract from common rate limit headers
	if val := resp.Header.Get("X-RateLimit-Remaining"); val != "" {
		if parsed, err := parseRateLimitHeader(val); err == nil {
			info.RequestsRemaining = parsed
		}
	}

	if val := resp.Header.Get("X-RateLimit-Limit"); val != "" {
		if parsed, err := parseRateLimitHeader(val); err == nil {
			info.RequestsLimit = parsed
		}
	}

	if val := resp.Header.Get("X-RateLimit-Reset"); val != "" {
		if parsed, err := parseRateLimitResetHeader(val); err == nil {
			info.ResetAt = parsed
		}
	}

	if val := resp.Header.Get("Retry-After"); val != "" {
		if parsed, err := parseRetryAfterHeader(val); err == nil {
			info.RetryAfter = parsed
		}
	}

	// Extract from OpenAI-specific headers
	if val := resp.Header.Get("X-RateLimit-Remaining-Requests"); val != "" {
		if parsed, err := parseRateLimitHeader(val); err == nil {
			info.RequestsRemaining = parsed
		}
	}

	// Extract from Kimi-specific headers
	if val := resp.Header.Get("X-Ratelimit-Remaining-Requests"); val != "" {
		if parsed, err := parseRateLimitHeader(val); err == nil {
			info.RequestsRemaining = parsed
		}
	}

	if val := resp.Header.Get("X-Ratelimit-Limit-Requests"); val != "" {
		if parsed, err := parseRateLimitHeader(val); err == nil {
			info.RequestsLimit = parsed
		}
	}

	if val := resp.Header.Get("X-Ratelimit-Reset-Requests"); val != "" {
		if parsed, err := parseRateLimitResetHeader(val); err == nil {
			info.ResetAt = parsed
		}
	}

	// Extract from Anthropic-specific headers
	if val := resp.Header.Get("anthropic-ratelimit-requests-remaining"); val != "" {
		if parsed, err := parseRateLimitHeader(val); err == nil {
			info.RequestsRemaining = parsed
		}
	}

	if val := resp.Header.Get("anthropic-ratelimit-requests-limit"); val != "" {
		if parsed, err := parseRateLimitHeader(val); err == nil {
			info.RequestsLimit = parsed
		}
	}

	if val := resp.Header.Get("anthropic-ratelimit-requests-reset"); val != "" {
		if parsed, err := parseRateLimitResetHeader(val); err == nil {
			info.ResetAt = parsed
		}
	}

	if val := resp.Header.Get("Retry-After"); val != "" {
		if parsed, err := parseRetryAfterHeader(val); err == nil {
			info.RetryAfter = parsed
		}
	}

	return info
}

// ExtractQuotaInfo extracts quota information from HTTP response headers
func ExtractQuotaInfo(resp *http.Response) *QuotaInfo {
	info := &QuotaInfo{}

	// Extract from common quota headers
	if val := resp.Header.Get("X-RateLimit-Remaining"); val != "" {
		if parsed, err := parseQuotaHeader(val); err == nil {
			info.TokensRemaining = parsed
		}
	}

	if val := resp.Header.Get("X-RateLimit-Limit"); val != "" {
		if parsed, err := parseQuotaHeader(val); err == nil {
			info.TokensLimit = parsed
		}
	}

	if val := resp.Header.Get("X-RateLimit-Reset"); val != "" {
		if parsed, err := parseRateLimitResetHeader(val); err == nil {
			info.ResetAt = parsed
		}
	}

	// Extract from OpenAI-specific headers
	if val := resp.Header.Get("X-Remaining-Tokens"); val != "" {
		if parsed, err := parseQuotaHeader(val); err == nil {
			info.TokensRemaining = parsed
		}
	}

	if val := resp.Header.Get("X-RateLimit-Remaining-Tokens"); val != "" {
		if parsed, err := parseQuotaHeader(val); err == nil {
			info.TokensRemaining = parsed
		}
	}

	// Extract from Kimi-specific headers
	if val := resp.Header.Get("X-Ratelimit-Remaining-Tokens"); val != "" {
		if parsed, err := parseQuotaHeader(val); err == nil {
			info.TokensRemaining = parsed
		}
	}

	if val := resp.Header.Get("X-Ratelimit-Limit-Tokens"); val != "" {
		if parsed, err := parseQuotaHeader(val); err == nil {
			info.TokensLimit = parsed
		}
	}

	// Extract from Anthropic-specific headers
	if val := resp.Header.Get("anthropic-ratelimit-tokens-remaining"); val != "" {
		if parsed, err := parseQuotaHeader(val); err == nil {
			info.TokensRemaining = parsed
		}
	}

	if val := resp.Header.Get("anthropic-ratelimit-tokens-limit"); val != "" {
		if parsed, err := parseQuotaHeader(val); err == nil {
			info.TokensLimit = parsed
		}
	}

	if val := resp.Header.Get("anthropic-ratelimit-tokens-reset"); val != "" {
		if parsed, err := parseRateLimitResetHeader(val); err == nil {
			info.ResetAt = parsed
		}
	}

	return info
}

// Helper functions for parsing headers
func parseRateLimitHeader(val string) (int, error) {
	var parsed int
	_, err := fmt.Sscanf(val, "%d", &parsed)
	return parsed, err
}

func parseQuotaHeader(val string) (int, error) {
	return parseRateLimitHeader(val)
}

func parseRateLimitResetHeader(val string) (time.Time, error) {
	var unix int64
	_, err := fmt.Sscanf(val, "%d", &unix)
	return time.Unix(unix, 0), err
}

func parseRetryAfterHeader(val string) (time.Duration, error) {
	var seconds int
	_, err := fmt.Sscanf(val, "%d", &seconds)
	return time.Duration(seconds) * time.Second, err
}

// isRetryableError determines if an error should be retried
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())

	// Check if error is already categorized
	if catErr, ok := err.(*errors.CategorizedError); ok {
		return catErr.Retryable
	}

	// Check for HTTP status codes in error message
	// Retryable: 429 (rate limit), 500, 502, 503, 504 (server errors)
	httpStatusRegex := regexp.MustCompile(`(?:error|status|code)[:\s]*(\d{3})`)
	matches := httpStatusRegex.FindStringSubmatch(errMsg)
	if len(matches) > 1 {
		statusCode, _ := strconv.Atoi(matches[1])
		return statusCode == 429 || statusCode >= 500 && statusCode < 600
	}

	// Check for network-related errors
	networkPatterns := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"no route to host",
		"network unreachable",
		"dial tcp",
		"temporary failure",
		"temporary error",
		"try again later",
		"rate limit",
		"quota exceeded",
	}

	for _, pattern := range networkPatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	// Non-retryable patterns
	nonRetryablePatterns := []string{
		"unauthorized",
		"forbidden",
		"not found",
		"invalid",
		"bad request",
		"authentication failed",
		"invalid api key",
	}

	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return false
		}
	}

	return false
}
