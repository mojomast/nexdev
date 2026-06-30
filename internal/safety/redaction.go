package safety

import "regexp"

const redactedValue = "[REDACTED]"

var redactionPatterns = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`(?is)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`), redactedValue},
	{regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`), "Bearer " + redactedValue},
	{regexp.MustCompile(`(?m)^(\s*(?:export\s+)?[A-Za-z_][A-Za-z0-9_]*(?:KEY|TOKEN|SECRET|PASSWORD|PASS|PWD|AUTH|CREDENTIAL|PRIVATE)[A-Za-z0-9_]*\s*=\s*)(?:'[^'\n]*'|"[^"\n]*"|[^\s#\n]+)`), "${1}" + redactedValue},
	{regexp.MustCompile(`(?i)\b(password|passwd|pwd|api[_-]?key|secret|token|access[_-]?token|refresh[_-]?token|client[_-]?secret)\b\s*[:=]\s*(?:'[^'\n]*'|"[^"\n]*"|[^\s,;}\]\n]+)`), "${1}: " + redactedValue},
	{regexp.MustCompile(`\b(?:sk-ant-[A-Za-z0-9_-]{16,}|sk-[A-Za-z0-9_-]{16,}|github_pat_[A-Za-z0-9_]{20,}|gh[pousr]_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{10,}|AKIA[0-9A-Z]{16})\b`), redactedValue},
	{regexp.MustCompile(`(?i)\bssh-(?:rsa|ed25519)\s+[A-Za-z0-9+/=]{20,}(?:\s+[^\s]+)?`), redactedValue},
}

// RedactSecrets deterministically scrubs secret-shaped values before text is
// written to logs, events, artifacts, prompts, or API responses.
func RedactSecrets(text string) string {
	if text == "" {
		return ""
	}
	redacted := text
	for _, pattern := range redactionPatterns {
		redacted = pattern.re.ReplaceAllString(redacted, pattern.repl)
	}
	return redacted
}
