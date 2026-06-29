package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/config"
	"github.com/mojomast/nexdev/internal/executor"
	"github.com/mojomast/nexdev/internal/state"
)

// ExecHandlers contains handlers for execution-related tools
type ExecHandlers struct {
	configManager *config.Manager
}

// NewExecHandlers creates a new execution handlers instance
func NewExecHandlers(configManager *config.Manager) *ExecHandlers {
	return &ExecHandlers{
		configManager: configManager,
	}
}

// RegisterHandlers registers execution tools with the registry
func (h *ExecHandlers) RegisterHandlers(registry *ToolRegistry) error {

	tools := []struct {
		tool    Tool
		handler ToolHandler
	}{
		{h.executePhaseTool(), h.handleExecutePhase},
		{h.executeTaskTool(), h.handleExecuteTask},
		{h.getTaskOutputTool(), h.handleGetTaskOutput},
		{h.handleBlockerTool(), h.handleHandleBlocker},
	}

	for _, t := range tools {
		if err := registry.RegisterTool(t.tool, t.handler); err != nil {
			return fmt.Errorf("failed to register tool %s: %w", t.tool.Name, err)
		}
	}

	return nil
}

// Tool definitions

func (h *ExecHandlers) executePhaseTool() Tool {
	return Tool{
		Name:        "execute_phase",
		Description: "Execute all tasks in a development phase, writing code and creating files",
		InputSchema: CreateInputSchema(
			map[string]interface{}{
				"projectPath":    StringParam("Absolute path to the project directory"),
				"phaseId":        StringParam("ID of the phase to execute (e.g., 'phase-0', 'phase-1')"),
				"model":          StringParam("Model to use for task execution"),
				"stopAfterPhase": BooleanParam("Stop after completing this phase"),
				"streamOutput":   BooleanParam("Stream task output in real-time (not supported in current transport)"),
			},
			[]string{"projectPath", "phaseId"},
		),
	}
}

func (h *ExecHandlers) executeTaskTool() Tool {
	return Tool{
		Name:        "execute_task",
		Description: "Execute a single task by ID",
		InputSchema: CreateInputSchema(
			map[string]interface{}{
				"projectPath": StringParam("Absolute path to the project directory"),
				"taskId":      StringParam("ID of task to execute (e.g., 'task-1.3')"),
				"model":       StringParam("Model to use for task execution"),
			},
			[]string{"projectPath", "taskId"},
		),
	}
}

func (h *ExecHandlers) getTaskOutputTool() Tool {
	return Tool{
		Name:        "get_task_output",
		Description: "Get detailed execution output for a task",
		InputSchema: CreateInputSchema(
			map[string]interface{}{
				"projectPath": StringParam("Absolute path to the project directory"),
				"taskId":      StringParam("ID of task"),
			},
			[]string{"projectPath", "taskId"},
		),
	}
}

func (h *ExecHandlers) handleBlockerTool() Tool {
	return Tool{
		Name:        "handle_blocker",
		Description: "Attempt to resolve a blocker or get guidance on resolution",
		InputSchema: CreateInputSchema(
			map[string]interface{}{
				"projectPath":  StringParam("Absolute path to the project directory"),
				"taskId":       StringParam("ID of task"),
				"action":       StringParam("Action to take: retry, skip, modify, analyze"),
				"modification": StringParam("Modified task description if action is 'modify'"),
			},
			[]string{"projectPath", "taskId", "action"},
		),
	}
}

// Handler implementations

func (h *ExecHandlers) handleExecutePhase(ctx context.Context, args map[string]interface{}) (*CallToolResult, error) {
	projectPath, err := ValidateAndGetString(args, "projectPath", true)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := validateProjectPath(projectPath); err != nil {
		return ErrorResult(err.Error()), nil
	}

	phaseID, err := ValidateAndGetString(args, "phaseId", true)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := validateIdentifier("phaseId", phaseID); err != nil {
		return ErrorResult(err.Error()), nil
	}

	model, _ := ValidateAndGetString(args, "model", false)
	stopAfterPhase, _ := ValidateAndGetBool(args, "stopAfterPhase", false, true)

	store, err := openStateStore(projectPath)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	defer store.Close()

	prov, modelName, err := initProviderForStage(h.configManager, "develop.execute", model)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to initialize provider: %v", err)), nil
	}

	exec := executor.NewExecutor(store, prov, modelName)
	defer exec.Close()

	logDir, err := ensureLogDir(projectPath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to create log directory: %v", err)), nil
	}

	collector := newExecutionCollector(logDir)
	done := collector.start(exec)

	startTime := time.Now()
	// Execute Phase
	var execErr error
	if stopAfterPhase {
		execErr = exec.ExecutePhase(phaseID)
	} else {
		execErr = exec.ExecuteProject(getProjectID(projectPath), phaseID, stopAfterPhase)
	}

	// Wait for updates to process
	exec.Close()
	<-done

	duration := time.Since(startTime)

	// Construct result
	resultText := fmt.Sprintf("✅ Phase execution completed (Duration: %s)\n\nTasks Summary:\n%s\nFiles Created: (check logs)\nTotal Cost: (check stats)",
		duration.Round(time.Second), collector.phaseSummary.String())

	if execErr != nil {
		resultText = fmt.Sprintf("⚠️ Phase execution failed (Duration: %s)\nError: %v\n\nTasks Summary:\n%s",
			duration.Round(time.Second), execErr, collector.phaseSummary.String())
		// Don't return error result, return tool result with error details
	}

	return SuccessResult(resultText), nil
}

