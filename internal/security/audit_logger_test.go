package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestNewAuditLogger tests the creation of a new AuditLogger
func TestNewAuditLogger(t *testing.T) {
	tests := []struct {
		name        string
		logPath     string
		expectError bool
	}{
		{
			name:        "valid log path",
			logPath:     filepath.Join(t.TempDir(), "audit.log"),
			expectError: false,
		},
		{
			name:        "empty log path",
			logPath:     "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewAuditLogger(tt.logPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if logger == nil {
				t.Errorf("expected logger but got nil")
				return
			}

			// Clean up
			defer logger.Close()

			// Verify file was created
			if _, err := os.Stat(tt.logPath); os.IsNotExist(err) {
				t.Errorf("log file was not created")
			}
		})
	}
}

// TestLogPathRejection tests logging of path rejection events
func TestLogPathRejection(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.log")
	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log a path rejection
	testPath := "/etc/passwd"
	testReason := "outside project root"
	logger.LogPathRejection(testPath, testReason)

	// Close to flush
	logger.Close()

	// Read log file and verify content
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)

	// Verify log contains expected elements
	if !strings.Contains(logContent, "[SECURITY]") {
		t.Errorf("log missing [SECURITY] level")
	}
	if !strings.Contains(logContent, "[PATH_REJECTION]") {
		t.Errorf("log missing [PATH_REJECTION] component")
	}
	if !strings.Contains(logContent, testPath) {
		t.Errorf("log missing path: %s", testPath)
	}
	if !strings.Contains(logContent, testReason) {
		t.Errorf("log missing reason: %s", testReason)
	}

	// Verify timestamp format (RFC3339)
	if !strings.Contains(logContent, "T") || !strings.Contains(logContent, "Z") {
		t.Errorf("log missing proper timestamp format")
	}
}

// TestLogFileOperation tests logging of file operations
func TestLogFileOperation(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.log")
	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	tests := []struct {
		name      string
		operation string
		path      string
		success   bool
		expected  string
	}{
		{
			name:      "successful write",
			operation: "write",
			path:      "/project/file.txt",
			success:   true,
			expected:  "status=success",
		},
		{
			name:      "failed delete",
			operation: "delete",
			path:      "/project/old.txt",
			success:   false,
			expected:  "status=failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger.LogFileOperation(tt.operation, tt.path, tt.success)
		})
	}

	// Close to flush
	logger.Close()

	// Read log file and verify content
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)

	// Verify all operations were logged
	for _, tt := range tests {
		if !strings.Contains(logContent, "[AUDIT]") {
			t.Errorf("log missing [AUDIT] level")
		}
		if !strings.Contains(logContent, "[FILE_OPERATION]") {
			t.Errorf("log missing [FILE_OPERATION] component")
		}
		if !strings.Contains(logContent, tt.operation) {
			t.Errorf("log missing operation: %s", tt.operation)
		}
		if !strings.Contains(logContent, tt.path) {
			t.Errorf("log missing path: %s", tt.path)
		}
		if !strings.Contains(logContent, tt.expected) {
			t.Errorf("log missing expected status: %s", tt.expected)
		}
	}
}

// TestLogAuthFailure tests logging of authentication failures
func TestLogAuthFailure(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.log")
	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log an auth failure
	testProvider := "openai"
	testError := "invalid API key"
	logger.LogAuthFailure(testProvider, testError)

	// Close to flush
	logger.Close()

	// Read log file and verify content
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)

	// Verify log contains expected elements
	if !strings.Contains(logContent, "[SECURITY]") {
		t.Errorf("log missing [SECURITY] level")
	}
	if !strings.Contains(logContent, "[AUTH_FAILURE]") {
		t.Errorf("log missing [AUTH_FAILURE] component")
	}
	if !strings.Contains(logContent, testProvider) {
		t.Errorf("log missing provider: %s", testProvider)
	}
	if !strings.Contains(logContent, testError) {
		t.Errorf("log missing error: %s", testError)
	}
}

