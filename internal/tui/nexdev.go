package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/safety"
)

type ViewMode int

const (
	ViewOverview ViewMode = iota
	ViewEvents
	ViewPlan
	ViewBlockers
	ViewArtifacts
)

type Client interface {
	Snapshot(ctx context.Context) (Snapshot, error)
	Pause(ctx context.Context, reason string) error
	Resume(ctx context.Context) error
	Skip(ctx context.Context, taskID, reason string) error
	Steer(ctx context.Context, taskID, message string) error
	RequestDetour(ctx context.Context, req contract.DetourRequest) (contract.DetourResult, error)
	Cancel(ctx context.Context, reason string) error
}

type Snapshot struct {
	ProjectID string
	Run       RunSummary
	Tasks     []TaskSummary
	Events    []contract.EventEnvelope
	Blockers  []BlockerSummary
	Artifacts []ArtifactSummary
	Config    ConfigSummary
	Providers []ProviderSummary
}

type RunSummary struct {
	RunID        string
	Status       string
	CurrentStage string
	CurrentTask  string
	Cost         string
}

type TaskSummary struct {
	ID          string
	PhaseID     string
	Title       string
	Status      string
	RiskLevel   string
	Description string
}

type BlockerSummary struct {
	ID          string
	TaskID      string
	Reason      string
	Description string
	Status      string
}

type ArtifactSummary struct {
	ID      string
	Kind    string
	Path    string
	Version int
}

type ConfigSummary struct {
	Profile      string
	AuthRequired bool
	Bind         string
	Redacted     bool
}

type ProviderSummary struct {
	Name   string
	Status string
}

type Model struct {
	client       Client
	snapshot     Snapshot
	view         ViewMode
	status       string
	err          error
	quitting     bool
	confirm      string
	width        int
	height       int
	lastAction   string
	selectedTask int
}

type snapshotMsg struct {
	snapshot Snapshot
	err      error
}

type actionMsg struct {
	label string
	err   error
}

func NewModel(client Client) Model {
	return Model{client: client, view: ViewOverview, status: "loading", width: 100, height: 30}
}

func Run(ctx context.Context, client Client) error {
	_, err := tea.NewProgram(NewModel(client), tea.WithContext(ctx)).Run()
	return err
}

