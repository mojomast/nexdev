package detour

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mojomast/nexdev/internal/devplan"
	"github.com/mojomast/nexdev/internal/interview"
	"github.com/mojomast/nexdev/internal/state"
)

// Detour represents a mid-execution change to the development plan
type Detour struct {
	ID          string
	ProjectID   string
	PhaseID     string
	TaskID      string
	Description string
	Reason      string
	NewTasks    []devplan.Task
	Status      DetourStatus
	CreatedAt   time.Time
	CompletedAt *time.Time
}

// DetourStatus represents the status of a detour
type DetourStatus string

const (
	DetourPending   DetourStatus = "pending"
	DetourGathering DetourStatus = "gathering"
	DetourPlanned   DetourStatus = "planned"
	DetourActive    DetourStatus = "active"
	DetourCompleted DetourStatus = "completed"
	DetourCancelled DetourStatus = "cancelled"
)

// Manager handles detour workflow
type Manager struct {
	store            *state.Store
	interviewEngine  *interview.Engine
	devplanGenerator *devplan.Generator
}

// NewManager creates a new detour manager
func NewManager(store *state.Store, interviewEngine *interview.Engine, devplanGenerator *devplan.Generator) *Manager {
	return &Manager{
		store:            store,
		interviewEngine:  interviewEngine,
		devplanGenerator: devplanGenerator,
	}
}

// RequestDetour initiates a detour request
func (m *Manager) RequestDetour(projectID, phaseID, taskID, description, reason string) (*Detour, error) {
	detour := &Detour{
		ID:          fmt.Sprintf("detour-%s-%d", projectID, time.Now().UnixNano()),
		ProjectID:   projectID,
		PhaseID:     phaseID,
		TaskID:      taskID,
		Description: description,
		Reason:      reason,
		NewTasks:    []devplan.Task{},
		Status:      DetourPending,
		CreatedAt:   time.Now(),
	}

	return detour, nil
}

// GatherDetourInformation uses the interview engine to gather information about the detour
func (m *Manager) GatherDetourInformation(detour *Detour) error {
	if detour.Status != DetourPending {
		return fmt.Errorf("detour must be in pending status to gather information")
	}

	detour.Status = DetourGathering

	// Create a mini-interview session for the detour
	// This would use the interview engine to ask clarifying questions
	// For now, we'll simulate this with a simplified approach

	// In a real implementation, we would:
	// 1. Create interview questions specific to the detour
	// 2. Gather user responses
	// 3. Analyze the responses to understand the change needed

	detour.Status = DetourPlanned
	return nil
}

// UpdateDevPlan updates the development plan with new tasks from the detour
func (m *Manager) UpdateDevPlan(detour *Detour, insertAfterTaskID string) error {
	if detour.Status != DetourPlanned {
		return fmt.Errorf("detour must be in planned status to update devplan")
	}

	// Get the current phase
	_, err := m.store.GetPhase(detour.PhaseID)
	if err != nil {
		return fmt.Errorf("failed to get phase: %w", err)
	}

	// Generate new tasks based on the detour description
	newTasks := m.generateDetourTasks(detour)
	detour.NewTasks = newTasks

	// Save each generated task to the state store
	for _, dt := range newTasks {
		storeTask := &state.Task{
			ID:          dt.ID,
			PhaseID:     detour.PhaseID,
			Number:      dt.Number,
			Description: dt.Description,
			Status:      state.TaskNotStarted,
		}
		if err := m.store.SaveTask(storeTask); err != nil {
			return fmt.Errorf("failed to save detour task %s: %w", dt.ID, err)
		}
	}

	detour.Status = DetourActive

	// Persist the detour state
	if saveErr := m.SaveDetour(detour); saveErr != nil {
		// Non-fatal: tasks are saved even if detour metadata fails to persist
		fmt.Printf("Warning: failed to persist detour state: %v\n", saveErr)
	}

	return nil
}

// generateDetourTasks generates new tasks for the detour
func (m *Manager) generateDetourTasks(detour *Detour) []devplan.Task {
	// Simplified task generation
	// In a real implementation, this would use the LLM
	tasks := []devplan.Task{
		{
			ID:                  fmt.Sprintf("%s-task-1", detour.ID),
			Number:              "detour-1",
			Description:         fmt.Sprintf("Implement detour: %s", detour.Description),
			AcceptanceCriteria:  []string{"Detour requirements met"},
			ImplementationNotes: []string{detour.Reason},
			Status:              devplan.TaskNotStarted,
		},
	}

	return tasks
}

