package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/config"
	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/safety"
	"github.com/mojomast/nexdev/internal/state"
)

const (
	repoAnalysisArtifactRelPath = ".nexdev/artifacts/repo_analysis.json"
	defaultRepoAnalyzeMaxFile   = 200000
	defaultRepoAnalyzeMaxCtx    = 500000
	repoInstructionSnippetBytes = 1200
)

var defaultRepoAnalyzeExcludeGlobs = []string{".git/**", "node_modules/**", "vendor/**", "dist/**", "build/**", ".nexdev/**"}

type RepoAnalyzeStage struct {
	projectRoot string
	analysis    contract.RepoAnalysis
	wrote       bool
}

func NewRepoAnalyzeStage(projectRoot string) *RepoAnalyzeStage {
	return &RepoAnalyzeStage{projectRoot: projectRoot}
}

func (s *RepoAnalyzeStage) Name() Stage { return StageRepoAnalyze }

func (s *RepoAnalyzeStage) Validate(ctx context.Context, env StageEnv) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if env.Project == nil {
		return fmt.Errorf("repo_analyze requires project")
	}
	if strings.TrimSpace(s.projectRoot) == "" {
		return fmt.Errorf("repo_analyze project root is required")
	}
	info, err := os.Stat(s.projectRoot)
	if err != nil {
		return fmt.Errorf("repo_analyze project root: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("repo_analyze project root is not a directory: %s", s.projectRoot)
	}
	return nil
}

func (s *RepoAnalyzeStage) Run(ctx context.Context, env StageEnv) error {
	if err := s.Validate(ctx, env); err != nil {
		return err
	}
	settings := repoAnalyzeSettingsFromEnv(env)
	analysis, err := analyzeRepo(ctx, s.projectRoot, settings)
	if err != nil {
		return err
	}
	if err := s.writeArtifact(ctx, env, analysis); err != nil {
		return err
	}
	s.analysis = analysis
	s.wrote = true
	return nil
}

func (s *RepoAnalyzeStage) Resume(ctx context.Context, env StageEnv) error {
	return s.Run(ctx, env)
}

func (s *RepoAnalyzeStage) Output(ctx context.Context, env StageEnv) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !s.wrote {
		return map[string]any{}, nil
	}
	data, err := json.Marshal(s.analysis)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *RepoAnalyzeStage) Analysis() contract.RepoAnalysis {
	return s.analysis
}

type repoAnalyzeSettings struct {
	MaxFileBytes    int64
	MaxContextBytes int64
	IncludeGlobs    []string
	ExcludeGlobs    []string
}

func repoAnalyzeSettingsFromEnv(env StageEnv) repoAnalyzeSettings {
	settings := repoAnalyzeSettings{
		MaxFileBytes:    defaultRepoAnalyzeMaxFile,
		MaxContextBytes: defaultRepoAnalyzeMaxCtx,
		ExcludeGlobs:    append([]string(nil), defaultRepoAnalyzeExcludeGlobs...),
	}
	if cfg, ok := env.Config.(config.NexdevConfig); ok {
		if cfg.RepoAnalyze.MaxFileBytes > 0 {
			settings.MaxFileBytes = int64(cfg.RepoAnalyze.MaxFileBytes)
		}
		if cfg.RepoAnalyze.MaxContextBytes > 0 {
			settings.MaxContextBytes = int64(cfg.RepoAnalyze.MaxContextBytes)
		}
		if len(cfg.RepoAnalyze.IncludeGlobs) > 0 {
			settings.IncludeGlobs = append([]string(nil), cfg.RepoAnalyze.IncludeGlobs...)
		}
		if len(cfg.RepoAnalyze.ExcludeGlobs) > 0 {
			settings.ExcludeGlobs = append([]string(nil), cfg.RepoAnalyze.ExcludeGlobs...)
		}
	}
	return settings
}

