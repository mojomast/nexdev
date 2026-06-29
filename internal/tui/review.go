package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mojomast/nexdev/internal/reviewer"
)

// ReviewModel represents the TUI model for phase review
type ReviewModel struct {
	report        *reviewer.ReviewReport
	selectedPhase int
	selectedIssue int
	mode          string // "review" or "improvements"
	err           error
	quitting      bool
}

// NewReviewModel creates a new review TUI model
func NewReviewModel(report *reviewer.ReviewReport) ReviewModel {
	return ReviewModel{
		report:        report,
		selectedPhase: 0,
		selectedIssue: 0,
		mode:          "review",
	}
}

// Init initializes the review model
func (m ReviewModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m ReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.mode == "review" {
				if m.selectedPhase > 0 {
					m.selectedPhase--
				}
			} else {
				if m.selectedIssue > 0 {
					m.selectedIssue--
				}
			}

		case "down", "j":
			if m.mode == "review" {
				if m.selectedPhase < len(m.report.PhaseReviews)-1 {
					m.selectedPhase++
				}
			} else {
				if m.selectedPhase < len(m.report.PhaseReviews) {
					issues := m.report.PhaseReviews[m.selectedPhase].Issues
					if m.selectedIssue < len(issues)-1 {
						m.selectedIssue++
					}
				}
			}

		case "enter":
			if m.mode == "review" {
				// Switch to improvements mode for selected phase
				m.mode = "improvements"
				m.selectedIssue = 0
			} else {
				// Return to phase list
				m.mode = "review"
			}

		case "esc":
			if m.mode == "improvements" {
				// Go back to review mode
				m.mode = "review"
			} else {
				m.quitting = true
				return m, tea.Quit
			}

		}
	}

	return m, nil
}

// View renders the review UI
func (m ReviewModel) View() string {
	if m.quitting {
		return "Review complete.\n"
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		MarginBottom(1)

	b.WriteString(headerStyle.Render("🔍 Phase Review"))
	b.WriteString("\n\n")

	if m.mode == "review" {
		b.WriteString(m.renderReviewMode())
	} else {
		b.WriteString(m.renderImprovementsMode())
	}

	return b.String()
}

func (m ReviewModel) renderReviewMode() string {
	var b strings.Builder

	// Summary
	summaryStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	b.WriteString(summaryStyle.Render("Summary"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Total Phases: %d\n", m.report.TotalPhases))
	b.WriteString(fmt.Sprintf("Issues Found: %d\n", m.report.IssuesFound))
	b.WriteString(fmt.Sprintf("  Critical: %d | Warning: %d | Info: %d\n\n",
		m.report.SeverityBreakdown[reviewer.SeverityCritical],
		m.report.SeverityBreakdown[reviewer.SeverityWarning],
		m.report.SeverityBreakdown[reviewer.SeverityInfo]))

	// Phase list
	phaseStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1).
		Width(80)

	var phaseList strings.Builder
	for i, phaseReview := range m.report.PhaseReviews {
		cursor := "  "
		if i == m.selectedPhase {
			cursor = "▶ "
		}

		statusIcon := "✅"
		if phaseReview.Status == reviewer.ReviewFailed {
			statusIcon = "❌"
		} else if phaseReview.Status == reviewer.ReviewWarning {
			statusIcon = "⚠️"
		}

		line := fmt.Sprintf("%s%s Phase %s - %d issues",
			cursor, statusIcon, phaseReview.PhaseID, len(phaseReview.Issues))

		if i == m.selectedPhase {
			selectedStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("226"))
			phaseList.WriteString(selectedStyle.Render(line))
		} else {
			phaseList.WriteString(line)
		}
		phaseList.WriteString("\n")
	}

	b.WriteString(phaseStyle.Render(phaseList.String()))
	b.WriteString("\n\n")

	// Help
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	b.WriteString(helpStyle.Render("↑/↓: Navigate | Enter: View Issues | Q: Quit"))

	return b.String()
}

func (m ReviewModel) renderImprovementsMode() string {
	var b strings.Builder

	if m.selectedPhase >= len(m.report.PhaseReviews) {
		return "Invalid phase selected"
	}

	phaseReview := m.report.PhaseReviews[m.selectedPhase]

	// Phase header
	phaseStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	b.WriteString(phaseStyle.Render(fmt.Sprintf("Phase %s - Issues", phaseReview.PhaseID)))
	b.WriteString("\n\n")

	// Issues list
	issueStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1).
		Width(80)

	var issueList strings.Builder
	for i, issue := range phaseReview.Issues {
		cursor := "  "
		if i == m.selectedIssue {
			cursor = "▶ "
		}

		severityColor := "241"
		switch issue.Severity {
		case reviewer.SeverityCritical:
			severityColor = "196"
		case reviewer.SeverityWarning:
			severityColor = "226"
		case reviewer.SeverityInfo:
			severityColor = "86"
		}

		severityStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(severityColor))

		line := fmt.Sprintf("%s%s %s: %s",
			cursor,
			severityStyle.Render(string(issue.Severity)),
			issue.Type, issue.Description)

		if i == m.selectedIssue {
			selectedStyle := lipgloss.NewStyle().
				Bold(true)
			issueList.WriteString(selectedStyle.Render(line))
		} else {
			issueList.WriteString(line)
		}
		issueList.WriteString("\n")

		// Show suggestion for selected issue
		if i == m.selectedIssue {
			suggestionStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("141")).
				Italic(true)
			issueList.WriteString(suggestionStyle.Render(fmt.Sprintf("  💡 %s", issue.Suggestion)))
			issueList.WriteString("\n")
		}
	}

	b.WriteString(issueStyle.Render(issueList.String()))
	b.WriteString("\n\n")

	// Help
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	b.WriteString(helpStyle.Render("↑/↓: Navigate | Enter/Esc: Back"))

	return b.String()
}
