package controlplane

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/state"
)

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("run_id")
	if runID == "" {
		writeError(w, r, http.StatusBadRequest, "run_required", "run_id is required", nil)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, r, http.StatusInternalServerError, "stream_unsupported", "streaming is unsupported", nil)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	lastID := r.Header.Get("Last-Event-ID")
	replay, err := s.store.ListEvents(r.Context(), state.EventListOptions{RunID: runID, AfterEventID: lastID, Limit: s.cfg.ReplayMaxEvents})
	if err != nil {
		if errors.Is(err, state.ErrEventNotFound) {
			writeError(w, r, http.StatusNotFound, "event_not_found", "Last-Event-ID was not found for this run", nil)
			return
		}
		writeError(w, r, http.StatusInternalServerError, "state_error", "failed to replay events", nil)
		return
	}
	var lastSequence int64
	for _, event := range replay {
		if err := writeSSEFrame(w, event, s.cfg.RetryMS); err != nil {
			return
		}
		lastSequence = event.Sequence
	}
	flusher.Flush()

	_, ch, unsubscribe := s.publisher.Subscribe()
	defer unsubscribe()
	heartbeat := time.NewTicker(s.cfg.HeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			if event.RunID != runID || event.Sequence <= lastSequence {
				continue
			}
			if err := writeSSEFrame(w, event, s.cfg.RetryMS); err != nil {
				return
			}
			lastSequence = event.Sequence
			flusher.Flush()
		case <-heartbeat.C:
			// Missed events from a dropped subscription are recovered on client
			// reconnect through Last-Event-ID replay; live delivery uses publisher only.
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeSSEFrame(w http.ResponseWriter, event contract.EventEnvelope, retryMS int) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "id: %s\nevent: %s\nretry: %d\ndata: %s\n\n", event.EventID, event.Type, retryMS, data)
	return err
}
