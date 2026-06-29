package resume

import (
	"fmt"
	"time"

	"github.com/mojomast/nexdev/internal/checkpoint"
	"github.com/mojomast/nexdev/internal/state"
)

// Manager handles resume detection and workflow
type Manager struct {
	store             *state.Store
	checkpointManager *checkpoint.Manager
}

// NewManager creates a new resume manager
func NewManager(store *state.Store, checkpointMgr *checkpoint.Manager) *Manager {
	return &Manager{
		store:             store,
		checkpointManager: checkpointMgr,
	}
}

// ResumeInfo contains information about resumable work
type ResumeInfo struct {
	ProjectID         string
	ProjectName       string
	CurrentStage      state.Stage
	CurrentPhaseID    string
	HasIncompleteWork bool
	LastCheckpoint    *state.Checkpoint
	Progress          *state.ProgressStats
	Summary           string
}

// DetectIncompleteWork checks if there's incomplete work for a project
func (m *Manager) DetectIncompleteWork(projectID string) (*ResumeInfo, error) {
	// Get project
	project, err := m.store.GetProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	info := &ResumeInfo{
		ProjectID:      project.ID,
		ProjectName:    project.Name,
		CurrentStage:   project.CurrentStage,
		CurrentPhaseID: project.CurrentPhase,
	}

	// Check if project is complete
	if project.CurrentStage == state.StageComplete {
		info.HasIncompleteWork = false
		info.Summary = "Project is complete"
		return info, nil
	}

	// Get progress stats
	progress, err := m.store.CalculateProgress(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate progress: %w", err)
	}
	info.Progress = progress

	// Determine if there's incomplete work
	info.HasIncompleteWork = (progress.CompletionPercentage < 100) ||
		(project.CurrentStage != state.StageComplete)

	// Get last checkpoint
	checkpoints, err := m.store.ListCheckpoints(projectID)
	if err == nil && len(checkpoints) > 0 {
		// Get the most recent checkpoint
		info.LastCheckpoint = checkpoints[len(checkpoints)-1]
	}

	// Generate summary
	info.Summary = m.generateSummary(project, progress)

	return info, nil
}

