package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestM12CommandTreeIncludesServeAuthAndControlCommands(t *testing.T) {
	want := []string{"run", "tui", "verify", "history", "serve", "auth", "events", "detour", "steer", "pause", "resume", "cancel", "blockers", "provider", "artifacts", "doctor"}
	for _, name := range want {
		if _, _, err := rootCmd.Find([]string{name}); err != nil {
			t.Fatalf("command %q not registered: %v", name, err)
		}
	}
	if rootCmd.Use != "nexdev" {
		t.Fatalf("root use = %q", rootCmd.Use)
	}
}

func TestAuthTokenCreateCommandPrintsPlaintextOnce(t *testing.T) {
	oldProjectDir, oldJSON, oldRole, oldName, oldTTL := projectDir, jsonOutput, authTokenRole, authTokenName, authTokenTTL
	defer func() {
		projectDir, jsonOutput, authTokenRole, authTokenName, authTokenTTL = oldProjectDir, oldJSON, oldRole, oldName, oldTTL
	}()
	projectDir = t.TempDir()
	jsonOutput = true
	authTokenRole = "operator"
	authTokenName = "test"
	authTokenTTL = "0"

	var out bytes.Buffer
	authTokenCreateCmd.SetOut(&out)
	authTokenCreateCmd.SetContext(context.Background())
	if err := authTokenCreateCmd.RunE(authTokenCreateCmd, nil); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out.String())
	}
	if decoded["token"] == "" || decoded["id"] == "" || decoded["role"] != "operator" {
		t.Fatalf("unexpected token output: %#v", decoded)
	}
}

func TestControlMutationCommandsRequireControlURL(t *testing.T) {
	oldControlURL := controlURL
	defer func() { controlURL = oldControlURL }()
	controlURL = ""
	err := pauseCmd.RunE(pauseCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "requires --control-url") {
		t.Fatalf("pause error = %v", err)
	}
}
