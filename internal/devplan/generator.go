package devplan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mojomast/nexdev/internal/common"
	"github.com/mojomast/nexdev/internal/design"
	"github.com/mojomast/nexdev/internal/logging"
	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
)

// CacheStore interface for caching AI responses
type CacheStore interface {
	GetCache(key string) (string, error)
	SetCache(key string, value string, ttl time.Duration) error
}

// Generator generates development plans from architecture
type Generator struct {
	provider provider.Provider
	model    string
	cache    CacheStore
	logger   *logging.Logger
}

// NewGenerator creates a new devplan generator
func NewGenerator(provider provider.Provider, model string, cache CacheStore) *Generator {
	return &Generator{
		provider: provider,
		model:    model,
		cache:    cache,
		logger:   logging.NewLogger(slog.LevelInfo, os.Stdout),
	}
}

// Phase represents a development phase
type Phase struct {
	ID              string      `json:"id"`
	Number          int         `json:"number"`
	Title           string      `json:"title"`
	Objective       string      `json:"objective"`
	SuccessCriteria []string    `json:"success_criteria"`
	Dependencies    []string    `json:"dependencies"`
	Tasks           []Task      `json:"tasks"`
	EstimatedTokens int         `json:"estimated_tokens"`
	EstimatedCost   float64     `json:"estimated_cost"`
	Status          PhaseStatus `json:"status"`
	CreatedAt       time.Time   `json:"created_at"`
}

// PhaseStatus represents the status of a phase
type PhaseStatus string

const (
	PhaseNotStarted PhaseStatus = "not_started"
	PhaseInProgress PhaseStatus = "in_progress"
	PhaseCompleted  PhaseStatus = "completed"
	PhaseBlocked    PhaseStatus = "blocked"
)

// Task represents a development task
type Task struct {
	ID                  string     `json:"id"`
	Number              string     `json:"number"`
	Description         string     `json:"description"`
	AcceptanceCriteria  []string   `json:"acceptance_criteria"`
	ImplementationNotes []string   `json:"implementation_notes"`
	BlockersEncountered []string   `json:"blockers_encountered"`
	Status              TaskStatus `json:"status"`
}

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskNotStarted TaskStatus = "not_started"
	TaskInProgress TaskStatus = "in_progress"
	TaskCompleted  TaskStatus = "completed"
	TaskBlocked    TaskStatus = "blocked"
	TaskSkipped    TaskStatus = "skipped"
)

// DevPlan represents the complete development plan
type DevPlan struct {
	ProjectID   string    `json:"project_id"`
	Phases      []Phase   `json:"phases"`
	TotalTokens int       `json:"total_tokens"`
	TotalCost   float64   `json:"total_cost"`
	CreatedAt   time.Time `json:"created_at"`
}

// GeneratePhases generates 7-10 executable phases from architecture and interview data
func (g *Generator) GeneratePhases(architecture *design.Architecture, interviewData *state.InterviewData) ([]Phase, error) {
	if g.provider == nil {
		return nil, fmt.Errorf("provider is required for phase generation")
	}

	prompt := g.buildPhasesPrompt(architecture, interviewData)
	cacheKey := g.generateCacheKey(prompt)

	// Check cache first
	if g.cache != nil {
		cached, err := g.cache.GetCache(cacheKey)
		if err == nil && cached != "" {
			// Validate cached content
			phases, parseErr := g.parsePhasesResponse(cached)
			if parseErr == nil {
				g.logger.Info("Using cached development plan")
				// Recalculate estimates since they are not cached in the JSON structure
				for i := range phases {
					phases[i].EstimatedTokens = g.estimatePhaseTokens(&phases[i])
					phases[i].EstimatedCost = g.estimatePhaseCost(phases[i].EstimatedTokens)
					phases[i].CreatedAt = time.Now()
				}
				return phases, nil
			}
			g.logger.Warn("Cached development plan invalid, regenerating", "error", parseErr)
		}
	}

	// Generate fresh content
	response, err := g.provider.Call(context.TODO(), g.model, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate phases: %w", err)
	}
	content := response.Content

	phases, err := g.parsePhasesResponse(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse phases: %w", err)
	}

	// Cache successful response (7 days TTL)
	if g.cache != nil {
		if err := g.cache.SetCache(cacheKey, content, 7*24*time.Hour); err != nil {
			g.logger.Warn("Failed to cache response", "error", err)
		}
	}

	// Estimate tokens and costs for each phase
	for i := range phases {
		phases[i].EstimatedTokens = g.estimatePhaseTokens(&phases[i])
		phases[i].EstimatedCost = g.estimatePhaseCost(phases[i].EstimatedTokens)
		phases[i].CreatedAt = time.Now()
	}

	return phases, nil
}

