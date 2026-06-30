package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/safety"
	"github.com/mojomast/nexdev/internal/state"
)

const (
	verifyReportArtifactRelPath = ".nexdev/artifacts/verify_report.json"
	changedFilesArtifactRelPath = ".nexdev/artifacts/changed_files.json"
	handoffArtifactRelPath      = ".nexdev/artifacts/handoff.md"
	runSummaryArtifactRelPath   = ".nexdev/artifacts/run_summary.json"
	defaultVerifyOutputCapBytes = 64 * 1024
	defaultVerifyTimeout        = 5 * time.Minute
	defaultVerifyRepairAttempts = 2
)

type VerifyStageConfig struct {
	ProjectRoot    string
	Commands       []string
	Policy         safety.ToolPolicy
	OutputCapBytes int
	TotalTimeout   time.Duration
	RepairAttempts int
	RepairFunc     func(context.Context, VerifyRepairAttempt) error
	Now            func() time.Time
}

type VerifyStage struct {
	config VerifyStageConfig
	report verifyReport
	wrote  bool
	now    func() time.Time
}

type verifyReport struct {
	Passed         bool                   `json:"passed"`
	Commands       []verifyCommandResult  `json:"commands"`
	ChangedFiles   []contract.ChangedFile `json:"changed_files"`
	Failures       []contract.Finding     `json:"failures"`
	Warnings       []contract.Finding     `json:"warnings"`
	RepairAttempts []VerifyRepairAttempt  `json:"repair_attempts,omitempty"`
}

type verifyCommandResult struct {
	Command         string `json:"command"`
	Passed          bool   `json:"passed"`
	PolicyDenied    bool   `json:"policy_denied"`
	ExitCode        int    `json:"exit_code"`
	TimedOut        bool   `json:"timed_out"`
	Attempts        int    `json:"attempts"`
	OutputTruncated bool   `json:"output_truncated"`
	StdoutTail      string `json:"stdout_tail"`
	StderrTail      string `json:"stderr_tail"`
	OutputSHA256    string `json:"output_sha256"`
	StartedAt       string `json:"started_at"`
	CompletedAt     string `json:"completed_at"`
}

type VerifyRepairAttempt struct {
	Command string `json:"command"`
	Attempt int    `json:"attempt"`
	Reason  string `json:"reason"`
	At      string `json:"at"`
}

func NewVerifyStage(cfg VerifyStageConfig) *VerifyStage {
	return &VerifyStage{config: cfg, now: normalizeStageClock(cfg.Now)}
}

func (s *VerifyStage) setClock(now func() time.Time) { s.now = normalizeStageClock(now) }

func (s *VerifyStage) Name() Stage { return StageVerify }

func (s *VerifyStage) Validate(ctx context.Context, env StageEnv) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if env.Project == nil || env.Run == nil {
		return fmt.Errorf("verify requires project and run")
	}
	if _, ok := env.Store.(*state.Store); !ok {
		return fmt.Errorf("verify requires *state.Store")
	}
	return nil
}

func (s *VerifyStage) Run(ctx context.Context, env StageEnv) error {
	if err := s.Validate(ctx, env); err != nil {
		return err
	}
	if err := persistVerifyEvent(ctx, env, contract.EventTypeVerifyStarted, "started", map[string]any{"command_count": len(s.commands())}); err != nil {
		return err
	}
	changed, err := detectChangedFiles(ctx, env.Store.(*state.Store), env.Run.RunID(), s.projectRoot())
	if err != nil {
		return err
	}
	commands := make([]verifyCommandResult, 0, len(s.commands()))
	repairs := []VerifyRepairAttempt{}
	deadline := s.now().Add(s.totalTimeout())
	passed := true
	for commandIndex, command := range s.commands() {
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}
		if !s.now().Before(deadline) {
			result := s.pendingTimedOutResult(command)
			commands = append(commands, result)
			passed = false
			continue
		}
		result := s.runCommandWithRepairs(ctx, env, commandIndex, command, deadline, &repairs)
		commands = append(commands, result)
		if !result.Passed {
			passed = false
		}
	}
	report := verifyReport{Passed: passed, Commands: commands, ChangedFiles: changed, RepairAttempts: repairs}
	for _, command := range commands {
		if !command.Passed {
			report.Failures = append(report.Failures, contract.Finding{Severity: "medium", Title: "Verification command failed", Description: command.Command})
		}
	}
	if err := writeStageArtifact(ctx, env, s.projectRoot(), verifyReportArtifactRelPath, contract.ArtifactKindVerifyReport, StageVerify, report, s.now); err != nil {
		return err
	}
	if err := persistVerifyEvent(ctx, env, contract.EventTypeVerifyCompleted, "completed", map[string]any{"passed": report.Passed, "changed_file_count": len(changed)}); err != nil {
		return err
	}
	s.report = report
	s.wrote = true
	return nil
}

