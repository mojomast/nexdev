package devplan

import (
	"fmt"
	"regexp"
	"strings"
)

// ParsePhaseMarkdown parses markdown content into a Phase struct
func ParsePhaseMarkdown(content string) (*Phase, error) {
	phase := &Phase{}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty content")
	}

	// Regex for Phase Title: "# Phase 1: Database & Models"
	titleRegex := regexp.MustCompile(`^# Phase (\d+): (.+)$`)
	matches := titleRegex.FindStringSubmatch(lines[0])
	if len(matches) == 3 {
		fmt.Sscanf(matches[1], "%d", &phase.Number)
		phase.Title = matches[2]
	} else {
		// Try fallback if line 0 is not the header (maybe empty lines)
		found := false
		for i, line := range lines {
			matches = titleRegex.FindStringSubmatch(line)
			if len(matches) == 3 {
				fmt.Sscanf(matches[1], "%d", &phase.Number)
				phase.Title = matches[2]
				lines = lines[i:] // Shift lines to start from header
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("invalid phase markdown format: missing title")
		}
	}

	currentSection := ""
	currentTaskSection := "" // "acceptance", "notes"
	var currentTask *Task

	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Parse Phase Status (at top level)
		if strings.HasPrefix(line, "**Status:**") && currentSection == "" {
			status := strings.TrimPrefix(line, "**Status:**")
			phase.Status = PhaseStatus(strings.TrimSpace(status))
			continue
		}

		// Parse Sections
		if strings.HasPrefix(line, "## ") {
			currentSection = strings.TrimPrefix(line, "## ")
			currentTask = nil
			currentTaskSection = ""
			continue
		}

		switch currentSection {
		case "Objective":
			if phase.Objective == "" {
				phase.Objective = line
			} else {
				phase.Objective += "\n" + line
			}

		case "Success Criteria":
			if strings.HasPrefix(line, "- ") {
				phase.SuccessCriteria = append(phase.SuccessCriteria, strings.TrimPrefix(line, "- "))
			}

		case "Dependencies":
			if strings.HasPrefix(line, "Depends on phases: ") {
				deps := strings.TrimPrefix(line, "Depends on phases: ")
				if deps != "" {
					phase.Dependencies = strings.Split(deps, ", ")
				}
			}

		case "Tasks":
			// Task Header: "### 1.1: Design database schema"
			if strings.HasPrefix(line, "### ") {
				taskTitle := strings.TrimPrefix(line, "### ")
				parts := strings.SplitN(taskTitle, ": ", 2)
				if len(parts) == 2 {
					task := Task{
						Number:      parts[0],
						Description: parts[1],
						// Defaults
						Status: TaskNotStarted,
					}
					phase.Tasks = append(phase.Tasks, task)
					// Get pointer to the task in the slice
					currentTask = &phase.Tasks[len(phase.Tasks)-1]
					currentTaskSection = ""
				}
				continue
			}

			if currentTask != nil {
				if strings.HasPrefix(line, "**Status:**") {
					status := strings.TrimPrefix(line, "**Status:**")
					currentTask.Status = TaskStatus(strings.TrimSpace(status))
					continue
				}

				if strings.HasPrefix(line, "**Acceptance Criteria:**") {
					currentTaskSection = "acceptance"
					continue
				}

				if strings.HasPrefix(line, "**Implementation Notes:**") {
					currentTaskSection = "notes"
					continue
				}

				if strings.HasPrefix(line, "- ") {
					item := strings.TrimPrefix(line, "- ")
					switch currentTaskSection {
					case "acceptance":
						currentTask.AcceptanceCriteria = append(currentTask.AcceptanceCriteria, item)
					case "notes":
						currentTask.ImplementationNotes = append(currentTask.ImplementationNotes, item)
					}
				}
			}

		case "Estimates":
			if strings.HasPrefix(line, "- **Tokens:**") {
				var tokens int
				fmt.Sscanf(strings.TrimPrefix(line, "- **Tokens:**"), "%d", &tokens)
				phase.EstimatedTokens = tokens
			}
			if strings.HasPrefix(line, "- **Cost:**") {
				var cost float64
				fmt.Sscanf(strings.TrimPrefix(line, "- **Cost:** $"), "%f", &cost)
				phase.EstimatedCost = cost
			}
		}
	}

	return phase, nil
}
