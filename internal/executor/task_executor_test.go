package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
)

// mockProvider implements provider.Provider for testing
type mockProvider struct{}

func (m *mockProvider) Name() string                              { return "mock" }
func (m *mockProvider) Authenticate(apiKey string) error          { return nil }
func (m *mockProvider) IsAuthenticated() bool                     { return true }
func (m *mockProvider) ListModels() ([]provider.Model, error)     { return nil, nil }
func (m *mockProvider) DiscoverModels() ([]provider.Model, error) { return nil, nil }
func (m *mockProvider) Stream(ctx context.Context, model string, prompt string) (<-chan string, error) {
	ch := make(chan string, 1)
	go func() {
		defer close(ch)
		ch <- `{"explanation":"test","files":[]}`
	}()
	return ch, nil
}
func (m *mockProvider) GetRateLimitInfo() (*provider.RateLimitInfo, error) { return nil, nil }
func (m *mockProvider) GetQuotaInfo() (*provider.QuotaInfo, error)         { return nil, nil }
func (m *mockProvider) SupportsCodingPlan() bool                           { return false }

func (m *mockProvider) Call(ctx context.Context, model string, prompt string) (*provider.Response, error) {
	return &provider.Response{
		Content:      `{"explanation":"test","files":[]}`,
		TokensInput:  10,
		TokensOutput: 5,
		Model:        model,
		Provider:     "mock",
		Timestamp:    time.Now(),
	}, nil
}

// TestTaskExecutor_PathSanitization tests that the TaskExecutor properly validates file paths
func TestTaskExecutor_PathSanitization(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "task-executor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory for the test
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Create in-memory store
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create mock provider
	mockProv := &mockProvider{}

	// Create a no-op send update function
	sendUpdate := func(update TaskUpdate) {}

	// Create task executor
	te := NewTaskExecutor(context.Background(), store, mockProv, sendUpdate, "test-model")

	// Verify that the task executor has security components initialized
	if te.pathSanitizer == nil {
		t.Fatal("expected pathSanitizer to be initialized")
	}

	if te.auditLogger == nil {
		t.Fatal("expected auditLogger to be initialized")
	}

	tests := []struct {
		name        string
		filePath    string
		content     string
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "valid relative path",
			filePath:    "test.txt",
			content:     "test content",
			shouldError: false,
		},
		{
			name:        "valid subdirectory path",
			filePath:    "subdir/test.txt",
			content:     "test content",
			shouldError: false,
		},
		{
			name:        "directory traversal attempt",
			filePath:    "../../../etc/passwd",
			content:     "malicious content",
			shouldError: true,
			errorMsg:    "path validation failed",
		},
	}

	// Add platform-specific tests
	if filepath.Separator == '/' {
		// Unix-like systems
		tests = append(tests, struct {
			name        string
			filePath    string
			content     string
			shouldError bool
			errorMsg    string
		}{
			name:        "absolute path outside project",
			filePath:    "/etc/passwd",
			content:     "malicious content",
			shouldError: true,
			errorMsg:    "path validation failed",
		})
	} else {
		// Windows
		tests = append(tests, struct {
			name        string
			filePath    string
			content     string
			shouldError bool
			errorMsg    string
		}{
			name:        "windows absolute path",
			filePath:    "C:\\Windows\\System32\\config",
			content:     "malicious content",
			shouldError: true,
			errorMsg:    "path validation failed",
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := File{
				Path:    tt.filePath,
				Content: tt.content,
			}

			err := te.writeFile(file)

			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error for path %s, but got none", tt.filePath)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error to contain %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for path %s: %v", tt.filePath, err)
				} else {
					// Verify file was created
					absPath := filepath.Join(tempDir, tt.filePath)
					if _, err := os.Stat(absPath); os.IsNotExist(err) {
						t.Errorf("expected file to be created at %s", absPath)
					}
				}
			}
		})
	}
}

// TestTaskExecutor_AuditLogging tests that file operations are logged to the audit log
func TestTaskExecutor_AuditLogging(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "task-executor-audit-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory for the test
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Create in-memory store
	store, err := state.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create mock provider
	mockProv := &mockProvider{}

	// Create a no-op send update function
	sendUpdate := func(update TaskUpdate) {}

	// Create task executor
	te := NewTaskExecutor(context.Background(), store, mockProv, sendUpdate, "test-model")
	defer te.auditLogger.Close()

	// Test valid file write
	validFile := File{
		Path:    "valid.txt",
		Content: "valid content",
	}

	err = te.writeFile(validFile)
	if err != nil {
		t.Fatalf("unexpected error writing valid file: %v", err)
	}

	// Test invalid file write (should be logged as rejection)
	invalidFile := File{
		Path:    "../../../etc/passwd",
		Content: "malicious content",
	}

	err = te.writeFile(invalidFile)
	if err == nil {
		t.Fatal("expected error for invalid path, but got none")
	}

	// Verify audit log was created
	auditLogPath := filepath.Join(tempDir, "geoffrussy-audit.log")
	if _, err := os.Stat(auditLogPath); os.IsNotExist(err) {
		t.Error("expected audit log file to be created")
	} else {
		// Read audit log and verify entries
		content, err := os.ReadFile(auditLogPath)
		if err != nil {
			t.Fatalf("failed to read audit log: %v", err)
		}

		logContent := string(content)

		// Check for successful file operation log
		if !contains(logContent, "FILE_OPERATION") {
			t.Error("expected audit log to contain FILE_OPERATION entry")
		}

		// Check for path rejection log
		if !contains(logContent, "PATH_REJECTION") {
			t.Error("expected audit log to contain PATH_REJECTION entry")
		}

		// Check that the rejected path is logged
		if !contains(logContent, "../../../etc/passwd") {
			t.Error("expected audit log to contain the rejected path")
		}
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