func (s *VerifyStage) Resume(ctx context.Context, env StageEnv) error { return s.Run(ctx, env) }

func (s *VerifyStage) Output(ctx context.Context, env StageEnv) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !s.wrote {
		return map[string]any{}, nil
	}
	return structToMap(s.report)
}

func (s *VerifyStage) projectRoot() string {
	if strings.TrimSpace(s.config.ProjectRoot) == "" {
		return "."
	}
	return s.config.ProjectRoot
}

func (s *VerifyStage) commands() []string { return append([]string{}, s.config.Commands...) }

func (s *VerifyStage) policy() safety.ToolPolicy {
	policy := s.config.Policy
	if policy.Shell.Default == "" && len(policy.Shell.AllowCommands) == 0 && policy.Shell.TimeoutSeconds == 0 && policy.Shell.OutputCapBytes == 0 {
		policy = safety.DefaultToolPolicy()
	}
	return policy
}

func (s *VerifyStage) outputCapBytes() int {
	if s.config.OutputCapBytes > 0 {
		return s.config.OutputCapBytes
	}
	return defaultVerifyOutputCapBytes
}

func (s *VerifyStage) totalTimeout() time.Duration {
	if s.config.TotalTimeout > 0 {
		return s.config.TotalTimeout
	}
	return defaultVerifyTimeout
}

func (s *VerifyStage) repairAttempts() int {
	if s.config.RepairAttempts > 0 {
		return s.config.RepairAttempts
	}
	return defaultVerifyRepairAttempts
}

func (s *VerifyStage) pendingTimedOutResult(command string) verifyCommandResult {
	now := s.now().Format(time.RFC3339Nano)
	sum := sha256.Sum256([]byte("timed-out-before-start:" + command))
	return verifyCommandResult{Command: command, Passed: false, ExitCode: -1, TimedOut: true, Attempts: 0, StderrTail: "verify total timeout exceeded before command started", OutputSHA256: hex.EncodeToString(sum[:]), StartedAt: now, CompletedAt: now}
}

func (s *VerifyStage) runCommandWithRepairs(ctx context.Context, env StageEnv, commandIndex int, command string, deadline time.Time, repairs *[]VerifyRepairAttempt) verifyCommandResult {
	var result verifyCommandResult
	maxAttempts := 1 + s.repairAttempts()
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result = s.runCommandOnce(ctx, command, deadline)
		result.Attempts = attempt
		_ = persistVerifyEvent(ctx, env, contract.EventTypeVerifyCommandOutput, fmt.Sprintf("command_%d_attempt_%d", commandIndex, attempt), map[string]any{"command": command, "attempt": attempt, "passed": result.Passed, "timed_out": result.TimedOut, "output_truncated": result.OutputTruncated})
		if result.Passed || result.PolicyDenied || attempt == maxAttempts || result.TimedOut {
			return result
		}
		repair := VerifyRepairAttempt{Command: command, Attempt: attempt, Reason: "verification command failed", At: s.now().Format(time.RFC3339Nano)}
		*repairs = append(*repairs, repair)
		if s.config.RepairFunc != nil {
			if err := s.config.RepairFunc(ctx, repair); err != nil {
				result.StderrTail = appendTail(result.StderrTail, "repair attempt failed: "+err.Error(), s.outputCapBytes())
				return result
			}
		}
	}
	return result
}