func (m Model) Init() tea.Cmd {
	return m.refreshCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case snapshotMsg:
		m.err = msg.err
		if msg.err == nil {
			m.snapshot = msg.snapshot
			m.status = "refreshed"
		}
		return m, nil
	case actionMsg:
		m.err = msg.err
		m.lastAction = msg.label
		m.confirm = ""
		if msg.err != nil {
			m.status = sanitizeDisplay(msg.err.Error())
			return m, nil
		}
		m.status = msg.label + " accepted"
		return m, m.refreshCmd()
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.confirm != "" {
		switch key {
		case "y", "Y":
			if m.confirm == "cancel" {
				return m, m.actionCmd("cancel", func(ctx context.Context) error { return m.client.Cancel(ctx, "confirmed from TUI") })
			}
			if m.confirm == "skip" {
				taskID := m.currentTaskID()
				return m, m.actionCmd("skip", func(ctx context.Context) error { return m.client.Skip(ctx, taskID, "confirmed from TUI") })
			}
		case "n", "N", "esc", "q":
			m.confirm = ""
			m.status = "confirmation cancelled"
			return m, nil
		}
		return m, nil
	}

	switch key {
	case "q", "esc", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "1":
		m.view = ViewOverview
	case "2":
		m.view = ViewEvents
	case "3":
		m.view = ViewPlan
	case "4":
		m.view = ViewBlockers
	case "5":
		m.view = ViewArtifacts
	case "r":
		return m, m.refreshCmd()
	case "p":
		if strings.EqualFold(m.snapshot.Run.Status, "paused") || strings.EqualFold(m.snapshot.Run.Status, "blocked") {
			return m, m.actionCmd("resume", func(ctx context.Context) error { return m.client.Resume(ctx) })
		}
		return m, m.actionCmd("pause", func(ctx context.Context) error { return m.client.Pause(ctx, "requested from TUI") })
	case "s":
		return m, m.actionCmd("steer", func(ctx context.Context) error {
			return m.client.Steer(ctx, m.currentTaskID(), "TUI steering entry is deferred until text input is wired")
		})
	case "d":
		return m, m.actionCmd("detour", func(ctx context.Context) error {
			_, err := m.client.RequestDetour(ctx, contract.DetourRequest{ProjectID: m.snapshot.ProjectID, RunID: m.snapshot.Run.RunID, TriggerTaskID: m.currentTaskID(), Reason: "requested from TUI", Source: contract.DetourSourceOperatorManual})
			return err
		})
	case "k":
		m.confirm = "skip"
		m.status = "confirm skip current task with y/n"
	case "c":
		m.confirm = "cancel"
		m.status = "confirm cancel run with y/n"
	case "up":
		if m.selectedTask > 0 {
			m.selectedTask--
		}
	case "down", "j":
		if m.selectedTask < len(m.snapshot.Tasks)-1 {
			m.selectedTask++
		}
	default:
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("Nexdev Terminal Control")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(sanitizeDisplay(fmt.Sprintf("project %s | run %s | status %s | stage %s", m.snapshot.ProjectID, m.snapshot.Run.RunID, m.snapshot.Run.Status, m.snapshot.Run.CurrentStage)))
	b.WriteString("\n")
	if m.err != nil {
		b.WriteString("error: " + sanitizeDisplay(m.err.Error()) + "\n")
	}
	if m.status != "" {
		b.WriteString("status: " + sanitizeDisplay(m.status) + "\n")
	}
	if m.confirm != "" {
		b.WriteString("confirmation required: press y to confirm or n to cancel\n")
	}
	b.WriteString("\n")
	b.WriteString("views: 1 overview 2 events 3 plan 4 blockers 5 artifacts | r refresh | p pause/resume | s steer | d detour | k skip | c cancel | q quit\n\n")
	switch m.view {
	case ViewEvents:
		b.WriteString(m.renderEvents())
	case ViewPlan:
		b.WriteString(m.renderPlan())
	case ViewBlockers:
		b.WriteString(m.renderBlockers())
	case ViewArtifacts:
		b.WriteString(m.renderArtifacts())
	default:
		b.WriteString(m.renderOverview())
	}
	return b.String()
}

func (m Model) renderOverview() string {
	var b strings.Builder
	b.WriteString("Overview\n")
	b.WriteString(fmt.Sprintf("current task: %s\n", sanitizeDisplay(emptyAs(m.snapshot.Run.CurrentTask, "none"))))
	b.WriteString(fmt.Sprintf("tasks: %d total, %d open blockers\n", len(m.snapshot.Tasks), len(m.snapshot.Blockers)))
	b.WriteString(fmt.Sprintf("cost: %s\n", sanitizeDisplay(emptyAs(m.snapshot.Run.Cost, "unavailable"))))
	b.WriteString("providers:\n")
	if len(m.snapshot.Providers) == 0 {
		b.WriteString("  deferred: provider summary service is not wired\n")
	} else {
		for _, provider := range m.snapshot.Providers {
			b.WriteString(fmt.Sprintf("  %s: %s\n", sanitizeDisplay(provider.Name), sanitizeDisplay(provider.Status)))
		}
	}
	b.WriteString("config:\n")
	b.WriteString(fmt.Sprintf("  profile=%s bind=%s auth_required=%t redacted=%t\n", sanitizeDisplay(m.snapshot.Config.Profile), sanitizeDisplay(m.snapshot.Config.Bind), m.snapshot.Config.AuthRequired, m.snapshot.Config.Redacted))
	return b.String()
}

func (m Model) renderEvents() string {
	var b strings.Builder
	b.WriteString("Event Stream\n")
	if len(m.snapshot.Events) == 0 {
		return b.String() + "  no events\n"
	}
	for _, event := range m.snapshot.Events {
		payload := sanitizeDisplay(string(event.Payload))
		b.WriteString(fmt.Sprintf("  #%d %s %s %s\n", event.Sequence, sanitizeDisplay(event.Type), sanitizeDisplay(event.TaskID), payload))
	}
	return b.String()
}

func (m Model) renderPlan() string {
	var b strings.Builder
	b.WriteString("Plan / Tasks\n")
	if len(m.snapshot.Tasks) == 0 {
		return b.String() + "  no tasks\n"
	}
	for i, task := range m.snapshot.Tasks {
		cursor := "  "
		if i == m.selectedTask {
			cursor = "> "
		}
		b.WriteString(fmt.Sprintf("%s%s [%s] %s (%s)\n", cursor, sanitizeDisplay(task.ID), sanitizeDisplay(task.Status), sanitizeDisplay(task.Title), sanitizeDisplay(task.PhaseID)))
	}
	b.WriteString("\nplan editing is deferred until the control-plane review edit service is wired; TUI will not mutate plan state directly.\n")
	return b.String()
}

func (m Model) renderBlockers() string {
	var b strings.Builder
	b.WriteString("Blockers / Detours\n")
	if len(m.snapshot.Blockers) == 0 {
		b.WriteString("  no open blockers\n")
	} else {
		for _, blocker := range m.snapshot.Blockers {
			b.WriteString(fmt.Sprintf("  %s task=%s reason=%s status=%s\n", sanitizeDisplay(blocker.ID), sanitizeDisplay(blocker.TaskID), sanitizeDisplay(blocker.Reason), sanitizeDisplay(blocker.Status)))
			b.WriteString("    " + sanitizeDisplay(blocker.Description) + "\n")
		}
	}
	b.WriteString("\ndetour action uses the injected service path when available; otherwise it returns a structured disabled state.\n")
	return b.String()
}

func (m Model) renderArtifacts() string {
	var b strings.Builder
	b.WriteString("Artifacts / Config / Providers\n")
	if len(m.snapshot.Artifacts) == 0 {
		b.WriteString("  no artifacts indexed\n")
	} else {
		for _, artifact := range m.snapshot.Artifacts {
			b.WriteString(fmt.Sprintf("  %s %s v%d %s\n", sanitizeDisplay(artifact.Kind), sanitizeDisplay(artifact.ID), artifact.Version, sanitizeDisplay(artifact.Path)))
		}
	}
	b.WriteString("\nprovider test is deferred unless a provider tester service is injected.\n")
	return b.String()
}

func (m Model) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return snapshotMsg{err: fmt.Errorf("TUI client is not wired")}
		}
		snapshot, err := m.client.Snapshot(context.Background())
		return snapshotMsg{snapshot: snapshot, err: err}
	}
}

