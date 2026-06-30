package state

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
)

func TestStorePersistEventStoresAndLoadsEnvelope(t *testing.T) {
	store := newEventTestStore(t)
	seedEventRun(t, store, "proj_events", "run_events")

	timestamp := time.Date(2026, 6, 30, 12, 34, 56, 123456789, time.FixedZone("offset", -4*60*60))
	stored, err := store.PersistEvent(context.Background(), contract.EventEnvelope{
		EventID:   "evt_one",
		Type:      contract.EventTypeTaskStarted,
		RunID:     "run_events",
		Stage:     "develop",
		TaskID:    "T1.01",
		Timestamp: timestamp,
		Source:    contract.EventSourceExecutor,
		Payload:   []byte(`{"status":"started"}`),
	})
	if err != nil {
		t.Fatalf("PersistEvent failed: %v", err)
	}

	if stored.Sequence != 1 {
		t.Fatalf("sequence = %d, want 1", stored.Sequence)
	}
	if stored.ProjectID != "proj_events" {
		t.Fatalf("project id = %q, want proj_events", stored.ProjectID)
	}
	if stored.ContractVersion != contract.EventContractVersion {
		t.Fatalf("contract version = %q", stored.ContractVersion)
	}
	if stored.Timestamp.Location() != time.UTC {
		t.Fatalf("timestamp location = %v, want UTC", stored.Timestamp.Location())
	}
	if got, want := stored.Timestamp.Format(time.RFC3339Nano), timestamp.UTC().Format(time.RFC3339Nano); got != want {
		t.Fatalf("timestamp = %s, want %s", got, want)
	}
	if string(stored.Payload) != `{"status":"started"}` {
		t.Fatalf("payload = %s", stored.Payload)
	}

	loaded, err := store.GetEvent(context.Background(), "evt_one")
	if err != nil {
		t.Fatalf("GetEvent failed: %v", err)
	}
	if loaded.EventID != stored.EventID || loaded.Sequence != stored.Sequence || loaded.Stage != stored.Stage || loaded.TaskID != stored.TaskID {
		t.Fatalf("loaded event mismatch: got %+v want %+v", loaded, stored)
	}
}

func TestStorePersistEventAllocatesMonotonicSequencePerRun(t *testing.T) {
	store := newEventTestStore(t)
	seedEventRun(t, store, "proj_events", "run_one")

	for i := 1; i <= 3; i++ {
		stored, err := store.PersistEvent(context.Background(), testEvent(fmt.Sprintf("evt_%d", i), "run_one"))
		if err != nil {
			t.Fatalf("PersistEvent %d failed: %v", i, err)
		}
		if stored.Sequence != int64(i) {
			t.Fatalf("sequence %d = %d, want %d", i, stored.Sequence, i)
		}
	}
}

func TestStorePersistEventSequencesAreIndependentAcrossRuns(t *testing.T) {
	store := newEventTestStore(t)
	seedEventRun(t, store, "proj_events", "run_one")
	seedEventRun(t, store, "proj_events", "run_two")

	first, err := store.PersistEvent(context.Background(), testEvent("evt_run_one", "run_one"))
	if err != nil {
		t.Fatalf("PersistEvent run_one failed: %v", err)
	}
	second, err := store.PersistEvent(context.Background(), testEvent("evt_run_two", "run_two"))
	if err != nil {
		t.Fatalf("PersistEvent run_two failed: %v", err)
	}
	if first.Sequence != 1 || second.Sequence != 1 {
		t.Fatalf("sequences = %d and %d, want both 1", first.Sequence, second.Sequence)
	}
}

func TestStoreListEventsReplaysAfterSequenceAndEventID(t *testing.T) {
	store := newEventTestStore(t)
	seedEventRun(t, store, "proj_events", "run_replay")

	for i := 1; i <= 5; i++ {
		if _, err := store.PersistEvent(context.Background(), testEvent(fmt.Sprintf("evt_replay_%d", i), "run_replay")); err != nil {
			t.Fatalf("PersistEvent %d failed: %v", i, err)
		}
	}

	afterSequence, err := store.ListEvents(context.Background(), EventListOptions{RunID: "run_replay", AfterSequence: 2})
	if err != nil {
		t.Fatalf("ListEvents after sequence failed: %v", err)
	}
	assertEventIDs(t, afterSequence, []string{"evt_replay_3", "evt_replay_4", "evt_replay_5"})

	afterEventID, err := store.ListEvents(context.Background(), EventListOptions{RunID: "run_replay", AfterEventID: "evt_replay_3", Limit: 1})
	if err != nil {
		t.Fatalf("ListEvents after event id failed: %v", err)
	}
	assertEventIDs(t, afterEventID, []string{"evt_replay_4"})

	sequence, err := store.EventSequenceForID(context.Background(), "run_replay", "evt_replay_3")
	if err != nil {
		t.Fatalf("EventSequenceForID failed: %v", err)
	}
	if sequence != 3 {
		t.Fatalf("sequence = %d, want 3", sequence)
	}
}

