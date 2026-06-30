package safety

import "testing"

func TestDefaultToolPolicyDeniesShellAndNetwork(t *testing.T) {
	policy := DefaultToolPolicy()
	if policy.AllowsShellCommand("go test ./...") {
		t.Fatal("default policy must deny shell commands")
	}
	if policy.AllowsNetwork() {
		t.Fatal("default policy must deny network")
	}
	if !policy.AllowsReadFile() {
		t.Fatal("default policy should allow read_file skeleton")
	}
}

func TestDefaultToolPolicyDenyGlobs(t *testing.T) {
	policy := DefaultToolPolicy()
	for _, path := range []string{".git/config", ".env", ".env.local", "id_rsa", "keys/id_ed25519", "certs/private.pem", "certs/private.key"} {
		if err := policy.ValidateWritePath(path); err == nil {
			t.Fatalf("expected %q to be denied", path)
		}
	}
	if err := policy.ValidateWritePath("src/main.go"); err != nil {
		t.Fatalf("expected normal write path to be allowed: %v", err)
	}
}

func TestToolPolicyWildcardShellRejectedOutsideDev(t *testing.T) {
	policy := DefaultToolPolicy()
	policy.Shell.AllowCommands = []string{"go test *"}
	if err := policy.Validate(ProfileDev); err != nil {
		t.Fatalf("dev profile should allow wildcard shell skeleton rules: %v", err)
	}
	if err := policy.Validate(ProfileTrustedLAN); err == nil {
		t.Fatal("trusted-lan profile should reject wildcard shell rules")
	}
	if err := policy.Validate(ProfileCI); err == nil {
		t.Fatal("ci profile should reject wildcard shell rules")
	}
}

func TestToolPolicyAllowsExactShellCommandOnly(t *testing.T) {
	policy := DefaultToolPolicy()
	policy.Shell.AllowCommands = []string{"go test ./..."}
	if !policy.AllowsShellCommand("go test ./...") {
		t.Fatal("exact shell allow rule should allow matching command")
	}
	if policy.AllowsShellCommand("go test ./internal/safety") {
		t.Fatal("non-matching shell command should remain denied")
	}
}

func TestToolPolicyTaskWriteRequiresExpectedAndUnlockedPath(t *testing.T) {
	policy := DefaultToolPolicy()
	if err := policy.ValidateTaskWritePath("src/main.go", WriteValidationOptions{}); err == nil {
		t.Fatal("write without expected files should fail")
	}
	if err := policy.ValidateTaskWritePath("src/main.go", WriteValidationOptions{ExpectedFiles: []string{"docs/**"}}); err == nil {
		t.Fatal("write outside expected files should fail")
	}
	if err := policy.ValidateTaskWritePath("src/main.go", WriteValidationOptions{ExpectedFiles: []string{"src/**"}, LockedFiles: []string{"src/main.go"}}); err == nil {
		t.Fatal("write to locked file should fail")
	}
	if err := policy.ValidateTaskWritePath("src/main.go", WriteValidationOptions{ExpectedFiles: []string{"src/**"}}); err != nil {
		t.Fatalf("expected write to pass: %v", err)
	}
}