func analyzeRepo(ctx context.Context, root string, settings repoAnalyzeSettings) (contract.RepoAnalysis, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return contract.RepoAnalysis{}, err
	}
	state := &repoAnalyzeAccumulator{
		languages:        map[string]bool{},
		frameworks:       map[string]bool{},
		packageManagers:  map[string]bool{},
		testCommands:     map[string]bool{},
		lintCommands:     map[string]bool{},
		entrypoints:      map[string]bool{},
		importantFiles:   map[string]bool{},
		forbiddenPaths:   map[string]bool{},
		repoInstructions: map[string]bool{},
		riskNotes:        map[string]bool{},
	}
	contextBytes := int64(0)

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			state.riskNotes["skipped unreadable path: "+relPath(root, path)] = true
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel := relPath(root, path)
		if d.IsDir() {
			if excludedPath(rel, settings.ExcludeGlobs) || alwaysExcludedDir(rel) {
				state.forbiddenPaths[rel+"/"] = true
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if excludedPath(rel, settings.ExcludeGlobs) || secretPath(rel) {
			state.forbiddenPaths[rel] = true
			if secretPath(rel) {
				state.riskNotes["secret-like file excluded from repo analysis: "+rel] = true
			}
			return nil
		}
		if len(settings.IncludeGlobs) > 0 && !matchesAny(rel, settings.IncludeGlobs) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			state.riskNotes["skipped unreadable file metadata: "+rel] = true
			return nil
		}
		if info.Size() > settings.MaxFileBytes {
			state.riskNotes[fmt.Sprintf("skipped large file over %d bytes: %s", settings.MaxFileBytes, rel)] = true
			return nil
		}
		if !candidateRepoAnalyzeFile(rel) {
			return nil
		}
		state.importantFiles[rel] = true
		data, err := os.ReadFile(path)
		if err != nil {
			state.riskNotes["skipped unreadable file: "+rel] = true
			return nil
		}
		if contextBytes+int64(len(data)) > settings.MaxContextBytes {
			state.riskNotes[fmt.Sprintf("repo analysis context cap reached before: %s", rel)] = true
			return nil
		}
		contextBytes += int64(len(data))
		text := string(data)
		analyzeFile(rel, text, state)
		return nil
	})
	if err != nil {
		return contract.RepoAnalysis{}, err
	}

	analysis := contract.RepoAnalysis{
		Languages:        sortedKeys(state.languages),
		Frameworks:       sortedKeys(state.frameworks),
		PackageManagers:  sortedKeys(state.packageManagers),
		TestCommands:     sortedKeys(state.testCommands),
		LintCommands:     sortedKeys(state.lintCommands),
		Entrypoints:      sortedKeys(state.entrypoints),
		ImportantFiles:   sortedKeys(state.importantFiles),
		ForbiddenPaths:   sortedKeys(state.forbiddenPaths),
		RepoInstructions: sortedKeys(state.repoInstructions),
		RiskNotes:        sortedKeys(state.riskNotes),
	}
	analysis.Summary = repoAnalysisSummary(analysis)
	return analysis, nil
}

type repoAnalyzeAccumulator struct {
	languages        map[string]bool
	frameworks       map[string]bool
	packageManagers  map[string]bool
	testCommands     map[string]bool
	lintCommands     map[string]bool
	entrypoints      map[string]bool
	importantFiles   map[string]bool
	forbiddenPaths   map[string]bool
	repoInstructions map[string]bool
	riskNotes        map[string]bool
}

func analyzeFile(rel, text string, state *repoAnalyzeAccumulator) {
	base := strings.ToLower(filepath.Base(rel))
	ext := strings.ToLower(filepath.Ext(rel))

	switch ext {
	case ".go":
		state.languages["go"] = true
	case ".js", ".jsx":
		state.languages["javascript"] = true
	case ".ts", ".tsx":
		state.languages["typescript"] = true
	case ".py":
		state.languages["python"] = true
	case ".rs":
		state.languages["rust"] = true
	case ".rb":
		state.languages["ruby"] = true
	case ".java":
		state.languages["java"] = true
	case ".cs":
		state.languages["csharp"] = true
	}
	if base == "main.go" && (rel == "main.go" || strings.HasPrefix(rel, "cmd/")) {
		state.entrypoints[rel] = true
	}

	switch base {
	case "readme.md", "readme", "agents.md", "contributing.md", "contributing":
		captureRepoInstruction(rel, text, state)
	case "go.mod":
		state.languages["go"] = true
		state.packageManagers["go"] = true
		state.testCommands["go test ./..."] = true
		state.lintCommands["go vet ./..."] = true
		detectGoFrameworks(text, state)
	case "go.sum":
		state.packageManagers["go"] = true
	case "package.json":
		state.packageManagers["npm"] = true
		detectPackageJSON(text, state)
	case "package-lock.json", "npm-shrinkwrap.json":
		state.packageManagers["npm"] = true
	case "yarn.lock":
		state.packageManagers["yarn"] = true
	case "pnpm-lock.yaml":
		state.packageManagers["pnpm"] = true
	case "tsconfig.json":
		state.languages["typescript"] = true
	case "vite.config.js", "vite.config.ts":
		state.frameworks["vite"] = true
	case "pyproject.toml":
		state.languages["python"] = true
		state.packageManagers["python"] = true
		state.testCommands["pytest"] = true
		if strings.Contains(text, "[tool.poetry]") {
			state.packageManagers["poetry"] = true
		}
	case "requirements.txt", "requirements-dev.txt":
		state.languages["python"] = true
		state.packageManagers["pip"] = true
	case "cargo.toml":
		state.languages["rust"] = true
		state.packageManagers["cargo"] = true
		state.testCommands["cargo test"] = true
		state.lintCommands["cargo clippy"] = true
	case "gemfile":
		state.languages["ruby"] = true
		state.packageManagers["bundler"] = true
		state.testCommands["bundle exec rspec"] = true
	case "makefile":
		detectMakeTargets(text, state)
	}
}

