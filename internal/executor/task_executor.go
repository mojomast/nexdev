package executor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/common"
	"github.com/mojomast/nexdev/internal/logging"
	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/security"
	"github.com/mojomast/nexdev/internal/state"
)

// SendUpdateFunc is the type of function used to send updates
type SendUpdateFunc func(update TaskUpdate)

// TaskExecutor implements actual task execution using LLM
type TaskExecutor struct {
	store         *state.Store
	provider      provider.Provider
	modelName     string
	ctx           context.Context
	sendUpdate    SendUpdateFunc // Function to send updates through TUI
	phaseID       string         // For update messages
	taskID        string         // For update messages
	pathSanitizer *security.PathSanitizer
	auditLogger   *security.AuditLogger
	logger        *logging.Logger
}

// NewTaskExecutor creates a new task executor that actually implements tasks
func NewTaskExecutor(ctx context.Context, store *state.Store, prov provider.Provider, sendUpdateFn SendUpdateFunc, modelName string) *TaskExecutor {
	// Get current working directory as project root
	projectRoot, err := os.Getwd()
	if err != nil {
		// Fallback to "." if we can't get working directory
		projectRoot = "."
	}

	// Initialize path sanitizer
	pathSanitizer, err := security.NewPathSanitizer(projectRoot)
	if err != nil {
		// This should rarely happen, but if it does, we'll create a sanitizer with "."
		pathSanitizer, _ = security.NewPathSanitizer(".")
	}

	// Initialize audit logger
	// Place audit log in .geoffrussy directory if it exists, otherwise in current directory
	auditLogPath := filepath.Join(projectRoot, ".geoffrussy", "audit.log")
	auditLogger, err := security.NewAuditLogger(auditLogPath)
	if err != nil {
		// Fallback to current directory if .geoffrussy doesn't exist
		auditLogPath = filepath.Join(projectRoot, "geoffrussy-audit.log")
		auditLogger, _ = security.NewAuditLogger(auditLogPath)
	}

	// Initialize structured logger
	logger := logging.NewLogger(slog.LevelInfo, os.Stdout)

	// Ensure context is not nil
	if ctx == nil {
		ctx = context.Background()
	}

	return &TaskExecutor{
		store:         store,
		provider:      prov,
		modelName:     modelName,
		ctx:           ctx,
		sendUpdate:    sendUpdateFn,
		pathSanitizer: pathSanitizer,
		auditLogger:   auditLogger,
		logger:        logger,
	}
}

// CodeGenerationResponse represents a LLM response for code generation
type CodeGenerationResponse struct {
	Explanation string    `json:"explanation"`
	Files       []File    `json:"files"`
	Commands    []Command `json:"commands,omitempty"`
	Tests       []Test    `json:"tests,omitempty"`
}

type File struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Language string `json:"language,omitempty"`
}

type Command struct {
	Command   string `json:"command"`
	Directory string `json:"directory,omitempty"`
}

type Test struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

