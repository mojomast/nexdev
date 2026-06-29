package errors

import (
	"errors"
	"testing"
	"time"
)

func TestExecuteWithRecovery_Success(t *testing.T) {
	strategy := DefaultRecoveryStrategy()
	callCount := 0

	operation := func() error {
		callCount++
		return nil
	}

	err := ExecuteWithRecovery(operation, strategy)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected operation to be called once, called %d times", callCount)
	}
}

func TestExecuteWithRecovery_RetryableError(t *testing.T) {
	strategy := DefaultRecoveryStrategy()
	strategy.MaxRetries = 2
	strategy.InitialDelay = 10 * time.Millisecond

	callCount := 0
	operation := func() error {
		callCount++
		if callCount < 2 {
			return NewNetworkError(errors.New("timeout"), "Network timeout")
		}
		return nil
	}

	err := ExecuteWithRecovery(operation, strategy)

	if err != nil {
		t.Errorf("expected no error after retry, got %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected operation to be called twice, called %d times", callCount)
	}
}

func TestExecuteWithRecovery_NonRetryable(t *testing.T) {
	strategy := DefaultRecoveryStrategy()
	callCount := 0

	operation := func() error {
		callCount++
		return NewUserError(errors.New("invalid"), "Invalid input", "Fix your input")
	}

	err := ExecuteWithRecovery(operation, strategy)

	if err == nil {
		t.Error("expected error, got nil")
	}

	if callCount != 1 {
		t.Errorf("expected operation to be called once (no retry), called %d times", callCount)
	}
}

func TestExecuteWithRecovery_FatalError(t *testing.T) {
	strategy := DefaultRecoveryStrategy()
	callCount := 0
	fatalCalled := false

	strategy.OnFatalError = func(err error) {
		fatalCalled = true
	}

	operation := func() error {
		callCount++
		return NewSystemError(errors.New("disk full"), "No space left", true)
	}

	err := ExecuteWithRecovery(operation, strategy)

	if err == nil {
		t.Error("expected error, got nil")
	}

	if callCount != 1 {
		t.Errorf("expected operation to be called once (no retry for fatal), called %d times", callCount)
	}

	if !fatalCalled {
		t.Error("expected OnFatalError to be called")
	}
}

func TestExecuteWithRecovery_RetriesExhausted(t *testing.T) {
	strategy := DefaultRecoveryStrategy()
	strategy.MaxRetries = 2
	strategy.InitialDelay = 10 * time.Millisecond

	callCount := 0
	retryCalled := 0

	strategy.OnRetry = func(attempt int, err error) {
		retryCalled++
	}

	operation := func() error {
		callCount++
		return NewNetworkError(errors.New("timeout"), "Network timeout")
	}

	err := ExecuteWithRecovery(operation, strategy)

	if err == nil {
		t.Error("expected error after exhausting retries, got nil")
	}

	// Should be called MaxRetries + 1 times (initial + retries)
	if callCount != 3 {
		t.Errorf("expected operation to be called 3 times, called %d times", callCount)
	}

	if retryCalled != 2 {
		t.Errorf("expected OnRetry to be called 2 times, called %d times", retryCalled)
	}
}

func TestCalculateDelay(t *testing.T) {
	testCases := []struct {
		name        string
		attempt     int
		initial     time.Duration
		max         time.Duration
		factor      float64
		expectedMin time.Duration
		expectedMax time.Duration
	}{
		{
			name:        "First retry",
			attempt:     0,
			initial:     1 * time.Second,
			max:         30 * time.Second,
			factor:      2.0,
			expectedMin: 1 * time.Second,
			expectedMax: 1 * time.Second,
		},
		{
			name:        "Second retry",
			attempt:     1,
			initial:     1 * time.Second,
			max:         30 * time.Second,
			factor:      2.0,
			expectedMin: 2 * time.Second,
			expectedMax: 2 * time.Second,
		},
		{
			name:        "Third retry",
			attempt:     2,
			initial:     1 * time.Second,
			max:         30 * time.Second,
			factor:      2.0,
			expectedMin: 4 * time.Second,
			expectedMax: 4 * time.Second,
		},
		{
			name:        "Exceeds max",
			attempt:     10,
			initial:     1 * time.Second,
			max:         30 * time.Second,
			factor:      2.0,
			expectedMin: 30 * time.Second,
			expectedMax: 30 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			delay := calculateDelay(tc.attempt, tc.initial, tc.max, tc.factor)

			if delay < tc.expectedMin || delay > tc.expectedMax {
				t.Errorf("expected delay between %v and %v, got %v", tc.expectedMin, tc.expectedMax, delay)
			}
		})
	}
}

