package controlplane

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/state"
)

func TestServerLoopbackNoAuthAllowsHealthAndStatus(t *testing.T) {
	store := newControlPlaneTestStore(t)
	seedProject(t, store, "proj_test")
	server, err := NewServer(ServerConfig{Bind: "127.0.0.1", ProjectID: "proj_test"}, store)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/health status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/status", nil)
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/status status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServerRejectsNonLoopbackWithoutAuth(t *testing.T) {
	store := newControlPlaneTestStore(t)
	_, err := NewServer(ServerConfig{Bind: "0.0.0.0", AuthRequired: false}, store)
	if err == nil {
		t.Fatal("expected remote bind without auth to fail")
	}
}

func TestAuthenticatedRouteBehavior(t *testing.T) {
	store := newControlPlaneTestStore(t)
	seedProject(t, store, "proj_auth")
	seedRun(t, store, "proj_auth", "run_auth")
	secret := []byte("server-secret")
	observer := seedToken(t, store, secret, "observer-token", RoleObserver)
	operator := seedToken(t, store, secret, "operator-token", RoleOperator)
	server, err := NewServer(ServerConfig{Bind: "127.0.0.1", AuthRequired: true, ServerSecret: secret, ProjectID: "proj_auth"}, store, WithDetourRequester(&fakeDetourRequester{}))
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /status = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/status", nil)
	req.Header.Set("Authorization", "Bearer "+observer)
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("observer /status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/detour", strings.NewReader(`{"project_id":"proj_auth","run_id":"run_auth","trigger_task_id":"T1.01","reason":"blocked","context":"ctx","source":"operator_manual"}`))
	req.Header.Set("Authorization", "Bearer "+observer)
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("observer /detour = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/detour", strings.NewReader(`{"project_id":"proj_auth","run_id":"run_auth","trigger_task_id":"T1.01","reason":"blocked","context":"ctx","source":"operator_manual"}`))
	req.Header.Set("Authorization", "Bearer "+operator)
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("operator /detour = %d body=%s", rec.Code, rec.Body.String())
	}
	audits, err := store.ListAuditRecords(context.Background(), state.AuditListOptions{ProjectID: "proj_auth"})
	if err != nil {
		t.Fatalf("ListAuditRecords failed: %v", err)
	}
	if len(audits) < 3 {
		t.Fatalf("audit records = %d, want at least auth failed, forbidden, and allowed", len(audits))
	}
	var sawFailed, sawForbidden, sawAllowed bool
	for _, audit := range audits {
		sawFailed = sawFailed || audit.Action == "auth" && audit.Outcome == "failed"
		sawForbidden = sawForbidden || audit.Action == "authorize" && audit.Outcome == "forbidden"
		sawAllowed = sawAllowed || audit.Action == "control_request" && audit.Outcome == "allowed"
	}
	if !sawFailed || !sawForbidden || !sawAllowed {
		t.Fatalf("missing expected audit outcomes: %#v", audits)
	}
}

func TestSSEReplayUsesPersistedEventsAndLastEventID(t *testing.T) {
	store := newControlPlaneTestStore(t)
	seedProject(t, store, "proj_sse")
	seedRun(t, store, "proj_sse", "run_sse")
	_, err := store.PersistEvent(context.Background(), contract.EventEnvelope{EventID: "evt_1", RunID: "run_sse", Type: contract.EventTypeRunStarted, Source: contract.EventSourceCore, Payload: json.RawMessage(`{"n":1}`)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.PersistEvent(context.Background(), contract.EventEnvelope{EventID: "evt_2", RunID: "run_sse", Type: contract.EventTypeTaskProgress, Source: contract.EventSourceExecutor, Payload: json.RawMessage(`{"n":2}`)})
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(ServerConfig{Bind: "127.0.0.1", ProjectID: "proj_sse", HeartbeatInterval: time.Hour, RetryMS: 123}, store)
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, httpServer.URL+"/runs/run_sse/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Last-Event-ID", "evt_1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stream status = %d", resp.StatusCode)
	}
	frame := readSSEFrame(t, resp.Body)
	if !strings.Contains(frame, "id: evt_2") || !strings.Contains(frame, "event: task_progress") || strings.Contains(frame, "[DONE]") {
		t.Fatalf("unexpected frame:\n%s", frame)
	}
}