// generateSummary generates a human-readable summary of project state
func (m *Manager) generateSummary(project *state.Project, progress *state.ProgressStats) string {
	summary := fmt.Sprintf("Project: %s\n", project.Name)
	summary += fmt.Sprintf("Current Stage: %s\n", project.CurrentStage)
	summary += fmt.Sprintf("Progress: %.1f%% complete\n", progress.CompletionPercentage)
	summary += fmt.Sprintf("Completed Tasks: %d / %d\n", progress.CompletedTasks, progress.TotalTasks)

	if progress.InProgressTasks > 0 {
		summary += fmt.Sprintf("In Progress: %d tasks\n", progress.InProgressTasks)
	}

	if progress.BlockedTasks > 0 {
		summary += fmt.Sprintf("⚠️  Blocked: %d tasks\n", progress.BlockedTasks)
	}

	summary += fmt.Sprintf("Time Elapsed: %s\n", formatDuration(progress.ElapsedTime))

	if progress.EstimatedRemaining > 0 {
		summary += fmt.Sprintf("Estimated Remaining: %s\n", formatDuration(progress.EstimatedRemaining))
	}

	return summary
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

// ResumeOptions contains options for resuming work
type ResumeOptions struct {
	// ProjectID is the project to resume
	ProjectID string

	// FromCheckpoint optionally specifies a checkpoint to resume from
	FromCheckpoint string

	// RestartStage if true, restarts the current stage
	RestartStage bool

	// Stage optionally specifies a specific stage to resume from
	Stage *state.Stage

	// SelectedModel optionally specifies which model to use for resuming
	SelectedModel string
}

// ResumeResult contains the result of a resume operation
type ResumeResult struct {
	ProjectID      string
	Stage          state.Stage
	PhaseID        string
	RestoredFrom   string // checkpoint ID or "current"
	NextAction     string // Description of what to do next
	ModelSelection string // Which model was/should be selected
}

// Resume resumes work on a project
func (m *Manager) Resume(options *ResumeOptions) (*ResumeResult, error) {
	// Get project
	project, err := m.store.GetProject(options.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	result := &ResumeResult{
		ProjectID:      project.ID,
		ModelSelection: options.SelectedModel,
	}

	// If resuming from a checkpoint, restore that state
	if options.FromCheckpoint != "" {
		checkpoint, err := m.store.GetCheckpoint(options.FromCheckpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to get checkpoint: %w", err)
		}

		// Rollback to checkpoint using checkpoint manager
		if err := m.checkpointManager.Rollback(checkpoint.ID); err != nil {
			return nil, fmt.Errorf("failed to rollback to checkpoint: %w", err)
		}

		// Reload project after rollback
		project, err = m.store.GetProject(options.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("failed to reload project after rollback: %w", err)
		}

		result.RestoredFrom = checkpoint.ID
		result.Stage = project.CurrentStage
		result.PhaseID = project.CurrentPhase
	} else {
		// Resume from current state
		result.RestoredFrom = "current"
		result.Stage = project.CurrentStage
		result.PhaseID = project.CurrentPhase
	}

	// If a specific stage is requested, update to that stage
	if options.Stage != nil {
		result.Stage = *options.Stage

		// Update project stage
		if err := m.store.UpdateProjectStage(project.ID, *options.Stage); err != nil {
			return nil, fmt.Errorf("failed to update project stage: %w", err)
		}
	}

	// If restarting stage, reset the stage state
	if options.RestartStage {
		if err := m.restartStage(project, result.Stage); err != nil {
			return nil, fmt.Errorf("failed to restart stage: %w", err)
		}
	}

	// Determine next action based on stage
	result.NextAction = m.determineNextAction(project, result.Stage)

	return result, nil
}

// restartStage resets the state for a specific stage
func (m *Manager) restartStage(project *state.Project, stage state.Stage) error {
	switch stage {
	case state.StageInterview:
		// Clear interview data would go here if needed
		// For now, we'll just update the stage
		return m.store.UpdateProjectStage(project.ID, state.StageInterview)

	case state.StageDesign:
		// Clear architecture data would go here if needed
		return m.store.UpdateProjectStage(project.ID, state.StageDesign)

	case state.StagePlan:
		// Clear devplan data would go here if needed
		return m.store.UpdateProjectStage(project.ID, state.StagePlan)

	case state.StageReview:
		// Clear review data would go here if needed
		return m.store.UpdateProjectStage(project.ID, state.StageReview)

	case state.StageDevelop:
		// Reset in-progress tasks to not started
		if err := m.store.ResetProjectProgress(project.ID); err != nil {
			return fmt.Errorf("failed to reset project progress: %w", err)
		}

		return m.store.UpdateProjectStage(project.ID, state.StageDevelop)

	default:
		return fmt.Errorf("unknown stage: %s", stage)
	}
}

// determineNextAction determines what the user should do next
func (m *Manager) determineNextAction(project *state.Project, stage state.Stage) string {
	switch stage {
	case state.StageInit:
		return "Run 'geoffrussy init' to initialize the project"

	case state.StageInterview:
		return "Run 'geoffrussy interview --resume' to continue the interview"

	case state.StageDesign:
		return "Run 'geoffrussy design' to generate or review the architecture"

	case state.StagePlan:
		return "Run 'geoffrussy plan' to generate or review the DevPlan"

	case state.StageReview:
		return "Run 'geoffrussy review' to review and validate the DevPlan"

	case state.StageDevelop:
		// Get next pending task
		phases, err := m.store.ListPhases(project.ID)
		if err != nil {
			return "Run 'geoffrussy develop' to continue development"
		}

		for _, phase := range phases {
			if phase.Status == state.PhaseInProgress || phase.Status == state.PhaseNotStarted {
				return fmt.Sprintf("Run 'geoffrussy develop' to continue with phase %d: %s",
					phase.Number, phase.Title)
			}
		}

		return "Run 'geoffrussy develop' to continue development"

	case state.StageComplete:
		return "Project is complete!"

	default:
		return "Unknown next action"
	}
}

// ListAvailableCheckpoints lists all checkpoints available for resuming
func (m *Manager) ListAvailableCheckpoints(projectID string) ([]*state.Checkpoint, error) {
	return m.store.ListCheckpoints(projectID)
}

// GetResumeContext gets full context for resuming a specific stage
type ResumeContext struct {
	ProjectID      string
	Stage          state.Stage
	InterviewData  *state.InterviewData
	Architecture   *state.Architecture
	Phases         []*state.Phase
	Progress       *state.ProgressStats
	ActiveBlockers []*state.Blocker
}

// GetResumeContext gets comprehensive context for resuming work
func (m *Manager) GetResumeContext(projectID string, stage state.Stage) (*ResumeContext, error) {
	ctx := &ResumeContext{
		ProjectID: projectID,
		Stage:     stage,
	}

	// Get progress
	progress, err := m.store.CalculateProgress(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate progress: %w", err)
	}
	ctx.Progress = progress

	// Get stage-specific data
	switch stage {
	case state.StageInterview, state.StageDesign, state.StagePlan, state.StageReview, state.StageDevelop:
		// Get interview data if it exists
		interviewData, err := m.store.GetInterviewData(projectID)
		if err == nil {
			ctx.InterviewData = interviewData
		}
	}

	switch stage {
	case state.StageDesign, state.StagePlan, state.StageReview, state.StageDevelop:
		// Get architecture if it exists
		arch, err := m.store.GetArchitecture(projectID)
		if err == nil {
			ctx.Architecture = arch
		}
	}

	switch stage {
	case state.StagePlan, state.StageReview, state.StageDevelop:
		// Get phases
		phases, err := m.store.ListPhases(projectID)
		if err == nil {
			ctx.Phases = phases
		}
	}

	switch stage {
	case state.StageDevelop:
		// Get active blockers
		blockers, err := m.store.ListActiveBlockers(projectID)
		if err == nil {
			ctx.ActiveBlockers = blockers
		}
	}

	return ctx, nil
}