func (h *ExecHandlers) handleExecuteTask(ctx context.Context, args map[string]interface{}) (*CallToolResult, error) {
	projectPath, err := ValidateAndGetString(args, "projectPath", true)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := validateProjectPath(projectPath); err != nil {
		return ErrorResult(err.Error()), nil
	}

	taskID, err := ValidateAndGetString(args, "taskId", true)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := validateIdentifier("taskId", taskID); err != nil {
		return ErrorResult(err.Error()), nil
	}

	model, _ := ValidateAndGetString(args, "model", false)

	store, err := openStateStore(projectPath)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	defer store.Close()

	prov, modelName, err := initProviderForStage(h.configManager, "develop.execute", model)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to initialize provider: %v", err)), nil
	}

	exec := executor.NewExecutor(store, prov, modelName)
	// Don't close exec immediately, we need channel

	logDir, err := ensureLogDir(projectPath)
	if err != nil {
		exec.Close()
		return ErrorResult(fmt.Sprintf("Failed to create log directory: %v", err)), nil
	}
	logFile := filepath.Join(logDir, fmt.Sprintf("%s.log", taskID))

	// Truncate log file for new run
	if err := os.WriteFile(logFile, []byte{}, 0o644); err != nil {
		exec.Close()
		return ErrorResult(fmt.Sprintf("Failed to initialize log file: %v", err)), nil
	}

	collector := newExecutionCollector(logDir)
	done := collector.start(exec)

	err = exec.ExecuteTask(taskID)
	exec.Close()
	<-done

	if err != nil {
		return ErrorResult(fmt.Sprintf("Task execution failed: %v\nOutput:\n%s", err, collector.output.String())), nil
	}

	return SuccessResult(fmt.Sprintf("✅ Task %s completed successfully.\n\nOutput:\n%s", taskID, collector.output.String())), nil
}

func (h *ExecHandlers) handleGetTaskOutput(ctx context.Context, args map[string]interface{}) (*CallToolResult, error) {
	projectPath, err := ValidateAndGetString(args, "projectPath", true)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := validateProjectPath(projectPath); err != nil {
		return ErrorResult(err.Error()), nil
	}

	taskID, err := ValidateAndGetString(args, "taskId", true)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := validateIdentifier("taskId", taskID); err != nil {
		return ErrorResult(err.Error()), nil
	}

	// Try to read log file
	logFile := filepath.Join(projectPath, ".geoffrussy", "logs", fmt.Sprintf("%s.log", taskID))
	content, err := os.ReadFile(logFile)
	if err != nil {
		// Fallback to basic status from DB
		store, err := openStateStore(projectPath)
		if err != nil {
			return ErrorResult("Log not found and DB unavailable"), nil
		}
		defer store.Close()

		task, err := store.GetTask(taskID)
		if err != nil {
			return ErrorResult(fmt.Sprintf("Task not found: %s", taskID)), nil
		}
		return SuccessResult(fmt.Sprintf("Task %s: %s\nStatus: %s\n(No detailed logs available)", task.Number, task.Description, task.Status)), nil
	}

	return SuccessResult(string(content)), nil
}

