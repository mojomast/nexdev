package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mojomast/nexdev/internal/app"
	"github.com/mojomast/nexdev/internal/controlplane"
	"github.com/mojomast/nexdev/internal/pipeline"
	"github.com/mojomast/nexdev/internal/safety"
	"github.com/mojomast/nexdev/internal/tui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the local Nexdev control plane",
	RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := app.OpenRuntime(cmd.Context(), appOptions(), true)
		if err != nil {
			return err
		}
		defer rt.Close()
		cfg, err := rt.ServerConfig()
		if err != nil {
			return err
		}
		if !jsonOutput {
			fmt.Fprintf(cmd.ErrOrStderr(), "nexdev serving project %s on http://%s\n", rt.ProjectID, cfg.Bind)
			if cfg.AuthRequired {
				fmt.Fprintln(cmd.ErrOrStderr(), "auth required; create a token with `nexdev auth token create --role operator`")
			}
		}
		return rt.Serve(cmd.Context())
	},
}

var authCmd = &cobra.Command{Use: "auth", Short: "Manage control-plane auth"}
var authTokenCmd = &cobra.Command{Use: "token", Short: "Manage opaque bearer tokens"}

var (
	authTokenRole = "operator"
	authTokenName string
	authTokenTTL  = "30d"
)

var (
	initName          string
	initDescription   string
	initImportDevussy string
)

var authTokenCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an opaque bearer token",
	RunE: func(cmd *cobra.Command, args []string) error {
		ttl, err := parseTTL(authTokenTTL)
		if err != nil {
			return err
		}
		role := controlplane.Role(authTokenRole)
		rt, err := app.OpenRuntime(cmd.Context(), appOptions(), false)
		if err != nil {
			return err
		}
		defer rt.Close()
		plain, record, err := rt.CreateAuthToken(cmd.Context(), role, authTokenName, ttl)
		if err != nil {
			return err
		}
		out := map[string]any{"id": record.ID, "role": record.Role, "name": record.Name, "expires_at": record.ExpiresAt, "token": plain}
		return printValue(cmd, out)
	},
}

var authTokenListCmd = &cobra.Command{
	Use:   "list",
	Short: "List auth token metadata",
	RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := app.OpenRuntime(cmd.Context(), appOptions(), false)
		if err != nil {
			return err
		}
		defer rt.Close()
		tokens, err := rt.Store.ListAuthTokens(cmd.Context())
		if err != nil {
			return err
		}
		return printValue(cmd, map[string]any{"tokens": tokens})
	},
}

var authTokenRevokeCmd = &cobra.Command{
	Use:   "revoke TOKEN_ID",
	Short: "Revoke an auth token",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := app.OpenRuntime(cmd.Context(), appOptions(), false)
		if err != nil {
			return err
		}
		defer rt.Close()
		if err := rt.Store.RevokeAuthToken(cmd.Context(), args[0], time.Now().UTC()); err != nil {
			return err
		}
		return printValue(cmd, map[string]any{"revoked": args[0]})
	},
}

var eventsFollow bool
var (
	runFromStage string
	runStage     string
	runYes       bool
	runCheap     bool
	runBrrrr     bool
	runFake      bool
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Open the terminal control UI",
	RunE: func(cmd *cobra.Command, args []string) error {
		if controlURL != "" {
			return tui.Run(cmd.Context(), tui.NewHTTPClient(controlURL, effectiveToken()))
		}
		rt, err := app.OpenRuntime(cmd.Context(), appOptions(), true)
		if err != nil {
			return err
		}
		defer rt.Close()
		server, err := rt.NewControlPlaneServer()
		if err != nil {
			return err
		}
		return tui.Run(cmd.Context(), tui.NewHandlerClient(server.Handler(), effectiveToken()))
	},
}