// ExecuteTask executes a single task using LLM to generate code
func (te *TaskExecutor) ExecuteTask(taskID string) error {
	// Store IDs for update messages
	te.taskID = taskID

	// Get task from store
	task, err := te.store.GetTaskWithContext(te.ctx, taskID)
	if err != nil {
		te.logger.Error("failed to get task",
			"task_id", taskID,
			"error", err)
		return fmt.Errorf("failed to get task: %w", err)
	}

	// Get phase to understand context
	phase, err := te.store.GetPhaseWithContext(te.ctx, task.PhaseID)
	if err != nil {
		te.logger.Error("failed to get phase",
			"task_id", taskID,
			"phase_id", task.PhaseID,
			"error", err)
		return fmt.Errorf("failed to get phase: %w", err)
	}

	// Store phase ID for updates
	te.phaseID = phase.ID

	// Get project
	project, err := te.store.GetProjectWithContext(te.ctx, phase.ProjectID)
	if err != nil {
		te.logger.Error("failed to get project",
			"task_id", taskID,
			"phase_id", phase.ID,
			"project_id", phase.ProjectID,
			"error", err)
		return fmt.Errorf("failed to get project: %w", err)
	}

	// Create contextual logger with task, phase, and project IDs
	contextLogger := te.logger.With(
		"task_id", taskID,
		"phase_id", phase.ID,
		"project_id", project.ID,
	)

	contextLogger.Info("starting task execution",
		"task_description", task.Description,
		"phase_title", phase.Title)

	// Get interview data for context
	interviewData, err := te.store.GetInterviewDataWithContext(te.ctx, project.ID)
	if err != nil {
		contextLogger.Error("failed to get interview data",
			"error", err)
		return fmt.Errorf("failed to get interview data: %w", err)
	}

	// Get architecture for context
	architecture, err := te.store.GetArchitectureWithContext(te.ctx, project.ID)
	if err != nil {
		contextLogger.Error("failed to get architecture",
			"error", err)
		return fmt.Errorf("failed to get architecture: %w", err)
	}

	// Build prompt for LLM
	prompt := te.buildExecutionPrompt(task, phase, interviewData, architecture)
	contextLogger.Debug("built execution prompt",
		"prompt_length", len(prompt))

	// Determine model to use
	modelName := te.getModelForTask(task)

	// Show task being worked on (through TUI)
	te.sendUpdate(TaskUpdate{
		TaskID:    taskID,
		PhaseID:   phase.ID,
		Type:      TaskProgress,
		Content:   fmt.Sprintf("Starting task: %s\nUsing model: %s", task.Description, modelName),
		Timestamp: time.Now(),
	})

	contextLogger.Info("calling LLM for code generation",
		"model", modelName)

	// Call LLM to generate code using streaming
	startTime := time.Now()
	streamChan, err := te.provider.Stream(te.ctx, modelName, prompt)
	if err != nil {
		contextLogger.Error("LLM call failed",
			"model", modelName,
			"error", err)
		te.sendUpdate(TaskUpdate{
			TaskID:    taskID,
			PhaseID:   phase.ID,
			Type:      TaskError,
			Content:   fmt.Sprintf("LLM call failed: %v", err),
			Error:     err,
			Timestamp: time.Now(),
		})
		return fmt.Errorf("failed to call LLM: %w", err)
	}

	// Accumulate response
	var responseBuilder strings.Builder
	var lastUpdate time.Time

	// Stream processing loop
	for chunk := range streamChan {
		responseBuilder.WriteString(chunk)

		// Update TUI periodically to avoid spamming (e.g. every 500ms)
		if time.Since(lastUpdate) > 500*time.Millisecond {
			contentLen := responseBuilder.Len()
			te.sendUpdate(TaskUpdate{
				TaskID:    taskID,
				PhaseID:   phase.ID,
				Type:      TaskProgress,
				Content:   fmt.Sprintf("Generating code... (%d chars received)", contentLen),
				Timestamp: time.Now(),
			})
			lastUpdate = time.Now()
		}

		// Check for context cancellation
		select {
		case <-te.ctx.Done():
			return fmt.Errorf("task execution cancelled: %w", te.ctx.Err())
		default:
		}
	}

	fullResponse := responseBuilder.String()
	duration := time.Since(startTime)

	contextLogger.Info("LLM call completed",
		"model", modelName,
		"response_length", len(fullResponse),
		"duration_ms", duration.Milliseconds())

	te.sendUpdate(TaskUpdate{
		TaskID:    taskID,
		PhaseID:   phase.ID,
		Type:      TaskProgress,
		Content:   fmt.Sprintf("LLM response complete (%d chars)", len(fullResponse)),
		Timestamp: time.Now(),
	})

	// Parse response
	var codeResp CodeGenerationResponse
	if err := common.ParseJSON(fullResponse, &codeResp); err != nil {
		// If JSON parsing fails, treat as entire response as code
		contextLogger.Warn("JSON parsing failed, treating response as markdown",
			"error", err)
		te.sendUpdate(TaskUpdate{
			TaskID:    taskID,
			PhaseID:   phase.ID,
			Type:      TaskProgress,
			Content:   fmt.Sprintf("JSON parsing failed, treating as markdown"),
			Timestamp: time.Now(),
		})
		codeResp = CodeGenerationResponse{
			Explanation: fullResponse,
			Files: []File{
				{
					Path:    "output.md",
					Content: fullResponse,
				},
			},
		}
	}

	// Show LLM's explanation
	if codeResp.Explanation != "" {
		contextLogger.Debug("LLM explanation",
			"explanation", truncateString(codeResp.Explanation, 200))
		te.sendUpdate(TaskUpdate{
			TaskID:    taskID,
			PhaseID:   phase.ID,
			Type:      TaskProgress,
			Content:   fmt.Sprintf("Explanation: %s", truncateString(codeResp.Explanation, 200)),
			Timestamp: time.Now(),
		})
	}

	contextLogger.Info("processing generated files",
		"file_count", len(codeResp.Files))

	te.sendUpdate(TaskUpdate{
		TaskID:    taskID,
		PhaseID:   phase.ID,
		Type:      TaskProgress,
		Content:   fmt.Sprintf("Generated %d file(s)", len(codeResp.Files)),
		Timestamp: time.Now(),
	})

	// Create files with graceful degradation - continue on single-file failures
	var fileErrors []error
	successfulWrites := 0
	totalFiles := len(codeResp.Files)

	for i, file := range codeResp.Files {
		preview := truncateString(file.Content, 200)

		te.sendUpdate(TaskUpdate{
			TaskID:    taskID,
			PhaseID:   phase.ID,
			Type:      TaskProgress,
			Content:   fmt.Sprintf("Writing file %d/%d: %s\nPreview: %s", i+1, len(codeResp.Files), file.Path, preview),
			Timestamp: time.Now(),
		})

		contextLogger.Info("writing file",
			"file_path", file.Path,
			"file_size", len(file.Content),
			"file_index", i+1,
			"total_files", len(codeResp.Files))

		if err := te.writeFile(file); err != nil {
			contextLogger.Error("failed to write file",
				"file_path", file.Path,
				"error", err)
			fileErrors = append(fileErrors, fmt.Errorf("failed to write file %s: %w", file.Path, err))
			te.sendUpdate(TaskUpdate{
				TaskID:    taskID,
				PhaseID:   phase.ID,
				Type:      TaskProgress,
				Content:   fmt.Sprintf("Failed: %s - %v", file.Path, err),
				Error:     err,
				Timestamp: time.Now(),
			})
		} else {
			successfulWrites++
			contextLogger.Info("file written successfully",
				"file_path", file.Path,
				"file_size", len(file.Content))
			te.sendUpdate(TaskUpdate{
				TaskID:    taskID,
				PhaseID:   phase.ID,
				Type:      TaskProgress,
				Content:   fmt.Sprintf("Created: %s (%d bytes)", file.Path, len(file.Content)),
				Timestamp: time.Now(),
			})
		}
	}

	// Execute commands (optional - might be dangerous in auto-execution)
	if len(codeResp.Commands) > 0 {
		contextLogger.Info("commands generated",
			"command_count", len(codeResp.Commands))
		cmdList := fmt.Sprintf("%d commands", len(codeResp.Commands))
		te.sendUpdate(TaskUpdate{
			TaskID:    taskID,
			PhaseID:   phase.ID,
			Type:      TaskProgress,
			Content:   cmdList,
			Timestamp: time.Now(),
		})
	}

	// Check if we had any failures
	if len(fileErrors) > 0 {
		if successfulWrites == 0 {
			// All files failed, return error
			return fmt.Errorf("failed to write any files (%d/%d), first error: %w", 0, totalFiles, fileErrors[0])
		}
		// Some files succeeded, log warning but continue
		contextLogger.Warn("partial file write failure",
			"successful_writes", successfulWrites,
			"failed_writes", len(fileErrors),
			"total_files", totalFiles)
		te.sendUpdate(TaskUpdate{
			TaskID:    taskID,
			PhaseID:   phase.ID,
			Type:      TaskProgress,
			Content:   fmt.Sprintf("⚠️  Completed with %d/%d files written (%d failed)", successfulWrites, totalFiles, len(fileErrors)),
			Timestamp: time.Now(),
		})
	}

	contextLogger.Info("task execution completed successfully")

	return nil
}