func TestDetourRouteCallsRequesterAndSurfacesPersistedResult(t *testing.T) {
	store := newControlPlaneTestStore(t)
	seedProject(t, store, "proj_detour")
	seedRun(t, store, "proj_detour", "run_detour")
	fake := &fakeDetourRequester{store: store}
	server, err := NewServer(ServerConfig{Bind: "127.0.0.1", ProjectID: "proj_detour"}, store, WithDetourRequester(fake))
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/detour", strings.NewReader(`{"project_id":"proj_detour","run_id":"run_detour","trigger_task_id":"T1.01","reason":"blocked","context":"ctx","source":"operator_manual"}`))
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("/detour status = %d body=%s", rec.Code, rec.Body.String())
	}
	if fake.called != 1 {
		t.Fatalf("detour requester called %d times, want 1", fake.called)
	}
	events, err := store.ListEvents(context.Background(), state.EventListOptions{RunID: "run_detour"})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != contract.EventTypeDetourCreated {
		t.Fatalf("persisted events = %#v", events)
	}
}

type fakeDetourRequester struct {
	store  *state.Store
	called int
}

func (f *fakeDetourRequester) Request(ctx context.Context, req contract.DetourRequest) (contract.DetourResult, error) {
	f.called++
	result := contract.DetourResult{ID: "detour_1", SplicedAfter: req.TriggerTaskID, Depth: 1, NewTasks: []contract.TaskSpec{{ID: "D1.01", PhaseID: "phase_001", Title: "Fix blocker", Description: "Fix", AcceptanceCriteria: []string{"done"}}}, IDConflicts: []string{}}
	if f.store != nil {
		payload, _ := json.Marshal(result)
		if _, err := f.store.PersistEvent(ctx, contract.EventEnvelope{EventID: "evt_detour_1", RunID: req.RunID, Type: contract.EventTypeDetourCreated, Stage: "detour", TaskID: req.TriggerTaskID, Source: contract.EventSourceCore, Payload: payload}); err != nil {
			return contract.DetourResult{}, err
		}
	}
	return result, nil
}

func (f *fakeDetourRequester) RequestForBlocker(context.Context, string, string) (contract.DetourResult, error) {
	return contract.DetourResult{}, nil
}

func newControlPlaneTestStore(t *testing.T) *state.Store {
	t.Helper()
	store, err := state.NewStore(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func seedProject(t *testing.T, store *state.Store, projectID string) {
	t.Helper()
	if err := store.CreateProject(&state.Project{ID: projectID, Name: projectID, CreatedAt: time.Now().UTC(), CurrentStage: state.StageInit}); err != nil {
		t.Fatal(err)
	}
}

func seedRun(t *testing.T, store *state.Store, projectID, runID string) {
	t.Helper()
	if err := store.CreateRun(context.Background(), &state.Run{ID: runID, ProjectID: projectID, Status: "running", CurrentStage: "develop", StartedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
}

func seedToken(t *testing.T, store *state.Store, secret []byte, value string, role Role) string {
	t.Helper()
	if err := store.CreateAuthToken(context.Background(), &state.AuthToken{ID: "tok_" + string(role), TokenHash: HashBearerToken(secret, value), Role: string(role), CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	return value
}

func readSSEFrame(t *testing.T, body interface{ Read([]byte) (int, error) }) string {
	t.Helper()
	scanner := bufio.NewScanner(body)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" && len(lines) > 0 {
			return strings.Join(lines, "\n")
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	t.Fatalf("no SSE frame read")
	return ""
}
