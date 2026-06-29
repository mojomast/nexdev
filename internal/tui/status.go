package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// StatusModel represents the TUI model for project status dashboard
type StatusModel struct {
	projectName    string
	currentStage   string
	currentPhase   string
	phasesTotal    int
	phasesComplete int
	phasesProgress int
	phasesPending  int
	blockers       []string
	totalTokens    int
	totalCost      float64
	err            error
	quitting       bool
	width          int
	height         int
	viewport       viewport.Model
	help           help.Model
	keys           statusKeyMap
	showHelp       bool
}

type statusKeyMap struct {
	Quit     key.Binding
	Help     key.Binding
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Top      key.Binding
	Bottom   key.Binding
}

func defaultStatusKeyMap() statusKeyMap {
	return statusKeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "scroll up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "scroll down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "b"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "space"),
			key.WithHelp("pgdn", "page down"),
		),
		Top: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "bottom"),
		),
	}
}

func (k statusKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Help, k.Quit}
}

func (k statusKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Top, k.Bottom},
		{k.Help, k.Quit},
	}
}

// NewStatusModel creates a new status TUI model
func NewStatusModel() StatusModel {
	vp := viewport.New(80, 18)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 0, 1, 1)

	m := StatusModel{
		blockers: []string{},
		viewport: vp,
		help:     help.New(),
		keys:     defaultStatusKeyMap(),
	}
	m.updateViewportContent()
	return m
}

// Init initializes the status model
func (m StatusModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.showHelp = !m.showHelp
			return m, nil
		case key.Matches(msg, m.keys.Up):
			m.viewport.LineUp(1)
			return m, nil
		case key.Matches(msg, m.keys.Down):
			m.viewport.LineDown(1)
			return m, nil
		case key.Matches(msg, m.keys.PageUp):
			m.viewport.HalfViewUp()
			return m, nil
		case key.Matches(msg, m.keys.PageDown):
			m.viewport.HalfViewDown()
			return m, nil
		case key.Matches(msg, m.keys.Top):
			m.viewport.GotoTop()
			return m, nil
		case key.Matches(msg, m.keys.Bottom):
			m.viewport.GotoBottom()
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalculateLayout()
	}

	return m, nil
}

// View renders the status dashboard UI
func (m StatusModel) View() string {
	if m.quitting {
		return ""
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	var b strings.Builder

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	b.WriteString(headerStyle.Render("Geoffrussy Project Status"))
	b.WriteString("\n")

	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	parts := []string{fmt.Sprintf("stage %s", m.currentStage)}
	if m.currentPhase != "" {
		parts = append(parts, fmt.Sprintf("phase %s", m.currentPhase))
	}
	parts = append(parts, fmt.Sprintf("phases %d/%d complete", m.phasesComplete, m.phasesTotal))
	parts = append(parts, fmt.Sprintf("tokens %d", m.totalTokens))
	parts = append(parts, fmt.Sprintf("cost $%.4f", m.totalCost))
	b.WriteString(statusStyle.Render(strings.Join(parts, " • ")))
	b.WriteString("\n\n")

	if m.phasesTotal > 0 {
		progress := float64(m.phasesComplete) / float64(m.phasesTotal)
		progressBar := renderProgressBar(progress, 50)
		b.WriteString(fmt.Sprintf("%s %.1f%%\n\n", progressBar, progress*100))
	}

	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141")).Render("Details"))
	b.WriteString("\n")
	b.WriteString(m.viewport.View())
	b.WriteString("\n\n")

	if m.showHelp {
		b.WriteString(m.help.View(m.keys))
	} else {
		b.WriteString(statusStyle.Render("press ? for keymap"))
	}

	return b.String()
}

// SetProjectInfo sets the project information
func (m *StatusModel) SetProjectInfo(name, stage, phase string) {
	m.projectName = name
	m.currentStage = stage
	m.currentPhase = phase
	m.updateViewportContent()
}

// SetPhaseProgress sets the phase progress information
func (m *StatusModel) SetPhaseProgress(total, complete, progress, pending int) {
	m.phasesTotal = total
	m.phasesComplete = complete
	m.phasesProgress = progress
	m.phasesPending = pending
	m.updateViewportContent()
}

// SetBlockers sets the list of active blockers
func (m *StatusModel) SetBlockers(blockers []string) {
	m.blockers = blockers
	m.updateViewportContent()
}

// SetTokenUsage sets the token usage information
func (m *StatusModel) SetTokenUsage(tokens int, cost float64) {
	m.totalTokens = tokens
	m.totalCost = cost
	m.updateViewportContent()
}

func (m *StatusModel) updateViewportContent() {
	lines := []string{}
	lines = append(lines, fmt.Sprintf("Project: %s", m.projectName))
	lines = append(lines, fmt.Sprintf("Stage: %s", m.currentStage))
	if m.currentPhase != "" {
		lines = append(lines, fmt.Sprintf("Current Phase: %s", m.currentPhase))
	}
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Completed: %d", m.phasesComplete))
	lines = append(lines, fmt.Sprintf("In Progress: %d", m.phasesProgress))
	lines = append(lines, fmt.Sprintf("Pending: %d", m.phasesPending))
	lines = append(lines, fmt.Sprintf("Total: %d", m.phasesTotal))
	lines = append(lines, "")

	if len(m.blockers) == 0 {
		lines = append(lines, "Active Blockers: none")
	} else {
		lines = append(lines, "Active Blockers:")
		for i, blocker := range m.blockers {
			lines = append(lines, fmt.Sprintf("  %d. %s", i+1, blocker))
		}
	}

	m.viewport.SetContent(strings.Join(lines, "\n"))
}

func (m *StatusModel) recalculateLayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	vpWidth := m.width - 4
	if vpWidth < 40 {
		vpWidth = 40
	}
	vpHeight := m.height - 13
	if vpHeight < 8 {
		vpHeight = 8
	}

	m.viewport.Width = vpWidth
	m.viewport.Height = vpHeight
}