// buildPhasesPrompt creates the prompt for phase generation
func (g *Generator) buildPhasesPrompt(architecture *design.Architecture, interviewData *state.InterviewData) string {
	prompt := `You are an expert software project planner. Based on the following architecture and requirements, generate 7-10 executable development phases.

CRITICAL OUTPUT RULES:
1. Return ONLY a valid JSON array.
2. The response must be strict JSON syntax.
3. Do NOT include commentary or prose before/after JSON.
4. Each phase must include 3-5 tasks and concrete acceptance criteria.
5. Keep dependency IDs consistent and acyclic.

PROJECT: ` + interviewData.ProjectName + `
PROBLEM: ` + interviewData.ProblemStatement + `

ARCHITECTURE OVERVIEW:
` + architecture.SystemOverview + `

Think step-by-step:
1. Analyze the architecture components and their dependencies.
2. Determine the logical implementation order (e.g., database -> API -> Frontend).
3. Break down the work into 7-10 distinct phases.
4. Ensure each phase results in verifiable working code.
5. Identify clear success criteria for each phase.

Each phase should:
1. Build on previous phases
2. Result in deployable, testable code
3. Be completable in 1-2 hours by an LLM agent
4. Include 3-5 actionable tasks
5. Have clear objective and success criteria

Follow this standard order:
- Phase 0: Setup & Infrastructure
- Phase 1: Database & Models
- Phase 2: Core API
- Phase 3: Authentication & Authorization
- Phase 4: Frontend Foundation
- Phase 5: Real-time Sync (if needed)
- Phase 6: Integrations
- Phase 7: Testing & Validation
- Phase 8: Performance & Observability
- Phase 9: Deployment & Hardening

Output your response in the following format:
[
  {
    "number": 0,
    "title": "Phase Title",
    "objective": "Clear objective",
    "success_criteria": ["Criterion 1", "Criterion 2"],
    "dependencies": ["0", "1"],
    "tasks": [
      {
        "number": "0.1",
        "description": "Task description",
        "acceptance_criteria": ["Acceptance 1", "Acceptance 2"],
        "implementation_notes": ["Note 1", "Note 2"]
      }
    ]
  }
]

Generate the JSON now:`

	return prompt
}

// parsePhasesResponse parses the LLM response into Phase structs
func (g *Generator) parsePhasesResponse(response string) ([]Phase, error) {
	phases := []Phase{}

	if response == "" {
		return nil, fmt.Errorf("empty response from provider")
	}

	// Extract JSON from the response
	jsonContent := response

	// Remove scratchpad if present
	if start := strings.Index(jsonContent, "<scratchpad>"); start != -1 {
		if end := strings.Index(jsonContent, "</scratchpad>"); end != -1 {
			// We keep what's after the scratchpad
			if end+13 < len(jsonContent) {
				jsonContent = jsonContent[end+13:]
			}
		}
	}

	if err := common.ParseJSON(jsonContent, &phases); err != nil {
		// Log the raw output for debugging
		g.logger.Error("Failed to parse phases JSON",
			"error", err,
			"raw_content", response,
		)
		return nil, fmt.Errorf("failed to parse AI response as JSON: %w", err)
	}

	// Post-process parsed phases
	for i := range phases {
		// Generate ID if missing
		if phases[i].ID == "" {
			phases[i].ID = fmt.Sprintf("phase-%d", phases[i].Number)
		}

		// Ensure Status is set
		if phases[i].Status == "" {
			phases[i].Status = PhaseNotStarted
		}

		// Process tasks
		for j := range phases[i].Tasks {
			// Generate Task ID if missing
			if phases[i].Tasks[j].ID == "" {
				phases[i].Tasks[j].ID = fmt.Sprintf("task-%d-%d", phases[i].Number, j+1)
			}

			// Ensure Task Status is set
			if phases[i].Tasks[j].Status == "" {
				phases[i].Tasks[j].Status = TaskNotStarted
			}
		}
	}

	return phases, nil
}

