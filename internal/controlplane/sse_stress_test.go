package controlplane

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
)

func TestSSESlowReaderStressDropsSlowClientWithoutAffectingFastReaders(t *testing.T) {
	const (
		projectID   = "proj_sse_stress"
		runID       = "run_sse_stress"
		fastReaders = 3
		eventCount  = 96
		queueLimit  = 4
	)

	goroutinesBefore := runtime.NumGoroutine()
	store := newControlPlaneTestStore(t)
	seedProject(t, store, projectID)
	seedRun(t, store, projectID, runID)
	server, err := NewServer(ServerConfig{Bind: "127.0.0.1", ProjectID: projectID, HeartbeatInterval: time.Hour, ClientQueueMaxEvents: queueLimit, RetryMS: 111}, store)
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server.Handler())

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	fastResults := make(chan error, fastReaders)
	fastCancels := make([]context.CancelFunc, 0, fastReaders)
	releaseFastReaders := make(chan struct{})
	for i := 0; i < fastReaders; i++ {
		resp, cancelFast := openSSEStream(t, ctx, httpServer.URL, runID)
		fastCancels = append(fastCancels, cancelFast)
		wg.Add(1)
		go func(readerID int, body io.ReadCloser) {
			defer wg.Done()
			defer body.Close()
			err := readStressEvents(ctx, body, eventCount)
			fastResults <- err
			if err == nil {
				select {
				case <-releaseFastReaders:
				case <-ctx.Done():
				}
			}
		}(i, resp.Body)
	}

	slowResp, slowCancel := openSSEStream(t, ctx, httpServer.URL, runID)
	defer slowCancel()
	defer slowResp.Body.Close()

	waitForSubscribers(t, ctx, server.Publisher(), fastReaders+1)

	payload := json.RawMessage(fmt.Sprintf(`{"blob":%q}`, strings.Repeat("x", 64*1024)))
	for i := 1; i <= eventCount; i++ {
		_, err := server.Publisher().Publish(ctx, contract.EventEnvelope{
			EventID: fmt.Sprintf("evt_stress_%03d", i),
			RunID:   runID,
			Type:    contract.EventTypeTaskProgress,
			Source:  contract.EventSourceCore,
			Payload: payload,
		})
		if err != nil {
			t.Fatalf("publish %d failed: %v", i, err)
		}
	}

	waitForSubscribers(t, ctx, server.Publisher(), fastReaders)

	for i := 0; i < fastReaders; i++ {
		select {
		case err := <-fastResults:
			if err != nil {
				t.Fatalf("fast reader %d failed: %v", i, err)
			}
		case <-ctx.Done():
			t.Fatalf("timed out waiting for fast reader %d: %v", i, ctx.Err())
		}
	}

	if _, err := server.Publisher().Publish(ctx, contract.EventEnvelope{EventID: "evt_stress_after_drop", RunID: runID, Type: contract.EventTypeTaskProgress, Source: contract.EventSourceCore, Payload: json.RawMessage(`{"after_drop":true}`)}); err != nil {
		t.Fatalf("publish after slow-reader drop failed: %v", err)
	}

	// The artificial slow reader intentionally does not drain during the burst.
	// After the publisher drops it, delayed reads should observe connection end or cancellation promptly.
	time.Sleep(500 * time.Millisecond)
	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, slowResp.Body)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil && !strings.Contains(err.Error(), "closed") && !strings.Contains(err.Error(), "EOF") {
			t.Fatalf("slow reader ended with unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("slow reader connection did not terminate after queue overflow")
	}

	for _, cancelFast := range fastCancels {
		cancelFast()
	}
	close(releaseFastReaders)
	wg.Wait()
	httpServer.Close()

	waitForGoroutineDelta(t, goroutinesBefore, 12)
}

func openSSEStream(t *testing.T, parent context.Context, baseURL, runID string) (*http.Response, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(parent)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/runs/"+runID+"/stream", nil)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		cancel()
		t.Fatalf("stream status = %d body=%s", resp.StatusCode, body)
	}
	return resp, cancel
}

func readStressEvents(ctx context.Context, body io.Reader, want int) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 128*1024), 2*1024*1024)
	seen := 0
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := scanner.Text()
		if line == "event: "+contract.EventTypeTaskProgress {
			seen++
			if seen == want {
				return nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return fmt.Errorf("stream ended after %d task_progress events, want %d", seen, want)
}

func waitForSubscribers(t *testing.T, ctx context.Context, publisher *Publisher, want int) {
	t.Helper()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if got := publisherSubscriberCount(publisher); got == want {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %d subscribers: %v", want, ctx.Err())
		case <-deadline.C:
			t.Fatalf("timed out waiting for %d subscribers, got %d", want, publisherSubscriberCount(publisher))
		case <-ticker.C:
		}
	}
}

func publisherSubscriberCount(publisher *Publisher) int {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	return len(publisher.clients)
}

func waitForGoroutineDelta(t *testing.T, before, maxDelta int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		if delta := runtime.NumGoroutine() - before; delta <= maxDelta {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	if delta := runtime.NumGoroutine() - before; delta > maxDelta {
		t.Fatalf("goroutine delta = %d, want <= %d", delta, maxDelta)
	}
}
