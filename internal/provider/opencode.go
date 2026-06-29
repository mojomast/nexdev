package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// OpenCodeProvider implements the Provider interface for OpenCode
type OpenCodeProvider struct {
	*BaseProvider
	opencodeCmd string
}

// NewOpenCodeProvider creates a new OpenCode provider
func NewOpenCodeProvider() *OpenCodeProvider {
	return &OpenCodeProvider{
		BaseProvider: NewBaseProvider("opencode"),
		opencodeCmd:  "opencode", // Assumes opencode is in PATH
	}
}

// Authenticate checks if OpenCode CLI is available
func (o *OpenCodeProvider) Authenticate(apiKey string) error {
	// Check if opencode CLI is available
	cmd := exec.Command(o.opencodeCmd, "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("opencode CLI not found or not working: %w. Please ensure OpenCode is installed and in PATH", err)
	}

	o.authenticated = true
	return nil
}

// DiscoverModels dynamically discovers available models through OpenCode
func (o *OpenCodeProvider) DiscoverModels() ([]Model, error) {
	if !o.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	// Use opencode to list available models
	// The exact command may vary - this is a placeholder
	cmd := exec.Command(o.opencodeCmd, "models", "list", "--json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If command doesn't exist, return a default set of models
		return o.getDefaultModels(), nil
	}

	// Parse JSON output
	var modelsData []struct {
		ID       string   `json:"id"`
		Name     string   `json:"name"`
		Provider string   `json:"provider"`
		Features []string `json:"features"`
	}

	if err := json.Unmarshal(output, &modelsData); err != nil {
		// If parsing fails, return default models
		return o.getDefaultModels(), nil
	}

	models := make([]Model, 0, len(modelsData))
	for _, m := range modelsData {
		model := Model{
			Provider:     "opencode",
			Name:         m.ID,
			DisplayName:  m.Name,
			Capabilities: m.Features,
			PriceInput:   0.0, // Prices depend on underlying provider
			PriceOutput:  0.0,
		}
		models = append(models, model)
	}

	return models, nil
}

// getDefaultModels returns a default set of models if discovery fails
func (o *OpenCodeProvider) getDefaultModels() []Model {
	return []Model{
		{
			Provider:     "opencode",
			Name:         "claude-sonnet-4",
			DisplayName:  "Claude Sonnet 4 (via OpenCode)",
			Capabilities: []string{"text", "code", "streaming"},
			PriceInput:   0.0,
			PriceOutput:  0.0,
		},
		{
			Provider:     "opencode",
			Name:         "gpt-4",
			DisplayName:  "GPT-4 (via OpenCode)",
			Capabilities: []string{"text", "code", "streaming"},
			PriceInput:   0.0,
			PriceOutput:  0.0,
		},
		{
			Provider:     "opencode",
			Name:         "gpt-4-turbo",
			DisplayName:  "GPT-4 Turbo (via OpenCode)",
			Capabilities: []string{"text", "code", "streaming"},
			PriceInput:   0.0,
			PriceOutput:  0.0,
		},
	}
}

// ListModels calls DiscoverModels for OpenCode
func (o *OpenCodeProvider) ListModels() ([]Model, error) {
	return o.DiscoverModels()
}

// Call makes a non-streaming API call using OpenCode CLI
func (o *OpenCodeProvider) Call(ctx context.Context, model string, prompt string) (*Response, error) {
	if !o.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	startTime := time.Now()
	var response *Response
	err := o.RetryWithBackoff(func() error {
		// Use opencode run command
		cmd := exec.CommandContext(ctx, o.opencodeCmd, "run", "--model", model, "--prompt", prompt, "--no-stream")

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("opencode command failed: %w\nStderr: %s", err, stderr.String())
		}

		content := strings.TrimSpace(stdout.String())

		// OpenCode doesn't provide token counts directly
		// We'll estimate based on content length
		tokensInput := len(prompt) / 4 // rough estimate: 1 token ≈ 4 chars
		tokensOutput := len(content) / 4

		duration := time.Since(startTime)

		// Log successful API call with metadata
		o.GetLogger().Info("API call completed",
			"provider", "opencode",
			"model", model,
			"tokens_input", tokensInput,
			"tokens_output", tokensOutput,
			"tokens_total", tokensInput+tokensOutput,
			"duration_ms", duration.Milliseconds(),
		)

		response = &Response{
			Content:      content,
			TokensInput:  tokensInput,
			TokensOutput: tokensOutput,
			Model:        model,
			Provider:     "opencode",
			Timestamp:    startTime,
		}

		return nil
	})

	if err != nil {
		o.GetLogger().Error("API call failed",
			"provider", "opencode",
			"model", model,
			"error", err.Error(),
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
	}

	return response, err
}

// Stream makes a streaming API call using OpenCode CLI
func (o *OpenCodeProvider) Stream(ctx context.Context, model string, prompt string) (<-chan string, error) {
	if !o.IsAuthenticated() {
		return nil, fmt.Errorf("provider not authenticated")
	}

	// Use opencode run command with streaming
	cmd := exec.CommandContext(ctx, o.opencodeCmd, "run", "--model", model, "--prompt", prompt, "--stream")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start opencode command: %w", err)
	}

	ch := make(chan string, 100)

	go func() {
		defer close(ch)
		defer cmd.Wait()

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if line != "" {
				ch <- line + "\n"
			}
		}
	}()

	return ch, nil
}

// GetRateLimitInfo returns nil for OpenCode (depends on underlying provider)
func (o *OpenCodeProvider) GetRateLimitInfo() (*RateLimitInfo, error) {
	// OpenCode abstracts away provider-specific rate limits
	return nil, nil
}

// GetQuotaInfo returns nil for OpenCode (depends on underlying provider)
func (o *OpenCodeProvider) GetQuotaInfo() (*QuotaInfo, error) {
	// OpenCode abstracts away provider-specific quotas
	return nil, nil
}
