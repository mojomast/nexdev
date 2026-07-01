package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/app"
)

const piExtensionRelPath = "extensions/nexdev/index.ts"
const piExtensionCacheVersion = "pi-0.80.3"

var piExtensionManifest = []string{
	"index.ts",
	"client.ts",
	"menu.ts",
	"steer.ts",
	"types.ts",
	"widgets.ts",
	"package.json",
	"tsconfig.json",
}

var (
	lookPathBinary  = exec.LookPath
	runPiProcess    = runPiProcessWithExec
	stdinIsTTY      = defaultStdinIsTTY
	userCacheDir    = os.UserCacheDir
	executablePath  = os.Executable
	writeFileAtomic = os.WriteFile
)

type piLaunchConfig struct {
	Binary        string
	Args          []string
	Env           []string
	ControlURL    string
	ExtensionPath string
	ProjectDir    string
	RunID         string
}

type piExitError struct {
	code int
	err  error
}

func (e piExitError) Error() string {
	return e.err.Error()
}

func (e piExitError) Unwrap() error {
	return e.err
}

func (e piExitError) ExitCode() int {
	return e.code
}

func shouldLaunchPiByDefault() bool {
	return !noPi && !noTUI && !jsonOutput && stdinIsTTY()
}

func shouldLaunchBubbleteaFallbackByDefault() bool {
	return noPi && !noTUI && !jsonOutput && stdinIsTTY()
}

func launchPiDefault(ctx context.Context) error {
	launch, cleanup, err := preparePiLaunch(ctx)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return err
	}
	return runPiProcess(ctx, launch.Binary, launch.Args, launch.Env)
}

func preparePiLaunch(ctx context.Context) (piLaunchConfig, func(), error) {
	binary, err := lookPathBinary("pi")
	if err != nil {
		return piLaunchConfig{}, nil, fmt.Errorf("pi binary not found on PATH; install Pi 0.80.3 or newer with Node >=22.19.0, or run `nexdev tui` for the Bubbletea fallback")
	}

	root, err := resolvePiProjectDir()
	if err != nil {
		return piLaunchConfig{}, nil, err
	}
	extensionPath, err := resolvePiExtensionPath(root)
	if err != nil {
		return piLaunchConfig{}, nil, err
	}

	controlBase := strings.TrimRight(controlURL, "/")
	runID := ""
	var cleanup func()
	if controlBase == "" {
		rt, err := app.OpenRuntime(ctx, appOptions(), true)
		if err != nil {
			return piLaunchConfig{}, nil, err
		}
		cleanup = func() { _ = rt.Close() }
		cfg, err := rt.ServerConfig()
		if err != nil {
			cleanup()
			return piLaunchConfig{}, nil, err
		}
		server, err := rt.NewControlPlaneServer()
		if err != nil {
			cleanup()
			return piLaunchConfig{}, nil, err
		}
		listener, err := net.Listen("tcp", cfg.Bind)
		if err != nil {
			cleanup()
			return piLaunchConfig{}, nil, fmt.Errorf("start local control plane for Pi at %s: %w", cfg.Bind, err)
		}
		httpServer := &http.Server{Handler: server.Handler(), ReadHeaderTimeout: 5 * time.Second}
		go func() {
			_ = httpServer.Serve(listener)
		}()
		oldCleanup := cleanup
		cleanup = func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = httpServer.Shutdown(shutdownCtx)
			oldCleanup()
		}
		controlBase = "http://" + listener.Addr().String()
		root = rt.ProjectRoot
		if runs, err := rt.Store.ListRunsByProject(ctx, rt.ProjectID); err == nil && len(runs) > 0 {
			runID = runs[len(runs)-1].ID
		}
	}

	env := buildPiEnv(os.Environ(), controlBase, effectiveToken(), root, runID)
	return piLaunchConfig{Binary: binary, Args: []string{"--extension", extensionPath}, Env: env, ControlURL: controlBase, ExtensionPath: extensionPath, ProjectDir: root, RunID: runID}, cleanup, nil
}

