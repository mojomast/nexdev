package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/config"
	"github.com/mojomast/nexdev/internal/controlplane"
	"github.com/mojomast/nexdev/internal/detour"
	"github.com/mojomast/nexdev/internal/executor"
	"github.com/mojomast/nexdev/internal/git"
	"github.com/mojomast/nexdev/internal/observability"
	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
)

const (
	projectIDFile     = "project_id"
	serverSecretFile  = "server.secret"
	defaultHTTPTimout = 30 * time.Second
)

type Options struct {
	ProjectDir string
	ConfigFile string
	StateDir   string
	Profile    string
}

type Runtime struct {
	ProjectRoot string
	Config      config.NexdevConfig
	StateDir    string
	ProjectID   string
	Store       *state.Store
	lock        *git.ProjectLock
}

func OpenRuntime(ctx context.Context, opts Options, acquireLock bool) (*Runtime, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	root, err := resolveProjectRoot(opts.ProjectDir)
	if err != nil {
		return nil, err
	}
	cfg, err := loadConfig(root, opts.ConfigFile)
	if err != nil {
		return nil, err
	}
	if opts.Profile != "" {
		cfg.Profile = opts.Profile
	}
	if opts.StateDir != "" {
		cfg.Project.StateDir = opts.StateDir
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	stateDir := cfg.Project.StateDir
	if !filepath.IsAbs(stateDir) {
		stateDir = filepath.Join(root, stateDir)
	}
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}

	var lock *git.ProjectLock
	if acquireLock {
		lock, err = git.AcquireProjectLock(root)
		if err != nil {
			return nil, err
		}
	}
	cleanupLock := true
	defer func() {
		if cleanupLock && lock != nil {
			_ = lock.Release()
		}
	}()

	projectID, err := ensureProjectID(stateDir)
	if err != nil {
		return nil, err
	}
	store, err := state.NewStore(filepath.Join(stateDir, "state.db"))
	if err != nil {
		return nil, err
	}
	cleanupStore := true
	defer func() {
		if cleanupStore {
			_ = store.Close()
		}
	}()
	if err := ensureProject(ctx, store, projectID, cfg, root); err != nil {
		return nil, err
	}

	cleanupLock = false
	cleanupStore = false
	return &Runtime{ProjectRoot: root, Config: cfg, StateDir: stateDir, ProjectID: projectID, Store: store, lock: lock}, nil
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	var errs []error
	if r.Store != nil {
		errs = append(errs, r.Store.Close())
		r.Store = nil
	}
	if r.lock != nil {
		errs = append(errs, r.lock.Release())
		r.lock = nil
	}
	return errors.Join(errs...)
}

func (r *Runtime) ServerConfig() (controlplane.ServerConfig, error) {
	secret, err := ensureServerSecret(r.StateDir)
	if err != nil {
		return controlplane.ServerConfig{}, err
	}
	addr := net.JoinHostPort(r.Config.ControlPlane.Bind, fmt.Sprintf("%d", r.Config.ControlPlane.Port))
	return controlplane.ServerConfig{
		Bind:                 addr,
		AuthRequired:         r.Config.ResolvedAuthRequired(),
		ServerSecret:         secret,
		ProjectID:            r.ProjectID,
		CORSAllowedOrigins:   r.Config.ControlPlane.CORSAllowOrigins,
		HeartbeatInterval:    15 * time.Second,
		ClientQueueMaxEvents: 1000,
		ReplayMaxEvents:      10000,
		RetryMS:              3000,
	}, nil
}

func (r *Runtime) NewControlPlaneServer() (*controlplane.Server, error) {
	cfg, err := r.ServerConfig()
	if err != nil {
		return nil, err
	}
	opts := []controlplane.Option{
		controlplane.WithRunStarter(&RunStarterService{runtime: r}),
	}
	if exec := r.executorForLatestRun(context.Background()); exec != nil {
		opts = append(opts, controlplane.WithExecutor(exec))
	}
	if manager, err := r.detourManager(); err == nil {
		opts = append(opts, controlplane.WithDetourManager(manager))
	}
	if os.Getenv(provider.RealProviderGateEnv) == "1" {
		opts = append(opts, controlplane.WithProviderTester(realProviderTester{}))
	}
	return controlplane.NewServer(cfg, r.Store, opts...)
}

func (r *Runtime) Serve(ctx context.Context) error {
	server, err := r.NewControlPlaneServer()
	if err != nil {
		return err
	}
	cfg, err := r.ServerConfig()
	if err != nil {
		return err
	}
	httpServer := &http.Server{Addr: cfg.Bind, Handler: server.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: defaultHTTPTimout, WriteTimeout: 0}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return ctx.Err()
}

func (r *Runtime) CreateAuthToken(ctx context.Context, role controlplane.Role, name string, ttl time.Duration) (string, *state.AuthToken, error) {
	if !controlplane.Allows(role, controlplane.RoleObserver) || role == controlplane.RoleNone || role == controlplane.RolePerTool {
		return "", nil, fmt.Errorf("invalid auth role %q", role)
	}
	secret, err := ensureServerSecret(r.StateDir)
	if err != nil {
		return "", nil, err
	}
	plain, err := controlplane.GenerateOpaqueToken()
	if err != nil {
		return "", nil, err
	}
	now := time.Now().UTC()
	var expires *time.Time
	if ttl > 0 {
		exp := now.Add(ttl).UTC()
		expires = &exp
	}
	record := &state.AuthToken{ID: newID("tok"), TokenHash: controlplane.HashBearerToken(secret, plain), Role: string(role), Name: name, CreatedAt: now, ExpiresAt: expires}
	if err := r.Store.CreateAuthToken(ctx, record); err != nil {
		return "", nil, err
	}
	return plain, record, nil
}

