package executor

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Monitor provides live monitoring of task execution
type Monitor struct {
	executor     *Executor
	projectID    string
	width        int
	height       int
	viewport     viewport.Model
	progress     progress.Model
	updates      []TaskUpdate
	currentTask  string
	currentPhase string
	startTime    time.Time
	completion   float64
	phasesDone   int
	totalPhases  int
	tasksDone    int
	totalTasks   int
	tokensIn     int
	tokensOut    int
	requests     int
	err          error
	keys         monitorKeyMap
	help         help.Model
	showHelp     bool
	followOutput bool
	paused       bool
	done         bool
}

type monitorDoneMsg struct{}

type monitorKeyMap struct {
	Quit     key.Binding
	Pause    key.Binding
	Resume   key.Binding
	Skip     key.Binding
	Follow   key.Binding
	Help     key.Binding
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Top      key.Binding
	Bottom   key.Binding
}

func defaultMonitorKeyMap() monitorKeyMap {
	return monitorKeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Pause: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "pause"),
		),
		Resume: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "resume"),
		),
		Skip: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "skip task"),
		),
		Follow: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "toggle follow"),
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

func (k monitorKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Pause, k.Resume, k.Skip, k.Follow, k.Help, k.Quit}
}

func (k monitorKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Pause, k.Resume, k.Skip, k.Follow, k.Help, k.Quit},
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Top, k.Bottom},
	}
}

// NewMonitor creates a new live monitor
func NewMonitor(executor *Executor, projectID string) *Monitor {
	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 0, 1, 1)

	prog := progress.New(progress.WithDefaultGradient())
	helpModel := help.New()

	return &Monitor{
		executor:     executor,
		projectID:    projectID,
		viewport:     vp,
		progress:     prog,
		updates:      []TaskUpdate{},
		startTime:    time.Now(),
		keys:         defaultMonitorKeyMap(),
		help:         helpModel,
		followOutput: true,
	}
}

// monitorMsg is a message containing a task update
type monitorMsg TaskUpdate

// tickMsg is sent every second to update the timer
type tickMsg time.Time

// Init initializes the monitor
func (m *Monitor) Init() tea.Cmd {
	return tea.Batch(
		m.waitForUpdate(),
		m.progress.Init(),
		tickCmd(),
	)
}

// tickCmd returns a command that sends a tick message every second
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update handles messages and updates the monitor
func (m *Monitor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.executor.Close()
			return m, tea.Quit

		case key.Matches(msg, m.keys.Pause):
			if err := m.executor.PauseExecution(); err != nil {
				m.err = err
			}
			m.paused = true
			return m, nil

		case key.Matches(msg, m.keys.Resume):
			if err := m.executor.ResumeExecution(); err != nil {
				m.err = err
			}
			m.paused = false
			return m, nil

		case key.Matches(msg, m.keys.Skip):
			if m.currentTask != "" {
				if err := m.executor.SkipTask(m.currentTask); err != nil {
					m.err = err
				}
			}
			return m, nil

		case key.Matches(msg, m.keys.Follow):
			m.followOutput = !m.followOutput
			if m.followOutput {
				m.viewport.GotoBottom()
			}
			return m, nil

		case key.Matches(msg, m.keys.Help):
			m.showHelp = !m.showHelp
			return m, nil

		case key.Matches(msg, m.keys.Up):
			m.followOutput = false
			m.viewport.LineUp(1)
			return m, nil

		case key.Matches(msg, m.keys.Down):
			m.followOutput = false
			m.viewport.LineDown(1)
			return m, nil

		case key.Matches(msg, m.keys.PageUp):
			m.followOutput = false
			m.viewport.HalfViewUp()
			return m, nil

		case key.Matches(msg, m.keys.PageDown):
			m.followOutput = false
			m.viewport.HalfViewDown()
			return m, nil

		case key.Matches(msg, m.keys.Top):
			m.followOutput = false
			m.viewport.GotoTop()
			return m, nil

		case key.Matches(msg, m.keys.Bottom):
			m.followOutput = false
			m.viewport.GotoBottom()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalculateLayout()

	case monitorMsg:
		update := TaskUpdate(msg)
		m.updates = append(m.updates, update)

		// Update current task and phase
		if update.TaskID != "" {
			m.currentTask = update.TaskID
		}
		if update.PhaseID != "" {
			m.currentPhase = update.PhaseID
		}

		if update.Type == TaskPaused {
			m.paused = true
		}
		if update.Type == TaskResumed {
			m.paused = false
		}

		// Update viewport content
		m.updateViewport()

		// Wait for next update
		return m, m.waitForUpdate()

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		cmds = append(cmds, cmd)

	case tickMsg:
		m.refreshStats()
		if m.totalTasks > 0 && m.tasksDone >= m.totalTasks {
			m.done = true
		}
		cmds = append(cmds, tickCmd())

	case monitorDoneMsg:
		m.done = true
		m.updateViewport()
		return m, nil
	}

	// Update viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the monitor UI
