package cli

import (
	"testing"
)

func TestExecute(t *testing.T) {
	// Test that Execute function exists and can be called
	// This is a basic smoke test for the CLI setup
	err := Execute("test-version")

	// We expect an error because no command is provided
	// but the function should not panic
	if err == nil {
		t.Log("Execute completed without error (expected)")
	}
}

func TestVersionCommand(t *testing.T) {
	// Test that version command is registered
	if versionCmd == nil {
		t.Fatal("versionCmd should not be nil")
	}

	if versionCmd.Use != "version" {
		t.Errorf("versionCmd.Use = %q, want %q", versionCmd.Use, "version")
	}
}

func TestInitCommand(t *testing.T) {
	// Test that init command is registered
	if initCmd == nil {
		t.Fatal("initCmd should not be nil")
	}

	if initCmd.Use != "init" {
		t.Errorf("initCmd.Use = %q, want %q", initCmd.Use, "init")
	}
}
