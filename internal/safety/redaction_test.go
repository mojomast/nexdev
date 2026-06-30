package safety

import (
	"strings"
	"testing"
)

func TestRedactSecretsScrubsCommonSecrets(t *testing.T) {
	input := "provider key sk-ant-abc1234567890abcdef password=supersecret token: ghp_abcdefghijklmnopqrstuvwxyz123456"
	got := RedactSecrets(input)
	for _, leaked := range []string{"sk-ant-abc1234567890abcdef", "supersecret", "ghp_abcdefghijklmnopqrstuvwxyz123456"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("redacted text leaked %q: %s", leaked, got)
		}
	}
	if strings.Count(got, redactedValue) < 3 {
		t.Fatalf("expected redaction markers, got %q", got)
	}
}

func TestRedactSecretsScrubsEnvAssignments(t *testing.T) {
	input := "ANTHROPIC_API_KEY=sk-ant-secretvalue123456\nexport DB_PASSWORD='database-password'\nNORMAL=value"
	got := RedactSecrets(input)
	if strings.Contains(got, "sk-ant-secretvalue123456") || strings.Contains(got, "database-password") {
		t.Fatalf("env secret leaked: %s", got)
	}
	if !strings.Contains(got, "NORMAL=value") {
		t.Fatalf("non-secret env assignment should remain: %s", got)
	}
}

func TestRedactSecretsScrubsBearerToken(t *testing.T) {
	got := RedactSecrets("Authorization: Bearer abc.def-ghi_jkl")
	if strings.Contains(got, "abc.def-ghi_jkl") || !strings.Contains(got, "Bearer "+redactedValue) {
		t.Fatalf("bearer token was not redacted: %s", got)
	}
}

func TestRedactSecretsScrubsPrivateKey(t *testing.T) {
	input := "-----BEGIN OPENSSH PRIVATE KEY-----\nabc123\n-----END OPENSSH PRIVATE KEY-----"
	got := RedactSecrets(input)
	if strings.Contains(got, "abc123") || strings.Contains(got, "OPENSSH PRIVATE KEY") {
		t.Fatalf("private key leaked: %s", got)
	}
}
