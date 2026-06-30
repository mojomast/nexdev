package controlplane

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/safety"
	"github.com/mojomast/nexdev/internal/state"
	"github.com/mojomast/nexdev/internal/steering"
)

type StartRunRequest struct {
	ProjectDir string `json:"project_dir,omitempty"`
	Prompt     string `json:"prompt,omitempty"`
	FromStage  string `json:"from_stage,omitempty"`
	Stage      string `json:"stage,omitempty"`
	Yes        bool   `json:"yes,omitempty"`
	Cheap      bool   `json:"cheap,omitempty"`
	Brrrr      bool   `json:"brrrr,omitempty"`
}

type controlRequest struct {
	RunID  string `json:"run_id,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type skipRequest struct {
	RunID  string `json:"run_id,omitempty"`
	Reason string `json:"reason,omitempty"`
	TaskID string `json:"task_id,omitempty"`
}

type steerRequest struct {
	RunID   string `json:"run_id,omitempty"`
	TaskID  string `json:"task_id,omitempty"`
	Message string `json:"message"`
	Source  string `json:"source,omitempty"`
}

func (s *Server) registerRoutes() {
	s.mux.Handle("GET /health", s.route(RoleNone, http.HandlerFunc(s.handleHealth)))
	s.mux.Handle("GET /status", s.route(RoleObserver, http.HandlerFunc(s.handleStatus)))
	s.mux.Handle("GET /plan", s.route(RoleObserver, http.HandlerFunc(s.handlePlan)))
	s.mux.Handle("GET /artifacts", s.route(RoleObserver, http.HandlerFunc(s.handleArtifacts)))
	s.mux.Handle("GET /events", s.route(RoleObserver, http.HandlerFunc(s.handleEvents)))
	s.mux.Handle("GET /runs/{run_id}/stream", s.route(RoleObserver, http.HandlerFunc(s.handleStream)))
	s.mux.Handle("POST /runs", s.route(RoleOperator, http.HandlerFunc(s.handleStartRun)))
	s.mux.Handle("POST /pause", s.route(RoleOperator, http.HandlerFunc(s.handlePause)))
	s.mux.Handle("POST /resume", s.route(RoleOperator, http.HandlerFunc(s.handleResume)))
	s.mux.Handle("POST /skip", s.route(RoleOperator, http.HandlerFunc(s.handleSkip)))
	s.mux.Handle("POST /steer", s.route(RoleOperator, http.HandlerFunc(s.handleSteer)))
	s.mux.Handle("POST /detour", s.route(RoleOperator, http.HandlerFunc(s.handleDetour)))
	s.mux.Handle("POST /cancel", s.route(RoleAdmin, http.HandlerFunc(s.handleCancel)))
	s.mux.Handle("PUT /tasks/{task_id}", s.route(RoleAdmin, http.HandlerFunc(s.notImplemented)))
	s.mux.Handle("DELETE /tasks/{task_id}", s.route(RoleAdmin, http.HandlerFunc(s.notImplemented)))
	s.mux.Handle("POST /blockers/{blocker_id}/resolve", s.route(RoleOperator, http.HandlerFunc(s.handleResolveBlocker)))
	s.mux.Handle("GET /config", s.route(RoleObserver, http.HandlerFunc(s.handleConfig)))
	s.mux.Handle("PUT /config", s.route(RoleAdmin, http.HandlerFunc(s.notImplemented)))
	s.mux.Handle("GET /providers", s.route(RoleObserver, http.HandlerFunc(s.handleProviders)))
	s.mux.Handle("POST /providers/{name}/test", s.route(RoleOperator, http.HandlerFunc(s.notImplemented)))
	s.mux.Handle("GET /mcp/tools", s.route(RoleObserver, http.HandlerFunc(s.handleMCPTools)))
	s.mux.Handle("POST /mcp/call", s.route(RoleObserver, http.HandlerFunc(s.handleMCPCall)))
}

func (s *Server) route(required Role, next http.Handler) http.Handler {
	if !s.cfg.AuthRequired || required == RoleNone {
		return next
	}
	return s.authenticator.Middleware(required, next)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "api_contract_version": "nexdev-api-v1"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	snapshot, err := s.statusSnapshot(r)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "state_error", "failed to load status", map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request) {
	run, err := s.selectedRun(r)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "run_not_found", "run not found", nil)
		return
	}
	tasks, err := s.store.ListNexdevTasks(r.Context(), state.NexdevTaskListOptions{RunID: run.ID})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "state_error", "failed to load plan", nil)
		return
	}
	writeJSON(w, http.StatusOK, planResponse(run.ProjectID, run.ID, tasks))
}

func (s *Server) handleArtifacts(w http.ResponseWriter, r *http.Request) {
	projectID := s.projectID(r)
	if projectID == "" {
		writeError(w, r, http.StatusBadRequest, "project_required", "project_id is required", nil)
		return
	}
	artifacts, err := s.store.ListArtifacts(r.Context(), state.ArtifactListOptions{ProjectID: projectID, RunID: r.URL.Query().Get("run_id")})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "state_error", "failed to load artifacts", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project_id": projectID, "run_id": r.URL.Query().Get("run_id"), "artifacts": artifacts})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	runID := r.URL.Query().Get("run_id")
	if runID == "" {
		run, err := s.selectedRun(r)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "run_required", "run_id is required", nil)
			return
		}
		runID = run.ID
	}
	after, _ := strconv.ParseInt(r.URL.Query().Get("after_sequence"), 10, 64)
	events, err := s.store.ListEvents(r.Context(), state.EventListOptions{RunID: runID, AfterSequence: after, Limit: s.cfg.ReplayMaxEvents})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "state_error", "failed to load events", nil)
		return
	}
	if typ := r.URL.Query().Get("type"); typ != "" {
		filtered := events[:0]
		for _, event := range events {
			if event.Type == typ {
				filtered = append(filtered, event)
			}
		}
		events = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (s *Server) handleStartRun(w http.ResponseWriter, r *http.Request) {
	if s.runStarter == nil {
		writeError(w, r, http.StatusServiceUnavailable, "service_unavailable", "run starter is not wired", nil)
		return
	}
	var req StartRunRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	run, err := s.runStarter.StartRun(r.Context(), req)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "run_start_failed", "failed to start run", map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, runSnapshot(run))
}

func (s *Server) handlePause(w http.ResponseWriter, r *http.Request) {
	var req controlRequest
	if !decodeJSON(w, r, &req) || !s.requireExecutor(w, r) {
		return
	}
	if err := s.executor.Pause(r.Context(), req.Reason); err != nil {
		writeError(w, r, http.StatusInternalServerError, "pause_failed", "failed to pause run", nil)
		return
	}
	s.acceptedStatus(w, r)
}

func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	var req controlRequest
	if !decodeJSON(w, r, &req) || !s.requireExecutor(w, r) {
		return
	}
	if err := s.executor.Resume(r.Context()); err != nil {
		writeError(w, r, http.StatusInternalServerError, "resume_failed", "failed to resume run", nil)
		return
	}
	s.acceptedStatus(w, r)
}

func (s *Server) handleSkip(w http.ResponseWriter, r *http.Request) {
	var req skipRequest
	if !decodeJSON(w, r, &req) || !s.requireExecutor(w, r) {
		return
	}
	if err := s.executor.SkipTask(r.Context(), req.TaskID, req.Reason); err != nil {
		writeError(w, r, http.StatusInternalServerError, "skip_failed", "failed to skip task", nil)
		return
	}
	s.acceptedStatus(w, r)
}

func (s *Server) handleSteer(w http.ResponseWriter, r *http.Request) {
	var req steerRequest
	if !decodeJSON(w, r, &req) || !s.requireExecutor(w, r) {
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "message is required", nil)
		return
	}
	if req.Source == "" {
		req.Source = string(steering.SourceAPI)
	}
	if err := s.executor.SetSteeringContext(r.Context(), req.TaskID, steering.Message{RunID: req.RunID, TaskID: req.TaskID, Message: req.Message, Source: steering.Source(req.Source), CreatedAt: time.Now().UTC()}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "steer_failed", "failed to add steering", nil)
		return
	}
	s.acceptedStatus(w, r)
}

func (s *Server) handleDetour(w http.ResponseWriter, r *http.Request) {
	if s.detours == nil {
		writeError(w, r, http.StatusServiceUnavailable, "service_unavailable", "detour manager is not wired", nil)
		return
	}
	var req contract.DetourRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Source == "" {
		req.Source = contract.DetourSourceOperatorManual
	}
	result, err := s.detours.Request(r.Context(), req)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "detour_failed", "detour request failed", map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	var req controlRequest
	if !decodeJSON(w, r, &req) || !s.requireExecutor(w, r) {
		return
	}
	if err := s.executor.Cancel(r.Context(), req.Reason); err != nil {
		writeError(w, r, http.StatusInternalServerError, "cancel_failed", "failed to cancel run", nil)
		return
	}
	s.acceptedStatus(w, r)
}

func (s *Server) handleResolveBlocker(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RunID      string `json:"run_id,omitempty"`
		Resolution string `json:"resolution"`
		Resume     bool   `json:"resume,omitempty"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.store.ResolveNexdevBlocker(r.Context(), r.PathValue("blocker_id"), req.Resolution, time.Now().UTC()); err != nil {
		writeError(w, r, http.StatusBadRequest, "resolve_failed", "failed to resolve blocker", nil)
		return
	}
	if req.Resume && s.executor != nil {
		_ = s.executor.Resume(r.Context())
	}
	s.acceptedStatus(w, r)
}

