package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/safety"
	"github.com/mojomast/nexdev/internal/state"
	"github.com/mojomast/nexdev/internal/steering"
)

type MCPTool struct {
	Name        string         `json:"name"`
	Role        Role           `json:"role"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type mcpCallRequest struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type mcpCallResult struct {
	Tool    string `json:"tool"`
	IsError bool   `json:"is_error"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
}

type mcpError struct {
	ErrorCode string         `json:"error_code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
}

var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)

func MCPTools() []MCPTool {
	tools := []MCPTool{
		{Name: "nexdev_start_run", Role: RoleOperator, Description: "Start a Nexdev pipeline run through the configured run service.", InputSchema: objectSchema(map[string]any{"project_dir": stringSchema("Optional project directory."), "prompt": stringSchema("Optional run prompt."), "from_stage": stringSchema("Optional starting stage."), "stage": stringSchema("Optional single stage."), "yes": boolSchema("Assume conservative defaults for unanswered questions."), "cheap": boolSchema("Prefer cheap execution profile."), "brrrr": boolSchema("Prefer maximum safe parallelism profile.")}, nil)},
		{Name: "nexdev_get_status", Role: RoleObserver, Description: "Read the durable project and active-run status snapshot.", InputSchema: objectSchema(map[string]any{"project_id": stringSchema("Optional project ID."), "run_id": stringSchema("Optional run ID.")}, nil)},
		{Name: "nexdev_get_plan", Role: RoleObserver, Description: "Read the current persisted devplan for a run.", InputSchema: objectSchema(map[string]any{"project_id": stringSchema("Optional project ID."), "run_id": stringSchema("Optional run ID.")}, nil)},
		{Name: "nexdev_list_artifacts", Role: RoleObserver, Description: "List SQLite-indexed artifacts for a project and optional run.", InputSchema: objectSchema(map[string]any{"project_id": stringSchema("Optional project ID."), "run_id": stringSchema("Optional run ID."), "kind": stringSchema("Optional artifact kind filter.")}, nil)},
		{Name: "nexdev_get_artifact", Role: RoleObserver, Description: "Read artifact metadata by ID or by kind/path from the durable artifact index.", InputSchema: objectSchema(map[string]any{"project_id": stringSchema("Optional project ID."), "run_id": stringSchema("Optional run ID."), "artifact_id": stringSchema("Artifact ID."), "kind": stringSchema("Artifact kind."), "path": stringSchema("Project-relative artifact path.")}, nil)},
		{Name: "nexdev_pause", Role: RoleOperator, Description: "Pause the active executor through the executor control service.", InputSchema: objectSchema(map[string]any{"run_id": stringSchema("Optional run ID."), "reason": stringSchema("Pause reason.")}, nil)},
		{Name: "nexdev_resume", Role: RoleOperator, Description: "Resume the active executor through the executor control service.", InputSchema: objectSchema(map[string]any{"run_id": stringSchema("Optional run ID.")}, nil)},
		{Name: "nexdev_cancel", Role: RoleAdmin, Description: "Cancel the active executor through the executor control service.", InputSchema: objectSchema(map[string]any{"run_id": stringSchema("Optional run ID."), "reason": stringSchema("Cancel reason.")}, nil)},
		{Name: "nexdev_steer", Role: RoleOperator, Description: "Add durable steering context through the executor steering service.", InputSchema: objectSchema(map[string]any{"run_id": stringSchema("Run ID."), "task_id": stringSchema("Optional task ID."), "message": stringSchema("Steering message.")}, []string{"message"})},
		{Name: "nexdev_detour", Role: RoleOperator, Description: "Request a detour through the existing detour workflow manager or blocker requester.", InputSchema: objectSchema(map[string]any{"project_id": stringSchema("Project ID."), "run_id": stringSchema("Run ID."), "trigger_task_id": stringSchema("Trigger task ID for manual detour."), "blocker_id": stringSchema("Existing blocker ID to request a blocker detour."), "reason": stringSchema("Detour reason."), "context": stringSchema("Bounded detour context.")}, nil)},
		{Name: "nexdev_resolve_blocker", Role: RoleOperator, Description: "Resolve a durable blocker and optionally resume executor controls.", InputSchema: objectSchema(map[string]any{"blocker_id": stringSchema("Blocker ID."), "resolution": stringSchema("Resolution text."), "resume": boolSchema("Resume after resolving."), "run_id": stringSchema("Optional run ID.")}, []string{"blocker_id", "resolution"})},
		{Name: "nexdev_provider_test", Role: RoleOperator, Description: "Invoke the configured provider-test service when it is wired; never calls providers directly from MCP.", InputSchema: objectSchema(map[string]any{"name": stringSchema("Provider name.")}, []string{"name"})},
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools
}

func (s *Server) handleMCPTools(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"tools": MCPTools()})
}

func (s *Server) handleMCPCall(w http.ResponseWriter, r *http.Request) {
	var req mcpCallRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	result := s.dispatchMCPCall(r.Context(), s.mcpActorRole(r), strings.TrimSpace(req.Name), req.Arguments)
	status := http.StatusOK
	if result.IsError {
		status = http.StatusBadRequest
		if errObj, ok := result.Error.(mcpError); ok && errObj.ErrorCode == "forbidden" {
			status = http.StatusForbidden
		}
	}
	writeJSON(w, status, result)
}

func (s *Server) dispatchMCPCall(ctx context.Context, actorRole Role, name string, args map[string]any) mcpCallResult {
	tool, ok := mcpToolByName(name)
	if !ok {
		return mcpFailure(name, "tool_not_found", "MCP tool is not registered", nil)
	}
	if !AllowsRoute(actorRole, RolePerTool, tool.Role) {
		return mcpFailure(name, "forbidden", "insufficient role for MCP tool", map[string]any{"required_role": string(tool.Role)})
	}
	if args == nil {
		args = map[string]any{}
	}
	if err := validateKnownArgs(args, schemaProperties(tool.InputSchema)); err != nil {
		return mcpFailure(name, "invalid_request", err.Error(), nil)
	}

	var result any
	var err error
	switch name {
	case "nexdev_start_run":
		result, err = s.mcpStartRun(ctx, args)
	case "nexdev_get_status":
		result, err = s.mcpStatus(ctx, args)
	case "nexdev_get_plan":
		result, err = s.mcpPlan(ctx, args)
	case "nexdev_list_artifacts":
		result, err = s.mcpListArtifacts(ctx, args)
	case "nexdev_get_artifact":
		result, err = s.mcpGetArtifact(ctx, args)
	case "nexdev_pause":
		result, err = s.mcpPause(ctx, args)
	case "nexdev_resume":
		result, err = s.mcpResume(ctx, args)
	case "nexdev_cancel":
		result, err = s.mcpCancel(ctx, args)
	case "nexdev_steer":
		result, err = s.mcpSteer(ctx, args)
	case "nexdev_detour":
		result, err = s.mcpDetour(ctx, args)
	case "nexdev_resolve_blocker":
		result, err = s.mcpResolveBlocker(ctx, args)
	case "nexdev_provider_test":
		result, err = s.mcpProviderTest(ctx, args)
	default:
		err = fmt.Errorf("tool handler is not implemented")
	}
	if err != nil {
		return mcpFailure(name, mcpErrorCode(err), err.Error(), nil)
	}
	return mcpCallResult{Tool: name, Result: result}
}

func (s *Server) mcpActorRole(r *http.Request) Role {
	if !s.cfg.AuthRequired {
		return RoleAdmin
	}
	actor, ok := ActorFromContext(r.Context())
	if !ok {
		return ""
	}
	return actor.Role
}

func (s *Server) mcpStartRun(ctx context.Context, args map[string]any) (any, error) {
	if s.runStarter == nil {
		return nil, mcpServiceUnavailable("run starter is not wired")
	}
	req := StartRunRequest{ProjectDir: optionalString(args, "project_dir"), Prompt: optionalString(args, "prompt"), FromStage: optionalString(args, "from_stage"), Stage: optionalString(args, "stage"), Yes: optionalBool(args, "yes"), Cheap: optionalBool(args, "cheap"), Brrrr: optionalBool(args, "brrrr")}
	if err := validateOptionalID("from_stage", req.FromStage); err != nil {
		return nil, err
	}
	if err := validateOptionalID("stage", req.Stage); err != nil {
		return nil, err
	}
	run, err := s.runStarter.StartRun(ctx, req)
	if err != nil {
		return nil, err
	}
	return runSnapshot(run), nil
}

func (s *Server) mcpStatus(ctx context.Context, args map[string]any) (any, error) {
	r, err := s.syntheticRequest(ctx, args)
	if err != nil {
		return nil, err
	}
	return s.statusSnapshot(r)
}

func (s *Server) mcpPlan(ctx context.Context, args map[string]any) (any, error) {
	r, err := s.syntheticRequest(ctx, args)
	if err != nil {
		return nil, err
	}
	run, err := s.selectedRun(r)
	if err != nil {
		return nil, err
	}
	tasks, err := s.store.ListNexdevTasks(ctx, state.NexdevTaskListOptions{RunID: run.ID})
	if err != nil {
		return nil, err
	}
	return planResponse(run.ProjectID, run.ID, tasks), nil
}

func (s *Server) mcpListArtifacts(ctx context.Context, args map[string]any) (any, error) {
	projectID := optionalString(args, "project_id")
	if projectID == "" {
		projectID = s.cfg.ProjectID
	}
	if err := validateRequiredID("project_id", projectID); err != nil {
		return nil, err
	}
	runID := optionalString(args, "run_id")
	if err := validateOptionalID("run_id", runID); err != nil {
		return nil, err
	}
	artifacts, err := s.store.ListArtifacts(ctx, state.ArtifactListOptions{ProjectID: projectID, RunID: runID, Kind: optionalString(args, "kind")})
	if err != nil {
		return nil, err
	}
	return map[string]any{"project_id": projectID, "run_id": runID, "artifacts": artifacts}, nil
}

func (s *Server) mcpGetArtifact(ctx context.Context, args map[string]any) (any, error) {
	if id := optionalString(args, "artifact_id"); id != "" {
		if err := validateRequiredID("artifact_id", id); err != nil {
			return nil, err
		}
		return s.store.GetArtifact(ctx, id)
	}
	projectID := optionalString(args, "project_id")
	if projectID == "" {
		projectID = s.cfg.ProjectID
	}
	if err := validateRequiredID("project_id", projectID); err != nil {
		return nil, err
	}
	kind := optionalString(args, "kind")
	artifactPath := optionalString(args, "path")
	if kind == "" || artifactPath == "" {
		return nil, fmt.Errorf("artifact_id or both kind and path are required")
	}
	if err := validateArtifactPath(artifactPath); err != nil {
		return nil, err
	}
	artifacts, err := s.store.ListArtifacts(ctx, state.ArtifactListOptions{ProjectID: projectID, RunID: optionalString(args, "run_id"), Kind: kind})
	if err != nil {
		return nil, err
	}
	for _, artifact := range artifacts {
		if artifact.Path == artifactPath {
			return artifact, nil
		}
	}
	return nil, fmt.Errorf("artifact not found")
}

func (s *Server) mcpPause(ctx context.Context, args map[string]any) (any, error) {
	if s.executor == nil {
		return nil, mcpServiceUnavailable("executor control is not wired")
	}
	if err := s.executor.Pause(ctx, optionalString(args, "reason")); err != nil {
		return nil, err
	}
	return map[string]any{"accepted": true}, nil
}

func (s *Server) mcpResume(ctx context.Context, _ map[string]any) (any, error) {
	if s.executor == nil {
		return nil, mcpServiceUnavailable("executor control is not wired")
	}
	if err := s.executor.Resume(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"accepted": true}, nil
}

func (s *Server) mcpCancel(ctx context.Context, args map[string]any) (any, error) {
	if s.executor == nil {
		return nil, mcpServiceUnavailable("executor control is not wired")
	}
	if err := s.executor.Cancel(ctx, optionalString(args, "reason")); err != nil {
		return nil, err
	}
	return map[string]any{"accepted": true}, nil
}

func (s *Server) mcpSteer(ctx context.Context, args map[string]any) (any, error) {
	if s.executor == nil {
		return nil, mcpServiceUnavailable("executor control is not wired")
	}
	message := strings.TrimSpace(optionalString(args, "message"))
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}
	taskID := optionalString(args, "task_id")
	if err := validateOptionalID("task_id", taskID); err != nil {
		return nil, err
	}
	if err := s.executor.SetSteeringContext(ctx, taskID, steering.Message{RunID: optionalString(args, "run_id"), TaskID: taskID, Message: message, Source: steering.SourceMCP, CreatedAt: time.Now().UTC()}); err != nil {
		return nil, err
	}
	return map[string]any{"accepted": true}, nil
}

func (s *Server) mcpDetour(ctx context.Context, args map[string]any) (any, error) {
	if s.detours == nil {
		return nil, mcpServiceUnavailable("detour manager is not wired")
	}
	runID := optionalString(args, "run_id")
	if err := validateRequiredID("run_id", runID); err != nil {
		return nil, err
	}
	if blockerID := optionalString(args, "blocker_id"); blockerID != "" {
		if err := validateRequiredID("blocker_id", blockerID); err != nil {
			return nil, err
		}
		return s.detours.RequestForBlocker(ctx, runID, blockerID)
	}
	req := contract.DetourRequest{ProjectID: optionalString(args, "project_id"), RunID: runID, TriggerTaskID: optionalString(args, "trigger_task_id"), Reason: optionalString(args, "reason"), Context: optionalString(args, "context"), Source: contract.DetourSourceOperatorManual}
	if err := validateRequiredID("project_id", req.ProjectID); err != nil {
		return nil, err
	}
	if err := validateRequiredID("trigger_task_id", req.TriggerTaskID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Reason) == "" {
		return nil, fmt.Errorf("reason is required")
	}
	return s.detours.Request(ctx, req)
}

func (s *Server) mcpResolveBlocker(ctx context.Context, args map[string]any) (any, error) {
	blockerID := optionalString(args, "blocker_id")
	resolution := strings.TrimSpace(optionalString(args, "resolution"))
	if err := validateRequiredID("blocker_id", blockerID); err != nil {
		return nil, err
	}
	if resolution == "" {
		return nil, fmt.Errorf("resolution is required")
	}
	if err := s.store.ResolveNexdevBlocker(ctx, blockerID, resolution, time.Now().UTC()); err != nil {
		return nil, err
	}
	if optionalBool(args, "resume") && s.executor != nil {
		_ = s.executor.Resume(ctx)
	}
	return map[string]any{"accepted": true}, nil
}

func (s *Server) mcpProviderTest(ctx context.Context, args map[string]any) (any, error) {
	name := optionalString(args, "name")
	if err := validateRequiredID("name", name); err != nil {
		return nil, err
	}
	if s.providerTests == nil {
		return nil, mcpNotImplemented("provider test service is not wired")
	}
	return s.providerTests.TestProvider(ctx, name)
}

func (s *Server) syntheticRequest(ctx context.Context, args map[string]any) (*http.Request, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	q := req.URL.Query()
	for _, key := range []string{"project_id", "run_id"} {
		value := optionalString(args, key)
		if err := validateOptionalID(key, value); err != nil {
			return nil, err
		}
		if value != "" {
			q.Set(key, value)
		}
	}
	req.URL.RawQuery = q.Encode()
	return req, nil
}

func mcpToolByName(name string) (MCPTool, bool) {
	for _, tool := range MCPTools() {
		if tool.Name == name {
			return tool, true
		}
	}
	return MCPTool{}, false
}

func mcpFailure(tool, code, message string, details map[string]any) mcpCallResult {
	if details == nil {
		details = map[string]any{}
	}
	for key, value := range details {
		if text, ok := value.(string); ok {
			details[key] = safety.RedactSecrets(text)
		}
	}
	return mcpCallResult{Tool: tool, IsError: true, Error: mcpError{ErrorCode: code, Message: safety.RedactSecrets(message), Details: details}}
}

func mcpErrorCode(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case strings.Contains(err.Error(), "not wired"):
		return "service_unavailable"
	case strings.Contains(err.Error(), "not implemented"):
		return "not_implemented"
	default:
		return "invalid_request"
	}
}

func mcpServiceUnavailable(message string) error { return fmt.Errorf("%s", message) }
func mcpNotImplemented(message string) error     { return fmt.Errorf("not implemented: %s", message) }

func validateKnownArgs(args map[string]any, allowed map[string]any) error {
	for key := range args {
		schema, ok := allowed[key]
		if !ok {
			return fmt.Errorf("unknown argument %q", key)
		}
		if err := validateMCPArgType(key, args[key], schema); err != nil {
			return err
		}
	}
	return nil
}

func validateMCPArgType(key string, value any, schema any) error {
	if value == nil {
		return nil
	}
	def, _ := schema.(map[string]any)
	want, _ := def["type"].(string)
	switch want {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("argument %q must be a string", key)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("argument %q must be a boolean", key)
		}
	}
	return nil
}

func validateRequiredID(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return validateOptionalID(name, value)
}

func validateOptionalID(name, value string) error {
	if value == "" {
		return nil
	}
	if !safeIDPattern.MatchString(value) {
		return fmt.Errorf("%s contains invalid characters", name)
	}
	return nil
}

func validateArtifactPath(value string) error {
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return fmt.Errorf("artifact path must be project-relative")
	}
	cleaned := path.Clean(value)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return fmt.Errorf("artifact path must not escape project root")
	}
	return nil
}

func optionalString(args map[string]any, key string) string {
	value, ok := args[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func optionalBool(args map[string]any, key string) bool {
	value, ok := args[key]
	if !ok || value == nil {
		return false
	}
	flag, _ := value.(bool)
	return flag
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	schema := map[string]any{"type": "object", "additionalProperties": false, "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func boolSchema(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

func schemaProperties(schema map[string]any) map[string]any {
	props, _ := schema["properties"].(map[string]any)
	return props
}

func marshalMCPToolsForManifest() ([]byte, error) {
	return json.MarshalIndent(map[string]any{"tools": MCPTools()}, "", "  ")
}
