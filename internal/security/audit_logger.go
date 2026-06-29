package security

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// AuditLogger logs security-relevant events for audit trail purposes.
// It provides structured logging with timestamps for path rejections,
// file operations, and authentication failures.
//
// The logger is safe for concurrent use and ensures all log entries
// are written to disk immediately for audit integrity.
type AuditLogger struct {
	logger *log.Logger
	file   *os.File
	mu     sync.Mutex // Protects concurrent writes
}

// NewAuditLogger creates a new AuditLogger that writes to the specified log file.
// The log file is created if it doesn't exist, or appended to if it does.
// The file is opened with appropriate permissions (0600) for security.
//
// Returns an error if:
// - The log path is empty
// - The log file cannot be created or opened
func NewAuditLogger(logPath string) (*AuditLogger, error) {
	if logPath == "" {
		return nil, fmt.Errorf("log path cannot be empty")
	}

	// Open log file with append mode, create if doesn't exist
	// Use 0600 permissions (read/write for owner only) for security
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	// Create logger with structured format
	// Format: [TIMESTAMP] [LEVEL] [COMPONENT] message
	logger := log.New(file, "", 0) // No prefix, we'll format ourselves

	return &AuditLogger{
		logger: logger,
		file:   file,
	}, nil
}

// LogPathRejection logs when a file path is rejected due to security validation failure.
// This is a critical security event that should always be audited.
//
// Parameters:
// - path: The rejected file path
// - reason: The reason for rejection (e.g., "outside project root", "directory traversal")
func (al *AuditLogger) LogPathRejection(path string, reason string) {
	al.mu.Lock()
	defer al.mu.Unlock()

	timestamp := time.Now().UTC().Format(time.RFC3339)
	al.logger.Printf("[%s] [SECURITY] [PATH_REJECTION] path=%q reason=%q\n",
		timestamp, path, reason)
}

// LogFileOperation logs file system operations performed by the system.
// This provides an audit trail of all file modifications.
//
// Parameters:
// - operation: The type of operation (e.g., "write", "delete", "create")
// - path: The file path being operated on
// - success: Whether the operation succeeded
func (al *AuditLogger) LogFileOperation(operation string, path string, success bool) {
	al.mu.Lock()
	defer al.mu.Unlock()

	timestamp := time.Now().UTC().Format(time.RFC3339)
	status := "success"
	if !success {
		status = "failure"
	}

	al.logger.Printf("[%s] [AUDIT] [FILE_OPERATION] operation=%q path=%q status=%s\n",
		timestamp, operation, path, status)
}

// LogAuthFailure logs authentication failures when interacting with API providers.
// This helps detect potential security issues or misconfigurations.
//
// Parameters:
// - provider: The name of the provider (e.g., "openai", "anthropic")
// - error: The error message or reason for authentication failure
func (al *AuditLogger) LogAuthFailure(provider string, error string) {
	al.mu.Lock()
	defer al.mu.Unlock()

	timestamp := time.Now().UTC().Format(time.RFC3339)
	al.logger.Printf("[%s] [SECURITY] [AUTH_FAILURE] provider=%q error=%q\n",
		timestamp, provider, error)
}

// Close closes the audit log file and releases associated resources.
// After calling Close, the AuditLogger should not be used.
//
// Returns an error if the file cannot be closed properly.
func (al *AuditLogger) Close() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.file != nil {
		if err := al.file.Close(); err != nil {
			return fmt.Errorf("failed to close audit log file: %w", err)
		}
		al.file = nil
	}

	return nil
}