func detectGoFrameworks(text string, state *repoAnalyzeAccumulator) {
	frameworks := map[string]string{
		"github.com/spf13/cobra":             "cobra",
		"github.com/charmbracelet/bubbletea": "bubbletea",
		"github.com/gin-gonic/gin":           "gin",
		"github.com/labstack/echo":           "echo",
	}
	for needle, framework := range frameworks {
		if strings.Contains(text, needle) {
			state.frameworks[framework] = true
		}
	}
}

func detectPackageJSON(text string, state *repoAnalyzeAccumulator) {
	var pkg struct {
		Scripts      map[string]string `json:"scripts"`
		Dependencies map[string]string `json:"dependencies"`
		DevDeps      map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal([]byte(text), &pkg); err != nil {
		state.riskNotes["package.json could not be parsed for script detection"] = true
		return
	}
	if _, ok := pkg.Scripts["test"]; ok {
		state.testCommands["npm test"] = true
	}
	if _, ok := pkg.Scripts["lint"]; ok {
		state.lintCommands["npm run lint"] = true
	}
	deps := map[string]string{}
	for k, v := range pkg.Dependencies {
		deps[strings.ToLower(k)] = v
	}
	for k, v := range pkg.DevDeps {
		deps[strings.ToLower(k)] = v
	}
	for dep := range deps {
		switch dep {
		case "react":
			state.frameworks["react"] = true
		case "next":
			state.frameworks["nextjs"] = true
		case "vite":
			state.frameworks["vite"] = true
		case "express":
			state.frameworks["express"] = true
		case "typescript":
			state.languages["typescript"] = true
		}
	}
}

var makeTargetRe = regexp.MustCompile(`(?m)^([A-Za-z0-9_.-]+):`)

func detectMakeTargets(text string, state *repoAnalyzeAccumulator) {
	for _, match := range makeTargetRe.FindAllStringSubmatch(text, -1) {
		switch match[1] {
		case "test":
			state.testCommands["make test"] = true
		case "lint":
			state.lintCommands["make lint"] = true
		}
	}
}

func captureRepoInstruction(rel, text string, state *repoAnalyzeAccumulator) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return
	}
	if len(trimmed) > repoInstructionSnippetBytes {
		trimmed = trimmed[:repoInstructionSnippetBytes] + "..."
	}
	trimmed = safety.RedactSecrets(trimmed)
	state.repoInstructions[fmt.Sprintf("%s: %s", rel, singleLine(trimmed))] = true
	for _, finding := range safety.DetectPromptInjection(trimmed) {
		state.riskNotes[fmt.Sprintf("untrusted instruction warning in %s: %s", rel, finding.Pattern)] = true
	}
}