var runCmd = &cobra.Command{
	Use:   "run [request]",
	Short: "Start or resume a Nexdev pipeline run",
	RunE: func(cmd *cobra.Command, args []string) error {
		request := strings.TrimSpace(strings.Join(args, " "))
		if controlURL != "" {
			return controlPost(cmd, "/runs", map[string]any{"prompt": request, "from_stage": runFromStage, "stage": runStage, "yes": runYes, "cheap": runCheap, "brrrr": runBrrrr})
		}
		if !runFake {
			return fmt.Errorf("local run requires --fake-provider until real provider run wiring is assigned")
		}
		rt, err := app.OpenRuntime(cmd.Context(), appOptions(), true)
		if err != nil {
			return err
		}
		defer rt.Close()
		result, err := rt.RunFakeProvider(cmd.Context(), app.RunRequest{Prompt: request, FromStage: runFromStage, Stage: runStage, Yes: runYes, Cheap: runCheap, Brrrr: runBrrrr, FakeProvider: true})
		if err != nil {
			return err
		}
		return printValue(cmd, result)
	},
}

var verifyCmd = &cobra.Command{Use: "verify", Short: "Run verification workflow", RunE: func(cmd *cobra.Command, args []string) error {
	if controlURL != "" {
		return fmt.Errorf("remote verify is deferred until a control-plane verify route is assigned")
	}
	rt, err := app.OpenRuntime(cmd.Context(), appOptions(), true)
	if err != nil {
		return err
	}
	defer rt.Close()
	runs, err := rt.Store.ListRunsByProject(cmd.Context(), rt.ProjectID)
	if err != nil {
		return err
	}
	if len(runs) == 0 {
		return fmt.Errorf("verify requires an existing run")
	}
	verifyCfg, err := loadVerifyCommandConfig(rt.ProjectRoot)
	if err != nil {
		return err
	}
	policy, err := loadProjectToolPolicy(rt.ProjectRoot, rt.Config.Security.ToolPolicyFile)
	if err != nil {
		return err
	}
	stage := pipeline.NewVerifyStage(pipeline.VerifyStageConfig{ProjectRoot: rt.ProjectRoot, Commands: verifyCfg.Commands, Policy: policy, OutputCapBytes: verifyCfg.OutputCapBytes, TotalTimeout: time.Duration(verifyCfg.TimeoutSeconds) * time.Second, RepairAttempts: verifyCfg.RepairAttempts})
	runID := runs[len(runs)-1].ID
	env := pipeline.StageEnv{Project: cliProjectRef{id: rt.ProjectID}, Run: cliRunRef{id: runID}, Store: rt.Store, Config: rt.Config}
	if err := stage.Run(cmd.Context(), env); err != nil {
		return err
	}
	out, err := stage.Output(cmd.Context(), env)
	if err != nil {
		return err
	}
	return printValue(cmd, out)
}}

var historyCmd = &cobra.Command{Use: "history", Short: "Show run history", RunE: func(cmd *cobra.Command, args []string) error { return localRead(cmd, "/events") }}

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "List or follow persisted events",
	RunE: func(cmd *cobra.Command, args []string) error {
		if eventsFollow {
			return followEvents(cmd)
		}
		if controlURL == "" {
			return localRead(cmd, "/events")
		}
		return remoteRequest(cmd.Context(), http.MethodGet, "/events", nil, cmd.OutOrStdout())
	},
}

var detourReason string
var detourCmd = &cobra.Command{
	Use:   "detour",
	Short: "Request a manual detour through the control plane",
	RunE: func(cmd *cobra.Command, args []string) error {
		body := map[string]any{"reason": detourReason, "source": "operator_manual"}
		return controlPost(cmd, "/detour", body)
	},
}

var steerCmd = &cobra.Command{
	Use:   "steer MESSAGE",
	Short: "Add steering context through executor controls",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return controlPost(cmd, "/steer", map[string]any{"message": args[0], "source": "cli"})
	},
}

var pauseReason string
var pauseCmd = &cobra.Command{Use: "pause", Short: "Pause executor controls", RunE: func(cmd *cobra.Command, args []string) error {
	return controlPost(cmd, "/pause", map[string]any{"reason": pauseReason})
}}
var cancelReason string
var cancelCmd = &cobra.Command{Use: "cancel", Short: "Cancel executor controls", RunE: func(cmd *cobra.Command, args []string) error {
	return controlPost(cmd, "/cancel", map[string]any{"reason": cancelReason})
}}