func (r *Runtime) executorForLatestRun(ctx context.Context) *executor.NexdevExecutor {
	runs, err := r.Store.ListRunsByProject(ctx, r.ProjectID)
	if err != nil || len(runs) == 0 {
		return nil
	}
	exec, err := executor.NewNexdevExecutor(executor.NexdevExecutorConfig{Store: r.Store, ProjectID: r.ProjectID, RunID: runs[len(runs)-1].ID, ProjectRoot: r.ProjectRoot})
	if err != nil {
		return nil
	}
	return exec
}

func (r *Runtime) detourManager() (*detour.WorkflowManager, error) {
	client, err := r.structuredProviderClient()
	if err != nil {
		return nil, err
	}
	return detour.NewWorkflowManager(detour.WorkflowManagerConfig{Store: r.Store, StructuredProvider: client})
}

func (r *Runtime) structuredProviderClient() (provider.StructuredClient, error) {
	primary := provider.Selection{Provider: r.Config.Provider.Primary.Name, Model: r.Config.Provider.Primary.Model}
	slots := make(map[provider.Slot]provider.Selection, len(r.Config.Provider.Stages))
	for name, selection := range r.Config.Provider.Stages {
		slot := provider.Slot(name)
		if !provider.IsKnownSlot(slot) {
			continue
		}
		slots[slot] = provider.Selection{Provider: selection.Name, Model: selection.Model}
	}
	router, err := provider.NewRouter(primary, slots)
	if err != nil {
		return provider.StructuredClient{}, err
	}
	providers := map[string]provider.Provider{}
	for _, selection := range append([]config.ProviderSelection{r.Config.Provider.Primary}, providerSelections(r.Config.Provider.Stages)...) {
		if selection.Name == "" {
			continue
		}
		if _, ok := providers[selection.Name]; ok {
			continue
		}
		instance, err := provider.CreateProvider(selection.Name)
		if err != nil {
			return provider.StructuredClient{}, err
		}
		if selection.APIKeyEnv != "" {
			if key := os.Getenv(selection.APIKeyEnv); key != "" {
				_ = instance.Authenticate(key)
			}
		}
		providers[selection.Name] = instance
	}
	runID := ""
	if runs, err := r.Store.ListRunsByProject(context.Background(), r.ProjectID); err == nil && len(runs) > 0 {
		runID = runs[len(runs)-1].ID
	}
	recorder := observability.NewUsageRecorder(observability.UsageRecorderConfig{Store: r.Store, ProjectID: r.ProjectID, RunID: runID, AuditCalls: true})
	return provider.StructuredClient{Router: router, Providers: providers, Recorder: recorder}, nil
}

func providerSelections(stages map[string]config.ProviderSelection) []config.ProviderSelection {
	out := make([]config.ProviderSelection, 0, len(stages))
	for _, selection := range stages {
		out = append(out, selection)
	}
	return out
}

func loadConfig(root, explicit string) (config.NexdevConfig, error) {
	path := explicit
	if path == "" {
		path = filepath.Join(root, "nexdev.yaml")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config.LoadNexdevYAML(nil)
		}
		return config.NexdevConfig{}, err
	}
	return config.LoadNexdevYAML(data)
}

func resolveProjectRoot(projectDir string) (string, error) {
	if projectDir == "" {
		return os.Getwd()
	}
	return filepath.Abs(projectDir)
}

func ensureProjectID(stateDir string) (string, error) {
	path := filepath.Join(stateDir, projectIDFile)
	data, err := os.ReadFile(path)
	if err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id, nil
		}
	}
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	id := newID("proj")
	if err := os.WriteFile(path, []byte(id+"\n"), 0600); err != nil {
		return "", err
	}
	return id, nil
}

func ensureServerSecret(stateDir string) ([]byte, error) {
	path := filepath.Join(stateDir, "run", serverSecretFile)
	if data, err := os.ReadFile(path); err == nil {
		return base64.RawURLEncoding.DecodeString(strings.TrimSpace(string(data)))
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(base64.RawURLEncoding.EncodeToString(secret)+"\n"), 0600); err != nil {
		return nil, err
	}
	return secret, nil
}

func ensureProject(ctx context.Context, store *state.Store, projectID string, cfg config.NexdevConfig, root string) error {
	if _, err := store.GetProjectWithContext(ctx, projectID); err == nil {
		return nil
	}
	name := cfg.Project.Name
	if name == "" {
		name = filepath.Base(root)
	}
	return store.CreateProject(&state.Project{ID: projectID, Name: name, CreatedAt: time.Now().UTC(), CurrentStage: state.StageInit})
}

func newID(prefix string) string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UTC().UnixNano())
	}
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(raw)
}

type realProviderTester struct{}

func (realProviderTester) TestProvider(ctx context.Context, name string) (map[string]any, error) {
	cfg, err := provider.RealProviderSmokeConfigFromEnv()
	if err != nil {
		return nil, err
	}
	if name != cfg.Provider {
		return nil, fmt.Errorf("provider test requested %q but %s=%q", name, provider.RealProviderNameEnv, cfg.Provider)
	}
	result, err := provider.RunRealProviderSmoke(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"provider":      result.Provider,
		"model":         result.Model,
		"structured_ok": result.StructuredOK,
		"attempts":      result.Attempts,
		"usage":         result.Usage,
		"estimated_usd": result.EstimatedUSD,
	}, nil
}