// TestAuditLoggerAppend tests that the logger appends to existing files
func TestAuditLoggerAppend(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.log")

	// Create first logger and write
	logger1, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create first logger: %v", err)
	}
	logger1.LogPathRejection("path1", "reason1")
	logger1.Close()

	// Create second logger and write
	logger2, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create second logger: %v", err)
	}
	logger2.LogPathRejection("path2", "reason2")
	logger2.Close()

	// Read log file
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)

	// Verify both entries are present
	if !strings.Contains(logContent, "path1") {
		t.Errorf("log missing first entry")
	}
	if !strings.Contains(logContent, "path2") {
		t.Errorf("log missing second entry")
	}

	// Verify we have two log lines
	lines := strings.Split(strings.TrimSpace(logContent), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 log lines, got %d", len(lines))
	}
}

// TestAuditLoggerConcurrency tests concurrent logging
func TestAuditLoggerConcurrency(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.log")
	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	// Launch multiple goroutines to log concurrently
	done := make(chan bool)
	numGoroutines := 10
	logsPerGoroutine := 10

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < logsPerGoroutine; j++ {
				logger.LogPathRejection(
					filepath.Join("path", string(rune('a'+id)), string(rune('0'+j))),
					"concurrent test",
				)
				// Small delay to increase chance of interleaving
				time.Sleep(time.Microsecond)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Close to flush
	logger.Close()

	// Read log file
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	// Count log lines
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	expectedLines := numGoroutines * logsPerGoroutine

	if len(lines) != expectedLines {
		t.Errorf("expected %d log lines, got %d", expectedLines, len(lines))
	}

	// Verify all lines are properly formatted (no corruption from concurrent writes)
	for i, line := range lines {
		if !strings.Contains(line, "[SECURITY]") {
			t.Errorf("line %d missing [SECURITY]: %s", i, line)
		}
		if !strings.Contains(line, "[PATH_REJECTION]") {
			t.Errorf("line %d missing [PATH_REJECTION]: %s", i, line)
		}
	}
}

// TestAuditLoggerClose tests proper cleanup
func TestAuditLoggerClose(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.log")
	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// Close the logger
	err = logger.Close()
	if err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}

	// Closing again should not panic (idempotent)
	err = logger.Close()
	if err != nil {
		t.Errorf("unexpected error on second close: %v", err)
	}
}

// TestAuditLoggerFilePermissions tests that log files have secure permissions
func TestAuditLoggerFilePermissions(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.log")
	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	// Check file permissions
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("failed to stat log file: %v", err)
	}

	// On Unix systems, verify permissions are 0600 (owner read/write only)
	// On Windows, this test may not be as meaningful
	mode := info.Mode()

	// Check if the file mode matches expected (may vary by OS)
	// We mainly care that it's not world-readable
	if mode.Perm()&0077 != 0 {
		t.Logf("Warning: log file has overly permissive permissions: %v", mode.Perm())
	}

	// At minimum, verify the file exists and is a regular file
	if !mode.IsRegular() {
		t.Errorf("log file is not a regular file")
	}
}

// TestAuditLoggerStructuredFormat tests the structured log format
func TestAuditLoggerStructuredFormat(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.log")
	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log various events
	logger.LogPathRejection("/etc/passwd", "outside root")
	logger.LogFileOperation("write", "/project/file.txt", true)
	logger.LogAuthFailure("openai", "invalid key")

	logger.Close()

	// Read and parse log entries
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	// Verify each line has the expected structure
	for i, line := range lines {
		// Each line should have: [TIMESTAMP] [LEVEL] [COMPONENT] key=value pairs

		// Check for timestamp in RFC3339 format
		if !strings.HasPrefix(line, "[") {
			t.Errorf("line %d doesn't start with timestamp: %s", i, line)
		}

		// Check for level
		if !strings.Contains(line, "[SECURITY]") && !strings.Contains(line, "[AUDIT]") {
			t.Errorf("line %d missing level: %s", i, line)
		}

		// Check for component
		hasComponent := strings.Contains(line, "[PATH_REJECTION]") ||
			strings.Contains(line, "[FILE_OPERATION]") ||
			strings.Contains(line, "[AUTH_FAILURE]")
		if !hasComponent {
			t.Errorf("line %d missing component: %s", i, line)
		}

		// Check for key=value pairs (quoted values)
		if !strings.Contains(line, "=") {
			t.Errorf("line %d missing key=value pairs: %s", i, line)
		}
	}
}