// CompleteDetour marks a detour as completed after verifying all tasks are done
func (m *Manager) CompleteDetour(detourID string) error {
	detour, err := m.GetDetour(detourID)
	if err != nil {
		return fmt.Errorf("failed to load detour: %w", err)
	}

	if detour.Status != DetourActive {
		return fmt.Errorf("detour must be in active status to complete (current: %s)", detour.Status)
	}

	// Verify all detour tasks are completed
	for _, task := range detour.NewTasks {
		if task.Status != devplan.TaskCompleted && task.Status != devplan.TaskSkipped {
			return fmt.Errorf("all detour tasks must be completed or skipped before completing detour (task %s is %s)", task.ID, task.Status)
		}
	}

	now := time.Now()
	detour.Status = DetourCompleted
	detour.CompletedAt = &now

	return m.SaveDetour(detour)
}

// ListDetours lists all detours for a project
func (m *Manager) ListDetours(projectID string) ([]*Detour, error) {
	prefix := fmt.Sprintf("detour_%s_", projectID)
	entries, err := m.store.ListConfigByPrefix(prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list detours: %w", err)
	}

	detours := make([]*Detour, 0, len(entries))
	for _, value := range entries {
		var detour Detour
		if err := json.Unmarshal([]byte(value), &detour); err != nil {
			// Skip malformed entries
			continue
		}
		detours = append(detours, &detour)
	}

	return detours, nil
}

// GetDetour retrieves a specific detour
func (m *Manager) GetDetour(detourID string) (*Detour, error) {
	// Try to find the detour by scanning all config entries
	// The key format is detour_{projectID}_{detourID}, but we don't know projectID
	// So we use the detourID directly as a suffix match
	entries, err := m.store.ListConfigByPrefix("detour_")
	if err != nil {
		return nil, fmt.Errorf("failed to query detours: %w", err)
	}

	for _, value := range entries {
		var detour Detour
		if err := json.Unmarshal([]byte(value), &detour); err != nil {
			continue
		}
		if detour.ID == detourID {
			return &detour, nil
		}
	}

	return nil, fmt.Errorf("detour not found: %s", detourID)
}

// SaveDetour persists a detour to the state store
func (m *Manager) SaveDetour(detour *Detour) error {
	data, err := json.Marshal(detour)
	if err != nil {
		return fmt.Errorf("failed to serialize detour: %w", err)
	}

	key := fmt.Sprintf("detour_%s_%s", detour.ProjectID, detour.ID)
	if err := m.store.SetConfig(key, string(data)); err != nil {
		return fmt.Errorf("failed to save detour: %w", err)
	}

	return nil
}

// TrackDetourInDirectory creates a detour tracking file in the detours directory
func (m *Manager) TrackDetourInDirectory(detour *Detour, detourDir string) error {
	// Create the detours directory if it doesn't exist
	if err := os.MkdirAll(detourDir, 0755); err != nil {
		return fmt.Errorf("failed to create detours directory: %w", err)
	}

	// Export as markdown
	md, err := m.ExportDetourMarkdown(detour)
	if err != nil {
		return fmt.Errorf("failed to export detour markdown: %w", err)
	}

	// Write the markdown file
	filename := filepath.Join(detourDir, fmt.Sprintf("%s.md", detour.ID))
	if err := os.WriteFile(filename, []byte(md), 0644); err != nil {
		return fmt.Errorf("failed to write detour file: %w", err)
	}

	return nil
}

// GetDetourDependencies returns all tasks that come after the detour's target task
// in the same phase. These tasks implicitly depend on the detour completing first.
func (m *Manager) GetDetourDependencies(detour *Detour) ([]string, error) {
	// Get all tasks in the phase
	tasks, err := m.store.ListTasks(detour.PhaseID)
	if err != nil {
		// If we can't list tasks (e.g. phase doesn't exist in store),
		// return empty dependencies rather than failing
		return []string{}, nil
	}

	// Find the detour's target task and collect all tasks that come after it
	foundTarget := false
	dependentIDs := []string{}

	for _, task := range tasks {
		if task.ID == detour.TaskID {
			foundTarget = true
			continue
		}
		if foundTarget {
			dependentIDs = append(dependentIDs, task.ID)
		}
	}

	return dependentIDs, nil
}