func (m Model) actionCmd(label string, fn func(context.Context) error) tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return actionMsg{label: label, err: fmt.Errorf("TUI client is not wired")}
		}
		return actionMsg{label: label, err: fn(context.Background())}
	}
}

func (m Model) currentTaskID() string {
	if m.snapshot.Run.CurrentTask != "" {
		return m.snapshot.Run.CurrentTask
	}
	if len(m.snapshot.Tasks) == 0 {
		return ""
	}
	if m.selectedTask < 0 || m.selectedTask >= len(m.snapshot.Tasks) {
		return m.snapshot.Tasks[0].ID
	}
	return m.snapshot.Tasks[m.selectedTask].ID
}

func sanitizeDisplay(text string) string {
	redacted := safety.RedactSecrets(text)
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r == 0x1b || (unicode.IsControl(r) && r != '\n' && r != '\t') {
			return -1
		}
		return r
	}, redacted)
}

func emptyAs(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

type HTTPClient struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

func NewHTTPClient(baseURL, token string) HTTPClient {
	return HTTPClient{BaseURL: strings.TrimRight(baseURL, "/"), Token: token, Client: http.DefaultClient}
}

func (c HTTPClient) Snapshot(ctx context.Context) (Snapshot, error) {
	status, err := c.get(ctx, "/status")
	if err != nil {
		return Snapshot{}, err
	}
	plan, _ := c.get(ctx, "/plan")
	events, _ := c.get(ctx, "/events")
	artifacts, _ := c.get(ctx, "/artifacts")
	config, _ := c.get(ctx, "/config")
	providers, _ := c.get(ctx, "/providers")
	return snapshotFromMaps(status, plan, events, artifacts, config, providers), nil
}

func (c HTTPClient) Pause(ctx context.Context, reason string) error {
	return c.post(ctx, "/pause", map[string]any{"reason": reason}, nil)
}

func (c HTTPClient) Resume(ctx context.Context) error {
	return c.post(ctx, "/resume", map[string]any{}, nil)
}

func (c HTTPClient) Skip(ctx context.Context, taskID, reason string) error {
	return c.post(ctx, "/skip", map[string]any{"task_id": taskID, "reason": reason}, nil)
}

func (c HTTPClient) Steer(ctx context.Context, taskID, message string) error {
	return c.post(ctx, "/steer", map[string]any{"task_id": taskID, "message": message, "source": "tui"}, nil)
}

func (c HTTPClient) RequestDetour(ctx context.Context, req contract.DetourRequest) (contract.DetourResult, error) {
	var result contract.DetourResult
	err := c.post(ctx, "/detour", req, &result)
	return result, err
}

func (c HTTPClient) Cancel(ctx context.Context, reason string) error {
	return c.post(ctx, "/cancel", map[string]any{"reason": reason}, nil)
}

func (c HTTPClient) get(ctx context.Context, path string) (map[string]any, error) {
	var out map[string]any
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	return out, err
}

func (c HTTPClient) post(ctx context.Context, path string, body any, out any) error {
	return c.do(ctx, http.MethodPost, path, body, out)
}

func (c HTTPClient) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	base := c.BaseURL
	if base == "" {
		base = "http://127.0.0.1"
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("control-plane request failed (%d): %s", resp.StatusCode, sanitizeDisplay(string(data)))
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return err
		}
	}
	return nil
}