func (s *RepoAnalyzeStage) writeArtifact(ctx context.Context, env StageEnv, analysis contract.RepoAnalysis) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	artifactPath := filepath.Join(s.projectRoot, repoAnalysisArtifactRelPath)
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		return fmt.Errorf("create repo analysis artifact dir: %w", err)
	}
	if err := os.WriteFile(artifactPath, data, 0o644); err != nil {
		return fmt.Errorf("write repo analysis artifact: %w", err)
	}
	store, ok := env.Store.(*state.Store)
	if !ok || store == nil || env.Project == nil {
		return nil
	}
	now := time.Now().UTC()
	runID := ""
	if env.Run != nil {
		runID = env.Run.RunID()
	}
	hash := sha256.Sum256(data)
	artifact := &state.Artifact{
		ID:        repoAnalysisArtifactID(env.Project.ProjectID(), runID),
		ProjectID: env.Project.ProjectID(),
		RunID:     runID,
		Kind:      string(contract.ArtifactKindRepoAnalysis),
		Path:      repoAnalysisArtifactRelPath,
		SHA256:    hex.EncodeToString(hash[:]),
		Version:   1,
		Metadata: map[string]any{
			"stage": string(StageRepoAnalyze),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	return store.UpsertArtifact(ctx, artifact)
}

func repoAnalysisArtifactID(projectID, runID string) string {
	seed := projectID + ":" + runID + ":repo_analysis"
	hash := sha256.Sum256([]byte(seed))
	return "artifact_" + hex.EncodeToString(hash[:8])
}

func candidateRepoAnalyzeFile(rel string) bool {
	base := strings.ToLower(filepath.Base(rel))
	if strings.HasPrefix(base, ".env") || secretPath(rel) {
		return false
	}
	switch base {
	case "readme", "readme.md", "agents.md", "contributing", "contributing.md", "go.mod", "go.sum", "package.json", "package-lock.json", "npm-shrinkwrap.json", "yarn.lock", "pnpm-lock.yaml", "tsconfig.json", "pyproject.toml", "requirements.txt", "requirements-dev.txt", "cargo.toml", "gemfile", "makefile", "vite.config.js", "vite.config.ts":
		return true
	}
	if strings.HasPrefix(rel, "cmd/") && filepath.Base(rel) == "main.go" {
		return true
	}
	return false
}

func alwaysExcludedDir(rel string) bool {
	clean := filepath.ToSlash(filepath.Clean(rel))
	switch strings.Split(clean, "/")[0] {
	case ".git", ".nexdev", "node_modules", "vendor", "dist", "build":
		return true
	default:
		return false
	}
}

func excludedPath(rel string, globs []string) bool {
	if secretPath(rel) {
		return true
	}
	return matchesAny(rel, globs)
}

func matchesAny(rel string, globs []string) bool {
	rel = filepath.ToSlash(filepath.Clean(rel))
	for _, glob := range globs {
		glob = filepath.ToSlash(strings.TrimSpace(glob))
		if glob == "" {
			continue
		}
		if strings.HasSuffix(glob, "/**") {
			prefix := strings.TrimSuffix(glob, "/**")
			if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
				return true
			}
			continue
		}
		matched, err := filepath.Match(glob, rel)
		if err == nil && matched {
			return true
		}
	}
	return false
}

func secretPath(rel string) bool {
	base := strings.ToLower(filepath.Base(rel))
	if base == ".env" || strings.HasPrefix(base, ".env.") {
		return true
	}
	if strings.Contains(base, "secret") || strings.Contains(base, "token") || strings.Contains(base, "password") {
		return true
	}
	return strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, "_rsa") || strings.HasSuffix(base, "_ed25519")
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func singleLine(text string) string {
	fields := strings.Fields(strings.ReplaceAll(text, "\x00", ""))
	return strings.Join(fields, " ")
}

func repoAnalysisSummary(a contract.RepoAnalysis) string {
	parts := []string{"Repository analysis completed deterministically."}
	if len(a.Languages) > 0 {
		parts = append(parts, "Languages: "+strings.Join(a.Languages, ", ")+".")
	}
	if len(a.Frameworks) > 0 {
		parts = append(parts, "Frameworks: "+strings.Join(a.Frameworks, ", ")+".")
	}
	if len(a.TestCommands) > 0 {
		parts = append(parts, "Likely tests: "+strings.Join(a.TestCommands, "; ")+".")
	}
	if len(a.RiskNotes) > 0 {
		parts = append(parts, fmt.Sprintf("Risk notes: %d.", len(a.RiskNotes)))
	}
	return strings.Join(parts, " ")
}

var _ PipelineStage = (*RepoAnalyzeStage)(nil)
var _ StageOutputter = (*RepoAnalyzeStage)(nil)