// generateCacheKey generates a cache key for a prompt
func (g *Generator) generateCacheKey(prompt string) string {
	hash := sha256.Sum256([]byte(prompt + g.model))
	return hex.EncodeToString(hash[:])
}

// estimatePhaseTokens estimates the token usage for a phase
func (g *Generator) estimatePhaseTokens(phase *Phase) int {
	// Rough estimation: 1000 tokens per task
	baseTokens := 1000
	taskTokens := len(phase.Tasks) * 1000
	return baseTokens + taskTokens
}

// estimatePhaseCost estimates the cost for a phase based on tokens
func (g *Generator) estimatePhaseCost(tokens int) float64 {
	// Rough estimation: $0.01 per 1000 tokens (average)
	return float64(tokens) / 1000.0 * 0.01
}

// ExportPhaseMarkdown exports a phase as markdown
func (g *Generator) ExportPhaseMarkdown(phase *Phase) (string, error) {
	var md strings.Builder

	md.WriteString(fmt.Sprintf("# Phase %d: %s\n\n", phase.Number, phase.Title))
	md.WriteString(fmt.Sprintf("**Status:** %s\n\n", phase.Status))
	md.WriteString(fmt.Sprintf("## Objective\n\n%s\n\n", phase.Objective))

	md.WriteString("## Success Criteria\n\n")
	for _, criterion := range phase.SuccessCriteria {
		md.WriteString(fmt.Sprintf("- %s\n", criterion))
	}
	md.WriteString("\n")

	if len(phase.Dependencies) > 0 {
		md.WriteString("## Dependencies\n\n")
		md.WriteString(fmt.Sprintf("Depends on phases: %s\n\n", strings.Join(phase.Dependencies, ", ")))
	}

	md.WriteString("## Tasks\n\n")
	for _, task := range phase.Tasks {
		md.WriteString(fmt.Sprintf("### %s: %s\n\n", task.Number, task.Description))
		md.WriteString(fmt.Sprintf("**Status:** %s\n\n", task.Status))

		if len(task.AcceptanceCriteria) > 0 {
			md.WriteString("**Acceptance Criteria:**\n")
			for _, criterion := range task.AcceptanceCriteria {
				md.WriteString(fmt.Sprintf("- %s\n", criterion))
			}
			md.WriteString("\n")
		}

		if len(task.ImplementationNotes) > 0 {
			md.WriteString("**Implementation Notes:**\n")
			for _, note := range task.ImplementationNotes {
				md.WriteString(fmt.Sprintf("- %s\n", note))
			}
			md.WriteString("\n")
		}
	}

	md.WriteString(fmt.Sprintf("## Estimates\n\n"))
	md.WriteString(fmt.Sprintf("- **Tokens:** %d\n", phase.EstimatedTokens))
	md.WriteString(fmt.Sprintf("- **Cost:** $%.2f\n", phase.EstimatedCost))

	return md.String(), nil
}

