package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type InterviewMode int

const (
	ModeSelect InterviewMode = iota
	ModeChat
	ModeSummary
)

type InterviewMessage struct {
	Role      string
	Content   string
	Timestamp string
}

type InterviewModel struct {
	mode         InterviewMode
	messages     []InterviewMessage
	input        textinput.Model
	viewport     viewport.Model
	help         help.Model
	keys         interviewKeyMap
	showHelp     bool
	width        int
	height       int
	err          error
	quitting     bool
	completed    bool
	providerName string
	modelName    string
	projectName  string

	onSendMessage func(string) (string, error)
	onComplete    func() error
}

type interviewKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	Help    key.Binding
	Quit    key.Binding
	Summary key.Binding
	Done    key.Binding
	Back    key.Binding
}

func defaultInterviewKeyMap() interviewKeyMap {
	return interviewKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "scroll up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "scroll down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send message"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c", "esc"),
			key.WithHelp("ctrl+c", "quit"),
		),
		Summary: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "show summary"),
		),
		Done: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "finish interview"),
		),
		Back: key.NewBinding(
			key.WithKeys("ctrl+b"),
			key.WithHelp("ctrl+b", "back to chat"),
		),
	}
}

func (k interviewKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Summary, k.Done, k.Help, k.Quit}
}

func (k interviewKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter},
		{k.Summary, k.Done, k.Help, k.Quit},
	}
}

func NewInterviewModel(projectName, providerName, modelName string) InterviewModel {
	ti := textinput.New()
	ti.Placeholder = "Type your message..."
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 60

	vp := viewport.New(80, 15)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1)

	return InterviewModel{
		mode:         ModeChat,
		messages:     []InterviewMessage{},
		input:        ti,
		viewport:     vp,
		help:         help.New(),
		keys:         defaultInterviewKeyMap(),
		projectName:  projectName,
		providerName: providerName,
		modelName:    modelName,
	}
}

func (m *InterviewModel) SetOnSendMessage(fn func(string) (string, error)) {
	m.onSendMessage = fn
}

func (m *InterviewModel) SetOnComplete(fn func() error) {
	m.onComplete = fn
}

func (m InterviewModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m InterviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		cmds  []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			if m.mode == ModeSummary {
				m.mode = ModeChat
				return m, nil
			}
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

		case key.Matches(msg, m.keys.Summary):
			m.mode = ModeSummary
			return m, nil

		case key.Matches(msg, m.keys.Back):
			if m.mode == ModeSummary {
				m.mode = ModeChat
			}
			return m, nil

		case key.Matches(msg, m.keys.Done):
			m.completed = true
			if m.onComplete != nil {
				if err := m.onComplete(); err != nil {
					m.err = err
				}
			}
			return m, tea.Quit

		case key.Matches(msg, m.keys.Enter):
			if m.input.Value() != "" {
				userMsg := m.input.Value()
				m.messages = append(m.messages, InterviewMessage{
					Role:    "user",
					Content: userMsg,
				})
				m.updateViewport()
				m.input.SetValue("")

				if m.onSendMessage != nil {
					response, err := m.onSendMessage(userMsg)
					if err != nil {
						m.err = err
						return m, nil
					}
					m.messages = append(m.messages, InterviewMessage{
						Role:    "assistant",
						Content: response,
					})
					m.updateViewport()

					if strings.Contains(response, "Interview Complete") ||
						strings.Contains(response, "interview completed") {
						m.completed = true
					}
				}
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalculateLayout()
	}

	m.input, tiCmd = m.input.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, tiCmd, vpCmd)

	return m, tea.Batch(cmds...)
}

func (m InterviewModel) View() string {
	if m.quitting {
		return "\n👋 Interview paused. Run 'geoffrussy interview --resume' to continue.\n"
	}

	if m.err != nil {
		return fmt.Sprintf("\n❌ Error: %v\n", m.err)
	}

	var b strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		MarginBottom(1)

	title := "🎤 Project Interview"
	if m.completed {
		title = "✅ Interview Complete"
	}
	b.WriteString(headerStyle.Render(title))
	b.WriteString("\n")

	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	b.WriteString(infoStyle.Render(fmt.Sprintf("Project: %s | Provider: %s | Model: %s",
		m.projectName, m.providerName, m.modelName)))
	b.WriteString("\n\n")

	if m.mode == ModeSummary {
		b.WriteString(m.renderSummary())
	} else {
		b.WriteString(m.renderChat())
	}

	b.WriteString("\n")

	if m.showHelp {
		b.WriteString(m.help.View(m.keys))
	} else {
		helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		b.WriteString(helpStyle.Render("Press ? for help | ctrl+d to finish | ctrl+s for summary"))
	}

	return b.String()
}