func (s *Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"profile": "dev", "redacted": true, "controlplane": map[string]any{"bind": s.cfg.Bind, "auth_required": s.cfg.AuthRequired}})
}

func (s *Server) handleProviders(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"providers": []any{}})
}

func (s *Server) notImplemented(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusNotImplemented, "not_implemented", "route is not implemented in this milestone", nil)
}

func (s *Server) requireExecutor(w http.ResponseWriter, r *http.Request) bool {
	if s.executor == nil {
		writeError(w, r, http.StatusServiceUnavailable, "service_unavailable", "executor control is not wired", nil)
		return false
	}
	return true
}

func (s *Server) acceptedStatus(w http.ResponseWriter, r *http.Request) {
	snapshot, err := s.statusSnapshot(r)
	if err != nil {
		writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true})
		return
	}
	writeJSON(w, http.StatusAccepted, snapshot)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.Body == nil || r.ContentLength == 0 {
		return true
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", "invalid JSON request", nil)
		return false
	}
	return true
}

func (s *Server) projectID(r *http.Request) string {
	if id := r.URL.Query().Get("project_id"); id != "" {
		return id
	}
	return s.cfg.ProjectID
}

func (s *Server) selectedRun(r *http.Request) (*state.Run, error) {
	if runID := r.URL.Query().Get("run_id"); runID != "" {
		return s.store.GetRun(r.Context(), runID)
	}
	projectID := s.projectID(r)
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	runs, err := s.store.ListRunsByProject(r.Context(), projectID)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, fmt.Errorf("no runs for project")
	}
	return runs[len(runs)-1], nil
}