func (te *TaskExecutor) getModelForTask(task *state.Task) string {
	return te.modelName
}

func (te *TaskExecutor) buildExecutionPrompt(
	task *state.Task,
	phase *state.Phase,
	interviewData *state.InterviewData,
	architecture *state.Architecture,
) string {
	promptBuilder := strings.Builder{}

	promptBuilder.WriteString("You are an expert software developer tasked with implementing a specific task.\n\n")

	promptBuilder.WriteString("PROJECT CONTEXT:\n")
	promptBuilder.WriteString(fmt.Sprintf("Project: %s\n", interviewData.ProjectName))
	promptBuilder.WriteString(fmt.Sprintf("Problem: %s\n\n", interviewData.ProblemStatement))

	promptBuilder.WriteString("PHASE: ")
	promptBuilder.WriteString(phase.Title)
	promptBuilder.WriteString("\n\n")

	promptBuilder.WriteString("TASK: ")
	promptBuilder.WriteString(task.Description)
	promptBuilder.WriteString("\n\n")

	// Add architecture context
	if architecture != nil && len(architecture.Content) > 0 {
		promptBuilder.WriteString("ARCHITECTURE CONTEXT:\n")
		promptBuilder.WriteString(architecture.Content[:min(2000, len(architecture.Content))])
		promptBuilder.WriteString("\n\n")
	}

	promptBuilder.WriteString("INSTRUCTIONS:\n")
	promptBuilder.WriteString("1. Analyze the task and architecture context\n")
	promptBuilder.WriteString("2. Generate working code that implements the task\n")
	promptBuilder.WriteString("3. Ensure code follows best practices for the language/framework\n")
	promptBuilder.WriteString("4. Return your response as JSON with the following structure\n")
	promptBuilder.WriteString("5. Include only files that should be created/updated for this task\n")
	promptBuilder.WriteString("6. Use repository-relative file paths only\n")
	promptBuilder.WriteString("7. Avoid placeholders like TODO/TBD unless explicitly required\n")
	promptBuilder.WriteString("8. Include validation commands in tests/commands where relevant\n")
	promptBuilder.WriteString("9. Response must be valid JSON only (no markdown, no commentary)\n\n")

	promptBuilder.WriteString(`{
  "explanation": "Brief explanation of your approach",
  "files": [
    {
      "path": "relative/path/to/file.ext",
      "content": "file content here",
      "language": "programming language (optional)"
    }
  ],
  "commands": [
    {
      "command": "shell command to run",
      "directory": "optional directory (default to current)"
    }
  ],
  "tests": [
    {
      "name": "test description",
      "command": "command to run test"
    }
  ]
}`)

	promptBuilder.WriteString("\n\nExecute the task now and return valid JSON.")

	return promptBuilder.String()
}

