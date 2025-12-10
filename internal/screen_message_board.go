package internal

import (
	"fmt"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jhalter/mobius-hotline-client/internal/style"
	"github.com/jhalter/mobius/hotline"
	"github.com/muesli/reflow/wordwrap"
)

// Messages sent from MessageBoardScreen to parent

// MessageBoardCancelledMsg signals user wants to close the message board
type MessageBoardCancelledMsg struct{}

// MessageBoardPostRequestedMsg signals user wants to post a new message
type MessageBoardPostRequestedMsg struct{}

// messageBoardScreenKeyMap defines key bindings for the message board screen
type messageBoardScreenKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Post     key.Binding
	Back     key.Binding
}

func (k messageBoardScreenKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.PageUp, k.PageDown, k.Post, k.Back}
}

func (k messageBoardScreenKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Post, k.Back},
	}
}

// MessageBoardScreen is a self-contained BubbleTea model for viewing the message board
type MessageBoardScreen struct {
	viewport      viewport.Model
	width, height int
	model         *Model
	help          help.Model
	keys          messageBoardScreenKeyMap
	content       string
}

// NewMessageBoardScreen creates a new message board screen
func NewMessageBoardScreen(content string, m *Model) *MessageBoardScreen {
	keys := messageBoardScreenKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdn", "page down"),
		),
		Post: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("^P", "post"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
	}

	vp := viewport.New(viewport.WithWidth(m.width-10), viewport.WithHeight(m.height-10))
	vp.SetContent(content)

	return &MessageBoardScreen{
		viewport: vp,
		width:    m.width,
		height:   m.height - 10,
		model:    m,
		help:     help.New(),
		keys:     keys,
		content:  content,
	}
}

// Init implements tea.Model
func (s *MessageBoardScreen) Init() tea.Cmd {
	return nil
}

// Update implements ScreenModel
func (s *MessageBoardScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.SetSize(msg.Width, msg.Height)
		return s, nil

	case MessageBoardCancelledMsg:
		s.model.PopScreen()
		return s, nil

	case MessageBoardPostRequestedMsg:
		return s, s.model.handleMessageBoardPostRequestedMsg()

	case tea.KeyPressMsg:
		return s.handleKeys(msg)
	}

	// Delegate to viewport for scrolling
	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

// View implements ScreenModel
func (s *MessageBoardScreen) View() tea.View {
	content := lipgloss.Place(
		s.width,
		s.height-10,
		lipgloss.Center,
		lipgloss.Center,
		style.SubScreenStyle.Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				style.SubTitleStyle.Render("Message Board"),
				wordwrap.String(s.viewport.View(), 58),
				" ",
				lipgloss.JoinHorizontal(
					lipgloss.Left,
					s.help.View(s.keys),
					"  ", // Spacer
					fmt.Sprintf("%3.f%%", s.viewport.ScrollPercent()*100),
				),
			),
		),
		lipgloss.WithWhitespaceChars("~"),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(style.Subtle)),
	)
	return tea.NewView(content)
}

// SetSize updates dimensions
func (s *MessageBoardScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.viewport.SetWidth(width - 10)
	s.viewport.SetHeight(height - 10)
}

// handleKeys handles keyboard input
func (s *MessageBoardScreen) handleKeys(msg tea.KeyPressMsg) (ScreenModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return s, func() tea.Msg { return MessageBoardCancelledMsg{} }

	case "ctrl+p":
		return s, func() tea.Msg { return MessageBoardPostRequestedMsg{} }
	}

	// Pass all other keys to viewport for scrolling
	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

// SetUserAccess updates key bindings based on user permissions
func (s *MessageBoardScreen) SetUserAccess(access hotline.AccessBitmap) {
	s.keys.Post.SetEnabled(access.IsSet(hotline.AccessNewsPostArt))
}
