package blocker

import (
	"fmt"
	"time"

	"github.com/mojomast/nexdev/internal/interview"
	"github.com/mojomast/nexdev/internal/state"
)

// FailureThreshold is the number of failures before marking a task as blocked
const FailureThreshold = 3

// Detector handles blocker detection and resolution
type Detector struct {
	store           *state.Store
	interviewEngine *interview.Engine
	failureTracker  map[string]int // taskID -> failure count
}

// NewDetector creates a new blocker detector
func NewDetector(store *state.Store, interviewEngine *interview.Engine) *Detector {
	return &Detector{
		store:           store,
		interviewEngine: interviewEngine,
		failureTracker:  make(map[string]int),
	}
}

// RecordFailure records a task failure and checks if it should be marked as blocked
func (d *Detector) RecordFailure(taskID, errorMessage string) (bool, error) {
	// Increment failure count
	d.failureTracker[taskID]++

	// Check if threshold reached
	if d.failureTracker[taskID] >= FailureThreshold {
		return true, nil
	}

	return false, nil
}

// MarkAsBlocked marks a task as blocked and creates a blocker record
func (d *Detector) MarkAsBlocked(taskID, phaseID, projectID, reason, context string) (*state.Blocker, error) {
	// Create blocker record
	blocker := &state.Blocker{
		ID:          fmt.Sprintf("blocker-%s-%d", taskID, time.Now().UnixNano()),
		TaskID:      taskID,
		Description: fmt.Sprintf("%s. Context: %s", reason, context),
		CreatedAt:   time.Now(),
	}

	// Save blocker to store
	if err := d.store.SaveBlocker(blocker); err != nil {
		return nil, fmt.Errorf("failed to save blocker: %w", err)
	}

	// Update task status to blocked
	if err := d.store.UpdateTaskStatus(taskID, "blocked"); err != nil {
		return nil, fmt.Errorf("failed to update task status: %w", err)
	}

	return blocker, nil
}

// GatherBlockerInformation uses the store to gather comprehensive information about the blocker
func (d *Detector) GatherBlockerInformation(blocker *state.Blocker) (map[string]string, error) {
	info := map[string]string{
		"blocker_id":    blocker.ID,
		"task_id":       blocker.TaskID,
		"description":   blocker.Description,
		"gathered_at":   time.Now().Format(time.RFC3339),
		"created_at":    blocker.CreatedAt.Format(time.RFC3339),
		"failure_count": fmt.Sprintf("%d", d.failureTracker[blocker.TaskID]),
	}

	// Enrich with task details if available
	task, err := d.store.GetTask(blocker.TaskID)
	if err == nil {
		info["task_description"] = task.Description
		info["task_status"] = string(task.Status)
		info["task_phase_id"] = task.PhaseID

		// Get phase info
		phase, err := d.store.GetPhase(task.PhaseID)
		if err == nil {
			info["phase_title"] = phase.Title
			info["phase_status"] = string(phase.Status)
			info["project_id"] = phase.ProjectID

			// Check for other blockers on the same project
			otherBlockers, err := d.store.ListActiveBlockers(phase.ProjectID)
			if err == nil {
				info["active_blockers_count"] = fmt.Sprintf("%d", len(otherBlockers))
			}
		}
	}

	if blocker.Resolution != "" {
		info["resolution"] = blocker.Resolution
	}
	if blocker.ResolvedAt != nil {
		info["resolved_at"] = blocker.ResolvedAt.Format(time.RFC3339)
	}

	return info, nil
}

// AttemptResolution attempts to resolve a blocker using various strategies
func (d *Detector) AttemptResolution(blocker *state.Blocker) (*ResolutionResult, error) {
	result := &ResolutionResult{
		BlockerID:           blocker.ID,
		Strategies:          []ResolutionStrategy{},
		AttemptedStrategies: []string{},
		Success:             false,
	}

	// Define resolution strategies in priority order
	strategies := []ResolutionStrategy{
		{
			Name:        "Retry with backoff",
			Description: "Reset failure count and allow the task to be retried",
			Automatic:   true,
		},
		{
			Name:        "Simplify task",
			Description: "Break the task into smaller subtasks that may succeed individually",
			Automatic:   false,
		},
		{
			Name:        "Skip and continue",
			Description: "Skip the blocked task and continue with others",
			Automatic:   false,
		},
		{
			Name:        "Request user intervention",
			Description: "Ask the user to manually resolve the issue",
			Automatic:   false,
		},
	}

	result.Strategies = strategies

	// Try the automatic "Retry with backoff" strategy
	retryStrategy := strategies[0]
	result.AttemptedStrategies = append(result.AttemptedStrategies, retryStrategy.Name)

	// Check if this blocker's task has been retried too many times already
	failureCount := d.failureTracker[blocker.TaskID]
	if failureCount <= FailureThreshold {
		// Reset failure count to allow retry
		d.failureTracker[blocker.TaskID] = 0

		// Update task status back to not_started so it can be retried
		if err := d.store.UpdateTaskStatus(blocker.TaskID, state.TaskNotStarted); err != nil {
			// Non-fatal: log the error but continue to try other strategies
			result.Resolution = fmt.Sprintf("Retry strategy failed: could not update task status: %v", err)
		} else {
			// Resolve the blocker
			if err := d.store.ResolveBlocker(blocker.ID, "Automatically resolved: retry with backoff"); err != nil {
				result.Resolution = fmt.Sprintf("Retry strategy failed: could not resolve blocker: %v", err)
			} else {
				result.Success = true
				result.Resolution = "Automatically resolved by resetting failure count and allowing retry"
				return result, nil
			}
		}
	}

	// If retry didn't work (too many failures), suggest manual strategies
	result.Resolution = fmt.Sprintf(
		"Automatic retry not applicable (failure count: %d). Manual intervention recommended: "+
			"either simplify the task, skip it, or resolve the underlying issue manually.",
		failureCount,
	)

	return result, nil
}