// ExportMasterPlan exports the master devplan overview
func (g *Generator) ExportMasterPlan(devplan *DevPlan) (string, error) {
	var md strings.Builder

	md.WriteString("# Development Plan\n\n")
	md.WriteString(fmt.Sprintf("**Project ID:** %s\n", devplan.ProjectID))
	md.WriteString(fmt.Sprintf("**Generated:** %s\n\n", devplan.CreatedAt.Format("2006-01-02 15:04:05")))

	md.WriteString("## Overview\n\n")
	md.WriteString(fmt.Sprintf("This development plan consists of %d phases.\n\n", len(devplan.Phases)))

	md.WriteString("## Phases\n\n")
	for _, phase := range devplan.Phases {
		md.WriteString(fmt.Sprintf("### Phase %d: %s\n\n", phase.Number, phase.Title))
		md.WriteString(fmt.Sprintf("**Objective:** %s\n\n", phase.Objective))
		md.WriteString(fmt.Sprintf("**Tasks:** %d\n", len(phase.Tasks)))
		md.WriteString(fmt.Sprintf("**Estimated Tokens:** %d\n", phase.EstimatedTokens))
		md.WriteString(fmt.Sprintf("**Estimated Cost:** $%.2f\n", phase.EstimatedCost))
		md.WriteString(fmt.Sprintf("**Status:** %s\n\n", phase.Status))
	}

	md.WriteString("## Total Estimates\n\n")
	md.WriteString(fmt.Sprintf("- **Total Tokens:** %d\n", devplan.TotalTokens))
	md.WriteString(fmt.Sprintf("- **Total Cost:** $%.2f\n", devplan.TotalCost))

	return md.String(), nil
}

// ExportJSON exports the devplan as JSON
func (g *Generator) ExportJSON(devplan *DevPlan) (string, error) {
	jsonData, err := json.MarshalIndent(devplan, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal devplan: %w", err)
	}
	return string(jsonData), nil
}

// MergePhases merges two phases into one
func (g *Generator) MergePhases(phase1, phase2 *Phase) (*Phase, error) {
	if phase1 == nil || phase2 == nil {
		return nil, fmt.Errorf("both phases must be non-nil")
	}

	merged := &Phase{
		ID:              fmt.Sprintf("%s-%s-merged", phase1.ID, phase2.ID),
		Number:          phase1.Number,
		Title:           fmt.Sprintf("%s & %s", phase1.Title, phase2.Title),
		Objective:       fmt.Sprintf("%s. %s", phase1.Objective, phase2.Objective),
		SuccessCriteria: append(phase1.SuccessCriteria, phase2.SuccessCriteria...),
		Dependencies:    mergeDependencies(phase1.Dependencies, phase2.Dependencies),
		Tasks:           append(phase1.Tasks, phase2.Tasks...),
		EstimatedTokens: phase1.EstimatedTokens + phase2.EstimatedTokens,
		EstimatedCost:   phase1.EstimatedCost + phase2.EstimatedCost,
		Status:          PhaseNotStarted,
		CreatedAt:       time.Now(),
	}

	// Renumber tasks
	for i := range merged.Tasks {
		merged.Tasks[i].Number = fmt.Sprintf("%d.%d", merged.Number, i+1)
	}

	return merged, nil
}

// mergeDependencies merges two dependency lists, removing duplicates
func mergeDependencies(deps1, deps2 []string) []string {
	depMap := make(map[string]bool)
	for _, dep := range deps1 {
		depMap[dep] = true
	}
	for _, dep := range deps2 {
		depMap[dep] = true
	}

	merged := []string{}
	for dep := range depMap {
		merged = append(merged, dep)
	}
	return merged
}