func buildPiEnv(base []string, controlBase, token, projectRoot, runID string) []string {
	env := append([]string{}, base...)
	env = setEnv(env, "NEXDEV_CONTROL_URL", controlBase)
	if token != "" {
		env = setEnv(env, "NEXDEV_CONTROL_TOKEN", token)
	}
	env = setEnv(env, "NEXDEV_PROJECT_DIR", projectRoot)
	if runID != "" {
		env = setEnv(env, "NEXDEV_RUN_ID", runID)
	}
	return env
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, item := range env {
		if strings.HasPrefix(item, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func resolvePiProjectDir() (string, error) {
	if strings.TrimSpace(projectDir) != "" {
		return filepath.Abs(projectDir)
	}
	return os.Getwd()
}

func resolvePiExtensionPath(projectRoot string) (string, error) {
	for _, start := range []string{projectRoot, mustGetwd()} {
		if start == "" {
			continue
		}
		if path, ok := findPiExtensionUpward(start); ok {
			return path, nil
		}
	}
	if sourceDir, ok := findInstalledPiExtensionDir(); ok {
		cachedDir, err := copyPiExtensionToCache(sourceDir)
		if err != nil {
			return "", err
		}
		return filepath.Join(cachedDir, "index.ts"), nil
	}
	return "", fmt.Errorf("Nexdev Pi extension not found; run from a source checkout with %s, install release extension files, or run `make pi-ext-build` before packaging", piExtensionRelPath)
}

func findPiExtensionUpward(start string) (string, bool) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		candidate := filepath.Join(current, piExtensionRelPath)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
		next := filepath.Dir(current)
		if next == current {
			return "", false
		}
		current = next
	}
}

func findInstalledPiExtensionDir() (string, bool) {
	var candidates []string
	if exe, err := executablePath(); err == nil && exe != "" {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "nexdev-pi-extension"),
			filepath.Join(exeDir, "extensions", "nexdev"),
		)
	}
	candidates = append(candidates,
		filepath.Join("/usr/local/share/nexdev", "pi-extension"),
		filepath.Join(os.Getenv("HOME"), ".local/share/nexdev", "pi-extension"),
	)
	for _, dir := range candidates {
		if dir == "" {
			continue
		}
		index := filepath.Join(dir, "index.ts")
		if info, err := os.Stat(index); err == nil && !info.IsDir() {
			return dir, true
		}
	}
	return "", false
}

func copyPiExtensionToCache(sourceDir string) (string, error) {
	cacheBase, err := userCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache dir for Pi extension: %w", err)
	}
	cacheDir := filepath.Join(cacheBase, "nexdev", "pi-extension", piExtensionCacheVersion)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("create Pi extension cache: %w", err)
	}
	cacheRoot, err := filepath.EvalSymlinks(cacheDir)
	if err != nil {
		return "", fmt.Errorf("validate Pi extension cache root: %w", err)
	}

	for _, rel := range piExtensionManifest {
		if err := validatePiExtensionManifestPath(rel); err != nil {
			return "", err
		}
		src := filepath.Join(sourceDir, rel)
		info, err := os.Lstat(src)
		if err != nil {
			return "", fmt.Errorf("read Pi extension source %s: %w", rel, err)
		}
		if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("Pi extension source %s must be a regular file", rel)
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return "", fmt.Errorf("read Pi extension source %s: %w", rel, err)
		}
		dst := filepath.Join(cacheDir, rel)
		if err := validatePiExtensionDestination(cacheRoot, dst); err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return "", fmt.Errorf("create Pi extension cache parent for %s: %w", rel, err)
		}
		if err := writeFileAtomic(dst, data, 0o644); err != nil {
			return "", fmt.Errorf("write Pi extension cache %s: %w", rel, err)
		}
	}
	return cacheDir, nil
}

func validatePiExtensionManifestPath(rel string) error {
	if rel == "" || filepath.IsAbs(rel) {
		return fmt.Errorf("invalid Pi extension manifest path %q", rel)
	}
	clean := filepath.Clean(rel)
	if clean != filepath.ToSlash(rel) && clean != rel {
		return fmt.Errorf("invalid Pi extension manifest path %q", rel)
	}
	if clean == "." || strings.HasPrefix(clean, "..") || strings.Contains(clean, string(filepath.Separator)+".."+string(filepath.Separator)) {
		return fmt.Errorf("invalid Pi extension manifest path %q", rel)
	}
	return nil
}

func validatePiExtensionDestination(cacheRoot, dst string) error {
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return fmt.Errorf("validate Pi extension cache parent: %w", err)
	}
	rel, err := filepath.Rel(cacheRoot, resolvedParent)
	if err != nil {
		return fmt.Errorf("validate Pi extension cache escape: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("Pi extension cache destination escapes cache root")
	}
	if info, err := os.Lstat(dst); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("Pi extension cache destination %s is a symlink", dst)
	}
	return nil
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

func defaultStdinIsTTY() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func runPiProcessWithExec(ctx context.Context, binary string, args []string, env []string) error {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 {
			return piExitError{code: exitErr.ExitCode(), err: err}
		}
		return err
	}
	return nil
}
