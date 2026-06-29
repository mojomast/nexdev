package main

import (
	"testing"
)

func TestVersion(t *testing.T) {
	// Test that Version variable exists
	if Version == "" {
		t.Error("Version should not be empty")
	}

	// Default version should be "dev"
	if Version != "dev" {
		t.Logf("Version = %q (expected 'dev' for tests)", Version)
	}
}
