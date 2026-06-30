package pipeline

import (
	"reflect"
	"testing"
)

func TestStageStatuses(t *testing.T) {
	want := []StageStatus{
		StageStatusPending,
		StageStatusRunning,
		StageStatusCompleted,
		StageStatusSkipped,
		StageStatusBlocked,
		StageStatusFailed,
		StageStatusCancelled,
	}

	if !reflect.DeepEqual(AllStageStatuses, want) {
		t.Fatalf("stage statuses mismatch\ngot:  %#v\nwant: %#v", AllStageStatuses, want)
	}
}

func TestAllowedStatusTransitions(t *testing.T) {
	allowed := [][2]StageStatus{
		{StageStatusPending, StageStatusRunning},
		{StageStatusPending, StageStatusSkipped},
		{StageStatusRunning, StageStatusCompleted},
		{StageStatusRunning, StageStatusBlocked},
		{StageStatusRunning, StageStatusFailed},
		{StageStatusRunning, StageStatusCancelled},
		{StageStatusBlocked, StageStatusRunning},
		{StageStatusFailed, StageStatusRunning},
	}

	allowedSet := map[[2]StageStatus]bool{}
	for _, transition := range allowed {
		allowedSet[transition] = true
		if err := ValidateStatusTransition(transition[0], transition[1]); err != nil {
			t.Fatalf("expected transition %s -> %s to be valid: %v", transition[0], transition[1], err)
		}
	}

	for _, from := range AllStageStatuses {
		for _, to := range AllStageStatuses {
			if allowedSet[[2]StageStatus{from, to}] {
				continue
			}
			if CanTransitionStatus(from, to) {
				t.Fatalf("unexpected allowed transition %s -> %s", from, to)
			}
		}
	}
}

func TestUnknownStatusTransitionsFail(t *testing.T) {
	if CanTransitionStatus(StageStatus("wat"), StageStatusRunning) {
		t.Fatal("unknown source status should not transition")
	}
	if err := ValidateStatusTransition(StageStatusPending, StageStatus("wat")); err == nil {
		t.Fatal("unknown target status should fail validation")
	}
}
