package controlplane

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/state"
)

type EventStore interface {
	PersistEvent(ctx context.Context, event contract.EventEnvelope) (contract.EventEnvelope, error)
	ListEvents(ctx context.Context, opts state.EventListOptions) ([]contract.EventEnvelope, error)
	EventSequenceForID(ctx context.Context, runID, eventID string) (int64, error)
}

type Publisher struct {
	store      EventStore
	queueLimit int
	mu         sync.Mutex
	nextID     int
	clients    map[int]chan contract.EventEnvelope
}

func NewPublisher(store EventStore, queueLimit int) *Publisher {
	if queueLimit <= 0 {
		queueLimit = 1000
	}
	return &Publisher{store: store, queueLimit: queueLimit, clients: map[int]chan contract.EventEnvelope{}}
}

func (p *Publisher) Publish(ctx context.Context, event contract.EventEnvelope) (contract.EventEnvelope, error) {
	stored, err := p.store.PersistEvent(ctx, event)
	if err != nil {
		return contract.EventEnvelope{}, err
	}
	p.broadcast(stored)
	return stored, nil
}

func (p *Publisher) Subscribe() (int, <-chan contract.EventEnvelope, func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nextID++
	id := p.nextID
	ch := make(chan contract.EventEnvelope, p.queueLimit)
	p.clients[id] = ch
	return id, ch, func() { p.unsubscribe(id) }
}

func (p *Publisher) broadcast(event contract.EventEnvelope) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, ch := range p.clients {
		select {
		case ch <- event:
		default:
			close(ch)
			delete(p.clients, id)
		}
	}
}

func (p *Publisher) unsubscribe(id int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ch, ok := p.clients[id]; ok {
		close(ch)
		delete(p.clients, id)
	}
}

type FollowOptions struct {
	RunID       string
	LastEventID string
	JSON        bool
	Token       string
	Client      *http.Client
}

func FollowPublisher(ctx context.Context, store EventStore, publisher *Publisher, out io.Writer, opts FollowOptions) error {
	if store == nil {
		return fmt.Errorf("event store is required")
	}
	if publisher == nil {
		return fmt.Errorf("event publisher is required")
	}
	if opts.RunID == "" {
		return fmt.Errorf("run id is required")
	}
	replay, err := store.ListEvents(ctx, state.EventListOptions{RunID: opts.RunID, AfterEventID: opts.LastEventID})
	if err != nil {
		return err
	}
	lastSequence := int64(0)
	for _, event := range replay {
		if err := WriteFollowEvent(out, event, opts.JSON); err != nil {
			return err
		}
		lastSequence = event.Sequence
		if IsTerminalEvent(event) {
			return nil
		}
	}
	_, ch, unsubscribe := publisher.Subscribe()
	defer unsubscribe()
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			if event.RunID != opts.RunID || event.Sequence <= lastSequence {
				continue
			}
			if err := WriteFollowEvent(out, event, opts.JSON); err != nil {
				return err
			}
			lastSequence = event.Sequence
			if IsTerminalEvent(event) {
				return nil
			}
		}
	}
}

func FollowSSE(ctx context.Context, baseURL string, out io.Writer, opts FollowOptions) error {
	runID := opts.RunID
	client := opts.Client
	if client == nil {
		client = http.DefaultClient
	}
	if runID == "" {
		var err error
		runID, err = LookupActiveRunID(ctx, client, baseURL, opts.Token)
		if err != nil {
			return err
		}
	}
	lastID := opts.LastEventID
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		err := readSSEOnce(ctx, client, baseURL, runID, lastID, opts.Token, out, opts.JSON, &lastID)
		if err == nil || errors.Is(err, errTerminalEvent) || ctx.Err() != nil {
			return nil
		}
	}
}

func LookupActiveRunID(ctx context.Context, client *http.Client, baseURL, token string) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/status", nil)
	if err != nil {
		return "", err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status lookup failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var decoded struct {
		ActiveRun map[string]any `json:"active_run"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", err
	}
	if decoded.ActiveRun == nil {
		return "", fmt.Errorf("status response has no active run")
	}
	if runID, ok := decoded.ActiveRun["run_id"].(string); ok && runID != "" {
		return runID, nil
	}
	return "", fmt.Errorf("status response has no run_id")
}

var errTerminalEvent = errors.New("terminal event")

func readSSEOnce(ctx context.Context, client *http.Client, baseURL, runID, lastID, token string, out io.Writer, jsonOutput bool, lastIDOut *string) error {
	streamURL := strings.TrimRight(baseURL, "/") + "/runs/" + url.PathEscape(runID) + "/stream"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("event stream failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return readSSEFrames(ctx, resp.Body, out, jsonOutput, lastIDOut)
}

func readSSEFrames(ctx context.Context, in io.Reader, out io.Writer, jsonOutput bool, lastID *string) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var id string
	var data strings.Builder
	flush := func() error {
		if data.Len() == 0 {
			id = ""
			return nil
		}
		var event contract.EventEnvelope
		if err := json.Unmarshal([]byte(data.String()), &event); err != nil {
			return err
		}
		if event.EventID == "" {
			event.EventID = id
		}
		if event.EventID != "" && lastID != nil {
			*lastID = event.EventID
		}
		data.Reset()
		id = ""
		if err := WriteFollowEvent(out, event, jsonOutput); err != nil {
			return err
		}
		if IsTerminalEvent(event) {
			return errTerminalEvent
		}
		return nil
	}
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimPrefix(value, " ")
		switch name {
		case "id":
			id = value
		case "data":
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(value)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}

func WriteFollowEvent(out io.Writer, event contract.EventEnvelope, jsonOutput bool) error {
	if jsonOutput {
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "%s\n", data)
		return err
	}
	parts := []string{fmt.Sprintf("#%d", event.Sequence), event.Type}
	if event.Stage != "" {
		parts = append(parts, "stage="+event.Stage)
	}
	if event.TaskID != "" {
		parts = append(parts, "task="+event.TaskID)
	}
	if event.EventID != "" {
		parts = append(parts, "id="+event.EventID)
	}
	_, err := fmt.Fprintln(out, strings.Join(parts, " "))
	return err
}

func IsTerminalEvent(event contract.EventEnvelope) bool {
	if event.Type == contract.EventTypeDone {
		return true
	}
	if event.Type != contract.EventTypeRunStatus || len(event.Payload) == 0 {
		return false
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return false
	}
	switch payload.Status {
	case "completed", "cancelled", "failed", "blocked":
		return true
	default:
		return false
	}
}
