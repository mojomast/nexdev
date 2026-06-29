package mcp

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mojomast/nexdev/internal/config"
	"github.com/mojomast/nexdev/internal/security"
	"github.com/mojomast/nexdev/internal/state"
)

// inputValidator is a package-level InputValidator instance shared across all MCP handlers.
var inputValidator = security.NewInputValidator()

// validateProjectPath validates the projectPath argument using InputValidator.
// It checks for null bytes, control characters, and invalid UTF-8.
func validateProjectPath(projectPath string) error {
	if err := inputValidator.ValidateFilePath(projectPath); err != nil {
		return fmt.Errorf("invalid project path: %w", err)
	}
	// Ensure it's an absolute path
	if !filepath.IsAbs(projectPath) {
		return fmt.Errorf("project path must be absolute, got: %s", projectPath)
	}
	return nil
}

// validateTextInput validates a free-text input (e.g. answers, guidance, descriptions).
// It checks the content is valid UTF-8 and within the specified size limit.
// maxSize of 0 means no limit.
func validateTextInput(name, value string, maxSize int) error {
	if err := inputValidator.ValidateFileContent(value, maxSize); err != nil {
		return fmt.Errorf("invalid %s: %w", name, err)
	}
	return nil
}

// validateIdentifier validates a short identifier string (e.g. taskId, phaseId, questionId).
// Identifiers must not contain null bytes, control characters, or path separators.
func validateIdentifier(name, value string) error {
	if err := inputValidator.ValidateFilePath(value); err != nil {
		return fmt.Errorf("invalid %s: %w", name, err)
	}
	// Identifiers should not contain path separators to prevent path injection
	if strings.Contains(value, "/") || strings.Contains(value, "\\") {
		return fmt.Errorf("invalid %s: must not contain path separators", name)
	}
	return nil
}

func openStateStore(projectPath string) (*state.Store, error) {
	dbPath := filepath.Join(projectPath, ".geoffrussy", "state.db")
	return state.NewStore(dbPath)
}

func getProviderAndModel(cfgMgr *config.Manager, stage, overrideModel string) (string, string, error) {
	cfg := cfgMgr.GetConfig()
	stage = normalizeModelStage(stage)

	modelName := overrideModel
	if modelName == "" {
		var err error
		modelName, err = cfgMgr.ResolveDefaultModel(stage)
		if err != nil || modelName == "" {
			for providerName := range cfg.APIKeys {
				if defaultModel, ok := cfg.DefaultModels[providerName]; ok && defaultModel != "" {
					return providerName, defaultModel, nil
				}
				if providerName == "requesty" {
					return providerName, "openai/gpt-4", nil
				}
				return providerName, "gpt-3.5-turbo", nil
			}
			return "", "", fmt.Errorf("no API keys configured")
		}
	}

	providerName := ""
	if strings.Contains(modelName, "/") {
		if _, ok := cfg.APIKeys["requesty"]; ok {
			providerName = "requesty"
		} else if _, ok := cfg.APIKeys["openrouter"]; ok {
			providerName = "openrouter"
		} else {
			providerName = guessProviderFromModel(modelName)
		}
	} else {
		providerName = guessProviderFromModel(modelName)
		if providerName == "" {
			for p := range cfg.APIKeys {
				providerName = p
				break
			}
		}
	}

	if providerName == "" {
		for p := range cfg.APIKeys {
			return p, modelName, nil
		}
		return "", "", fmt.Errorf("no provider configured for model: %s", modelName)
	}

	return providerName, modelName, nil
}

func normalizeModelStage(stage string) string {
	if stage == "plan" || stage == "plan.generate" {
		return "devplan.generate"
	}
	return stage
}

func guessProviderFromModel(model string) string {
	lowerModel := strings.ToLower(model)
	switch {
	case strings.Contains(lowerModel, "gpt"):
		return "openai"
	case strings.Contains(lowerModel, "codex"):
		return "openai-codex"
	case strings.Contains(lowerModel, "claude"):
		return "anthropic"
	case strings.Contains(lowerModel, "moonshot"), strings.Contains(lowerModel, "kimi"):
		return "kimi"
	case strings.Contains(lowerModel, "glm"), strings.Contains(lowerModel, "zai"):
		return "zai"
	case strings.Contains(lowerModel, "opencode"):
		return "opencode"
	case strings.Contains(lowerModel, "llama"), strings.Contains(lowerModel, "mixtral"):
		return "groq"
	case strings.Contains(lowerModel, "mistral"), strings.Contains(lowerModel, "pixtral"):
		return "mistral"
	case strings.Contains(lowerModel, "sonar"), strings.Contains(lowerModel, "perplexity"):
		return "perplexity"
	case strings.Contains(lowerModel, "fireworks"):
		return "fireworks"
	case strings.Contains(lowerModel, "deepinfra"):
		return "deepinfra"
	case strings.Contains(lowerModel, "qwen"), strings.Contains(lowerModel, "deepseek"):
		return "together"
	default:
		return ""
	}
}