func (s *VerifyStage) runCommandOnce(ctx context.Context, command string, deadline time.Time) verifyCommandResult {
	started := s.now()
	result := verifyCommandResult{Command: command, ExitCode: -1, StartedAt: started.Format(time.RFC3339Nano)}
	decision := s.policy().AuthorizeShellCommand(command)
	if !decision.Allowed {
		sum := sha256.Sum256([]byte("policy-denied:" + command))
		result.PolicyDenied = true
		result.ExitCode = 126
		result.StderrTail = decision.Reason
		result.OutputSHA256 = hex.EncodeToString(sum[:])
		result.CompletedAt = s.now().Format(time.RFC3339Nano)
		return result
	}
	parts := strings.Fields(command)
	if len(parts) == 0 {
		result.StderrTail = "command cannot be empty"
		result.CompletedAt = s.now().Format(time.RFC3339Nano)
		return result
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return s.pendingTimedOutResult(command)
	}
	runCtx, cancel := context.WithTimeout(ctx, remaining)
	defer cancel()
	stdout := &cappedOutput{cap: s.outputCapBytes()}
	stderr := &cappedOutput{cap: s.outputCapBytes()}
	cmd := exec.CommandContext(runCtx, parts[0], parts[1:]...)
	cmd.Dir = s.projectRoot()
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	result.TimedOut = runCtx.Err() == context.DeadlineExceeded
	result.StdoutTail = stdout.String()
	result.StderrTail = stderr.String()
	result.OutputTruncated = stdout.truncated || stderr.truncated
	result.CompletedAt = s.now().Format(time.RFC3339Nano)
	sum := sha256.Sum256([]byte(result.StdoutTail + result.StderrTail))
	result.OutputSHA256 = hex.EncodeToString(sum[:])
	if err == nil {
		result.ExitCode = 0
		result.Passed = true
		return result
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	} else if result.TimedOut {
		result.ExitCode = -1
		result.StderrTail = appendTail(result.StderrTail, "command timed out", s.outputCapBytes())
	} else {
		result.StderrTail = appendTail(result.StderrTail, err.Error(), s.outputCapBytes())
	}
	return result
}

type cappedOutput struct {
	cap       int
	buf       []byte
	truncated bool
}

func (w *cappedOutput) Write(p []byte) (int, error) {
	if w.cap <= 0 {
		w.cap = defaultVerifyOutputCapBytes
	}
	remaining := w.cap - len(w.buf)
	if remaining > 0 {
		if len(p) <= remaining {
			w.buf = append(w.buf, p...)
		} else {
			w.buf = append(w.buf, p[:remaining]...)
			w.truncated = true
		}
	} else if len(p) > 0 {
		w.truncated = true
	}
	return len(p), nil
}

func (w *cappedOutput) String() string { return string(w.buf) }

func appendTail(existing, msg string, capBytes int) string {
	if strings.TrimSpace(msg) == "" {
		return existing
	}
	if existing != "" {
		existing += "\n"
	}
	existing += msg
	if capBytes > 0 && len(existing) > capBytes {
		return existing[len(existing)-capBytes:]
	}
	return existing
}

type HandoffStageConfig struct {
	ProjectRoot string
	Request     string
	Now         func() time.Time
}

type HandoffStage struct {
	config HandoffStageConfig
	output map[string]any
	now    func() time.Time
}

func NewHandoffStage(cfg HandoffStageConfig) *HandoffStage {
	return &HandoffStage{config: cfg, now: normalizeStageClock(cfg.Now)}
}

func (s *HandoffStage) setClock(now func() time.Time) { s.now = normalizeStageClock(now) }

func (s *HandoffStage) Name() Stage { return StageHandoff }

