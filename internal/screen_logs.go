package internal

import (
	"fmt"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jhalter/mobius-hotline-client/internal/style"
)

// Messages sent from LogsScreen to parent

// LogsCancelledMsg signals user wants to close logs
type LogsCancelledMsg struct{}

// logsScreenKeyMap defines key bindings for the logs screen help display
type logsScreenKeyMap struct {
	Up   key.Binding
	Down key.Binding
	Back key.Binding
}

func (k logsScreenKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Back}
}

func (k logsScreenKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Back}}
}

// LogsScreen is a self-contained BubbleTea model for viewing debug logs
type LogsScreen struct {
	viewport      viewport.Model
	width, height int
	model         *Model
	help          help.Model
	keys          logsScreenKeyMap
	debugBuffer   *DebugBuffer
}

// NewLogsScreen creates a new logs screen
func NewLogsScreen(debugBuffer *DebugBuffer, m *Model) *LogsScreen {
	keys := logsScreenKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
	}

	vp := viewport.New(viewport.WithWidth(m.width-10), viewport.WithHeight(m.height-10))
	vp.SetContent(debugBuffer.String())
	vp.GotoBottom()

	return &LogsScreen{
		viewport:    vp,
		width:       m.width,
		height:      m.height,
		model:       m,
		help:        style.NewHelp(),
		keys:        keys,
		debugBuffer: debugBuffer,
	}
}

// Init implements tea.Model
func (s *LogsScreen) Init() tea.Cmd {
	return nil
}

// Update implements ScreenModel
func (s *LogsScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.SetSize(msg.Width, msg.Height)
		return s, nil

	case LogsCancelledMsg:
		s.model.PopScreen()
		return s, nil

	case tea.KeyPressMsg:
		return s.handleKeys(msg)
	}

	// Delegate to viewport for scrolling
	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

// View implements ScreenModel
func (s *LogsScreen) View() tea.View {
	content := style.RenderSubscreen(s.width, s.height, "Logs",
		lipgloss.JoinVertical(
			lipgloss.Left,
			s.viewport.View(),
			" ",
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				s.help.View(s.keys),
				"  ",
				fmt.Sprintf("%3.f%%", s.viewport.ScrollPercent()*100),
			),
		),
	)
	return tea.NewView(content)
}

// SetSize updates dimensions
func (s *LogsScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.viewport.SetWidth(width - 10)
	s.viewport.SetHeight(height - 10)
}

// handleKeys handles keyboard input
func (s *LogsScreen) handleKeys(msg tea.KeyPressMsg) (ScreenModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return s, func() tea.Msg { return LogsCancelledMsg{} }
	}

	// Pass all other keys to viewport for scrolling
	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

// RefreshContent updates the viewport content from the debug buffer
func (s *LogsScreen) RefreshContent() {
	s.viewport.SetContent(s.debugBuffer.String())
	s.viewport.GotoBottom()
}
