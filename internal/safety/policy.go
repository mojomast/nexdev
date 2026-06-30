package safety

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ProfileDev        = "dev"
	ProfileTrustedLAN = "trusted-lan"
	ProfileCI         = "ci"

	DecisionAllow = "allow"
	DecisionDeny  = "deny"
)

var DefaultWriteDenyGlobs = []string{
	".git",
	".git/**",
	".env",
	".env.*",
	"*_rsa",
	"**/*_rsa",
	"*_ed25519",
	"**/*_ed25519",
	"*.pem",
	"**/*.pem",
	"*.key",
	"**/*.key",
}

type ToolPolicy struct {
	ReadFile  BasicToolPolicy
	WriteFile FileToolPolicy
	Shell     ShellToolPolicy
	Network   BasicToolPolicy
}

type WriteValidationOptions struct {
	ExpectedFiles []string
	LockedFiles   []string
}

type BasicToolPolicy struct {
	Default string `yaml:"default"`
}

type FileToolPolicy struct {
	Default string   `yaml:"default"`
	Paths   []string `yaml:"paths"`
	Deny    []string `yaml:"deny"`
}

type ShellToolPolicy struct {
	Default        string   `yaml:"default"`
	AllowCommands  []string `yaml:"allow_commands"`
	TimeoutSeconds int      `yaml:"timeout_s"`
	OutputCapBytes int      `yaml:"output_cap_bytes"`
}

type CommandExecutionDecision struct {
	Allowed        bool
	Reason         string
	TimeoutSeconds int
	OutputCapBytes int
}

func DefaultToolPolicy() ToolPolicy {
	return ToolPolicy{
		ReadFile:  BasicToolPolicy{Default: DecisionAllow},
		WriteFile: FileToolPolicy{Default: DecisionAllow, Paths: []string{"**"}, Deny: append([]string{}, DefaultWriteDenyGlobs...)},
		Shell:     ShellToolPolicy{Default: DecisionDeny, TimeoutSeconds: 300, OutputCapBytes: 200000},
		Network:   BasicToolPolicy{Default: DecisionDeny},
	}
}

