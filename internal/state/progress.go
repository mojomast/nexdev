package state

import (
	"fmt"
	"time"
)

// ProgressStats represents overall project progress statistics
type ProgressStats struct {
	// Overall stats
	TotalPhases      int
	CompletedPhases  int
	InProgressPhases int
	BlockedPhases    int
	PendingPhases    int

	TotalTasks      int
	CompletedTasks  int
	InProgressTasks int
	BlockedTasks    int
	SkippedTasks    int
	PendingTasks    int

	// Progress percentage
	CompletionPercentage float64

	// Time tracking
	StartedAt          time.Time
	ElapsedTime        time.Duration
	EstimatedRemaining time.Duration

	// Current state
	CurrentStage   Stage
	CurrentPhaseID string
}

// PhaseProgress represents progress for a single phase
type PhaseProgress struct {
	PhaseID         string
	PhaseNumber     int
	PhaseTitle      string
	Status          PhaseStatus
	TotalTasks      int
	CompletedTasks  int
	InProgressTasks int
	BlockedTasks    int
	SkippedTasks    int
	Percentage      float64
}

// CalculateProgress calculates overall project progress
func (s *Store) CalculateProgress(projectID string) (*ProgressStats, error) {
	// Get project
	project, err := s.GetProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	stats := &ProgressStats{
		CurrentStage:   project.CurrentStage,
		CurrentPhaseID: project.CurrentPhase,
		StartedAt:      project.CreatedAt,
		ElapsedTime:    time.Since(project.CreatedAt),
	}

	// Get all phases for the project
	phases, err := s.ListPhases(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list phases: %w", err)
	}

	stats.TotalPhases = len(phases)

	// Get all tasks for the project to avoid N+1 query
	allTasks, err := s.ListTasksByProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks for project: %w", err)
	}

	// Group tasks by phase ID
	tasksByPhaseID := make(map[string][]Task)
	for _, task := range allTasks {
		tasksByPhaseID[task.PhaseID] = append(tasksByPhaseID[task.PhaseID], task)
	}

	// Count phases by status
	for _, phase := range phases {
		switch phase.Status {
		case PhaseCompleted:
			stats.CompletedPhases++
		case PhaseInProgress:
			stats.InProgressPhases++
		case PhaseBlocked:
			stats.BlockedPhases++
		case PhaseNotStarted:
			stats.PendingPhases++
		}

		// Get tasks for this phase from map
		tasks := tasksByPhaseID[phase.ID]

		stats.TotalTasks += len(tasks)

		for _, task := range tasks {
			switch task.Status {
			case TaskCompleted:
				stats.CompletedTasks++
			case TaskInProgress:
				stats.InProgressTasks++
			case TaskBlocked:
				stats.BlockedTasks++
			case TaskSkipped:
				stats.SkippedTasks++
			case TaskNotStarted:
				stats.PendingTasks++
			}
		}
	}

	// Calculate completion percentage
	activeTasks := stats.TotalTasks - stats.SkippedTasks
	if activeTasks > 0 {
		stats.CompletionPercentage = float64(stats.CompletedTasks) / float64(activeTasks) * 100
	}

	// Estimate remaining time based on completion rate
	if stats.CompletedTasks > 0 && stats.CompletionPercentage > 0 {
		avgTimePerTask := stats.ElapsedTime / time.Duration(stats.CompletedTasks)
		// Only count remaining active tasks
		remainingTasks := stats.TotalTasks - stats.SkippedTasks - stats.CompletedTasks
		if remainingTasks < 0 {
			remainingTasks = 0
		}
		stats.EstimatedRemaining = avgTimePerTask * time.Duration(remainingTasks)
	}

	return stats, nil
}

// GetPhaseProgress gets progress for a specific phase
func (s *Store) GetPhaseProgress(phaseID string) (*PhaseProgress, error) {
	phase, err := s.GetPhase(phaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get phase: %w", err)
	}

	tasks, err := s.ListTasks(phaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	progress := &PhaseProgress{
		PhaseID:     phase.ID,
		PhaseNumber: phase.Number,
		PhaseTitle:  phase.Title,
		Status:      phase.Status,
		TotalTasks:  len(tasks),
	}

	for _, task := range tasks {
		switch task.Status {
		case TaskCompleted:
			progress.CompletedTasks++
		case TaskInProgress:
			progress.InProgressTasks++
		case TaskBlocked:
			progress.BlockedTasks++
		case TaskSkipped:
			progress.SkippedTasks++
		}
	}

	activeTasks := progress.TotalTasks - progress.SkippedTasks
	if activeTasks > 0 {
		progress.Percentage = float64(progress.CompletedTasks) / float64(activeTasks) * 100
	}

	return progress, nil
}

// ListAllPhaseProgress gets progress for all phases in a project
func (s *Store) ListAllPhaseProgress(projectID string) ([]*PhaseProgress, error) {
	phases, err := s.ListPhases(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list phases: %w", err)
	}

	// Get all tasks for the project in one query
	tasks, err := s.ListTasksByProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list project tasks: %w", err)
	}

	// Group tasks by phase ID
	tasksByPhase := make(map[string][]Task)
	for _, task := range tasks {
		tasksByPhase[task.PhaseID] = append(tasksByPhase[task.PhaseID], task)
	}

	var progressList []*PhaseProgress

	for _, phase := range phases {
		phaseTasks := tasksByPhase[phase.ID]

		progress := &PhaseProgress{
			PhaseID:     phase.ID,
			PhaseNumber: phase.Number,
			PhaseTitle:  phase.Title,
			Status:      phase.Status,
			TotalTasks:  len(phaseTasks),
		}

		for _, task := range phaseTasks {
			switch task.Status {
			case TaskCompleted:
				progress.CompletedTasks++
			case TaskInProgress:
				progress.InProgressTasks++
			case TaskBlocked:
				progress.BlockedTasks++
			case TaskSkipped:
				progress.SkippedTasks++
			}
		}

		activeTasks := progress.TotalTasks - progress.SkippedTasks
		if activeTasks > 0 {
			progress.Percentage = float64(progress.CompletedTasks) / float64(activeTasks) * 100
		}

		progressList = append(progressList, progress)
	}

	return progressList, nil
}

// FilterProgress filters progress by phase or component
type ProgressFilter struct {
	PhaseID      string
	PhaseNumbers []int
	StatusFilter []PhaseStatus
}

// GetFilteredProgress gets progress with filters applied
func (s *Store) GetFilteredProgress(projectID string, filter *ProgressFilter) ([]*PhaseProgress, error) {
	allProgress, err := s.ListAllPhaseProgress(projectID)
	if err != nil {
		return nil, err
	}

	if filter == nil {
		return allProgress, nil
	}

	var filtered []*PhaseProgress

	for _, progress := range allProgress {
		// Filter by phase ID
		if filter.PhaseID != "" && progress.PhaseID != filter.PhaseID {
			continue
		}

		// Filter by phase numbers
		if len(filter.PhaseNumbers) > 0 {
			found := false
			for _, num := range filter.PhaseNumbers {
				if progress.PhaseNumber == num {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by status
		if len(filter.StatusFilter) > 0 {
			found := false
			for _, status := range filter.StatusFilter {
				if progress.Status == status {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		filtered = append(filtered, progress)
	}

	return filtered, nil
}
