package safety

import (
	"regexp"
	"strings"
)

type PromptInjectionFinding struct {
	Pattern  string
	Message  string
	Severity string
}

var promptInjectionPatterns = []struct {
	name     string
	message  string
	severity string
	re       *regexp.Regexp
}{
	{"ignore-previous-instructions", "untrusted text asks the model to ignore or override prior instructions", "high", regexp.MustCompile(`(?i)\b(ignore|disregard|forget)\s+(all\s+)?(previous|prior|above)\s+(instructions|rules|messages)\b`)},
	{"system-prompt-exfiltration", "untrusted text asks to reveal hidden system, developer, or policy content", "high", regexp.MustCompile(`(?i)\b(reveal|print|show|dump|exfiltrate)\s+(the\s+)?(system|developer|hidden|secret)\s+(prompt|message|instructions|policy|rules|secrets?)\b`)},
	{"role-override", "untrusted text attempts to assign a new role or authority", "medium", regexp.MustCompile(`(?i)\b(you\s+are\s+now|act\s+as|developer\s+mode|jailbreak|sudo\s+mode)\b`)},
	{"safety-policy-override", "untrusted text attempts to bypass safety or tool policy", "high", regexp.MustCompile(`(?i)\b(bypass|disable|override)\s+(safety|security|tool\s+policy|policy|guardrails|permissions)\b`)},
	{"secret-exfiltration", "untrusted text asks to leak credentials or environment secrets", "high", regexp.MustCompile(`(?i)\b(send|upload|post|curl|exfiltrate|leak)\s+.*\b(env|environment|api\s*key|token|password|secret|credentials?)\b`)},
}

// DetectPromptInjection returns warning findings for untrusted text. It does not
// enforce policy; callers decide how to surface findings, such as events or review notes.
func DetectPromptInjection(text string) []PromptInjectionFinding {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	findings := make([]PromptInjectionFinding, 0)
	for _, pattern := range promptInjectionPatterns {
		if pattern.re.MatchString(text) {
			findings = append(findings, PromptInjectionFinding{
				Pattern:  pattern.name,
				Message:  pattern.message,
				Severity: pattern.severity,
			})
		}
	}
	return findings
}