func (m *Monitor) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	var b strings.Builder

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	b.WriteString(headerStyle.Render("Geoffrussy Live Execution"))
	b.WriteString("\n")

	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	statusParts := []string{}
	if m.totalTasks > 0 {
		statusParts = append(statusParts, fmt.Sprintf("tasks %d/%d", m.tasksDone, m.totalTasks))
		statusParts = append(statusParts, fmt.Sprintf("phases %d/%d", m.phasesDone, m.totalPhases))
		statusParts = append(statusParts, fmt.Sprintf("%.0f%%", m.completion))
	}

	if m.tokensIn > 0 || m.tokensOut > 0 {
		statusParts = append(statusParts, fmt.Sprintf("in %d / out %d", m.tokensIn, m.tokensOut))
	}
	if m.paused {
		statusParts = append(statusParts, "paused")
	}
	if m.done {
		statusParts = append(statusParts, "done")
	}
	if len(statusParts) > 0 {
		b.WriteString(statusStyle.Render(strings.Join(statusParts, " • ")))
		b.WriteString("\n")
	}
	b.WriteString(statusStyle.Render(fmt.Sprintf("elapsed %s • follow %t", formatDuration(time.Since(m.startTime)), m.followOutput)))
	b.WriteString("\n\n")

	// Current phase and task
	if m.currentPhase != "" {
		phaseStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86"))
		b.WriteString(phaseStyle.Render(fmt.Sprintf("Phase: %s", m.currentPhase)))
		b.WriteString("\n")
	}

	if m.currentTask != "" {
		taskStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("141"))
		b.WriteString(taskStyle.Render(fmt.Sprintf("Task: %s", m.currentTask)))
		b.WriteString("\n")
	}
	b.WriteString("\n\n")

	// Progress bar
	if m.totalTasks > 0 {
		m.progress.SetPercent(m.completion / 100)
		b.WriteString(m.progress.View())
		b.WriteString("\n\n")
	}

	outputTitle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141")).Render("Execution Log")
	b.WriteString(outputTitle)
	b.WriteString("\n")
	b.WriteString(m.viewport.View())
	b.WriteString("\n\n")

	if m.showHelp {
		b.WriteString(m.help.View(m.keys))
	} else {
		helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		b.WriteString(helpStyle.Render("press ? for keymap"))
	}

	return b.String()
}

// waitForUpdate waits for the next task update
func (m *Monitor) waitForUpdate() tea.Cmd {
	return func() tea.Msg {
		update, ok := <-m.executor.StreamOutput()
		if !ok {
			return monitorDoneMsg{}
		}
		return monitorMsg(update)
	}
}

// updateViewport updates the viewport content with recent updates
func (m *Monitor) updateViewport() {
	var lines []string

	// Show last 30 updates (fewer to keep UI cleaner)
	start := 0
	if len(m.updates) > 30 {
		start = len(m.updates) - 30
	}

	for _, update := range m.updates[start:] {
		line := m.formatUpdate(update)
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")
	m.viewport.SetContent(content)
	if m.followOutput {
		m.viewport.GotoBottom()
	}
}

func (m *Monitor) recalculateLayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	vpWidth := m.width - 4
	if vpWidth < 40 {
		vpWidth = 40
	}
	vpHeight := m.height - 16
	if vpHeight < 8 {
		vpHeight = 8
	}

	m.viewport.Width = vpWidth
	m.viewport.Height = vpHeight
}

// formatUpdate formats a task update for display
func (m *Monitor) formatUpdate(update TaskUpdate) string {
	timestamp := update.Timestamp.Format("15:04:05")

	var icon string
	var color lipgloss.Color

	switch update.Type {
	case TaskStarted:
		icon = "▶"
		color = lipgloss.Color("86")
	case TaskProgress:
		icon = "⋯"
		color = lipgloss.Color("141")
	case TaskCompleted:
		icon = "✓"
		color = lipgloss.Color("82")
	case TaskError:
		icon = "✗"
		color = lipgloss.Color("196")
	case TaskBlocked:
		icon = "⚠"
		color = lipgloss.Color("226")
	case TaskPaused:
		icon = "⏸"
		color = lipgloss.Color("226")
	case TaskResumed:
		icon = "▶"
		color = lipgloss.Color("86")
	case TaskSkipped:
		icon = "⏭"
		color = lipgloss.Color("241")
	default:
		icon = "•"
		color = lipgloss.Color("241")
	}

	style := lipgloss.NewStyle().Foreground(color)
	timestampStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	return fmt.Sprintf("%s %s %s",
		timestampStyle.Render(timestamp),
		style.Render(icon),
		update.Content)
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// Run runs the monitor as a Bubbletea program
func (m *Monitor) Run() error {
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running monitor: %w", err)
	}
	return nil
}

// refreshStats refreshes statistics from the store
func (m *Monitor) refreshStats() {
	stats, err := m.executor.store.CalculateProgress(m.projectID)
	if err != nil {
		return
	}

	m.completion = stats.CompletionPercentage
	m.phasesDone = stats.CompletedPhases
	m.totalPhases = stats.TotalPhases
	m.tasksDone = stats.CompletedTasks
	m.totalTasks = stats.TotalTasks

	tokenStats, err := m.executor.store.GetTokenStats(m.projectID)
	if err != nil {
		return
	}

	m.tokensIn = tokenStats.TotalInput
	m.tokensOut = tokenStats.TotalOutput
}

// RunWithOutput runs the monitor and writes output to the given writer
func (m *Monitor) RunWithOutput(w io.Writer) error {
	p := tea.NewProgram(m, tea.WithOutput(w), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running monitor: %w", err)
	}
	return nil
}