func (s *Server) statusSnapshot(r *http.Request) (map[string]any, error) {
	projectID := s.projectID(r)
	var active any
	var stages any = []any{}
	var blockers any = []any{}
	var currentTask any
	if run, err := s.selectedRun(r); err == nil {
		active = runSnapshot(run)
		stageRuns, err := s.store.ListStageRunsByRun(r.Context(), run.ID)
		if err != nil {
			return nil, err
		}
		stages = stageRuns
		openBlockers, err := s.store.ListNexdevBlockers(r.Context(), state.NexdevBlockerListOptions{RunID: run.ID, Status: state.NexdevBlockerStatusOpen})
		if err != nil {
			return nil, err
		}
		blockers = openBlockers
		if s.executor != nil {
			if snap, err := s.executor.CurrentTask(r.Context()); err == nil && snap != nil {
				currentTask = snap.Task
			}
		}
	} else if projectID == "" {
		return nil, err
	}
	return map[string]any{"project_id": projectID, "active_run": active, "stages": stages, "current_task": currentTask, "blockers": blockers, "updated_at": time.Now().UTC()}, nil
}

func runSnapshot(run *state.Run) map[string]any {
	if run == nil {
		return nil
	}
	return map[string]any{"run_id": run.ID, "project_id": run.ProjectID, "status": run.Status, "current_stage": run.CurrentStage, "started_at": run.StartedAt, "completed_at": run.CompletedAt, "metadata": run.Metadata}
}

func planResponse(projectID, runID string, tasks []*state.NexdevTask) map[string]any {
	version := 0
	phaseMap := map[string][]contract.TaskSpec{}
	for _, task := range tasks {
		if task.PlanVersion > version {
			version = task.PlanVersion
		}
		phaseMap[task.Spec.PhaseID] = append(phaseMap[task.Spec.PhaseID], task.Spec)
	}
	phaseIDs := make([]string, 0, len(phaseMap))
	for id := range phaseMap {
		phaseIDs = append(phaseIDs, id)
	}
	sort.Strings(phaseIDs)
	phases := make([]map[string]any, 0, len(phaseIDs))
	for i, id := range phaseIDs {
		phases = append(phases, map[string]any{"id": id, "number": i + 1, "title": id, "tasks": phaseMap[id]})
	}
	return map[string]any{"project_id": projectID, "run_id": runID, "version": version, "phases": phases}
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && s.originAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Last-Event-ID, X-Request-ID")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originAllowed(origin string) bool {
	for _, allowed := range s.cfg.CORSAllowedOrigins {
		if allowed == origin || (!s.cfg.AuthRequired && allowed == "*") {
			return true
		}
	}
	return false
}

func redactError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrForbidden) {
		return err.Error()
	}
	return safety.RedactSecrets(err.Error())
}
