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
	want := []string{"init", "run", "develop", "verify", "status", "plan", "review", "navigate", "detour", "steer", "pause", "resume", "cancel", "blockers", "provider", "events", "artifacts", "history", "config", "auth", "serve", "tui", "doctor"}
	for _, name := range want {
		if _, _, err := rootCmd.Find([]string{name}); err != nil {
			t.Fatalf("command %q not registered: %v", name, err)
		}
	}
	if rootCmd.Use != "nexdev" {
		t.Fatalf("root use = %q", rootCmd.Use)
	}
	if rootCmd.PersistentFlags().Lookup("no-pi") == nil {
		t.Fatal("global --no-pi fallback flag is not registered")
	}
	legacy := []string{"interview", "design", "validate", "stats", "quota", "checkpoint", "rollback", "mcp-server", "version"}
	for _, name := range legacy {
		if cmd, _, err := rootCmd.Find([]string{name}); err == nil && cmd != nil && cmd.Name() == name {
			t.Fatalf("legacy command %q is still reachable", name)
		}
	}
}

func TestRootHelpShowsOnlySpecCommandsAndNoGeoffrussyState(t *testing.T) {
	oldJSON := jsonOutput
	defer func() { jsonOutput = oldJSON }()
	jsonOutput = true

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"--help"})
	defer rootCmd.SetArgs(nil)

	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	help := out.String()
	commandsSection := help
	if parts := strings.SplitN(commandsSection, "Flags:", 2); len(parts) == 2 {
		commandsSection = parts[0]
	}
	want := []string{"init", "run", "develop", "verify", "status", "plan", "review", "navigate", "detour", "steer", "pause", "resume", "cancel", "blockers", "provider", "events", "artifacts", "history", "config", "auth", "serve", "tui", "doctor"}
	for _, name := range want {
		if !strings.Contains(commandsSection, "  "+name+" ") {
			t.Fatalf("help missing spec command %q:\n%s", name, help)
		}
	}
	mustNotContain := []string{"interview", "design", "validate", "stats", "quota", "checkpoint", "rollback", "mcp-server", "version", "help"}
	for _, text := range mustNotContain {
		if strings.Contains(commandsSection, "  "+text+" ") {
			t.Fatalf("help contains legacy text %q:\n%s", text, help)
		}
	}
	if strings.Contains(help, ".geoffrussy") || strings.Contains(help, "geoffrussy") {
		t.Fatalf("help contains legacy state text:\n%s", help)
	}
}

func TestRootWithoutSubcommandDoesNotProbeGeoffrussyState(t *testing.T) {
	oldJSON := jsonOutput
	defer func() { jsonOutput = oldJSON }()
	jsonOutput = true

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(nil)
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	if strings.Contains(output, ".geoffrussy") || strings.Contains(output, "geoffrussy") {
		t.Fatalf("root command output contains legacy state reference:\n%s", output)
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