func LoadToolPolicyYAML(data []byte) (ToolPolicy, error) {
	policy := DefaultToolPolicy()
	if len(strings.TrimSpace(string(data))) == 0 {
		return policy, nil
	}
	var raw struct {
		Tools struct {
			ReadFile  BasicToolPolicy `yaml:"read_file"`
			WriteFile FileToolPolicy  `yaml:"write_file"`
			Shell     ShellToolPolicy `yaml:"shell"`
			Network   BasicToolPolicy `yaml:"network"`
		} `yaml:"tools"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return policy, fmt.Errorf("parse tool policy: %w", err)
	}
	if raw.Tools.ReadFile.Default != "" {
		policy.ReadFile = raw.Tools.ReadFile
	}
	if raw.Tools.WriteFile.Default != "" || len(raw.Tools.WriteFile.Paths) > 0 || len(raw.Tools.WriteFile.Deny) > 0 {
		policy.WriteFile = raw.Tools.WriteFile
		if policy.WriteFile.Default == "" {
			policy.WriteFile.Default = DecisionAllow
		}
		if len(policy.WriteFile.Deny) == 0 {
			policy.WriteFile.Deny = append([]string{}, DefaultWriteDenyGlobs...)
		}
	}
	if raw.Tools.Shell.Default != "" || len(raw.Tools.Shell.AllowCommands) > 0 || raw.Tools.Shell.TimeoutSeconds > 0 || raw.Tools.Shell.OutputCapBytes > 0 {
		policy.Shell = raw.Tools.Shell
		if policy.Shell.Default == "" {
			policy.Shell.Default = DecisionDeny
		}
	}
	if raw.Tools.Network.Default != "" {
		policy.Network = raw.Tools.Network
	}
	return policy, nil
}

func (p ToolPolicy) Validate(profile string) error {
	switch profile {
	case "", ProfileDev, ProfileTrustedLAN, ProfileCI:
	default:
		return fmt.Errorf("unknown profile %q", profile)
	}
	if p.Shell.Default != "" && p.Shell.Default != DecisionAllow && p.Shell.Default != DecisionDeny {
		return fmt.Errorf("invalid shell default %q", p.Shell.Default)
	}
	if profile == ProfileTrustedLAN || profile == ProfileCI {
		for _, command := range p.Shell.AllowCommands {
			if isWildcardShellRule(command) {
				return fmt.Errorf("wildcard shell allow rule %q is invalid in %s profile", command, profile)
			}
		}
	}
	return nil
}

func (p ToolPolicy) AllowsReadFile() bool {
	return p.ReadFile.Default != DecisionDeny
}

func (p ToolPolicy) AllowsNetwork() bool {
	return p.Network.Default == DecisionAllow
}

func (p ToolPolicy) AllowsShellCommand(command string) bool {
	return p.AuthorizeShellCommand(command).Allowed
}

func (p ToolPolicy) AuthorizeShellCommand(command string) CommandExecutionDecision {
	command = strings.TrimSpace(command)
	if strings.TrimSpace(command) == "" || p.Shell.Default != DecisionAllow && len(p.Shell.AllowCommands) == 0 {
		return CommandExecutionDecision{Allowed: false, Reason: "shell command execution denied by policy", TimeoutSeconds: p.Shell.TimeoutSeconds, OutputCapBytes: p.Shell.OutputCapBytes}
	}
	for _, allowed := range p.Shell.AllowCommands {
		if command == allowed {
			return CommandExecutionDecision{Allowed: true, Reason: "shell command explicitly allowed by policy", TimeoutSeconds: p.Shell.TimeoutSeconds, OutputCapBytes: p.Shell.OutputCapBytes}
		}
	}
	if p.Shell.Default == DecisionAllow {
		return CommandExecutionDecision{Allowed: true, Reason: "shell command allowed by default policy", TimeoutSeconds: p.Shell.TimeoutSeconds, OutputCapBytes: p.Shell.OutputCapBytes}
	}
	return CommandExecutionDecision{Allowed: false, Reason: "shell command not present in allow_commands", TimeoutSeconds: p.Shell.TimeoutSeconds, OutputCapBytes: p.Shell.OutputCapBytes}
}

func (p ToolPolicy) ValidateWritePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("path cannot be empty")
	}
	if p.WriteFile.Default == DecisionDeny {
		return errors.New("write_file is denied by default")
	}
	rel := filepath.ToSlash(filepath.Clean(path))
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, "../") {
		return fmt.Errorf("write path %q must be project-relative", path)
	}
	if matchesAnyDenyGlob(rel, p.WriteFile.Deny) {
		return fmt.Errorf("write path %q is denied by policy", path)
	}
	if len(p.WriteFile.Paths) == 0 {
		return nil
	}
	for _, allowed := range p.WriteFile.Paths {
		allowed = filepath.ToSlash(filepath.Clean(allowed))
		if allowed == "**" || allowed == rel {
			return nil
		}
		if strings.HasSuffix(allowed, "/**") {
			prefix := strings.TrimSuffix(allowed, "/**")
			if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
				return nil
			}
		}
		if ok, _ := filepath.Match(allowed, rel); ok {
			return nil
		}
	}
	return fmt.Errorf("write path %q is not allowed by policy", path)
}

func (p ToolPolicy) ValidateTaskWritePath(path string, opts WriteValidationOptions) error {
	if err := p.ValidateWritePath(path); err != nil {
		return err
	}
	rel, err := cleanProjectRelative(path)
	if err != nil {
		return err
	}
	if matchesAnyDenyGlob(rel, opts.LockedFiles) {
		return fmt.Errorf("write path %q is locked", path)
	}
	if len(opts.ExpectedFiles) == 0 {
		return errors.New("task expected files are required for writes")
	}
	if !matchesAnyDenyGlob(rel, opts.ExpectedFiles) {
		return fmt.Errorf("write path %q is outside task expected files", path)
	}
	return nil
}

func cleanProjectRelative(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("path cannot be empty")
	}
	rel := filepath.ToSlash(filepath.Clean(path))
	if filepath.IsAbs(path) || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("write path %q must be project-relative", path)
	}
	return rel, nil
}

func isWildcardShellRule(command string) bool {
	return strings.ContainsAny(command, "*?")
}
