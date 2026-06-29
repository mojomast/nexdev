package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mojomast/nexdev/internal/provider"
)

type ModelItem struct {
	Name        string
	Provider    string
	DisplayName string
	IsFavorite  bool
}

func (i ModelItem) FilterValue() string {
	return i.Name + " " + i.Provider + " " + i.DisplayName
}

func (i ModelItem) Title() string {
	prefix := "   "
	if i.IsFavorite {
		prefix = "⭐ "
	}
	return prefix + i.Name
}

func (i ModelItem) Description() string {
	return fmt.Sprintf("(%s)", strings.Title(i.Provider))
}

type ProviderItem struct {
	Name        string
	DisplayName string
	Status      string
	Configured  bool
}

func (i ProviderItem) FilterValue() string {
	return i.Name
}

func (i ProviderItem) Title() string {
	status := "○"
	if i.Configured {
		status = "●"
	}
	return fmt.Sprintf("%s %s", status, strings.Title(i.Name))
}

func (i ProviderItem) Description() string {
	if i.Configured {
		return i.Status
	}
	return "not configured"
}

type SelectorMode int

const (
	SelectorModeProvider SelectorMode = iota
	SelectorModeModel
)

type ModelSelectorModel struct {
	mode             SelectorMode
	providerList     list.Model
	modelList        list.Model
	help             help.Model
	keys             selectorKeyMap
	showHelp         bool
	width            int
	height           int
	err              error
	quitting         bool
	selected         bool
	selectedModel    string
	selectedProvider string
	providers        []provider.Provider
	models           []provider.Model
	favorites        []string
	onModelSelected  func(provider, model string)
}

type selectorKeyMap struct {
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding
	Back  key.Binding
	Help  key.Binding
	Quit  key.Binding
}

func defaultSelectorKeyMap() selectorKeyMap {
	return selectorKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "backspace"),
			key.WithHelp("esc", "back"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c", "q"),
			key.WithHelp("ctrl+c", "quit"),
		),
	}
}

func (k selectorKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Help, k.Quit}
}

func (k selectorKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter, k.Back},
		{k.Help, k.Quit},
	}
}

func NewModelSelectorModel(providers []provider.Provider, models []provider.Model, favorites []string) ModelSelectorModel {
	providerItems := make([]list.Item, 0, len(providers))
	for _, p := range providers {
		providerItems = append(providerItems, ProviderItem{
			Name:        p.Name(),
			DisplayName: strings.Title(p.Name()),
			Configured:  p.IsAuthenticated(),
			Status:      "configured",
		})
	}

	providerDelegate := list.NewDefaultDelegate()
	providerList := list.New(providerItems, providerDelegate, 60, 15)
	providerList.Title = "Select Provider"
	providerList.SetShowStatusBar(false)
	providerList.SetFilteringEnabled(true)

	modelItems := make([]list.Item, 0, len(models))
	for _, m := range models {
		isFav := false
		for _, fav := range favorites {
			if fav == m.Name {
				isFav = true
				break
			}
		}
		modelItems = append(modelItems, ModelItem{
			Name:        m.Name,
			Provider:    m.Provider,
			DisplayName: m.DisplayName,
			IsFavorite:  isFav,
		})
	}

	sort.Slice(modelItems, func(i, j int) bool {
		a := modelItems[i].(ModelItem)
		b := modelItems[j].(ModelItem)
		if a.IsFavorite != b.IsFavorite {
			return a.IsFavorite
		}
		if a.Provider != b.Provider {
			return a.Provider < b.Provider
		}
		return a.Name < b.Name
	})

	modelDelegate := list.NewDefaultDelegate()
	modelList := list.New(modelItems, modelDelegate, 60, 15)
	modelList.Title = "Select Model"
	modelList.SetShowStatusBar(false)
	modelList.SetFilteringEnabled(true)

	return ModelSelectorModel{
		mode:         SelectorModeProvider,
		providerList: providerList,
		modelList:    modelList,
		help:         help.New(),
		keys:         defaultSelectorKeyMap(),
		providers:    providers,
		models:       models,
		favorites:    favorites,
	}
}

func (m *ModelSelectorModel) SetOnModelSelected(fn func(provider, model string)) {
	m.onModelSelected = fn
}

func (m ModelSelectorModel) Init() tea.Cmd {
	return nil
}

func (m ModelSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, m.keys.Help):
			m.showHelp = !m.showHelp
			return m, nil

		case key.Matches(msg, m.keys.Enter):
			if m.mode == SelectorModeProvider {
				if item, ok := m.providerList.SelectedItem().(ProviderItem); ok {
					m.selectedProvider = item.Name
					m.mode = SelectorModeModel
					m.filterModelsByProvider(item.Name)
				}
			} else {
				if item, ok := m.modelList.SelectedItem().(ModelItem); ok {
					m.selectedModel = item.Name
					m.selected = true
					if m.onModelSelected != nil {
						m.onModelSelected(m.selectedProvider, m.selectedModel)
					}
					return m, tea.Quit
				}
			}
			return m, nil

		case key.Matches(msg, m.keys.Back):
			if m.mode == SelectorModeModel {
				m.mode = SelectorModeProvider
				m.selectedProvider = ""
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.providerList.SetSize(msg.Width-4, msg.Height-8)
		m.modelList.SetSize(msg.Width-4, msg.Height-8)
	}

	var cmd tea.Cmd
	if m.mode == SelectorModeProvider {
		m.providerList, cmd = m.providerList.Update(msg)
	} else {
		m.modelList, cmd = m.modelList.Update(msg)
	}

	return m, cmd
}

func (m *ModelSelectorModel) filterModelsByProvider(providerName string) {
	filtered := make([]list.Item, 0)
	for _, model := range m.models {
		if model.Provider == providerName {
			isFav := false
			for _, fav := range m.favorites {
				if fav == model.Name {
					isFav = true
					break
				}
			}
			filtered = append(filtered, ModelItem{
				Name:        model.Name,
				Provider:    model.Provider,
				DisplayName: model.DisplayName,
				IsFavorite:  isFav,
			})
		}
	}
	m.modelList.SetItems(filtered)
	m.modelList.Title = fmt.Sprintf("Select Model (%s)", strings.Title(providerName))
}

func (m ModelSelectorModel) View() string {
	if m.quitting {
		return "\n👋 Selection cancelled.\n"
	}

	var b strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	b.WriteString(headerStyle.Render("🤖 Configure Interview"))
	b.WriteString("\n\n")

	if m.mode == SelectorModeProvider {
		b.WriteString(m.providerList.View())
	} else {
		b.WriteString(m.modelList.View())
	}

	b.WriteString("\n")

	if m.showHelp {
		b.WriteString(m.help.View(m.keys))
	} else {
		helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		if m.mode == SelectorModeProvider {
			b.WriteString(helpStyle.Render("↑/↓: Navigate | Enter: Select | /: Filter | ?: Help | q: Quit"))
		} else {
			b.WriteString(helpStyle.Render("↑/↓: Navigate | Enter: Select | Esc: Back | /: Filter | ?: Help"))
		}
	}

	return b.String()
}

func (m ModelSelectorModel) GetSelectedProvider() string {
	return m.selectedProvider
}

func (m ModelSelectorModel) GetSelectedModel() string {
	return m.selectedModel
}

func (m ModelSelectorModel) IsSelected() bool {
	return m.selected
}
