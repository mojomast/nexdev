package steering

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/safety"
)

const (
	DefaultArtifactBudgetChars = 2000
	DefaultArtifactTotalChars  = 8000
)

type ArtifactContextConfig struct {
	ArtifactRoot string
	PerArtifact  int
	Total        int
}

type ArtifactContext struct {
	Kind      contract.ArtifactKind `json:"kind"`
	Path      string                `json:"path"`
	Text      string                `json:"text"`
	Truncated bool                  `json:"truncated"`
}

type TaskContext struct {
	Description        string   `json:"description,omitempty"`
	ExpectedFiles      []string `json:"expected_files,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
}

var promptArtifactFiles = []struct {
	kind contract.ArtifactKind
	name string
}{
	{contract.ArtifactKindRepoAnalysis, "repo_analysis.json"},
	{contract.ArtifactKindInterview, "interview.json"},
	{contract.ArtifactKindDesignDraft, "design_draft.md"},
	{contract.ArtifactKindComplexityProfile, "complexity_profile.json"},
}

func LoadArtifactContext(cfg ArtifactContextConfig) ([]ArtifactContext, error) {
	root := strings.TrimSpace(cfg.ArtifactRoot)
	if root == "" {
		return nil, nil
	}
	perArtifact := cfg.PerArtifact
	if perArtifact <= 0 {
		perArtifact = DefaultArtifactBudgetChars
	}
	total := cfg.Total
	if total <= 0 {
		total = DefaultArtifactTotalChars
	}
	if total < perArtifact {
		perArtifact = total
	}

	remaining := total
	contexts := make([]ArtifactContext, 0, len(promptArtifactFiles))
	for _, file := range promptArtifactFiles {
		if remaining <= 0 {
			break
		}
		path := filepath.Join(root, file.name)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		text := safety.RedactSecrets(string(data))
		budget := perArtifact
		if remaining < budget {
			budget = remaining
		}
		truncated := false
		if runeCount(text) > budget {
			text = truncateRunes(text, budget)
			truncated = true
		}
		remaining -= runeCount(text)
		contexts = append(contexts, ArtifactContext{Kind: file.kind, Path: path, Text: text, Truncated: truncated})
	}
	return contexts, nil
}

func ContextFromTask(task contract.TaskSpec) TaskContext {
	return TaskContext{
		Description:        task.Description,
		ExpectedFiles:      append([]string{}, task.ExpectedFiles...),
		AcceptanceCriteria: append([]string{}, task.AcceptanceCriteria...),
	}
}

func runeCount(text string) int {
	return len([]rune(text))
}

func truncateRunes(text string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max])
}
