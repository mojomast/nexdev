package testutil

import (
	"sort"
	"sync"
	"testing"

	"github.com/mojomast/nexdev/internal/contract"
)

type EventRecorder struct {
	mu     sync.Mutex
	events []contract.EventEnvelope
}

func NewEventRecorder() *EventRecorder {
	return &EventRecorder{}
}

func (r *EventRecorder) Record(event contract.EventEnvelope) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *EventRecorder) Events() []contract.EventEnvelope {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]contract.EventEnvelope(nil), r.events...)
}

func (r *EventRecorder) EventsBySequence() []contract.EventEnvelope {
	events := r.Events()
	sort.Slice(events, func(i, j int) bool {
		return events[i].Sequence < events[j].Sequence
	})
	return events
}

func (r *EventRecorder) AssertMonotonicSequences(t testing.TB) {
	t.Helper()
	events := r.Events()
	for i := 1; i < len(events); i++ {
		if events[i].RunID != events[i-1].RunID {
			continue
		}
		if events[i].Sequence <= events[i-1].Sequence {
			t.Fatalf("event sequence at index %d = %d after %d", i, events[i].Sequence, events[i-1].Sequence)
		}
	}
}

func (r *EventRecorder) AssertTypesBySequence(t testing.TB, want ...string) {
	t.Helper()
	events := r.EventsBySequence()
	if len(events) != len(want) {
		t.Fatalf("event count = %d, want %d", len(events), len(want))
	}
	for i := range want {
		if events[i].Type != want[i] {
			t.Fatalf("event type at sequence index %d = %q, want %q", i, events[i].Type, want[i])
		}
	}
}
