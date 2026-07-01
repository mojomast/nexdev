package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestPreparePiLaunchMissingBinaryGivesGuidance(t *testing.T) {
	oldLookPath := lookPathBinary
	defer func() { lookPathBinary = oldLookPath }()
	lookPathBinary = func(string) (string, error) { return "", os.ErrNotExist }

	_, cleanup, err := preparePiLaunch(context.Background())
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		t.Fatal("expected missing Pi error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "pi binary not found") || !strings.Contains(msg, "nexdev tui") || strings.Contains(msg, "NEXDEV_CONTROL_TOKEN") {
		t.Fatalf("unexpected guidance: %v", err)
	}
}

func TestBuildPiEnv(t *testing.T) {
	env := buildPiEnv([]string{"PATH=/bin", "NEXDEV_CONTROL_URL=old", "NEXDEV_CONTROL_TOKEN=old"}, "http://127.0.0.1:7432", "secret-token", "/tmp/project", "run_1")
	want := map[string]string{
		"NEXDEV_CONTROL_URL":   "http://127.0.0.1:7432",
		"NEXDEV_CONTROL_TOKEN": "secret-token",
		"NEXDEV_PROJECT_DIR":   "/tmp/project",
		"NEXDEV_RUN_ID":        "run_1",
	}
	for key, value := range want {
		if got := envValue(env, key); got != value {
			t.Fatalf("%s = %q, want %q in %#v", key, got, value, env)
		}
	}
}