// SplitPhase splits a phase into two phases
func (g *Generator) SplitPhase(phase *Phase, splitPoint int) ([]*Phase, error) {
	if phase == nil {
		return nil, fmt.Errorf("phase must be non-nil")
	}

	if splitPoint <= 0 || splitPoint >= len(phase.Tasks) {
		return nil, fmt.Errorf("invalid split point: must be between 1 and %d", len(phase.Tasks)-1)
	}

	// Split tasks
	tasks1 := phase.Tasks[:splitPoint]
	tasks2 := phase.Tasks[splitPoint:]

	// Estimate tokens and costs for each part
	tokens1 := g.estimateTasksTokens(tasks1)
	tokens2 := g.estimateTasksTokens(tasks2)

	phase1 := &Phase{
		ID:              fmt.Sprintf("%s-part1", phase.ID),
		Number:          phase.Number,
		Title:           fmt.Sprintf("%s (Part 1)", phase.Title),
		Objective:       phase.Objective,
		SuccessCriteria: phase.SuccessCriteria[:len(phase.SuccessCriteria)/2],
		Dependencies:    phase.Dependencies,
		Tasks:           tasks1,
		EstimatedTokens: tokens1,
		EstimatedCost:   g.estimatePhaseCost(tokens1),
		Status:          PhaseNotStarted,
		CreatedAt:       time.Now(),
	}

	phase2 := &Phase{
		ID:              fmt.Sprintf("%s-part2", phase.ID),
		Number:          phase.Number + 1,
		Title:           fmt.Sprintf("%s (Part 2)", phase.Title),
		Objective:       phase.Objective,
		SuccessCriteria: phase.SuccessCriteria[len(phase.SuccessCriteria)/2:],
		Dependencies:    []string{fmt.Sprintf("%d", phase.Number)},
		Tasks:           tasks2,
		EstimatedTokens: tokens2,
		EstimatedCost:   g.estimatePhaseCost(tokens2),
		Status:          PhaseNotStarted,
		CreatedAt:       time.Now(),
	}

	// Renumber tasks
	for i := range phase1.Tasks {
		phase1.Tasks[i].Number = fmt.Sprintf("%d.%d", phase1.Number, i+1)
	}
	for i := range phase2.Tasks {
		phase2.Tasks[i].Number = fmt.Sprintf("%d.%d", phase2.Number, i+1)
	}

	return []*Phase{phase1, phase2}, nil
}

// estimateTasksTokens estimates tokens for a list of tasks
func (g *Generator) estimateTasksTokens(tasks []Task) int {
	return 1000 + (len(tasks) * 1000)
}

// ReorderPhases reorders phases according to the new order
func (g *Generator) ReorderPhases(phases []Phase, newOrder []int) ([]Phase, error) {
	if len(newOrder) != len(phases) {
		return nil, fmt.Errorf("new order must have same length as phases")
	}

	// Validate new order
	seen := make(map[int]bool)
	for _, idx := range newOrder {
		if idx < 0 || idx >= len(phases) {
			return nil, fmt.Errorf("invalid index in new order: %d", idx)
		}
		if seen[idx] {
			return nil, fmt.Errorf("duplicate index in new order: %d", idx)
		}
		seen[idx] = true
	}

	// Reorder phases
	reordered := make([]Phase, len(phases))
	for newIdx, oldIdx := range newOrder {
		reordered[newIdx] = phases[oldIdx]
		reordered[newIdx].Number = newIdx

		// Update task numbers
		for i := range reordered[newIdx].Tasks {
			reordered[newIdx].Tasks[i].Number = fmt.Sprintf("%d.%d", newIdx, i+1)
		}
	}

	// Update dependencies
	for i := range reordered {
		updatedDeps := []string{}
		for _, dep := range reordered[i].Dependencies {
			// Find the old phase number in the dependency
			for oldIdx, phase := range phases {
				if fmt.Sprintf("%d", phase.Number) == dep {
					// Find where this phase ended up in the new order
					for newIdx, reorderedIdx := range newOrder {
						if reorderedIdx == oldIdx {
							updatedDeps = append(updatedDeps, fmt.Sprintf("%d", newIdx))
							break
						}
					}
					break
				}
			}
		}
		reordered[i].Dependencies = updatedDeps
	}

	return reordered, nil
}

// ValidatePhaseOrder checks if phases are in a valid order (dependencies satisfied)
func (g *Generator) ValidatePhaseOrder(phases []Phase) (bool, []string) {
	var issues []string

	for i, phase := range phases {
		for _, dep := range phase.Dependencies {
			// Check if dependency is satisfied
			depNum := -1
			fmt.Sscanf(dep, "%d", &depNum)

			if depNum >= i {
				issues = append(issues, fmt.Sprintf("Phase %d depends on phase %d which comes after it", i, depNum))
			}
		}
	}

	return len(issues) == 0, issues
}