func (m InterviewModel) renderChat() string {
	var b strings.Builder

	b.WriteString(m.viewport.View())
	b.WriteString("\n\n")

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1).
		Width(70)

	promptStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	b.WriteString(promptStyle.Render("👤 You: "))
	b.WriteString(inputStyle.Render(m.input.View()))

	return b.String()
}

func (m InterviewModel) renderSummary() string {
	var b strings.Builder

	summaryStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86")).
		Padding(1, 2).
		Width(70)

	var summaryLines []string
	summaryLines = append(summaryLines, lipgloss.NewStyle().Bold(true).Render("📋 Conversation Summary"))
	summaryLines = append(summaryLines, "")
	summaryLines = append(summaryLines, fmt.Sprintf("Total messages: %d", len(m.messages)))

	userCount := 0
	assistantCount := 0
	for _, msg := range m.messages {
		if msg.Role == "user" {
			userCount++
		} else {
			assistantCount++
		}
	}
	summaryLines = append(summaryLines, fmt.Sprintf("Your messages: %d", userCount))
	summaryLines = append(summaryLines, fmt.Sprintf("AI responses: %d", assistantCount))
	summaryLines = append(summaryLines, "")
	summaryLines = append(summaryLines, "Topics covered:")

	topics := m.extractTopics()
	for _, topic := range topics {
		summaryLines = append(summaryLines, fmt.Sprintf("  • %s", topic))
	}

	summaryLines = append(summaryLines, "")
	summaryLines = append(summaryLines, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Press ctrl+b to return to chat"))

	b.WriteString(summaryStyle.Render(strings.Join(summaryLines, "\n")))

	return b.String()
}

func (m InterviewModel) extractTopics() []string {
	topics := []string{}
	conversation := ""
	for _, msg := range m.messages {
		conversation += msg.Content + " "
	}

	lowerConv := strings.ToLower(conversation)

	topicKeywords := map[string]string{
		"problem":  "Problem Statement",
		"user":     "Target Users",
		"success":  "Success Metrics",
		"value":    "Value Proposition",
		"language": "Technical Stack",
		"api":      "Integrations",
		"database": "Database Design",
		"auth":     "Authentication",
		"mvp":      "MVP Features",
		"timeline": "Timeline",
		"feature":  "Feature Planning",
	}

	for keyword, topic := range topicKeywords {
		if strings.Contains(lowerConv, keyword) {
			topics = append(topics, topic)
		}
	}

	if len(topics) == 0 {
		topics = append(topics, "General discussion")
	}

	return topics
}

func (m *InterviewModel) updateViewport() {
	var lines []string

	maxMessages := 50
	start := 0
	if len(m.messages) > maxMessages {
		start = len(m.messages) - maxMessages
	}

	for i := start; i < len(m.messages); i++ {
		msg := m.messages[i]
		var prefix string
		var content string

		if msg.Role == "user" {
			prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true).Render("👤 You:")
			content = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(msg.Content)
		} else {
			prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true).Render("🤖 AI:")
			content = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(msg.Content)
		}

		lines = append(lines, prefix)
		lines = append(lines, wordWrap(content, m.viewport.Width-4))
		lines = append(lines, "")
	}

	m.viewport.SetContent(strings.Join(lines, "\n"))
	m.viewport.GotoBottom()
}

func (m *InterviewModel) recalculateLayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	vpWidth := m.width - 4
	if vpWidth < 40 {
		vpWidth = 40
	}

	vpHeight := m.height - 12
	if vpHeight < 8 {
		vpHeight = 8
	}

	m.viewport.Width = vpWidth
	m.viewport.Height = vpHeight

	inputWidth := m.width - 12
	if inputWidth < 40 {
		inputWidth = 40
	}
	m.input.Width = inputWidth

	m.updateViewport()
}

func (m *InterviewModel) AddMessage(role, content string) {
	m.messages = append(m.messages, InterviewMessage{
		Role:    role,
		Content: content,
	})
	m.updateViewport()
}

func (m *InterviewModel) SetCompleted(completed bool) {
	m.completed = completed
}

func (m *InterviewModel) IsCompleted() bool {
	return m.completed
}

func wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	words := strings.Fields(text)
	currentLen := 0

	for _, word := range words {
		wordLen := len(word)
		if currentLen+wordLen+1 > width {
			if currentLen > 0 {
				result.WriteString("\n")
				currentLen = 0
			}
			result.WriteString(word)
			currentLen = wordLen
		} else {
			if currentLen > 0 {
				result.WriteString(" ")
				currentLen++
			}
			result.WriteString(word)
			currentLen += wordLen
		}
	}

	return result.String()
}