type HandlerClient struct {
	Handler http.Handler
	Token   string
}

func NewHandlerClient(handler http.Handler, token string) HandlerClient {
	return HandlerClient{Handler: handler, Token: token}
}

func (c HandlerClient) Snapshot(ctx context.Context) (Snapshot, error) {
	return NewHTTPClient("", c.Token).withRoundTripper(handlerRoundTripper{handler: c.Handler}).Snapshot(ctx)
}

func (c HandlerClient) Pause(ctx context.Context, reason string) error {
	return NewHTTPClient("", c.Token).withRoundTripper(handlerRoundTripper{handler: c.Handler}).Pause(ctx, reason)
}

func (c HandlerClient) Resume(ctx context.Context) error {
	return NewHTTPClient("", c.Token).withRoundTripper(handlerRoundTripper{handler: c.Handler}).Resume(ctx)
}

func (c HandlerClient) Skip(ctx context.Context, taskID, reason string) error {
	return NewHTTPClient("", c.Token).withRoundTripper(handlerRoundTripper{handler: c.Handler}).Skip(ctx, taskID, reason)
}

func (c HandlerClient) Steer(ctx context.Context, taskID, message string) error {
	return NewHTTPClient("", c.Token).withRoundTripper(handlerRoundTripper{handler: c.Handler}).Steer(ctx, taskID, message)
}

func (c HandlerClient) RequestDetour(ctx context.Context, req contract.DetourRequest) (contract.DetourResult, error) {
	return NewHTTPClient("", c.Token).withRoundTripper(handlerRoundTripper{handler: c.Handler}).RequestDetour(ctx, req)
}

func (c HandlerClient) Cancel(ctx context.Context, reason string) error {
	return NewHTTPClient("", c.Token).withRoundTripper(handlerRoundTripper{handler: c.Handler}).Cancel(ctx, reason)
}

func (c HTTPClient) withRoundTripper(rt http.RoundTripper) HTTPClient {
	c.Client = &http.Client{Transport: rt}
	return c
}

type handlerRoundTripper struct{ handler http.Handler }

func (rt handlerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.handler == nil {
		return nil, fmt.Errorf("control-plane handler is not wired")
	}
	rec := &memoryResponse{header: http.Header{}}
	if req.URL.Scheme == "" {
		req.URL.Scheme = "http"
	}
	if req.URL.Host == "" {
		req.URL.Host = "127.0.0.1"
	}
	rt.handler.ServeHTTP(rec, req)
	return &http.Response{StatusCode: rec.codeOrOK(), Header: rec.header, Body: io.NopCloser(bytes.NewReader(rec.body.Bytes())), Request: req}, nil
}

type memoryResponse struct {
	code   int
	header http.Header
	body   bytes.Buffer
}

func (r *memoryResponse) Header() http.Header  { return r.header }
func (r *memoryResponse) WriteHeader(code int) { r.code = code }
func (r *memoryResponse) Write(data []byte) (int, error) {
	if r.code == 0 {
		r.code = http.StatusOK
	}
	return r.body.Write(data)
}
func (r *memoryResponse) codeOrOK() int {
	if r.code == 0 {
		return http.StatusOK
	}
	return r.code
}