// ChangelogEntry represents a single changelog entry
type ChangelogEntry struct {
	Timestamp   time.Time
	Type        string // "task_completed", "phase_completed", "detour_added", "phase_modified"
	Description string
	Author      string
	Details     map[string]string
}

// Changelog maintains the history of changes to the DevPlan
type Changelog struct {
	Entries []ChangelogEntry
}

// AddEntry adds a new entry to the changelog
func (c *Changelog) AddEntry(entryType, description, author string, details map[string]string) {
	entry := ChangelogEntry{
		Timestamp:   time.Now(),
		Type:        entryType,
		Description: description,
		Author:      author,
		Details:     details,
	}
	c.Entries = append(c.Entries, entry)
}

// ExportMarkdown exports the changelog as markdown
func (c *Changelog) ExportMarkdown() string {
	var md strings.Builder

	md.WriteString("# DevPlan Changelog\n\n")
	md.WriteString("This changelog tracks all modifications to the development plan.\n\n")

	for _, entry := range c.Entries {
		md.WriteString(fmt.Sprintf("## %s - %s\n\n", entry.Timestamp.Format("2006-01-02 15:04:05"), entry.Type))
		md.WriteString(fmt.Sprintf("**Description:** %s\n\n", entry.Description))

		if entry.Author != "" {
			md.WriteString(fmt.Sprintf("**Author:** %s\n\n", entry.Author))
		}

		if len(entry.Details) > 0 {
			md.WriteString("**Details:**\n")
			for key, value := range entry.Details {
				md.WriteString(fmt.Sprintf("- %s: %s\n", key, value))
			}
			md.WriteString("\n")
		}
	}

	return md.String()
}

// UpdatePhaseMarkdown updates a phase markdown file with current status
func (g *Generator) UpdatePhaseMarkdown(phase *Phase, filePath string) error {
	content, err := g.ExportPhaseMarkdown(phase)
	if err != nil {
		return fmt.Errorf("failed to export phase markdown: %w", err)
	}

	// Write to file
	err = writeFile(filePath, content)
	if err != nil {
		return fmt.Errorf("failed to write phase markdown: %w", err)
	}

	return nil
}

// writeFile is a helper to write content to a file
func writeFile(path, content string) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Write the file with secure permissions (owner read/write only)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	return nil
}

// UpdateMasterPlanWithChangelog updates the master plan with changelog
func (g *Generator) UpdateMasterPlanWithChangelog(devplan *DevPlan, changelog *Changelog) (string, error) {
	var md strings.Builder

	// Export the base master plan
	basePlan, err := g.ExportMasterPlan(devplan)
	if err != nil {
		return "", err
	}

	md.WriteString(basePlan)
	md.WriteString("\n")

	// Append changelog
	md.WriteString("## Changelog\n\n")
	md.WriteString(changelog.ExportMarkdown())

	return md.String(), nil
}