func (h *ExecHandlers) handleHandleBlocker(ctx context.Context, args map[string]interface{}) (*CallToolResult, error) {
	projectPath, err := ValidateAndGetString(args, "projectPath", true)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := validateProjectPath(projectPath); err != nil {
		return ErrorResult(err.Error()), nil
	}

	taskID, err := ValidateAndGetString(args, "taskId", true)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := validateIdentifier("taskId", taskID); err != nil {
		return ErrorResult(err.Error()), nil
	}

	action, err := ValidateAndGetString(args, "action", true)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	store, err := openStateStore(projectPath)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	defer store.Close()

	switch action {
	case "retry":
		return h.handleExecuteTask(ctx, map[string]interface{}{
			"projectPath": projectPath,
			"taskId":      taskID,
		})

	case "skip":
		exec, err := h.newExecutorForAction(store)
		if err != nil {
			return ErrorResult(fmt.Sprintf("Failed to initialize executor: %v", err)), nil
		}
		defer exec.Close()

		if err := exec.SkipTask(taskID); err != nil {
			return ErrorResult(fmt.Sprintf("Failed to skip task: %v", err)), nil
		}
		_ = exec.ResolveBlocker(taskID, "Skipped by user")
		return SuccessResult(fmt.Sprintf("Task %s skipped.", taskID)), nil

	case "modify":
		modification, _ := ValidateAndGetString(args, "modification", false)
		if modification == "" {
			return ErrorResult("Modification description required for 'modify' action"), nil
		}
		// Validate modification text (max 10KB to prevent abuse)
		if err := validateTextInput("modification", modification, 10240); err != nil {
			return ErrorResult(err.Error()), nil
		}

		task, err := store.GetTask(taskID)
		if err != nil {
			return ErrorResult("Task not found"), nil
		}

		task.Description = modification
		if err := store.SaveTask(task); err != nil {
			return ErrorResult(fmt.Sprintf("Failed to update task: %v", err)), nil
		}

		exec, err := h.newExecutorForAction(store)
		if err != nil {
			return ErrorResult(fmt.Sprintf("Failed to initialize executor: %v", err)), nil
		}
		defer exec.Close()

		_ = exec.ResolveBlocker(taskID, "Task modified")

		return SuccessResult(fmt.Sprintf("Task %s modified. You can now retry it.", taskID)), nil

	case "analyze":
		prov, modelName, err := initProviderForStage(h.configManager, "develop.blocker_analyze", "")
		if err != nil {
			return ErrorResult(fmt.Sprintf("Failed to initialize provider: %v", err)), nil
		}

		logFile := filepath.Join(projectPath, ".geoffrussy", "logs", fmt.Sprintf("%s.log", taskID))
		logs, err := os.ReadFile(logFile)
		if err != nil {
			logs = []byte("(no log file found)")
		}

		prompt := fmt.Sprintf("Analyze the failure for task %s.\n\nLogs:\n%s\n\nReturn exactly four sections with these headers:\n1) ROOT_CAUSE\n2) EVIDENCE\n3) FIX_PLAN\n4) VERIFY_COMMANDS\n\nKeep fixes concrete and scoped to this task. Do not include markdown code fences.", taskID, string(logs))
		resp, err := prov.Call(context.TODO(), modelName, prompt)
		if err != nil {
			return ErrorResult(fmt.Sprintf("Failed to analyze: %v", err)), nil
		}

		return SuccessResult(fmt.Sprintf("Analysis:\n%s", resp.Content)), nil

	default:
		return ErrorResult(fmt.Sprintf("Unknown action: %s", action)), nil
	}
}

type executionCollector struct {
	logDir       string
	output       strings.Builder
	phaseSummary strings.Builder
}

func newExecutionCollector(logDir string) *executionCollector {
	return &executionCollector{logDir: logDir}
}

func (c *executionCollector) start(exec *executor.Executor) chan bool {
	done := make(chan bool)

	go func() {
		defer func() { done <- true }()
		for update := range exec.StreamOutput() {
			line := fmt.Sprintf("[%s] %s\n", update.Type, update.Content)
			c.output.WriteString(line)

			if update.TaskID != "" {
				_ = appendTaskLog(c.logDir, update.TaskID, fmt.Sprintf("[%s] %s %s\n", time.Now().Format(time.RFC3339), update.Type, update.Content))
			}

			switch update.Type {
			case executor.TaskCompleted:
				if update.TaskID != "" {
					c.phaseSummary.WriteString(fmt.Sprintf("  ✅ Task %s: Completed\n", update.TaskID))
				}
			case executor.TaskError:
				if update.TaskID != "" {
					c.phaseSummary.WriteString(fmt.Sprintf("  ❌ Task %s: Failed - %v\n", update.TaskID, update.Error))
				}
			}
		}
	}()

	return done
}

func ensureLogDir(projectPath string) (string, error) {
	logDir := filepath.Join(projectPath, ".geoffrussy", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return "", err
	}
	return logDir, nil
}

func appendTaskLog(logDir, taskID, line string) error {
	logFile := filepath.Join(logDir, fmt.Sprintf("%s.log", taskID))
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(line)
	return err
}

func (h *ExecHandlers) newExecutorForAction(store *state.Store) (*executor.Executor, error) {
	prov, modelName, err := initProviderForStage(h.configManager, "develop.execute", "")
	if err != nil {
		return nil, err
	}

	return executor.NewExecutor(store, prov, modelName), nil
}