func snapshotFromMaps(status, plan, events, artifacts, config, providers map[string]any) Snapshot {
	s := Snapshot{ProjectID: stringValue(status, "project_id")}
	if run, ok := status["active_run"].(map[string]any); ok {
		s.Run = RunSummary{RunID: stringValue(run, "run_id"), Status: stringValue(run, "status"), CurrentStage: stringValue(run, "current_stage")}
	}
	if task, ok := status["current_task"].(map[string]any); ok {
		s.Run.CurrentTask = stringValue(task, "id")
	}
	s.Blockers = blockersFromAny(status["blockers"])
	s.Tasks = tasksFromPlan(plan)
	s.Events = eventsFromAny(events["events"])
	s.Artifacts = artifactsFromAny(artifacts["artifacts"])
	s.Config = configFromMap(config)
	s.Providers = providersFromAny(providers["providers"])
	return s
}

func tasksFromPlan(plan map[string]any) []TaskSummary {
	var tasks []TaskSummary
	phases, _ := plan["phases"].([]any)
	for _, rawPhase := range phases {
		phase, _ := rawPhase.(map[string]any)
		phaseID := stringValue(phase, "id")
		rawTasks, _ := phase["tasks"].([]any)
		for _, rawTask := range rawTasks {
			task, _ := rawTask.(map[string]any)
			tasks = append(tasks, TaskSummary{ID: stringValue(task, "id"), PhaseID: emptyAs(stringValue(task, "phase_id"), phaseID), Title: stringValue(task, "title"), RiskLevel: stringValue(task, "risk_level"), Description: stringValue(task, "description")})
		}
	}
	return tasks
}

func blockersFromAny(value any) []BlockerSummary {
	items, _ := value.([]any)
	out := make([]BlockerSummary, 0, len(items))
	for _, raw := range items {
		m, _ := raw.(map[string]any)
		out = append(out, BlockerSummary{ID: stringValue(m, "ID", "id"), TaskID: stringValue(m, "TaskID", "task_id"), Reason: stringValue(m, "Reason", "reason"), Description: stringValue(m, "Description", "description"), Status: stringValue(m, "Status", "status")})
	}
	return out
}

func eventsFromAny(value any) []contract.EventEnvelope {
	data, _ := json.Marshal(value)
	var events []contract.EventEnvelope
	_ = json.Unmarshal(data, &events)
	return events
}

func artifactsFromAny(value any) []ArtifactSummary {
	items, _ := value.([]any)
	out := make([]ArtifactSummary, 0, len(items))
	for _, raw := range items {
		m, _ := raw.(map[string]any)
		out = append(out, ArtifactSummary{ID: stringValue(m, "ID", "id"), Kind: stringValue(m, "Kind", "kind"), Path: stringValue(m, "Path", "path"), Version: intValue(m, "Version", "version")})
	}
	return out
}

func configFromMap(m map[string]any) ConfigSummary {
	cp := ConfigSummary{Profile: stringValue(m, "profile"), Redacted: boolValue(m, "redacted")}
	if control, ok := m["controlplane"].(map[string]any); ok {
		cp.Bind = stringValue(control, "bind")
		cp.AuthRequired = boolValue(control, "auth_required")
	}
	return cp
}

func providersFromAny(value any) []ProviderSummary {
	items, _ := value.([]any)
	out := make([]ProviderSummary, 0, len(items))
	for _, raw := range items {
		m, _ := raw.(map[string]any)
		out = append(out, ProviderSummary{Name: stringValue(m, "name", "Name"), Status: stringValue(m, "status", "Status")})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func stringValue(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := m[key]; ok && value != nil {
			switch typed := value.(type) {
			case string:
				return typed
			case fmt.Stringer:
				return typed.String()
			default:
				return fmt.Sprint(typed)
			}
		}
	}
	return ""
}

func boolValue(m map[string]any, keys ...string) bool {
	for _, key := range keys {
		if value, ok := m[key].(bool); ok {
			return value
		}
	}
	return false
}

func intValue(m map[string]any, keys ...string) int {
	for _, key := range keys {
		switch value := m[key].(type) {
		case int:
			return value
		case float64:
			return int(value)
		}
	}
	return 0
}

func URLWithQuery(basePath string, values url.Values) string {
	if len(values) == 0 {
		return basePath
	}
	return basePath + "?" + values.Encode()
}
