package navigation

import (
	"fmt"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/git"
	"github.com/mojomast/nexdev/internal/state"
)

// Navigator handles pipeline stage navigation
type Navigator struct {
	store   *state.Store
	gitMgr  *git.Manager
	history *HistoryTracker
}

// NewNavigator creates a new stage navigator
func NewNavigator(store *state.Store, gitMgr *git.Manager) *Navigator {
	return &Navigator{
		store:   store,
		gitMgr:  gitMgr,
		history: NewHistoryTracker(store, gitMgr),
	}
}

// NavigationResult contains the result of a navigation operation
type NavigationResult struct {
	FromStage            state.Stage
	ToStage              state.Stage
	PreservedWork        []string // List of preserved artifacts
	RegeneratedArtifacts []string // List of artifacts that will need regeneration
	NextAction           string
}

// NavigateToStage navigates from current stage to target stage
func (n *Navigator) NavigateToStage(projectID string, targetStage state.Stage) (*NavigationResult, error) {
	// Get current project state
	project, err := n.store.GetProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	currentStage := project.CurrentStage

	// Validate navigation (with prerequisite checks)
	if err := n.ValidateNavigationWithPrereqs(projectID, currentStage, targetStage); err != nil {
		return nil, fmt.Errorf("invalid navigation: %w", err)
	}

	result := &NavigationResult{
		FromStage:            currentStage,
		ToStage:              targetStage,
		PreservedWork:        []string{},
		RegeneratedArtifacts: []string{},
	}

	// Determine what work to preserve and what to regenerate
	if err := n.determineArtifacts(projectID, currentStage, targetStage, result); err != nil {
		return nil, fmt.Errorf("failed to determine artifacts: %w", err)
	}

	// Record navigation in history
	if err := n.history.RecordNavigation(projectID, currentStage, targetStage); err != nil {
		return nil, fmt.Errorf("failed to record navigation: %w", err)
	}

	// Update project stage
	if err := n.store.UpdateProjectStage(projectID, targetStage); err != nil {
		return nil, fmt.Errorf("failed to update project stage: %w", err)
	}

	// Commit navigation with stage marker
	commitMsg := fmt.Sprintf("Navigate from %s to %s stage\n\nPreserved: %v\nWill regenerate: %v",
		currentStage, targetStage, result.PreservedWork, result.RegeneratedArtifacts)

	metadata := map[string]string{
		"type":       "navigation",
		"from_stage": string(currentStage),
		"to_stage":   string(targetStage),
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	if err := n.gitMgr.CommitAll(commitMsg, metadata); err != nil {
		// Non-fatal - log but continue
		fmt.Printf("Warning: failed to commit navigation: %v\n", err)
	}

	// Determine next action
	result.NextAction = n.determineNextAction(targetStage)

	return result, nil
}

// ValidateNavigation checks if navigation from one stage to another is valid
func (n *Navigator) ValidateNavigation(from, to state.Stage) error {
	// Get stage order
	stageOrder := map[state.Stage]int{
		state.StageInit:      0,
		state.StageInterview: 1,
		state.StageDesign:    2,
		state.StagePlan:      3,
		state.StageReview:    4,
		state.StageDevelop:   5,
		state.StageComplete:  6,
	}

	fromOrder, fromOk := stageOrder[from]
	toOrder, toOk := stageOrder[to]

	if !fromOk {
		return fmt.Errorf("unknown source stage: %s", from)
	}
	if !toOk {
		return fmt.Errorf("unknown target stage: %s", to)
	}

	// Can't navigate to the same stage
	if from == to {
		return fmt.Errorf("already at stage %s", to)
	}

	// Can't navigate forward more than one stage (must complete each stage)
	if toOrder > fromOrder+1 {
		return fmt.Errorf("cannot skip stages: must complete %s before moving to %s",
			n.getStageAtOrder(fromOrder+1, stageOrder), to)
	}

	// Can always go backwards (to reiterate)
	if toOrder < fromOrder {
		return nil
	}

	// Moving forward one stage requires prerequisites
	// Note: checkPrerequisites needs project context but ValidateNavigation
	// doesn't have projectID. We skip prerequisite checks when called without
	// project context (e.g. direct validation). NavigateToStage does its own
	// prerequisite check before calling ValidateNavigation.
	return nil
}

// ValidateNavigationWithPrereqs validates navigation including prerequisite checks
func (n *Navigator) ValidateNavigationWithPrereqs(projectID string, from, to state.Stage) error {
	// Get stage order
	stageOrder := map[state.Stage]int{
		state.StageInit:      0,
		state.StageInterview: 1,
		state.StageDesign:    2,
		state.StagePlan:      3,
		state.StageReview:    4,
		state.StageDevelop:   5,
		state.StageComplete:  6,
	}

	fromOrder, fromOk := stageOrder[from]
	toOrder, toOk := stageOrder[to]

	if !fromOk {
		return fmt.Errorf("unknown source stage: %s", from)
	}
	if !toOk {
		return fmt.Errorf("unknown target stage: %s", to)
	}

	if from == to {
		return fmt.Errorf("already at stage %s", to)
	}

	if toOrder > fromOrder+1 {
		return fmt.Errorf("cannot skip stages: must complete %s before moving to %s",
			n.getStageAtOrder(fromOrder+1, stageOrder), to)
	}

	// Can always go backwards
	if toOrder < fromOrder {
		return nil
	}

	// Moving forward one stage requires prerequisites
	return n.checkPrerequisites(projectID, from, to)
}

// getStageAtOrder returns the stage at a given order
func (n *Navigator) getStageAtOrder(order int, stageOrder map[state.Stage]int) state.Stage {
	for stage, o := range stageOrder {
		if o == order {
			return stage
		}
	}
	return ""
}

// checkPrerequisites checks if prerequisites are met for stage transition
func (n *Navigator) checkPrerequisites(projectID string, from, to state.Stage) error {
	// Define prerequisites for each stage
	switch to {
	case state.StageInterview:
		// Init must be complete - project must exist
		_, err := n.store.GetProject(projectID)
		if err != nil {
			return fmt.Errorf("project must be initialized before starting interview: %w", err)
		}
		return nil

	case state.StageDesign:
		// Interview must be complete - interview data must exist
		_, err := n.store.GetInterviewData(projectID)
		if err != nil {
			return fmt.Errorf("interview must be completed before starting design: %w", err)
		}
		return nil

	case state.StagePlan:
		// Design must be complete - architecture must exist
		_, err := n.store.GetArchitecture(projectID)
		if err != nil {
			return fmt.Errorf("architecture must be generated before creating a plan: %w", err)
		}
		return nil

	case state.StageReview:
		// Plan must be complete - phases must exist
		phases, err := n.store.ListPhases(projectID)
		if err != nil {
			return fmt.Errorf("failed to check phases: %w", err)
		}
		if len(phases) == 0 {
			return fmt.Errorf("development plan must be created before review")
		}
		return nil

	case state.StageDevelop:
		// Review must be complete - phases must exist (review acknowledges them)
		phases, err := n.store.ListPhases(projectID)
		if err != nil {
			return fmt.Errorf("failed to check phases: %w", err)
		}
		if len(phases) == 0 {
			return fmt.Errorf("development plan must exist before starting development")
		}
		return nil

	case state.StageComplete:
		// All tasks must be complete
		tasks, err := n.store.ListTasksByProject(projectID)
		if err != nil {
			return fmt.Errorf("failed to check tasks: %w", err)
		}
		if len(tasks) == 0 {
			return fmt.Errorf("no tasks found - cannot mark project as complete")
		}
		for _, task := range tasks {
			if task.Status != state.TaskCompleted && task.Status != state.TaskSkipped {
				return fmt.Errorf("all tasks must be completed or skipped before marking project as complete (task %s is %s)", task.ID, task.Status)
			}
		}
		return nil

	default:
		return fmt.Errorf("unknown stage: %s", to)
	}
}

// determineArtifacts determines what to preserve and what to regenerate
func (n *Navigator) determineArtifacts(projectID string, from, to state.Stage, result *NavigationResult) error {
	// When going backwards, preserve current work
	stageOrder := map[state.Stage]int{
		state.StageInit:      0,
		state.StageInterview: 1,
		state.StageDesign:    2,
		state.StagePlan:      3,
		state.StageReview:    4,
		state.StageDevelop:   5,
		state.StageComplete:  6,
	}

	fromOrder := stageOrder[from]
	toOrder := stageOrder[to]

	// Going backwards - preserve everything at current stage and above
	if toOrder < fromOrder {
		// Check what artifacts exist
		if _, err := n.store.GetInterviewData(projectID); err == nil {
			result.PreservedWork = append(result.PreservedWork, "Interview data")
		}
		if _, err := n.store.GetArchitecture(projectID); err == nil {
			result.PreservedWork = append(result.PreservedWork, "Architecture")
		}
		phases, err := n.store.ListPhases(projectID)
		if err == nil && len(phases) > 0 {
			result.PreservedWork = append(result.PreservedWork, "DevPlan")
		}

		// Determine what will need regeneration based on target stage
		switch to {
		case state.StageInterview:
			if fromOrder > stageOrder[state.StageDesign] {
				result.RegeneratedArtifacts = append(result.RegeneratedArtifacts, "Architecture (if interview changes)")
			}
			if fromOrder > stageOrder[state.StagePlan] {
				result.RegeneratedArtifacts = append(result.RegeneratedArtifacts, "DevPlan (if architecture changes)")
			}

		case state.StageDesign:
			if fromOrder > stageOrder[state.StagePlan] {
				result.RegeneratedArtifacts = append(result.RegeneratedArtifacts, "DevPlan (if architecture changes)")
			}

		case state.StagePlan:
			// No regeneration needed for plan stage
		}
	}

	return nil
}

// determineNextAction determines the next action for a stage
func (n *Navigator) determineNextAction(stage state.Stage) string {
	switch stage {
	case state.StageInit:
		return "Run 'geoffrussy init' to initialize the project"
	case state.StageInterview:
		return "Run 'geoffrussy interview' to start or continue the interview"
	case state.StageDesign:
		return "Run 'geoffrussy design' to generate or refine the architecture"
	case state.StagePlan:
		return "Run 'geoffrussy plan' to generate or refine the DevPlan"
	case state.StageReview:
		return "Run 'geoffrussy review' to review and validate the DevPlan"
	case state.StageDevelop:
		return "Run 'geoffrussy develop' to start development"
	case state.StageComplete:
		return "Project is complete!"
	default:
		return "Unknown stage"
	}
}

// GetNavigationOptions returns available navigation options from current stage
func (n *Navigator) GetNavigationOptions(projectID string) (*NavigationOptions, error) {
	project, err := n.store.GetProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	options := &NavigationOptions{
		CurrentStage:   project.CurrentStage,
		CanGoBack:      []state.Stage{},
		CanGoForward:   []state.Stage{},
		NextStage:      "",
		PreviousStages: []state.Stage{},
	}

	// Define stage order
	stageOrder := map[state.Stage]int{
		state.StageInit:      0,
		state.StageInterview: 1,
		state.StageDesign:    2,
		state.StagePlan:      3,
		state.StageReview:    4,
		state.StageDevelop:   5,
		state.StageComplete:  6,
	}

	allStages := []state.Stage{
		state.StageInit,
		state.StageInterview,
		state.StageDesign,
		state.StagePlan,
		state.StageReview,
		state.StageDevelop,
		state.StageComplete,
	}

	currentOrder := stageOrder[project.CurrentStage]

	// Can go back to any previous stage
	for _, stage := range allStages {
		order := stageOrder[stage]
		if order < currentOrder {
			options.CanGoBack = append(options.CanGoBack, stage)
			options.PreviousStages = append(options.PreviousStages, stage)
		}
	}

	// Can go forward to next stage only
	if currentOrder < len(allStages)-1 {
		nextStage := allStages[currentOrder+1]
		options.NextStage = nextStage
		options.CanGoForward = append(options.CanGoForward, nextStage)
	}

	return options, nil
}

// NavigationOptions contains available navigation options
type NavigationOptions struct {
	CurrentStage   state.Stage
	CanGoBack      []state.Stage
	CanGoForward   []state.Stage
	NextStage      state.Stage
	PreviousStages []state.Stage
}

// HistoryTracker tracks pipeline navigation history
type HistoryTracker struct {
	store  *state.Store
	gitMgr *git.Manager
}

// NewHistoryTracker creates a new history tracker
func NewHistoryTracker(store *state.Store, gitMgr *git.Manager) *HistoryTracker {
	return &HistoryTracker{
		store:  store,
		gitMgr: gitMgr,
	}
}

// NavigationEvent represents a navigation event in the pipeline
type NavigationEvent struct {
	ID        string
	ProjectID string
	FromStage state.Stage
	ToStage   state.Stage
	Timestamp time.Time
	Reason    string
	GitCommit string
}

// RecordNavigation records a navigation event
func (h *HistoryTracker) RecordNavigation(projectID string, from, to state.Stage) error {
	// Store in database as config entry for now
	// In a full implementation, this would have its own table
	event := NavigationEvent{
		ProjectID: projectID,
		FromStage: from,
		ToStage:   to,
		Timestamp: time.Now(),
	}

	key := fmt.Sprintf("nav_history_%s_%d", projectID, time.Now().Unix())
	value := fmt.Sprintf("%s->%s at %s", from, to, event.Timestamp.Format(time.RFC3339))

	return h.store.SetConfig(key, value)
}

// GetNavigationHistory returns the navigation history for a project
func (h *HistoryTracker) GetNavigationHistory(projectID string) ([]NavigationEvent, error) {
	prefix := fmt.Sprintf("nav_history_%s_", projectID)
	entries, err := h.store.ListConfigByPrefix(prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to query navigation history: %w", err)
	}

	events := make([]NavigationEvent, 0, len(entries))
	for key, value := range entries {
		event, err := parseNavigationEvent(projectID, key, value)
		if err != nil {
			// Skip malformed entries rather than failing
			continue
		}
		events = append(events, event)
	}

	// Sort by timestamp (oldest first)
	sortNavigationEvents(events)

	return events, nil
}

// parseNavigationEvent parses a stored navigation event from config key-value pair.
// Key format: nav_history_{projectID}_{unixTimestamp}
// Value format: {fromStage}->{toStage} at {RFC3339Timestamp}
func parseNavigationEvent(projectID, key, value string) (NavigationEvent, error) {
	// Parse the value: "{from}->{to} at {timestamp}"
	arrowIdx := strings.Index(value, "->")
	if arrowIdx < 0 {
		return NavigationEvent{}, fmt.Errorf("malformed navigation event value: %s", value)
	}

	atIdx := strings.Index(value, " at ")
	if atIdx < 0 {
		return NavigationEvent{}, fmt.Errorf("malformed navigation event value: %s", value)
	}

	from := state.Stage(value[:arrowIdx])
	to := state.Stage(value[arrowIdx+2 : atIdx])
	timestampStr := value[atIdx+4:]

	ts, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return NavigationEvent{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	return NavigationEvent{
		ID:        key,
		ProjectID: projectID,
		FromStage: from,
		ToStage:   to,
		Timestamp: ts,
	}, nil
}

// sortNavigationEvents sorts events by timestamp ascending
func sortNavigationEvents(events []NavigationEvent) {
	for i := 1; i < len(events); i++ {
		for j := i; j > 0 && events[j].Timestamp.Before(events[j-1].Timestamp); j-- {
			events[j], events[j-1] = events[j-1], events[j]
		}
	}
}

// GetIterationCount returns the number of times a stage has been visited
func (h *HistoryTracker) GetIterationCount(projectID string, stage state.Stage) (int, error) {
	history, err := h.GetNavigationHistory(projectID)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, event := range history {
		if event.ToStage == stage {
			count++
		}
	}

	return count, nil
}