var blockersCmd = &cobra.Command{Use: "blockers", Short: "Blocker commands"}
var blockersListCmd = &cobra.Command{Use: "list", Short: "List blockers via status", RunE: func(cmd *cobra.Command, args []string) error { return localRead(cmd, "/status") }}
var blockersResolveCmd = &cobra.Command{Use: "resolve BLOCKER_ID", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	return controlPost(cmd, "/blockers/"+args[0]+"/resolve", map[string]any{"resolution": "resolved via CLI"})
}}

var providerCmd = &cobra.Command{Use: "provider", Short: "Provider commands"}
var providerListCmd = &cobra.Command{Use: "list", Short: "List provider status", RunE: func(cmd *cobra.Command, args []string) error { return localRead(cmd, "/providers") }}
var providerTestCmd = &cobra.Command{Use: "test NAME", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	return controlPost(cmd, "/providers/"+args[0]+"/test", map[string]any{})
}}

var artifactsCmd = &cobra.Command{Use: "artifacts", Short: "Artifact commands"}
var artifactsListCmd = &cobra.Command{Use: "list", Short: "List artifact metadata", RunE: func(cmd *cobra.Command, args []string) error { return localRead(cmd, "/artifacts") }}
var artifactsOpenCmd = &cobra.Command{Use: "open", Short: "Open artifact content", RunE: func(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("artifact content open is deferred until a validated artifact reader is implemented")
}}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check local Nexdev wiring",
	RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := app.OpenRuntime(cmd.Context(), appOptions(), false)
		if err != nil {
			return err
		}
		defer rt.Close()
		cfg, err := rt.ServerConfig()
		if err != nil {
			return err
		}
		return printValue(cmd, map[string]any{"ok": true, "project_id": rt.ProjectID, "state_dir": rt.StateDir, "controlplane_bind": cfg.Bind, "auth_required": cfg.AuthRequired})
	},
}

var configPrintCmd = &cobra.Command{Use: "print", Short: "Print resolved Nexdev config", RunE: func(cmd *cobra.Command, args []string) error {
	rt, err := app.OpenRuntime(cmd.Context(), appOptions(), false)
	if err != nil {
		return err
	}
	defer rt.Close()
	return printValue(cmd, rt.Config)
}}

var configValidateCmd = &cobra.Command{Use: "validate", Short: "Validate Nexdev config", RunE: func(cmd *cobra.Command, args []string) error {
	rt, err := app.OpenRuntime(cmd.Context(), appOptions(), false)
	if err != nil {
		return err
	}
	defer rt.Close()
	return printValue(cmd, map[string]any{"ok": true, "project_id": rt.ProjectID})
}}

var configSetCmd = &cobra.Command{Use: "set KEY VALUE", Short: "Set Nexdev config", Args: cobra.ExactArgs(2), RunE: notYetImplemented("config set")}