// VisualizeProgress generates a visual representation of DevPlan progress
func (g *Generator) VisualizeProgress(devplan *DevPlan) string {
	var vis strings.Builder

	vis.WriteString("# DevPlan Progress\n\n")

	// Calculate overall statistics
	totalTasks := 0
	completedTasks := 0
	inProgressTasks := 0
	blockedTasks := 0

	for _, phase := range devplan.Phases {
		totalTasks += len(phase.Tasks)
		for _, task := range phase.Tasks {
			switch task.Status {
			case TaskCompleted:
				completedTasks++
			case TaskInProgress:
				inProgressTasks++
			case TaskBlocked:
				blockedTasks++
			}
		}
	}

	completionPercentage := 0.0
	if totalTasks > 0 {
		completionPercentage = float64(completedTasks) / float64(totalTasks) * 100
	}

	vis.WriteString(fmt.Sprintf("**Overall Progress:** %.1f%% (%d/%d tasks completed)\n\n",
		completionPercentage, completedTasks, totalTasks))
	vis.WriteString(fmt.Sprintf("**In Progress:** %d tasks\n", inProgressTasks))
	vis.WriteString(fmt.Sprintf("**Blocked:** %d tasks\n\n", blockedTasks))

	// Progress bar
	barLength := 50
	completedBars := int(completionPercentage / 100 * float64(barLength))
	vis.WriteString("```\n[")
	for i := 0; i < barLength; i++ {
		if i < completedBars {
			vis.WriteString("=")
		} else {
			vis.WriteString(" ")
		}
	}
	vis.WriteString("]\n```\n\n")

	// Phase-by-phase breakdown
	vis.WriteString("## Phase Status\n\n")

	for _, phase := range devplan.Phases {
		statusIcon := getStatusIcon(phase.Status)
		vis.WriteString(fmt.Sprintf("### %s Phase %d: %s\n\n", statusIcon, phase.Number, phase.Title))
		vis.WriteString(fmt.Sprintf("**Status:** %s\n\n", phase.Status))

		// Task breakdown
		phaseCompleted := 0
		phaseTotal := len(phase.Tasks)

		vis.WriteString("**Tasks:**\n")
		for _, task := range phase.Tasks {
			taskIcon := getStatusIcon(task.Status)
			vis.WriteString(fmt.Sprintf("- %s %s: %s\n", taskIcon, task.Number, task.Description))
			if task.Status == TaskCompleted {
				phaseCompleted++
			}
		}

		vis.WriteString(fmt.Sprintf("\n**Phase Progress:** %d/%d tasks completed\n\n", phaseCompleted, phaseTotal))
	}

	return vis.String()
}

// getStatusIcon returns an icon for a given status
func getStatusIcon(status interface{}) string {
	switch v := status.(type) {
	case PhaseStatus:
		switch v {
		case PhaseCompleted:
			return "✅"
		case PhaseInProgress:
			return "🔄"
		case PhaseBlocked:
			return "🚫"
		default:
			return "⬜"
		}
	case TaskStatus:
		switch v {
		case TaskCompleted:
			return "✅"
		case TaskInProgress:
			return "🔄"
		case TaskBlocked:
			return "🚫"
		case TaskSkipped:
			return "⏭️"
		default:
			return "⬜"
		}
	}
	return "⬜"
}

// RecordTaskCompletion records a task completion in the changelog
func (changelog *Changelog) RecordTaskCompletion(taskID, taskDescription, phaseTitle string) {
	changelog.AddEntry(
		"task_completed",
		fmt.Sprintf("Completed task: %s", taskDescription),
		"geoffrussy-agent",
		map[string]string{
			"task_id":      taskID,
			"phase":        phaseTitle,
			"completed_at": time.Now().Format(time.RFC3339),
		},
	)
}

// RecordPhaseCompletion records a phase completion in the changelog
func (changelog *Changelog) RecordPhaseCompletion(phaseID, phaseTitle string, tasksCompleted int) {
	changelog.AddEntry(
		"phase_completed",
		fmt.Sprintf("Completed phase: %s (%d tasks)", phaseTitle, tasksCompleted),
		"geoffrussy-agent",
		map[string]string{
			"phase_id":     phaseID,
			"tasks_count":  fmt.Sprintf("%d", tasksCompleted),
			"completed_at": time.Now().Format(time.RFC3339),
		},
	)
}

// RecordDetourAdded records a detour addition in the changelog
func (changelog *Changelog) RecordDetourAdded(detourDescription string, tasksAdded int) {
	changelog.AddEntry(
		"detour_added",
		fmt.Sprintf("Added detour: %s (%d new tasks)", detourDescription, tasksAdded),
		"geoffrussy-agent",
		map[string]string{
			"tasks_added": fmt.Sprintf("%d", tasksAdded),
			"added_at":    time.Now().Format(time.RFC3339),
		},
	)
}
