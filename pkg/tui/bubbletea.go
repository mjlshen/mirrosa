package tui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mjlshen/mirrosa/pkg/mirrosa"
)

type Model struct {
	client      mirrosa.Client
	components  list.Model
	viewport    viewport.Model
	initialized bool
}

/* STYLING */
var (
	columnStyle = lipgloss.NewStyle().
			Border(lipgloss.HiddenBorder())
	focusedStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#89cff0")) // Baby Blue
)

func InitModel() *Model {
	return &Model{}
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		columnStyle.Width(3 * msg.Width / 4)
		columnStyle.Height(msg.Height / 2)
		focusedStyle.Width(msg.Width / 4)
		focusedStyle.Height(msg.Height / 2)
		m.initLists(focusedStyle.GetWidth(), focusedStyle.GetHeight())
		m.initViewport(columnStyle.GetWidth(), columnStyle.GetHeight())
		m.initialized = true
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}

	newListModel, cmd := m.components.Update(msg)
	m.components = newListModel
	return m, cmd
}

func (m *Model) View() string {
	if m.initialized {
		c, ok := m.components.SelectedItem().(mirrosa.Component)
		if ok {
			m.viewport.SetContent(lipgloss.NewStyle().Padding(1, 2).Width(m.viewport.Width).Render(c.Description()))
		}
		return lipgloss.JoinHorizontal(
			lipgloss.Left,
			focusedStyle.Render(m.components.View()),
			columnStyle.Render(m.viewport.View()),
		)
	} else {
		return "loading..."
	}
}

func (m *Model) initLists(width, height int) {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false

	defaultList := list.New([]list.Item{}, delegate, width, height)
	defaultList.SetShowHelp(true)
	defaultList.SetShowStatusBar(false)

	m.components = defaultList
	m.components.Title = "ROSA AWS Component"
	m.components.SetItems([]list.Item{
		mirrosa.Vpc{},
		mirrosa.DhcpOptions{},
		mirrosa.SecurityGroup{},
		mirrosa.VpcEndpointService{},
		mirrosa.PublicHostedZone{},
		mirrosa.PrivateHostedZone{},
		mirrosa.Instances{},
	})
}

func (m *Model) initViewport(width, height int) {
	m.viewport = viewport.New(width, height)
}
