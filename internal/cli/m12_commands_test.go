package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

func TestRunCommandWiresExplicitFakeProvider(t *testing.T) {
	oldProjectDir, oldJSON, oldFake, oldControlURL := projectDir, jsonOutput, runFake, controlURL
	defer func() {
		projectDir, jsonOutput, runFake, controlURL = oldProjectDir, oldJSON, oldFake, oldControlURL
	}()
	projectDir = t.TempDir()
	jsonOutput = true
	runFake = true
	controlURL = ""
	if err := os.WriteFile(filepath.Join(projectDir, "README.md"), []byte("# fake\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	runCmd.SetOut(&out)
	runCmd.SetContext(context.Background())
	if err := runCmd.RunE(runCmd, []string{"fake", "run"}); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if decoded["status"] != "completed" || decoded["run_id"] == "" {
		t.Fatalf("unexpected run output: %#v", decoded)
	}
}