func (s *HandoffStage) Validate(ctx context.Context, env StageEnv) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if env.Project == nil || env.Run == nil {
		return fmt.Errorf("handoff requires project and run")
	}
	if _, ok := env.Store.(*state.Store); !ok {
		return fmt.Errorf("handoff requires *state.Store")
	}
	return nil
}

func (s *HandoffStage) Run(ctx context.Context, env StageEnv) error {
	if err := s.Validate(ctx, env); err != nil {
		return err
	}
	store := env.Store.(*state.Store)
	changed, err := detectChangedFiles(ctx, store, env.Run.RunID(), s.projectRoot())
	if err != nil {
		return err
	}
	if err := writeStageArtifact(ctx, env, s.projectRoot(), changedFilesArtifactRelPath, contract.ArtifactKindChangedFiles, StageHandoff, map[string]any{"changed_files": changed}, s.now); err != nil {
		return err
	}
	run, err := store.GetRun(ctx, env.Run.RunID())
	if err != nil {
		return err
	}
	stages, err := store.ListStageRunsByRun(ctx, env.Run.RunID())
	if err != nil {
		return err
	}
	summary := contract.RunSummary{ProjectID: env.Project.ProjectID(), RunID: env.Run.RunID(), Status: run.Status, StartedAt: run.StartedAt.UTC().Format(time.RFC3339Nano), ChangedFiles: changed}
	if run.CompletedAt != nil {
		summary.CompletedAt = run.CompletedAt.UTC().Format(time.RFC3339Nano)
	}
	for _, stage := range stages {
		item := contract.StageSummary{Stage: stage.Stage, Status: stage.Status}
		if stage.StartedAt != nil {
			item.StartedAt = stage.StartedAt.UTC().Format(time.RFC3339Nano)
		}
		if stage.CompletedAt != nil {
			item.CompletedAt = stage.CompletedAt.UTC().Format(time.RFC3339Nano)
		}
		summary.Stages = append(summary.Stages, item)
	}
	if err := writeStageArtifact(ctx, env, s.projectRoot(), runSummaryArtifactRelPath, contract.ArtifactKindRunSummary, StageHandoff, summary, s.now); err != nil {
		return err
	}
	markdown := renderHandoffMarkdown(s.config.Request, summary, changed)
	if err := writeMarkdownStageArtifact(ctx, env, s.projectRoot(), handoffArtifactRelPath, contract.ArtifactKindHandoff, StageHandoff, markdown, s.now); err != nil {
		return err
	}
	s.output = map[string]any{"handoff": handoffArtifactRelPath, "changed_files": changedFilesArtifactRelPath, "run_summary": runSummaryArtifactRelPath}
	return nil
}

func (s *HandoffStage) Resume(ctx context.Context, env StageEnv) error { return s.Run(ctx, env) }

func (s *HandoffStage) Output(ctx context.Context, env StageEnv) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.output == nil {
		return map[string]any{}, nil
	}
	return s.output, nil
}

func (s *HandoffStage) projectRoot() string {
	if strings.TrimSpace(s.config.ProjectRoot) == "" {
		return "."
	}
	return s.config.ProjectRoot
}

type CompleteStage struct{}

func NewCompleteStage() *CompleteStage                                    { return &CompleteStage{} }
func (s *CompleteStage) Name() Stage                                      { return StageComplete }
func (s *CompleteStage) Validate(ctx context.Context, env StageEnv) error { return ctx.Err() }
func (s *CompleteStage) Run(ctx context.Context, env StageEnv) error      { return ctx.Err() }
func (s *CompleteStage) Resume(ctx context.Context, env StageEnv) error   { return s.Run(ctx, env) }
func (s *CompleteStage) Output(ctx context.Context, env StageEnv) (map[string]any, error) {
	return map[string]any{"complete": true}, ctx.Err()
}