func init() {
	initCmd.Short = "Initialize Nexdev state in the current project"
	initCmd.Long = "Initialize Nexdev project-local runtime state."
	initCmd.RunE = runNexdevInit
	initCmd.Flags().StringVar(&initName, "name", "", "project name")
	initCmd.Flags().StringVar(&initDescription, "description", "", "project description")
	initCmd.Flags().StringVar(&initImportDevussy, "import-devussy", "", "import devussy artifacts from path")

	developCmd.Short = "Run pending approved development tasks"
	developCmd.RunE = notYetImplemented("develop")
	statusCmd.Short = "Display Nexdev project status"
	statusCmd.RunE = func(cmd *cobra.Command, args []string) error { return localRead(cmd, "/status") }
	planCmd.Short = "Display Nexdev plan"
	planCmd.RunE = func(cmd *cobra.Command, args []string) error { return localRead(cmd, "/plan") }
	reviewCmd.Short = "Review the current Nexdev plan"
	reviewCmd.RunE = notYetImplemented("review")
	navigateCmd.Use = "navigate STAGE"
	navigateCmd.Short = "Navigate to a Nexdev pipeline stage"
	navigateCmd.Args = cobra.ExactArgs(1)
	navigateCmd.RunE = notYetImplemented("navigate")
	resumeCmd.Short = "Resume a Nexdev run"
	resumeCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if controlURL != "" {
			return controlPost(cmd, "/resume", map[string]any{})
		}
		return notYetImplemented("local resume")(cmd, args)
	}
	configCmd.Short = "Manage Nexdev configuration"
	configCmd.Long = "Manage Nexdev configuration."
	configCmd.RunE = func(cmd *cobra.Command, args []string) error { return cmd.Help() }
	configCmd.AddCommand(configPrintCmd, configValidateCmd, configSetCmd)

	runCmd.Flags().StringVar(&runFromStage, "from", "", "start from stage")
	runCmd.Flags().StringVar(&runStage, "stage", "", "run a single stage")
	runCmd.Flags().BoolVar(&runYes, "yes", false, "assume conservative defaults")
	runCmd.Flags().BoolVar(&runCheap, "cheap", false, "prefer cheap execution profile")
	runCmd.Flags().BoolVar(&runBrrrr, "brrrr", false, "prefer maximum safe parallelism")
	runCmd.Flags().BoolVar(&runFake, "fake-provider", false, "run with the deterministic constructor-only fake provider")
	authTokenCreateCmd.Flags().StringVar(&authTokenRole, "role", "operator", "role: observer, operator, admin")
	authTokenCreateCmd.Flags().StringVar(&authTokenName, "name", "", "token name")
	authTokenCreateCmd.Flags().StringVar(&authTokenTTL, "ttl", "30d", "token TTL, e.g. 30d, 12h, 0")
	authTokenCmd.AddCommand(authTokenCreateCmd, authTokenListCmd, authTokenRevokeCmd)
	authCmd.AddCommand(authTokenCmd)
	eventsCmd.Flags().BoolVar(&eventsFollow, "follow", false, "follow event stream")
	detourCmd.Flags().StringVar(&detourReason, "reason", "", "detour reason")
	pauseCmd.Flags().StringVar(&pauseReason, "reason", "", "pause reason")
	cancelCmd.Flags().StringVar(&cancelReason, "reason", "", "cancel reason")
	blockersCmd.AddCommand(blockersListCmd, blockersResolveCmd)
	providerCmd.AddCommand(providerListCmd, providerTestCmd)
	artifactsCmd.AddCommand(artifactsListCmd, artifactsOpenCmd)
}

func runNexdevInit(cmd *cobra.Command, args []string) error {
	if strings.TrimSpace(initImportDevussy) != "" {
		return fmt.Errorf("not yet implemented: init --import-devussy")
	}
	rt, err := app.OpenRuntime(cmd.Context(), appOptions(), false)
	if err != nil {
		return err
	}
	defer rt.Close()
	out := map[string]any{"ok": true, "project_id": rt.ProjectID, "state_dir": rt.StateDir}
	if initName != "" {
		out["name"] = initName
	}
	if initDescription != "" {
		out["description"] = initDescription
	}
	return printValue(cmd, out)
}

func notYetImplemented(feature string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("not yet implemented: %s", feature)
	}
}

func appOptions() app.Options {
	return app.Options{ProjectDir: projectDir, ConfigFile: cfgFile, StateDir: stateDir, Profile: profile}
}

func controlPost(cmd *cobra.Command, path string, body map[string]any) error {
	if controlURL == "" {
		return fmt.Errorf("%s requires --control-url so it can use the control-plane service path", cmd.CommandPath())
	}
	return remoteRequest(cmd.Context(), http.MethodPost, path, body, cmd.OutOrStdout())
}

func localRead(cmd *cobra.Command, path string) error {
	if controlURL != "" {
		return remoteRequest(cmd.Context(), http.MethodGet, path, nil, cmd.OutOrStdout())
	}
	rt, err := app.OpenRuntime(cmd.Context(), appOptions(), false)
	if err != nil {
		return err
	}
	defer rt.Close()
	server, err := rt.NewControlPlaneServer()
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	rec := &responseRecorder{header: http.Header{}}
	server.Handler().ServeHTTP(rec, req)
	if rec.code >= 400 {
		return fmt.Errorf("control-plane read failed: %s", strings.TrimSpace(rec.body.String()))
	}
	_, err = io.Copy(cmd.OutOrStdout(), &rec.body)
	return err
}