func TestRetryableOperation_Execute(t *testing.T) {
	callCount := 0
	op := &RetryableOperation{
		Name: "test-operation",
		Operation: func() error {
			callCount++
			return nil
		},
		Strategy: DefaultRecoveryStrategy(),
	}

	err := op.Execute()

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected operation to be called once, called %d times", callCount)
	}
}

func TestRetryableOperation_ExecuteWithProgress(t *testing.T) {
	callCount := 0
	progressCount := 0

	op := &RetryableOperation{
		Name: "test-operation",
		Operation: func() error {
			callCount++
			if callCount < 2 {
				return NewNetworkError(errors.New("timeout"), "Timeout")
			}
			return nil
		},
		Strategy: &RecoveryStrategy{
			MaxRetries:    2,
			InitialDelay:  10 * time.Millisecond,
			MaxDelay:      30 * time.Second,
			BackoffFactor: 2.0,
		},
	}

	err := op.ExecuteWithProgress(func(attempt, total int, err error) {
		progressCount++
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if progressCount != 1 {
		t.Errorf("expected progress callback once, called %d times", progressCount)
	}
}

type mockStatePreserver struct {
	saveCount   int
	description string
}

func (m *mockStatePreserver) SaveState() error {
	m.saveCount++
	return nil
}

func (m *mockStatePreserver) GetStateDescription() string {
	return m.description
}

func TestPreserveStateOnError(t *testing.T) {
	preserver := &mockStatePreserver{
		description: "test state",
	}

	// System error should preserve state
	sysErr := NewSystemError(errors.New("test"), "System error", false)
	err := PreserveStateOnError(sysErr, preserver)

	if err == nil {
		t.Error("expected error to be returned")
	}

	if preserver.saveCount != 1 {
		t.Errorf("expected state to be saved once, saved %d times", preserver.saveCount)
	}

	// User error should NOT preserve state
	preserver.saveCount = 0
	userErr := NewUserError(errors.New("test"), "User error", "Fix it")
	err = PreserveStateOnError(userErr, preserver)

	if err == nil {
		t.Error("expected error to be returned")
	}

	if preserver.saveCount != 0 {
		t.Errorf("expected state not to be saved, saved %d times", preserver.saveCount)
	}

	// Nil error should not save state
	preserver.saveCount = 0
	err = PreserveStateOnError(nil, preserver)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}

	if preserver.saveCount != 0 {
		t.Errorf("expected state not to be saved for nil error, saved %d times", preserver.saveCount)
	}
}

func TestErrorLogger(t *testing.T) {
	logCount := 0
	var loggedCategory ErrorCategory
	var loggedMessage string

	logger := NewErrorLogger(func(category ErrorCategory, message string, context map[string]interface{}) {
		logCount++
		loggedCategory = category
		loggedMessage = message
	})

	err := NewAPIError(errors.New("test"), "API failed", true)
	logger.Log(err)

	if logCount != 1 {
		t.Errorf("expected log to be called once, called %d times", logCount)
	}

	if loggedCategory != APIError {
		t.Errorf("expected category APIError, got %s", loggedCategory)
	}

	if loggedMessage != "API failed" {
		t.Errorf("expected message 'API failed', got %s", loggedMessage)
	}
}

func TestDefaultRecoveryStrategy(t *testing.T) {
	strategy := DefaultRecoveryStrategy()

	if strategy.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", strategy.MaxRetries)
	}

	if strategy.InitialDelay != 1*time.Second {
		t.Errorf("expected InitialDelay 1s, got %v", strategy.InitialDelay)
	}

	if !strategy.SaveStateOnFail {
		t.Error("expected SaveStateOnFail to be true")
	}
}

func TestAPIRecoveryStrategy(t *testing.T) {
	strategy := APIRecoveryStrategy()

	if strategy.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", strategy.MaxRetries)
	}

	if strategy.InitialDelay != 2*time.Second {
		t.Errorf("expected InitialDelay 2s, got %v", strategy.InitialDelay)
	}
}

func TestNetworkRecoveryStrategy(t *testing.T) {
	strategy := NetworkRecoveryStrategy()

	if strategy.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", strategy.MaxRetries)
	}

	if strategy.SaveStateOnFail {
		t.Error("expected SaveStateOnFail to be false for network errors")
	}
}
