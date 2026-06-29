package cli

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

type bannerModel struct {
	lines       []string
	currentLine int
	lineVisible []bool
	done        bool
	startTime   time.Time
}

type tickMsg time.Time

func doTick() tea.Cmd {
	return tea.Tick(time.Millisecond*40, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m bannerModel) Init() tea.Cmd {
	return tea.Batch(
		doTick(),
	)
}

func (m bannerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tickMsg:
		if m.currentLine < len(m.lines) {
			m.lineVisible[m.currentLine] = true
			m.currentLine++
			return m, doTick()
		}
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m bannerModel) View() string {
	var sb strings.Builder
	for i, line := range m.lines {
		if m.lineVisible[i] {
			sb.WriteString(line)
			if i < len(m.lines)-1 {
				sb.WriteString("\r\n")
			}
		}
	}
	if !m.done {
		sb.WriteString("\033[K")
	}
	return sb.String()
}

func bannerLines() []string {
	bannerText := `
  /$$$$$$                       /$$$$$$   /$$$$$$                                                
 /$$__  $$                     /$$__  $$ /$$__  $$                                               
| $$  \__/  /$$$$$$   /$$$$$$ | $$  \__/| $$  \__//$$$$$$  /$$   /$$  /$$$$$$$ /$$$$$$$ /$$   /$$
| $$ /$$$$ /$$__  $$ /$$__  $$| $$$$    | $$$$   /$$__  $$| $$  | $$ /$$_____//$$_____/| $$  | $$
| $$|_  $$| $$$$$$$$| $$  \ $$| $$_/    | $$_/  | $$  \__/| $$  | $$|  $$$$$$|  $$$$$$ | $$  | $$
| $$  \ $$| $$_____/| $$  | $$| $$      | $$    | $$      | $$  | $$ \____  $$\____  $$| $$  | $$
|  $$$$$$/|  $$$$$$$|  $$$$$$/| $$      | $$    | $$      |  $$$$$$/ /$$$$$$$//$$$$$$$/|  $$$$$$$
 \______/  \_______/ \______/ |__/      |__/    |__/       \______/ |_______/|_______/  \____  $$
                                                                                        /$$  | $$
                                                                                       |  $$$$$$/
                                                                                        \______/  
`

	startColor, _ := colorful.Hex("#3CADFF")
	endColor, _ := colorful.Hex("#BA3CFF")

	lines := strings.Split(bannerText, "\n")
	maxWidth := 0
	for _, line := range lines {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}

	var coloredLines []string
	for _, line := range lines {
		var result strings.Builder
		for i, char := range line {
			if char == ' ' {
				result.WriteRune(char)
			} else {
				var t float64
				if maxWidth <= 1 {
					t = 0
				} else {
					t = float64(i) / float64(maxWidth-1)
				}
				gradientColor := startColor.BlendLuv(endColor, t)
				coloredChar := lipgloss.NewStyle().
					Foreground(lipgloss.Color(gradientColor.Hex())).
					Render(string(char))
				result.WriteString(coloredChar)
			}
		}
		coloredLines = append(coloredLines, result.String())
	}

	return coloredLines
}

func Banner() string {
	return strings.Join(bannerLines(), "\n")
}

func BannerAnimated() {
	lines := bannerLines()
	lineVisible := make([]bool, len(lines))

	model := bannerModel{
		lines:       lines,
		currentLine: 0,
		lineVisible: lineVisible,
		done:        false,
		startTime:   time.Now(),
	}

	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h")

	fmt.Print("\r\033[K")

	p := tea.NewProgram(model, tea.WithoutRenderer())
	if _, err := p.Run(); err != nil {
		fmt.Print(Banner())
	}

	fmt.Print("\r\n")
}
