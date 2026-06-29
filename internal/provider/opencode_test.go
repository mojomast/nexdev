package provider

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNewOpenCodeProvider(t *testing.T) {
	provider := NewOpenCodeProvider()
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
	if provider.Name() != "opencode" {
		t.Errorf("expected provider name 'opencode', got '%s'", provider.Name())
	}
}

func TestOpenCodeProvider_Authenticate(t *testing.T) {
	provider := NewOpenCodeProvider()

	// Create a mock opencode script for testing
	tmpDir := t.TempDir()
	mockScript := filepath.Join(tmpDir, "opencode")

	var scriptContent string
	if runtime.GOOS == "windows" {
		mockScript += ".bat"
		scriptContent = "@echo off\nif \"%1\"==\"version\" (echo v1.0.0)"
	} else {
		scriptContent = "#!/bin/sh\nif [ \"$1\" = \"version\" ]; then echo v1.0.0; fi"
	}

	if err := os.WriteFile(mockScript, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to create mock script: %v", err)
	}

	// Set provider to use mock script
	provider.opencodeCmd = mockScript

	// Test successful authentication
	err := provider.Authenticate("")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !provider.IsAuthenticated() {
		t.Error("expected provider to be authenticated")
	}
}

func TestOpenCodeProvider_AuthenticateNotInstalled(t *testing.T) {
	provider := NewOpenCodeProvider()
	provider.opencodeCmd = "nonexistent-command-that-does-not-exist"

	// Test authentication failure when OpenCode not installed
	err := provider.Authenticate("")
	if err == nil {
		t.Error("expected error when opencode CLI not found")
	}
}

func TestOpenCodeProvider_DiscoverModels(t *testing.T) {
	provider := NewOpenCodeProvider()

	// Create a mock opencode script that returns model list
	tmpDir := t.TempDir()
	mockScript := filepath.Join(tmpDir, "opencode")

	var scriptContent string
	if runtime.GOOS == "windows" {
		mockScript += ".bat"
		scriptContent = `@echo off
if "%1"=="version" (
	echo v1.0.0
) else if "%1"=="models" (
	echo [{"id":"claude-sonnet-4","name":"Claude Sonnet 4","provider":"anthropic","features":["text","code"]},{"id":"gpt-4","name":"GPT-4","provider":"openai","features":["text","code"]}]
)`
	} else {
		scriptContent = `#!/bin/sh
if [ "$1" = "version" ]; then
	echo v1.0.0
elif [ "$1" = "models" ]; then
	echo '[{"id":"claude-sonnet-4","name":"Claude Sonnet 4","provider":"anthropic","features":["text","code"]},{"id":"gpt-4","name":"GPT-4","provider":"openai","features":["text","code"]}]'
fi`
	}

	if err := os.WriteFile(mockScript, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to create mock script: %v", err)
	}

	// Set provider to use mock script
	provider.opencodeCmd = mockScript
	provider.Authenticate("")

	// Test model discovery
	models, err := provider.DiscoverModels()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}

	expectedModels := map[string]bool{
		"claude-sonnet-4": false,
		"gpt-4":           false,
	}

	for _, model := range models {
		if model.Provider != "opencode" {
			t.Errorf("expected provider 'opencode', got '%s'", model.Provider)
		}
		if _, ok := expectedModels[model.Name]; ok {
			expectedModels[model.Name] = true
		}
	}

	for name, found := range expectedModels {
		if !found {
			t.Errorf("expected model '%s' not found", name)
		}
	}
}

func TestOpenCodeProvider_DiscoverModelsDefaultFallback(t *testing.T) {
	provider := NewOpenCodeProvider()

	// Create a mock opencode script that doesn't support model listing
	tmpDir := t.TempDir()
	mockScript := filepath.Join(tmpDir, "opencode")

	var scriptContent string
	if runtime.GOOS == "windows" {
		mockScript += ".bat"
		scriptContent = "@echo off\nif \"%1\"==\"version\" (echo v1.0.0)"
	} else {
		scriptContent = "#!/bin/sh\nif [ \"$1\" = \"version\" ]; then echo v1.0.0; fi"
	}

	if err := os.WriteFile(mockScript, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to create mock script: %v", err)
	}

	// Set provider to use mock script
	provider.opencodeCmd = mockScript
	provider.Authenticate("")

	// Test model discovery falls back to defaults
	models, err := provider.DiscoverModels()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(models) == 0 {
		t.Error("expected default models when discovery fails")
	}

	// Check that we got default models
	foundDefault := false
	for _, model := range models {
		if model.Name == "claude-sonnet-4" || model.Name == "gpt-4" || model.Name == "gpt-4-turbo" {
			foundDefault = true
			break
		}
	}

	if !foundDefault {
		t.Error("expected at least one default model")
	}
}

func TestOpenCodeProvider_ListModels(t *testing.T) {
	provider := NewOpenCodeProvider()

	// Create a mock opencode script
	tmpDir := t.TempDir()
	mockScript := filepath.Join(tmpDir, "opencode")

	var scriptContent string
	if runtime.GOOS == "windows" {
		mockScript += ".bat"
		scriptContent = "@echo off\nif \"%1\"==\"version\" (echo v1.0.0)"
	} else {
		scriptContent = "#!/bin/sh\nif [ \"$1\" = \"version\" ]; then echo v1.0.0; fi"
	}

	if err := os.WriteFile(mockScript, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to create mock script: %v", err)
	}

	provider.opencodeCmd = mockScript
	provider.Authenticate("")

	// ListModels should call DiscoverModels
	models, err := provider.ListModels()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(models) == 0 {
		t.Error("expected at least one model")
	}
}