// ResolutionResult represents the result of a resolution attempt
type ResolutionResult struct {
	BlockerID           string
	Strategies          []ResolutionStrategy
	AttemptedStrategies []string
	Success             bool
	Resolution          string
}

// ResolutionStrategy represents a strategy for resolving a blocker
type ResolutionStrategy struct {
	Name        string
	Description string
	Automatic   bool
}

// RequestUserIntervention stores an intervention request for a blocker so it can be
// displayed by the UI/TUI layer. The context provides additional detail about what
// the user needs to do.
func (d *Detector) RequestUserIntervention(blocker *state.Blocker, context string) error {
	// Store intervention request as a config entry so the UI can poll for it
	key := fmt.Sprintf("intervention_%s", blocker.ID)
	value := fmt.Sprintf("PENDING|%s|%s|%s",
		blocker.TaskID,
		blocker.Description,
		context,
	)

	if err := d.store.SetConfig(key, value); err != nil {
		return fmt.Errorf("failed to store intervention request: %w", err)
	}

	return nil
}

// ResolveBlocker marks a blocker as resolved
func (d *Detector) ResolveBlocker(blockerID, resolution string) error {
	// Resolve the blocker in the store
	if err := d.store.ResolveBlocker(blockerID, resolution); err != nil {
		return fmt.Errorf("failed to resolve blocker: %w", err)
	}

	// Get all blockers to find the task ID
	// We need to query all blockers (not just active ones) since we just resolved it
	allBlockers, err := d.store.ListActiveBlockers("")
	if err != nil {
		return fmt.Errorf("failed to list blockers: %w", err)
	}

	var taskID string
	for _, b := range allBlockers {
		if b.ID == blockerID {
			taskID = b.TaskID
			break
		}
	}

	// If not found in active blockers, it might have just been resolved
	// In that case, we still need to reset the failure count and update task status
	// For now, we'll just skip if not found
	if taskID == "" {
		// Try to find it in the failure tracker
		for tid := range d.failureTracker {
			// This is a simplified approach - in production we'd need better tracking
			taskID = tid
			break
		}
		if taskID == "" {
			return fmt.Errorf("blocker not found: %s", blockerID)
		}
	}

	// Reset failure count
	delete(d.failureTracker, taskID)

	// Update task status back to pending
	if err := d.store.UpdateTaskStatus(taskID, "pending"); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	return nil
}

// GetFailureCount returns the current failure count for a task
func (d *Detector) GetFailureCount(taskID string) int {
	return d.failureTracker[taskID]
}

// ResetFailureCount resets the failure count for a task
func (d *Detector) ResetFailureCount(taskID string) {
	delete(d.failureTracker, taskID)
}

// ListActiveBlockers lists all active blockers for a project
func (d *Detector) ListActiveBlockers(projectID string) ([]*state.Blocker, error) {
	return d.store.ListActiveBlockers(projectID)
}

// GetBlocker retrieves a specific blocker
func (d *Detector) GetBlocker(blockerID string) (*state.Blocker, error) {
	// Try to get all blockers (pass empty string to get all)
	blockers, err := d.store.ListActiveBlockers("")
	if err != nil {
		return nil, fmt.Errorf("failed to list blockers: %w", err)
	}

	for _, blocker := range blockers {
		if blocker.ID == blockerID {
			return blocker, nil
		}
	}

	// If not found, it might be because we need to query by project
	// In a real implementation, we would have a GetBlockerByID method in the store
	return nil, fmt.Errorf("blocker not found: %s", blockerID)
}

// AnalyzeBlockerPattern analyzes blocker patterns to identify recurring issues
func (d *Detector) AnalyzeBlockerPattern(projectID string) (*BlockerAnalysis, error) {
	blockers, err := d.store.ListActiveBlockers(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list blockers: %w", err)
	}

	analysis := &BlockerAnalysis{
		TotalBlockers:      len(blockers),
		BlockersByTask:     make(map[string]int),
		CommonDescriptions: make(map[string]int),
		Recommendations:    []string{},
	}

	for _, blocker := range blockers {
		analysis.BlockersByTask[blocker.TaskID]++
		analysis.CommonDescriptions[blocker.Description]++
	}

	// Generate recommendations based on patterns
	for taskID, count := range analysis.BlockersByTask {
		if count >= 3 {
			analysis.Recommendations = append(analysis.Recommendations,
				fmt.Sprintf("Task %s has %d blockers - consider decomposing it into smaller tasks or skipping it", taskID, count))
		}
	}

	for desc, count := range analysis.CommonDescriptions {
		if count >= 2 {
			analysis.Recommendations = append(analysis.Recommendations,
				fmt.Sprintf("Recurring issue (%dx): %q - this may indicate a systemic problem that needs addressing", count, desc))
		}
	}

	if analysis.TotalBlockers > 5 {
		analysis.Recommendations = append(analysis.Recommendations,
			"High number of active blockers detected - consider reviewing the overall approach or navigating back to the design/plan stage")
	}

	if len(analysis.Recommendations) == 0 && analysis.TotalBlockers > 0 {
		analysis.Recommendations = append(analysis.Recommendations,
			"No recurring patterns detected - blockers appear to be independent issues")
	}

	return analysis, nil
}

// BlockerAnalysis contains analysis of blocker patterns
type BlockerAnalysis struct {
	TotalBlockers      int
	BlockersByTask     map[string]int
	CommonDescriptions map[string]int
	Recommendations    []string
}
