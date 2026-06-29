package errors

import (
	"fmt"
	"math"
	"time"
)

// RecoveryStrategy defines how to recover from an error
type RecoveryStrategy struct {
	MaxRetries      int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	OnRetry         func(attempt int, err error)
	OnFatalError    func(err error)
	SaveStateOnFail bool
}

// DefaultRecoveryStrategy returns a default recovery strategy
func DefaultRecoveryStrategy() *RecoveryStrategy {
	return &RecoveryStrategy{
		MaxRetries:      3,
		InitialDelay:    1 * time.Second,
		MaxDelay:        30 * time.Second,
		BackoffFactor:   2.0,
		SaveStateOnFail: true,
	}
}

// APIRecoveryStrategy returns a recovery strategy for API errors
func APIRecoveryStrategy() *RecoveryStrategy {
	return &RecoveryStrategy{
		MaxRetries:      5,
		InitialDelay:    2 * time.Second,
		MaxDelay:        60 * time.Second,
		BackoffFactor:   2.0,
		SaveStateOnFail: true,
	}
}

// NetworkRecoveryStrategy returns a recovery strategy for network errors
func NetworkRecoveryStrategy() *RecoveryStrategy {
	return &RecoveryStrategy{
		MaxRetries:      3,
		InitialDelay:    5 * time.Second,
		MaxDelay:        30 * time.Second,
		BackoffFactor:   1.5,
		SaveStateOnFail: false,
	}
}

// ExecuteWithRecovery executes an operation with automatic recovery
func ExecuteWithRecovery(operation func() error, strategy *RecoveryStrategy) error {
	var lastErr error

	for attempt := 0; attempt <= strategy.MaxRetries; attempt++ {
		// Execute the operation
		err := operation()

		// Success!
		if err == nil {
			return nil
		}

		lastErr = err

		// Categorize the error
		catErr := Categorize(err)

		// If fatal, don't retry
		if catErr.Fatal {
			if strategy.OnFatalError != nil {
				strategy.OnFatalError(err)
			}
			return err
		}

		// If not retryable, don't retry
		if !catErr.Retryable {
			return err
		}

		// If we've exhausted retries, fail
		if attempt == strategy.MaxRetries {
			break
		}

		// Calculate delay with exponential backoff
		delay := calculateDelay(attempt, strategy.InitialDelay, strategy.MaxDelay, strategy.BackoffFactor)

		// Call retry callback if provided
		if strategy.OnRetry != nil {
			strategy.OnRetry(attempt+1, err)
		}

		// Wait before retrying
		time.Sleep(delay)
	}

	// All retries exhausted
	return fmt.Errorf("operation failed after %d retries: %w", strategy.MaxRetries, lastErr)
}

// calculateDelay calculates the delay for a retry attempt using exponential backoff
func calculateDelay(attempt int, initial, max time.Duration, factor float64) time.Duration {
	delay := float64(initial) * math.Pow(factor, float64(attempt))

	if delay > float64(max) {
		delay = float64(max)
	}

	return time.Duration(delay)
}

// RetryableOperation wraps an operation that can be retried
type RetryableOperation struct {
	Name        string
	Operation   func() error
	Strategy    *RecoveryStrategy
	Description string
}

// Execute executes the retryable operation
func (r *RetryableOperation) Execute() error {
	return ExecuteWithRecovery(r.Operation, r.Strategy)
}

// ExecuteWithProgress executes with progress reporting
func (r *RetryableOperation) ExecuteWithProgress(onProgress func(attempt int, total int, err error)) error {
	strategy := r.Strategy
	if strategy == nil {
		strategy = DefaultRecoveryStrategy()
	}

	// Override the OnRetry callback
	originalOnRetry := strategy.OnRetry
	strategy.OnRetry = func(attempt int, err error) {
		if onProgress != nil {
			onProgress(attempt, strategy.MaxRetries, err)
		}
		if originalOnRetry != nil {
			originalOnRetry(attempt, err)
		}
	}

	return ExecuteWithRecovery(r.Operation, strategy)
}

// StatePreserver handles saving state on errors
type StatePreserver interface {
	SaveState() error
	GetStateDescription() string
}

// PreserveStateOnError saves state if an error occurs
func PreserveStateOnError(err error, preserver StatePreserver) error {
	if err == nil {
		return nil
	}

	catErr := Categorize(err)

	// Only preserve state for certain error categories
	shouldPreserve := catErr.Category == SystemError ||
		catErr.Category == APIError ||
		catErr.Fatal

	if shouldPreserve {
		if preserveErr := preserver.SaveState(); preserveErr != nil {
			// Failed to save state - this is critical
			return fmt.Errorf("original error: %w\nfailed to save state: %v", err, preserveErr)
		}

		// Add context about state preservation
		catErr.WithContext("state_saved", true)
		catErr.WithContext("state_description", preserver.GetStateDescription())
	}

	return err
}

// ErrorLogger provides structured error logging
type ErrorLogger struct {
	LogFunc func(category ErrorCategory, message string, context map[string]interface{})
}

// NewErrorLogger creates a new error logger
func NewErrorLogger(logFunc func(ErrorCategory, string, map[string]interface{})) *ErrorLogger {
	return &ErrorLogger{
		LogFunc: logFunc,
	}
}

// Log logs an error with full context
func (l *ErrorLogger) Log(err error) {
	if err == nil {
		return
	}

	catErr := Categorize(err)

	if l.LogFunc != nil {
		l.LogFunc(catErr.Category, catErr.Message, catErr.Context)
	}
}

// LogWithContext logs an error with additional context
func (l *ErrorLogger) LogWithContext(err error, context map[string]interface{}) {
	if err == nil {
		return
	}

	catErr := Categorize(err)

	// Merge context
	for k, v := range context {
		catErr.Context[k] = v
	}

	if l.LogFunc != nil {
		l.LogFunc(catErr.Category, catErr.Message, catErr.Context)
	}
}