func TestOpenCodeProvider_Call(t *testing.T) {
	provider := NewOpenCodeProvider()

	// Create a mock opencode script
	tmpDir := t.TempDir()
	mockScript := filepath.Join(tmpDir, "opencode")

	var scriptContent string
	if runtime.GOOS == "windows" {
		mockScript += ".bat"
		scriptContent := `@echo off
if "%1"=="version" (
	echo v1.0.0
) else if "%1"=="run" (
	echo Hello! How can I help you?
)`
		if err := os.WriteFile(mockScript, []byte(scriptContent), 0755); err != nil {
			t.Fatalf("failed to create mock script: %v", err)
		}
	} else {
		scriptContent = `#!/bin/sh
if [ "$1" = "version" ]; then
	echo v1.0.0
elif [ "$1" = "run" ]; then
	echo "Hello! How can I help you?"
fi`
		if err := os.WriteFile(mockScript, []byte(scriptContent), 0755); err != nil {
			t.Fatalf("failed to create mock script: %v", err)
		}
	}

	provider.opencodeCmd = mockScript
	provider.Authenticate("")

	// Make API call
	resp, err := provider.Call(context.TODO(), "claude-sonnet-4", "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify response
	if resp.Content == "" {
		t.Error("expected non-empty content")
	}
	if resp.Model != "claude-sonnet-4" {
		t.Errorf("expected model 'claude-sonnet-4', got '%s'", resp.Model)
	}
	if resp.Provider != "opencode" {
		t.Errorf("expected provider 'opencode', got '%s'", resp.Provider)
	}
}

func TestOpenCodeProvider_Stream(t *testing.T) {
	provider := NewOpenCodeProvider()

	// Create a mock opencode script
	tmpDir := t.TempDir()
	mockScript := filepath.Join(tmpDir, "opencode")

	var scriptContent string
	if runtime.GOOS == "windows" {
		mockScript += ".bat"
		scriptContent = `@echo off
if "%1"=="version" (
	echo v1.0.0
) else if "%1"=="run" (
	echo Hello
	echo there
	echo !
)`
	} else {
		scriptContent = `#!/bin/sh
if [ "$1" = "version" ]; then
	echo v1.0.0
elif [ "$1" = "run" ]; then
	echo "Hello"
	echo "there"
	echo "!"
fi`
	}

	if err := os.WriteFile(mockScript, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to create mock script: %v", err)
	}

	provider.opencodeCmd = mockScript
	provider.Authenticate("")

	// Make streaming call
	ch, err := provider.Stream(context.TODO(), "claude-sonnet-4", "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Collect chunks
	var chunks []string
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	// Verify we got some output
	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}
}

func TestOpenCodeProvider_RateLimitsAndQuotas(t *testing.T) {
	provider := NewOpenCodeProvider()

	// OpenCode abstracts away rate limits and quotas
	rateLimitInfo, err := provider.GetRateLimitInfo()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if rateLimitInfo != nil {
		t.Error("expected nil rate limit info for OpenCode")
	}

	quotaInfo, err := provider.GetQuotaInfo()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if quotaInfo != nil {
		t.Error("expected nil quota info for OpenCode")
	}
}

func TestOpenCodeProvider_CallError(t *testing.T) {
	provider := NewOpenCodeProvider()

	// Create a mock opencode script that returns an error
	tmpDir := t.TempDir()
	mockScript := filepath.Join(tmpDir, "opencode")

	var scriptContent string
	if runtime.GOOS == "windows" {
		mockScript += ".bat"
		scriptContent = `@echo off
if "%1"=="version" (
	echo v1.0.0
) else if "%1"=="run" (
	>&2 echo Error: model not found
	exit /b 1
)`
	} else {
		scriptContent = `#!/bin/sh
if [ "$1" = "version" ]; then
	echo v1.0.0
elif [ "$1" = "run" ]; then
	echo "Error: model not found" >&2
	exit 1
fi`
	}

	if err := os.WriteFile(mockScript, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to create mock script: %v", err)
	}

	provider.opencodeCmd = mockScript
	provider.Authenticate("")
	provider.SetMaxRetries(0) // Don't retry for this test

	// Make API call
	_, err := provider.Call(context.TODO(), "nonexistent", "Hello")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// Helper function to create a mock opencode command for integration testing
func createMockOpenCode(t *testing.T, behavior string) string {
	tmpDir := t.TempDir()
	mockScript := filepath.Join(tmpDir, "opencode")

	var scriptContent string
	if runtime.GOOS == "windows" {
		mockScript += ".bat"
		scriptContent = fmt.Sprintf("@echo off\n%s", behavior)
	} else {
		scriptContent = fmt.Sprintf("#!/bin/sh\n%s", behavior)
	}

	if err := os.WriteFile(mockScript, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to create mock script: %v", err)
	}

	// Ensure the script is executable
	if runtime.GOOS != "windows" {
		cmd := exec.Command("chmod", "+x", mockScript)
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to make script executable: %v", err)
		}
	}

	return mockScript
}