func TestBuildPiArgsDefaultsToOpenRouterDeepSeekWhenKeyIsSet(t *testing.T) {
	args := buildPiArgs("/tmp/index.ts", []string{"OPENROUTER_API_KEY=secret"})
	want := []string{"--extension", "/tmp/index.ts", "--provider", "openrouter", "--model", defaultOpenRouterPiModel}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestBuildPiArgsAllowsProviderModelOverrides(t *testing.T) {
	args := buildPiArgs("/tmp/index.ts", []string{
		"OPENROUTER_API_KEY=secret",
		"NEXDEV_PI_PROVIDER=openrouter",
		"NEXDEV_PI_MODEL=deepseek/deepseek-v4-pro",
	})
	want := []string{"--extension", "/tmp/index.ts", "--provider", "openrouter", "--model", "deepseek/deepseek-v4-pro"}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestLaunchPiDefaultUsesExtensionAndInheritedConfig(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	oldLookPath, oldRun, oldProjectDir, oldControlURL, oldToken, oldCacheDir := lookPathBinary, runPiProcess, projectDir, controlURL, controlToken, userCacheDir
	defer func() {
		lookPathBinary, runPiProcess = oldLookPath, oldRun
		projectDir, controlURL, controlToken = oldProjectDir, oldControlURL, oldToken
		userCacheDir = oldCacheDir
	}()
	lookPathBinary = func(string) (string, error) { return "/usr/bin/pi", nil }
	userCacheDir = func() (string, error) { return filepath.Join(t.TempDir(), "cache"), nil }
	projectDir = t.TempDir()
	controlURL = "http://127.0.0.1:7432/"
	controlToken = "test-token"

	var gotBinary string
	var gotArgs []string
	var gotEnv []string
	runPiProcess = func(ctx context.Context, binary string, args []string, env []string) error {
		gotBinary = binary
		gotArgs = append([]string{}, args...)
		gotEnv = append([]string{}, env...)
		return nil
	}

	if err := launchPiDefault(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotBinary != "/usr/bin/pi" {
		t.Fatalf("binary = %q", gotBinary)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "--extension" || !strings.HasSuffix(filepath.ToSlash(gotArgs[1]), "extensions/nexdev/index.ts") {
		t.Fatalf("args = %#v", gotArgs)
	}
	if got := envValue(gotEnv, "NEXDEV_CONTROL_URL"); got != "http://127.0.0.1:7432" {
		t.Fatalf("control url env = %q", got)
	}
	if got := envValue(gotEnv, "NEXDEV_CONTROL_TOKEN"); got != "test-token" {
		t.Fatalf("token env = %q", got)
	}
	if got := envValue(gotEnv, "NEXDEV_PROJECT_DIR"); got != projectDir {
		t.Fatalf("project env = %q, want %q", got, projectDir)
	}
}

func TestResolvePiExtensionUsesInstalledCopyCache(t *testing.T) {
	oldProjectDir, oldCacheDir, oldExecutablePath := projectDir, userCacheDir, executablePath
	defer func() {
		projectDir = oldProjectDir
		userCacheDir = oldCacheDir
		executablePath = oldExecutablePath
	}()
	projectDir = t.TempDir()
	t.Chdir(projectDir)
	exeDir := t.TempDir()
	installed := filepath.Join(exeDir, "nexdev-pi-extension")
	writePiExtensionManifest(t, installed)
	executablePath = func() (string, error) { return filepath.Join(exeDir, "nexdev"), nil }
	cacheRoot := filepath.Join(t.TempDir(), "cache")
	userCacheDir = func() (string, error) { return cacheRoot, nil }

	path, err := resolvePiExtensionPath(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := filepath.Join(cacheRoot, "nexdev", "pi-extension", piExtensionCacheVersion)
	if !strings.HasPrefix(path, wantPrefix) || filepath.Base(path) != "index.ts" {
		t.Fatalf("extension path = %q, want cached index under %q", path, wantPrefix)
	}
	for _, rel := range piExtensionManifest {
		if _, err := os.Stat(filepath.Join(wantPrefix, rel)); err != nil {
			t.Fatalf("cached manifest file %s: %v", rel, err)
		}
	}
}

func TestCopyPiExtensionRejectsManifestTraversal(t *testing.T) {
	oldManifest, oldCacheDir := piExtensionManifest, userCacheDir
	defer func() { piExtensionManifest, userCacheDir = oldManifest, oldCacheDir }()
	piExtensionManifest = []string{"../escape.ts"}
	userCacheDir = func() (string, error) { return filepath.Join(t.TempDir(), "cache"), nil }
	_, err := copyPiExtensionToCache(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "invalid Pi extension manifest path") {
		t.Fatalf("expected manifest validation error, got %v", err)
	}
}

func TestCopyPiExtensionRejectsCacheSymlinkDestination(t *testing.T) {
	if testing.Short() {
		t.Skip("symlink test skipped in short mode")
	}
	oldCacheDir := userCacheDir
	defer func() { userCacheDir = oldCacheDir }()
	source := filepath.Join(t.TempDir(), "extension")
	writePiExtensionManifest(t, source)
	cacheRoot := filepath.Join(t.TempDir(), "cache")
	userCacheDir = func() (string, error) { return cacheRoot, nil }
	destDir := filepath.Join(cacheRoot, "nexdev", "pi-extension", piExtensionCacheVersion)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "escape"), filepath.Join(destDir, "index.ts")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	_, err := copyPiExtensionToCache(source)
	if err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
}

func TestRootNoSubcommandDispatchesPiOnlyWhenInteractive(t *testing.T) {
	oldJSON, oldNoTUI, oldNoPi, oldTTY, oldRun := jsonOutput, noTUI, noPi, stdinIsTTY, runPiProcess
	defer func() {
		jsonOutput, noTUI, noPi = oldJSON, oldNoTUI, oldNoPi
		stdinIsTTY, runPiProcess = oldTTY, oldRun
	}()
	jsonOutput = false
	noTUI = false
	noPi = false
	stdinIsTTY = func() bool { return true }
	called := false
	runPiProcess = func(context.Context, string, []string, []string) error {
		called = true
		return nil
	}
	if !shouldLaunchPiByDefault() {
		t.Fatal("interactive root should launch Pi")
	}
	jsonOutput = true
	if shouldLaunchPiByDefault() {
		t.Fatal("json root should keep help/non-Pi behavior")
	}
	jsonOutput = false
	noTUI = true
	if shouldLaunchPiByDefault() {
		t.Fatal("--no-tui root should keep help/non-Pi behavior")
	}
	noTUI = false
	noPi = true
	if shouldLaunchPiByDefault() {
		t.Fatal("--no-pi root should keep Pi disabled")
	}
	if !shouldLaunchBubbleteaFallbackByDefault() {
		t.Fatal("--no-pi interactive root should select Bubbletea fallback")
	}
	if called {
		t.Fatal("shouldLaunchPiByDefault should not launch process")
	}
}

func TestRootNoSubcommandRunELaunchesPiWhenInteractive(t *testing.T) {
	oldLookPath, oldRun, oldProjectDir, oldControlURL, oldToken := lookPathBinary, runPiProcess, projectDir, controlURL, controlToken
	oldJSON, oldNoTUI, oldNoPi, oldTTY := jsonOutput, noTUI, noPi, stdinIsTTY
	defer func() {
		lookPathBinary, runPiProcess = oldLookPath, oldRun
		projectDir, controlURL, controlToken = oldProjectDir, oldControlURL, oldToken
		jsonOutput, noTUI, noPi, stdinIsTTY = oldJSON, oldNoTUI, oldNoPi, oldTTY
	}()
	lookPathBinary = func(string) (string, error) { return "/usr/bin/pi", nil }
	projectDir = t.TempDir()
	controlURL = "http://127.0.0.1:7432"
	controlToken = "test-token"
	jsonOutput = false
	noTUI = false
	noPi = false
	stdinIsTTY = func() bool { return true }
	called := false
	runPiProcess = func(ctx context.Context, binary string, args []string, env []string) error {
		called = true
		return nil
	}
	if err := rootCmd.RunE(rootCmd, nil); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("root no-subcommand did not launch Pi")
	}
}

func TestRootNoSubcommandNoPiLaunchesBubbleteaFallback(t *testing.T) {
	oldJSON, oldNoTUI, oldNoPi, oldTTY, oldFallback := jsonOutput, noTUI, noPi, stdinIsTTY, launchBubbleteaFallback
	defer func() {
		jsonOutput, noTUI, noPi = oldJSON, oldNoTUI, oldNoPi
		stdinIsTTY, launchBubbleteaFallback = oldTTY, oldFallback
	}()
	jsonOutput = false
	noTUI = false
	noPi = true
	stdinIsTTY = func() bool { return true }
	called := false
	launchBubbleteaFallback = func(cmd *cobra.Command, args []string) error {
		called = true
		return nil
	}

	if err := rootCmd.RunE(rootCmd, nil); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("root --no-pi did not launch Bubbletea fallback")
	}
}

func TestRootNoSubcommandSafeModesShowHelp(t *testing.T) {
	oldJSON, oldNoTUI, oldNoPi, oldTTY, oldRun, oldFallback := jsonOutput, noTUI, noPi, stdinIsTTY, runPiProcess, launchBubbleteaFallback
	defer func() {
		jsonOutput, noTUI, noPi = oldJSON, oldNoTUI, oldNoPi
		stdinIsTTY, runPiProcess, launchBubbleteaFallback = oldTTY, oldRun, oldFallback
	}()
	stdinIsTTY = func() bool { return false }
	runPiProcess = func(context.Context, string, []string, []string) error {
		t.Fatal("noninteractive root launched Pi")
		return nil
	}
	launchBubbleteaFallback = func(*cobra.Command, []string) error {
		t.Fatal("safe root mode launched Bubbletea fallback")
		return nil
	}

	jsonOutput = false
	noTUI = false
	noPi = false
	if err := rootCmd.RunE(rootCmd, nil); err != nil {
		t.Fatal(err)
	}
	stdinIsTTY = func() bool { return true }
	jsonOutput = true
	if err := rootCmd.RunE(rootCmd, nil); err != nil {
		t.Fatal(err)
	}
	jsonOutput = false
	noTUI = true
	if err := rootCmd.RunE(rootCmd, nil); err != nil {
		t.Fatal(err)
	}
}

func TestRunNoTUIHeadlessUnaffected(t *testing.T) {
	oldNoTUI, oldNoPi, oldFake, oldControlURL := noTUI, noPi, runFake, controlURL
	defer func() { noTUI, noPi, runFake, controlURL = oldNoTUI, oldNoPi, oldFake, oldControlURL }()
	noTUI = true
	noPi = false
	runFake = false
	controlURL = ""
	err := runCmd.RunE(runCmd, []string{"headless"})
	if err == nil || !strings.Contains(err.Error(), "local run requires --fake-provider") {
		t.Fatalf("run --no-tui behavior changed, err=%v", err)
	}
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			return strings.TrimPrefix(item, prefix)
		}
	}
	return ""
}

func writePiExtensionManifest(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range piExtensionManifest {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(rel+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
