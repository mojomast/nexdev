package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/controlplane"
	"github.com/mojomast/nexdev/internal/state"
)

func TestEventsFollowInProcessPublisherEmitsInOrder(t *testing.T) {
	store := newEventsFollowStore(t, "proj_follow", "run_follow")
	pub := controlplane.NewPublisher(store, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- controlplane.FollowPublisher(ctx, store, pub, &out, controlplane.FollowOptions{RunID: "run_follow"})
	}()
	time.Sleep(25 * time.Millisecond)
	for i, typ := range []string{contract.EventTypeTaskStarted, contract.EventTypeTaskProgress, contract.EventTypeDone} {
		_, err := pub.Publish(ctx, contract.EventEnvelope{EventID: fmt.Sprintf("evt_order_%d", i+1), RunID: "run_follow", Type: typ, Source: contract.EventSourceCore, Payload: json.RawMessage(`{}`)})
		if err != nil {
			t.Fatalf("publish %d failed: %v", i+1, err)
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, "#1 task_started") || !strings.Contains(got, "#2 task_progress") || !strings.Contains(got, "#3 done") {
		t.Fatalf("unexpected follow output:\n%s", got)
	}
	if strings.Index(got, "#1 task_started") > strings.Index(got, "#2 task_progress") || strings.Index(got, "#2 task_progress") > strings.Index(got, "#3 done") {
		t.Fatalf("events out of order:\n%s", got)
	}
}

func TestEventsFollowHTTPReconnectsWithLastEventID(t *testing.T) {
	var requests int
	seenLastID := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"active_run":{"run_id":"run_http"}}`))
		case "/runs/run_http/stream":
			requests++
			w.Header().Set("Content-Type", "text/event-stream")
			if requests == 1 {
				writeTestSSE(t, w, contract.EventEnvelope{EventID: "evt_http_1", Sequence: 1, RunID: "run_http", Type: contract.EventTypeTaskProgress, Source: contract.EventSourceCore, Payload: json.RawMessage(`{}`)})
				return
			}
			seenLastID <- r.Header.Get("Last-Event-ID")
			writeTestSSE(t, w, contract.EventEnvelope{EventID: "evt_http_2", Sequence: 2, RunID: "run_http", Type: contract.EventTypeDone, Source: contract.EventSourceCore, Payload: json.RawMessage(`{}`)})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var out bytes.Buffer
	if err := controlplane.FollowSSE(ctx, server.URL, &out, controlplane.FollowOptions{}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-seenLastID:
		if got != "evt_http_1" {
			t.Fatalf("Last-Event-ID = %q, want evt_http_1", got)
		}
	default:
		t.Fatal("second stream request did not observe Last-Event-ID")
	}
	if !strings.Contains(out.String(), "evt_http_1") || !strings.Contains(out.String(), "evt_http_2") {
		t.Fatalf("missing streamed events:\n%s", out.String())
	}
}

func TestEventsFollowContextCancellationUnblocks(t *testing.T) {
	store := newEventsFollowStore(t, "proj_cancel", "run_cancel")
	pub := controlplane.NewPublisher(store, 10)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- controlplane.FollowPublisher(ctx, store, pub, &bytes.Buffer{}, controlplane.FollowOptions{RunID: "run_cancel"})
	}()
	time.Sleep(25 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("follow did not exit after cancellation")
	}
}

func TestEventsFollowJSONOutputsEnvelopeLines(t *testing.T) {
	store := newEventsFollowStore(t, "proj_json", "run_json")
	pub := controlplane.NewPublisher(store, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- controlplane.FollowPublisher(ctx, store, pub, &out, controlplane.FollowOptions{RunID: "run_json", JSON: true})
	}()
	time.Sleep(25 * time.Millisecond)
	if _, err := pub.Publish(ctx, contract.EventEnvelope{EventID: "evt_json_done", RunID: "run_json", Type: contract.EventTypeDone, Source: contract.EventSourceCore, Payload: json.RawMessage(`{"ok":true}`)}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("lines = %d output=%q", len(lines), out.String())
	}
	var event contract.EventEnvelope
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("invalid event JSON line: %v\n%s", err, lines[0])
	}
	if event.EventID != "evt_json_done" || event.Type != contract.EventTypeDone || event.Sequence != 1 {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func newEventsFollowStore(t *testing.T, projectID, runID string) *state.Store {
	t.Helper()
	store, err := state.NewStore(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.CreateProject(&state.Project{ID: projectID, Name: projectID}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRun(context.Background(), &state.Run{ID: runID, ProjectID: projectID, Status: "running", CurrentStage: "develop"}); err != nil {
		t.Fatal(err)
	}
	return store
}

func writeTestSSE(t *testing.T, w http.ResponseWriter, event contract.EventEnvelope) {
	t.Helper()
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fmt.Fprintf(w, "id: %s\nevent: %s\nretry: 1\ndata: %s\n\n", event.EventID, event.Type, data)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}