// UpdateTaskDependencies saves detour tasks to the state store so they become
// part of the project's task list. The affected task IDs are tasks that should
// logically wait for detour tasks to complete.
func (m *Manager) UpdateTaskDependencies(detour *Detour, affectedTaskIDs []string) error {
	// Save each detour task to the state store
	for _, dt := range detour.NewTasks {
		storeTask := &state.Task{
			ID:          dt.ID,
			PhaseID:     detour.PhaseID,
			Number:      dt.Number,
			Description: dt.Description,
			Status:      state.TaskNotStarted,
		}
		if err := m.store.SaveTask(storeTask); err != nil {
			return fmt.Errorf("failed to save detour task %s: %w", dt.ID, err)
		}
	}

	return nil
}

// ExportDetourMarkdown exports a detour as markdown for tracking
func (m *Manager) ExportDetourMarkdown(detour *Detour) (string, error) {
	md := fmt.Sprintf("# Detour: %s\n\n", detour.ID)
	md += fmt.Sprintf("**Project:** %s\n", detour.ProjectID)
	md += fmt.Sprintf("**Phase:** %s\n", detour.PhaseID)
	md += fmt.Sprintf("**Original Task:** %s\n", detour.TaskID)
	md += fmt.Sprintf("**Status:** %s\n", detour.Status)
	md += fmt.Sprintf("**Created:** %s\n\n", detour.CreatedAt.Format("2006-01-02 15:04:05"))

	if detour.CompletedAt != nil {
		md += fmt.Sprintf("**Completed:** %s\n\n", detour.CompletedAt.Format("2006-01-02 15:04:05"))
	}

	md += fmt.Sprintf("## Description\n\n%s\n\n", detour.Description)
	md += fmt.Sprintf("## Reason\n\n%s\n\n", detour.Reason)

	if len(detour.NewTasks) > 0 {
		md += "## New Tasks\n\n"
		for i, task := range detour.NewTasks {
			md += fmt.Sprintf("### %d. %s\n\n", i+1, task.Description)
			md += fmt.Sprintf("**Status:** %s\n\n", task.Status)

			if len(task.AcceptanceCriteria) > 0 {
				md += "**Acceptance Criteria:**\n"
				for _, criterion := range task.AcceptanceCriteria {
					md += fmt.Sprintf("- %s\n", criterion)
				}
				md += "\n"
			}
		}
	}

	return md, nil
}

// ValidateDetourDependencies checks if the detour conflicts with existing tasks
func (m *Manager) ValidateDetourDependencies(detour *Detour, phase *devplan.Phase) (bool, []string) {
	var conflicts []string

	// Build a set of existing task IDs
	existingIDs := make(map[string]bool)
	for _, task := range phase.Tasks {
		existingIDs[task.ID] = true
	}

	// Check for ID collisions between detour tasks and existing tasks
	for _, dt := range detour.NewTasks {
		if existingIDs[dt.ID] {
			conflicts = append(conflicts, fmt.Sprintf("task ID conflict: %s already exists in phase", dt.ID))
		}
	}

	// Check for number collisions
	existingNumbers := make(map[string]bool)
	for _, task := range phase.Tasks {
		existingNumbers[task.Number] = true
	}
	for _, dt := range detour.NewTasks {
		if existingNumbers[dt.Number] {
			conflicts = append(conflicts, fmt.Sprintf("task number conflict: %s already used in phase", dt.Number))
		}
	}

	return len(conflicts) == 0, conflicts
}

// ResolveDetourConflict resolves a conflict between detour tasks and an existing task.
// The resolution parameter controls the strategy: "rename" renames the detour task,
// "skip" marks the conflicting detour task as skipped.
func (m *Manager) ResolveDetourConflict(detour *Detour, conflictingTaskID string, resolution string) error {
	switch resolution {
	case "rename":
		for i, task := range detour.NewTasks {
			if task.ID == conflictingTaskID {
				detour.NewTasks[i].ID = fmt.Sprintf("%s-detour-%d", conflictingTaskID, time.Now().UnixNano())
				return nil
			}
		}
		return fmt.Errorf("conflicting task %s not found in detour", conflictingTaskID)

	case "skip":
		for i, task := range detour.NewTasks {
			if task.ID == conflictingTaskID {
				detour.NewTasks[i].Status = devplan.TaskSkipped
				return nil
			}
		}
		return fmt.Errorf("conflicting task %s not found in detour", conflictingTaskID)

	default:
		return fmt.Errorf("unknown resolution strategy: %s (supported: rename, skip)", resolution)
	}
}
