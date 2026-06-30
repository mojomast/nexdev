package pipeline

import "fmt"

// StageStatus is the canonical status for a stage run.
type StageStatus string

const (
	StageStatusPending   StageStatus = "pending"
	StageStatusRunning   StageStatus = "running"
	StageStatusCompleted StageStatus = "completed"
	StageStatusSkipped   StageStatus = "skipped"
	StageStatusBlocked   StageStatus = "blocked"
	StageStatusFailed    StageStatus = "failed"
	StageStatusCancelled StageStatus = "cancelled"
)

var AllStageStatuses = []StageStatus{
	StageStatusPending,
	StageStatusRunning,
	StageStatusCompleted,
	StageStatusSkipped,
	StageStatusBlocked,
	StageStatusFailed,
	StageStatusCancelled,
}

var allowedStatusTransitions = map[StageStatus]map[StageStatus]struct{}{
	StageStatusPending: {
		StageStatusRunning: {},
		StageStatusSkipped: {},
	},
	StageStatusRunning: {
		StageStatusCompleted: {},
		StageStatusBlocked:   {},
		StageStatusFailed:    {},
		StageStatusCancelled: {},
	},
	StageStatusBlocked: {
		StageStatusRunning: {},
	},
	StageStatusFailed: {
		StageStatusRunning: {},
	},
}

func IsKnownStageStatus(status StageStatus) bool {
	for _, candidate := range AllStageStatuses {
		if candidate == status {
			return true
		}
	}
	return false
}

func CanTransitionStatus(from, to StageStatus) bool {
	if !IsKnownStageStatus(from) || !IsKnownStageStatus(to) {
		return false
	}
	allowed, ok := allowedStatusTransitions[from]
	if !ok {
		return false
	}
	_, ok = allowed[to]
	return ok
}

func ValidateStatusTransition(from, to StageStatus) error {
	if !IsKnownStageStatus(from) {
		return fmt.Errorf("unknown source stage status %q", from)
	}
	if !IsKnownStageStatus(to) {
		return fmt.Errorf("unknown target stage status %q", to)
	}
	if !CanTransitionStatus(from, to) {
		return fmt.Errorf("invalid stage status transition %s -> %s", from, to)
	}
	return nil
}