func detectChangedFiles(ctx context.Context, store *state.Store, runID, root string) ([]contract.ChangedFile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tasks, err := store.ListNexdevTasks(ctx, state.NexdevTaskListOptions{RunID: runID})
	if err != nil {
		return nil, err
	}
	owners := map[string]map[string]bool{}
	for _, task := range tasks {
		for _, expected := range task.Spec.ExpectedFiles {
			for _, path := range expandExpectedPath(root, expected) {
				if owners[path] == nil {
					owners[path] = map[string]bool{}
				}
				owners[path][task.Spec.ID] = true
			}
		}
	}
	paths := make([]string, 0, len(owners))
	for path := range owners {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	changed := make([]contract.ChangedFile, 0, len(paths))
	for _, rel := range paths {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		hash := sha256.Sum256(data)
		ownerIDs := make([]string, 0, len(owners[rel]))
		for id := range owners[rel] {
			ownerIDs = append(ownerIDs, id)
		}
		sort.Strings(ownerIDs)
		changed = append(changed, contract.ChangedFile{Path: rel, Status: "modified", SHA256: hex.EncodeToString(hash[:]), ByteSize: int64(len(data)), OwningTasks: ownerIDs})
	}
	return changed, nil
}

func expandExpectedPath(root, pattern string) []string {
	pattern = filepath.ToSlash(filepath.Clean(strings.TrimSpace(pattern)))
	if pattern == "" || pattern == "." || strings.ContainsAny(pattern, "*?[") {
		return nil
	}
	abs := filepath.Join(root, pattern)
	info, err := os.Stat(abs)
	if err != nil {
		return nil
	}
	if !info.IsDir() {
		return []string{pattern}
	}
	var out []string
	_ = filepath.WalkDir(abs, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err == nil && !strings.HasPrefix(filepath.ToSlash(rel), ".nexdev/") {
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out
}

func persistVerifyEvent(ctx context.Context, env StageEnv, eventType, suffix string, payload any) error {
	store, ok := env.Store.(*state.Store)
	if !ok || store == nil || env.Project == nil || env.Run == nil {
		return nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	seed := env.Project.ProjectID() + ":" + env.Run.RunID() + ":" + string(StageVerify) + ":" + eventType + ":" + suffix
	hash := sha256.Sum256([]byte(seed))
	_, err = store.PersistEvent(ctx, contract.EventEnvelope{EventID: "evt_verify_" + hex.EncodeToString(hash[:8]), ProjectID: env.Project.ProjectID(), RunID: env.Run.RunID(), Stage: string(StageVerify), Type: eventType, Source: contract.EventSourceCore, Timestamp: time.Now().UTC(), Payload: data})
	return err
}

func renderHandoffMarkdown(request string, summary contract.RunSummary, changed []contract.ChangedFile) string {
	var b strings.Builder
	b.WriteString("# Nexdev Handoff\n\n")
	b.WriteString("## Request\n")
	b.WriteString("- ")
	b.WriteString(safety.RedactSecrets(singleLine(request)))
	b.WriteString("\n\n## Summary\n")
	b.WriteString("- Run: ")
	b.WriteString(summary.RunID)
	b.WriteString("\n- Status: ")
	b.WriteString(summary.Status)
	b.WriteString("\n\n## Changed Files\n")
	if len(changed) == 0 {
		b.WriteString("- None\n")
	} else {
		for _, file := range changed {
			b.WriteString("- ")
			b.WriteString(file.Path)
			b.WriteString("\n")
		}
	}
	b.WriteString("\n## Verification\n- Fake-provider E2E verify stage completed without executing shell or network commands.\n\n## Open Risks\n- Real provider and policy-gated shell verification remain opt-in/follow-up.\n")
	return b.String()
}

var _ PipelineStage = (*VerifyStage)(nil)
var _ StageOutputter = (*VerifyStage)(nil)
var _ PipelineStage = (*HandoffStage)(nil)
var _ StageOutputter = (*HandoffStage)(nil)
var _ PipelineStage = (*CompleteStage)(nil)
var _ StageOutputter = (*CompleteStage)(nil)