func (te *TaskExecutor) writeFile(file File) error {
	return te.writeFileSafe(file)
}

// writeFileSafe validates the file path and content, then writes the file with audit logging
func (te *TaskExecutor) writeFileSafe(file File) error {
	// Check context
	select {
	case <-te.ctx.Done():
		return te.ctx.Err()
	default:
	}

	// Validate path using PathSanitizer before writing
	safePath, err := te.pathSanitizer.ValidatePath(file.Path)
	if err != nil {
		// Log rejected path to audit log
		te.auditLogger.LogPathRejection(file.Path, err.Error())
		te.logger.Warn("path validation failed",
			"file_path", file.Path,
			"error", err,
			"task_id", te.taskID,
			"phase_id", te.phaseID)
		return fmt.Errorf("path validation failed for '%s': %w", file.Path, err)
	}

	// Validate file content: must be valid UTF-8, max 1MB
	iv := security.NewInputValidator()
	if err := iv.ValidateFileContent(file.Content, 1048576); err != nil {
		te.auditLogger.LogFileOperation("write", safePath, false)
		te.logger.Warn("file content validation failed",
			"file_path", safePath,
			"error", err,
			"content_size", len(file.Content),
			"task_id", te.taskID,
			"phase_id", te.phaseID)
		return fmt.Errorf("file content validation failed for '%s': %w", safePath, err)
	}

	te.logger.Debug("path validated successfully",
		"original_path", file.Path,
		"safe_path", safePath,
		"task_id", te.taskID,
		"phase_id", te.phaseID)

	// Create directory if needed
	dir := filepath.Dir(safePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			te.auditLogger.LogFileOperation("write", safePath, false)
			te.logger.Error("failed to create directory",
				"directory", dir,
				"file_path", safePath,
				"error", err,
				"task_id", te.taskID,
				"phase_id", te.phaseID)
			return fmt.Errorf("failed to create directory: %w", err)
		}
		te.logger.Debug("created directory",
			"directory", dir,
			"task_id", te.taskID,
			"phase_id", te.phaseID)
	}

	// Write file
	if err := os.WriteFile(safePath, []byte(file.Content), 0644); err != nil {
		te.auditLogger.LogFileOperation("write", safePath, false)
		te.logger.Error("failed to write file",
			"file_path", safePath,
			"error", err,
			"task_id", te.taskID,
			"phase_id", te.phaseID)
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Log successful file operation to audit log
	te.auditLogger.LogFileOperation("write", safePath, true)
	te.logger.Debug("file written successfully",
		"file_path", safePath,
		"file_size", len(file.Content),
		"task_id", te.taskID,
		"phase_id", te.phaseID)

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// truncateString truncates a string to max length with "..." suffix
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