func remoteRequest(ctx context.Context, method, path string, body any, out io.Writer) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(controlURL, "/")+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token := effectiveToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("control-plane request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	_, err = out.Write(data)
	return err
}

func followEvents(cmd *cobra.Command) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if controlURL != "" {
		return controlplane.FollowSSE(ctx, controlURL, cmd.OutOrStdout(), controlplane.FollowOptions{JSON: jsonOutput, Token: effectiveToken()})
	}
	rt, err := app.OpenRuntime(ctx, appOptions(), false)
	if err != nil {
		return err
	}
	defer rt.Close()
	runs, err := rt.Store.ListRunsByProject(ctx, rt.ProjectID)
	if err != nil {
		return err
	}
	if len(runs) == 0 {
		return fmt.Errorf("no runs for project %s", rt.ProjectID)
	}
	server, err := rt.NewControlPlaneServer()
	if err != nil {
		return err
	}
	return controlplane.FollowPublisher(ctx, rt.Store, server.Publisher(), cmd.OutOrStdout(), controlplane.FollowOptions{RunID: runs[len(runs)-1].ID, JSON: jsonOutput})
}

func effectiveToken() string {
	if controlToken != "" {
		return controlToken
	}
	return os.Getenv("NEXDEV_CONTROL_TOKEN")
}

func printValue(cmd *cobra.Command, v any) error {
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func parseTTL(raw string) (time.Duration, error) {
	if raw == "" || raw == "0" {
		return 0, nil
	}
	if strings.HasSuffix(raw, "d") {
		days, err := time.ParseDuration(strings.TrimSuffix(raw, "d") + "h")
		if err != nil {
			return 0, err
		}
		return days * 24, nil
	}
	return time.ParseDuration(raw)
}

type responseRecorder struct {
	code   int
	header http.Header
	body   bytes.Buffer
}

func (r *responseRecorder) Header() http.Header  { return r.header }
func (r *responseRecorder) WriteHeader(code int) { r.code = code }
func (r *responseRecorder) Write(p []byte) (int, error) {
	if r.code == 0 {
		r.code = http.StatusOK
	}
	return r.body.Write(p)
}

type cliVerifyConfig struct {
	Commands       []string `yaml:"commands"`
	TimeoutSeconds int      `yaml:"timeout_s"`
	OutputCapBytes int      `yaml:"output_cap_bytes"`
	RepairAttempts int      `yaml:"repair_attempts"`
}

func loadVerifyCommandConfig(root string) (cliVerifyConfig, error) {
	cfg := cliVerifyConfig{TimeoutSeconds: 300, OutputCapBytes: 64 * 1024, RepairAttempts: 2}
	data, err := os.ReadFile(filepath.Join(root, "nexdev.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	var raw struct {
		Verify cliVerifyConfig `yaml:"verify"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return cfg, fmt.Errorf("parse verify config: %w", err)
	}
	if raw.Verify.TimeoutSeconds > 0 {
		cfg.TimeoutSeconds = raw.Verify.TimeoutSeconds
	}
	if raw.Verify.OutputCapBytes > 0 {
		cfg.OutputCapBytes = raw.Verify.OutputCapBytes
	}
	if raw.Verify.RepairAttempts > 0 {
		cfg.RepairAttempts = raw.Verify.RepairAttempts
	}
	cfg.Commands = raw.Verify.Commands
	return cfg, nil
}

func loadProjectToolPolicy(root, policyPath string) (safety.ToolPolicy, error) {
	if strings.TrimSpace(policyPath) == "" {
		return safety.DefaultToolPolicy(), nil
	}
	if !filepath.IsAbs(policyPath) {
		policyPath = filepath.Join(root, policyPath)
	}
	data, err := os.ReadFile(policyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return safety.DefaultToolPolicy(), nil
		}
		return safety.ToolPolicy{}, err
	}
	policy, err := safety.LoadToolPolicyYAML(data)
	if err != nil {
		return policy, err
	}
	return policy, policy.Validate(safety.ProfileDev)
}

type cliProjectRef struct{ id string }
type cliRunRef struct{ id string }

func (r cliProjectRef) ProjectID() string { return r.id }
func (r cliRunRef) RunID() string         { return r.id }
