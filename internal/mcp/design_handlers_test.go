package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mojomast/nexdev/internal/design"
)

func TestIdentifyTargetSections(t *testing.T) {
	allSections := []string{
		"system_overview",
		"components",
		"technology_rationale",
		"scaling_strategy",
		"api_contract",
		"database_schema",
		"security",
		"observability",
		"deployment",
		"risks",
	}

	t.Run("MatchesSecurityKeywords", func(t *testing.T) {
		sections := identifyTargetSections("improve the authentication and encryption", allSections)
		found := false
		for _, s := range sections {
			if s == "security" {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected 'security' section, got %v", sections)
		}
	})

	t.Run("MatchesDeploymentKeywords", func(t *testing.T) {
		sections := identifyTargetSections("switch to kubernetes for production deployment", allSections)
		found := false
		for _, s := range sections {
			if s == "deployment" {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected 'deployment' section, got %v", sections)
		}
	})

	t.Run("MatchesMultipleSections", func(t *testing.T) {
		sections := identifyTargetSections("update the database schema and add API endpoints", allSections)
		sort.Strings(sections)
		hasDB := false
		hasAPI := false
		for _, s := range sections {
			if s == "database_schema" {
				hasDB = true
			}
			if s == "api_contract" {
				hasAPI = true
			}
		}
		if !hasDB {
			t.Errorf("Expected 'database_schema' section, got %v", sections)
		}
		if !hasAPI {
			t.Errorf("Expected 'api_contract' section, got %v", sections)
		}
	})

	t.Run("MatchesExactSectionName", func(t *testing.T) {
		sections := identifyTargetSections("please update the scaling_strategy", allSections)
		found := false
		for _, s := range sections {
			if s == "scaling_strategy" {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected 'scaling_strategy' section, got %v", sections)
		}
	})

	t.Run("MatchesSpaceSeparatedSectionName", func(t *testing.T) {
		sections := identifyTargetSections("refine the system overview section", allSections)
		found := false
		for _, s := range sections {
			if s == "system_overview" {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected 'system_overview' section, got %v", sections)
		}
	})

	t.Run("NoMatchReturnsEmpty", func(t *testing.T) {
		sections := identifyTargetSections("make everything purple", allSections)
		if len(sections) != 0 {
			t.Errorf("Expected no sections matched, got %v", sections)
		}
	})

	t.Run("ObservabilityKeywords", func(t *testing.T) {
		sections := identifyTargetSections("add better logging and metrics", allSections)
		found := false
		for _, s := range sections {
			if s == "observability" {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected 'observability' section, got %v", sections)
		}
	})

	t.Run("RiskKeywords", func(t *testing.T) {
		sections := identifyTargetSections("add mitigation for data loss risk", allSections)
		found := false
		for _, s := range sections {
			if s == "risks" {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected 'risks' section, got %v", sections)
		}
	})

	t.Run("ComponentKeywords", func(t *testing.T) {
		sections := identifyTargetSections("add a new service component for notifications", allSections)
		found := false
		for _, s := range sections {
			if s == "components" {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected 'components' section, got %v", sections)
		}
	})

	t.Run("CaseInsensitive", func(t *testing.T) {
		sections := identifyTargetSections("Improve SECURITY and DEPLOYMENT", allSections)
		hasSecurity := false
		hasDeployment := false
		for _, s := range sections {
			if s == "security" {
				hasSecurity = true
			}
			if s == "deployment" {
				hasDeployment = true
			}
		}
		if !hasSecurity || !hasDeployment {
			t.Errorf("Expected both 'security' and 'deployment', got %v", sections)
		}
	})
}

func TestValidateProjectPath(t *testing.T) {
	t.Run("ValidAbsolutePath", func(t *testing.T) {
		err := validateProjectPath("/home/user/project")
		if err != nil {
			t.Errorf("Expected no error for valid absolute path, got: %v", err)
		}
	})

	t.Run("RejectsRelativePath", func(t *testing.T) {
		err := validateProjectPath("relative/path")
		if err == nil {
			t.Error("Expected error for relative path")
		}
		if !strings.Contains(err.Error(), "absolute") {
			t.Errorf("Expected error about absolute path, got: %v", err)
		}
	})

	t.Run("RejectsEmptyPath", func(t *testing.T) {
		err := validateProjectPath("")
		if err == nil {
			t.Error("Expected error for empty path")
		}
	})

	t.Run("RejectsNullBytes", func(t *testing.T) {
		err := validateProjectPath("/home/user/\x00evil")
		if err == nil {
			t.Error("Expected error for path with null bytes")
		}
	})

	t.Run("RejectsControlCharacters", func(t *testing.T) {
		err := validateProjectPath("/home/user/\x01bad")
		if err == nil {
			t.Error("Expected error for path with control characters")
		}
	})
}

func TestValidateIdentifier(t *testing.T) {
	t.Run("ValidIdentifiers", func(t *testing.T) {
		validIDs := []string{"task-1.3", "phase-0", "task_abc", "simple"}
		for _, id := range validIDs {
			err := validateIdentifier("test", id)
			if err != nil {
				t.Errorf("Expected no error for identifier %q, got: %v", id, err)
			}
		}
	})

	t.Run("RejectsForwardSlash", func(t *testing.T) {
		err := validateIdentifier("taskId", "../../etc/passwd")
		if err == nil {
			t.Error("Expected error for identifier with forward slash")
		}
		if !strings.Contains(err.Error(), "path separator") {
			t.Errorf("Expected error about path separators, got: %v", err)
		}
	})

	t.Run("RejectsBackslash", func(t *testing.T) {
		err := validateIdentifier("taskId", "task\\evil")
		if err == nil {
			t.Error("Expected error for identifier with backslash")
		}
		if !strings.Contains(err.Error(), "path separator") {
			t.Errorf("Expected error about path separators, got: %v", err)
		}
	})

	t.Run("RejectsNullBytes", func(t *testing.T) {
		err := validateIdentifier("phaseId", "phase\x00evil")
		if err == nil {
			t.Error("Expected error for identifier with null bytes")
		}
	})

	t.Run("RejectsEmpty", func(t *testing.T) {
		err := validateIdentifier("taskId", "")
		if err == nil {
			t.Error("Expected error for empty identifier")
		}
	})
}

func TestValidateTextInput(t *testing.T) {
	t.Run("ValidText", func(t *testing.T) {
		err := validateTextInput("answer", "This is a valid answer with unicode: cafe\u0301", 1000)
		if err != nil {
			t.Errorf("Expected no error for valid text, got: %v", err)
		}
	})

	t.Run("RejectsOversizedText", func(t *testing.T) {
		longText := strings.Repeat("a", 1001)
		err := validateTextInput("answer", longText, 1000)
		if err == nil {
			t.Error("Expected error for oversized text")
		}
		if !strings.Contains(err.Error(), "exceeds") {
			t.Errorf("Expected error about exceeding size, got: %v", err)
		}
	})

	t.Run("AcceptsExactSizeLimit", func(t *testing.T) {
		exactText := strings.Repeat("b", 500)
		err := validateTextInput("answer", exactText, 500)
		if err != nil {
			t.Errorf("Expected no error for text at exact size limit, got: %v", err)
		}
	})

	t.Run("NoLimitWhenMaxSizeZero", func(t *testing.T) {
		largeText := strings.Repeat("c", 100000)
		err := validateTextInput("guidance", largeText, 0)
		if err != nil {
			t.Errorf("Expected no error with maxSize=0, got: %v", err)
		}
	})

	t.Run("RejectsInvalidUTF8", func(t *testing.T) {
		invalidUTF8 := "hello \xff\xfe world"
		err := validateTextInput("answer", invalidUTF8, 1000)
		if err == nil {
			t.Error("Expected error for invalid UTF-8")
		}
	})
}

func TestLoadExistingArchitecture(t *testing.T) {
	t.Run("LoadsValidArchitecture", func(t *testing.T) {
		tmpDir := t.TempDir()
		geoffDir := filepath.Join(tmpDir, ".geoffrussy")
		if err := os.MkdirAll(geoffDir, 0o755); err != nil {
			t.Fatal(err)
		}

		arch := &design.Architecture{
			ProjectID:      "test-project",
			SystemOverview: "Test system overview",
			Components: []design.Component{
				{
					Name:    "Backend",
					Type:    design.ComponentBackend,
					Purpose: "API server",
				},
			},
			SecurityApproach: design.SecurityPlan{
				Authentication: "JWT",
				Authorization:  "RBAC",
			},
		}

		data, err := json.MarshalIndent(arch, "", "  ")
		if err != nil {
			t.Fatal(err)
		}

		archPath := filepath.Join(geoffDir, "architecture.json")
		if err := os.WriteFile(archPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		loaded, err := loadExistingArchitecture(tmpDir, "test-project")
		if err != nil {
			t.Fatalf("Failed to load architecture: %v", err)
		}

		if loaded.ProjectID != "test-project" {
			t.Errorf("Expected project ID 'test-project', got '%s'", loaded.ProjectID)
		}
		if loaded.SystemOverview != "Test system overview" {
			t.Errorf("Expected system overview 'Test system overview', got '%s'", loaded.SystemOverview)
		}
		if len(loaded.Components) != 1 {
			t.Errorf("Expected 1 component, got %d", len(loaded.Components))
		}
		if loaded.SecurityApproach.Authentication != "JWT" {
			t.Errorf("Expected authentication 'JWT', got '%s'", loaded.SecurityApproach.Authentication)
		}
	})

	t.Run("SetsProjectIDIfMissing", func(t *testing.T) {
		tmpDir := t.TempDir()
		geoffDir := filepath.Join(tmpDir, ".geoffrussy")
		if err := os.MkdirAll(geoffDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Architecture with no ProjectID
		arch := &design.Architecture{
			SystemOverview: "Test",
		}

		data, _ := json.Marshal(arch)
		archPath := filepath.Join(geoffDir, "architecture.json")
		if err := os.WriteFile(archPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		loaded, err := loadExistingArchitecture(tmpDir, "my-project")
		if err != nil {
			t.Fatalf("Failed to load: %v", err)
		}

		if loaded.ProjectID != "my-project" {
			t.Errorf("Expected project ID 'my-project', got '%s'", loaded.ProjectID)
		}
	})

	t.Run("ErrorWhenFileNotFound", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := loadExistingArchitecture(tmpDir, "test-project")
		if err == nil {
			t.Error("Expected error when file not found")
		}
	})

	t.Run("ErrorWhenInvalidJSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		geoffDir := filepath.Join(tmpDir, ".geoffrussy")
		if err := os.MkdirAll(geoffDir, 0o755); err != nil {
			t.Fatal(err)
		}

		archPath := filepath.Join(geoffDir, "architecture.json")
		if err := os.WriteFile(archPath, []byte("not json"), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := loadExistingArchitecture(tmpDir, "test-project")
		if err == nil {
			t.Error("Expected error when JSON is invalid")
		}
	})
}