func TestStorePersistEventRejectsUnsafeCallerSequence(t *testing.T) {
	store := newEventTestStore(t)
	seedEventRun(t, store, "proj_events", "run_conflict")

	first := testEvent("evt_first", "run_conflict")
	first.Sequence = 1
	if _, err := store.PersistEvent(context.Background(), first); err != nil {
		t.Fatalf("PersistEvent first failed: %v", err)
	}

	duplicate := testEvent("evt_duplicate", "run_conflict")
	duplicate.Sequence = 1
	if _, err := store.PersistEvent(context.Background(), duplicate); !errors.Is(err, ErrEventSequenceConflict) {
		t.Fatalf("duplicate sequence error = %v, want ErrEventSequenceConflict", err)
	}

	gap := testEvent("evt_gap", "run_conflict")
	gap.Sequence = 3
	if _, err := store.PersistEvent(context.Background(), gap); !errors.Is(err, ErrEventSequenceConflict) {
		t.Fatalf("gap sequence error = %v, want ErrEventSequenceConflict", err)
	}

	next := testEvent("evt_next", "run_conflict")
	next.Sequence = 2
	stored, err := store.PersistEvent(context.Background(), next)
	if err != nil {
		t.Fatalf("PersistEvent next failed: %v", err)
	}
	if stored.Sequence != 2 {
		t.Fatalf("next sequence = %d, want 2", stored.Sequence)
	}
}

func TestStorePersistEventRejectsDuplicateEventID(t *testing.T) {
	store := newEventTestStore(t)
	seedEventRun(t, store, "proj_events", "run_duplicate_id")

	if _, err := store.PersistEvent(context.Background(), testEvent("evt_duplicate", "run_duplicate_id")); err != nil {
		t.Fatalf("PersistEvent first failed: %v", err)
	}
	if _, err := store.PersistEvent(context.Background(), testEvent("evt_duplicate", "run_duplicate_id")); err == nil {
		t.Fatal("expected duplicate event id to fail")
	}
}

func TestStorePersistEventConcurrentPublishers(t *testing.T) {
	store := newEventTestStore(t)
	seedEventRun(t, store, "proj_events", "run_concurrent")

	const publishers = 25
	var wg sync.WaitGroup
	errs := make(chan error, publishers)
	for i := 0; i < publishers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.PersistEvent(context.Background(), testEvent(fmt.Sprintf("evt_concurrent_%02d", i), "run_concurrent"))
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent PersistEvent failed: %v", err)
		}
	}

	events, err := store.ListEvents(context.Background(), EventListOptions{RunID: "run_concurrent"})
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	if len(events) != publishers {
		t.Fatalf("event count = %d, want %d", len(events), publishers)
	}
	for i, event := range events {
		want := int64(i + 1)
		if event.Sequence != want {
			t.Fatalf("event %d sequence = %d, want %d", i, event.Sequence, want)
		}
	}
}

func newEventTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(t.TempDir() + "/events.db")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	})
	return store
}

func seedEventRun(t *testing.T, store *Store, projectID, runID string) {
	t.Helper()
	_, err := store.DB().Exec(`
		INSERT OR IGNORE INTO projects (id, name, created_at, current_stage)
		VALUES (?, ?, ?, 'init')
	`, projectID, projectID, time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("seed project failed: %v", err)
	}
	_, err = store.DB().Exec(`
		INSERT INTO runs (id, project_id, status, started_at)
		VALUES (?, ?, 'running', ?)
	`, runID, projectID, time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano))
	if err != nil {
		t.Fatalf("seed run failed: %v", err)
	}
}

func testEvent(eventID, runID string) contract.EventEnvelope {
	return contract.EventEnvelope{
		EventID:   eventID,
		RunID:     runID,
		Type:      contract.EventTypeRunStatus,
		Source:    contract.EventSourceCore,
		Payload:   []byte(`{}`),
		Timestamp: time.Date(2026, 6, 30, 1, 2, 3, 0, time.UTC),
	}
}

func assertEventIDs(t *testing.T, events []contract.EventEnvelope, want []string) {
	t.Helper()
	got := make([]string, 0, len(events))
	for _, event := range events {
		got = append(got, event.EventID)
	}
	if len(got) != len(want) {
		t.Fatalf("event ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event ids = %v, want %v", got, want)
		}
	}
	if !sort.SliceIsSorted(events, func(i, j int) bool { return events[i].Sequence < events[j].Sequence }) {
		t.Fatalf("events are not sorted by sequence: %+v", events)
	}
}
