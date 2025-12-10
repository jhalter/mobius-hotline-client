package internal

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jhalter/mobius-hotline-client/internal/style"
)

// Messages sent from QuickMenuScreen to parent
type QuickMenuJoinServerMsg struct{}
type QuickMenuBookmarksMsg struct{}
type QuickMenuTrackerMsg struct{}
type QuickMenuCancelledMsg struct{}

// QuickMenuScreen is a compact menu for quick access to connection options
type QuickMenuScreen struct {
	width, height int
	selectedIndex int // For arrow key navigation (0=Join, 1=Bookmarks, 2=Tracker)
	model         *Model
}

// NewQuickMenuScreen creates a new quick menu screen
func NewQuickMenuScreen(m *Model) *QuickMenuScreen {
	return &QuickMenuScreen{
		width:         m.width,
		height:        m.height,
		selectedIndex: 0,
		model:         m,
	}
}

// Init implements tea.Model
func (s *QuickMenuScreen) Init() tea.Cmd {
	return nil
}

// Update implements ScreenModel
func (s *QuickMenuScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.SetSize(msg.Width, msg.Height)
		return s, nil

	case QuickMenuJoinServerMsg:
		s.model.PopScreen()
		return s, s.model.handleHomeJoinServerMsg()

	case QuickMenuBookmarksMsg:
		s.model.PopScreen()
		s.model.handleHomeBookmarksMsg()
		return s, nil

	case QuickMenuTrackerMsg:
		s.model.PopScreen()
		return s, s.model.handleHomeTrackerMsg()

	case QuickMenuCancelledMsg:
		s.model.PopScreen()
		return s, nil

	case tea.KeyPressMsg:
		return s.handleKeys(msg)
	}

	return s, nil
}

// View implements ScreenModel
func (s *QuickMenuScreen) View() tea.View {
	// Build menu items
	items := []string{
		s.renderItem(0, "j", "Join Server"),
		s.renderItem(1, "b", "Bookmarks"),
		s.renderItem(2, "t", "Browse Tracker"),
	}

	menuContent := lipgloss.JoinVertical(lipgloss.Left, items...)

	// Compact dialog box
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(style.CurrentTheme.DialogBorder).
		Padding(1, 2)

	content := lipgloss.Place(
		s.width,
		s.height,
		lipgloss.Center,
		lipgloss.Center,
		dialogStyle.Render(menuContent),
		lipgloss.WithWhitespaceChars("~"),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(style.Subtle)),
	)
	return tea.NewView(content)
}

// SetSize updates the screen dimensions
func (s *QuickMenuScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// handleKeys handles key input for the quick menu
func (s *QuickMenuScreen) handleKeys(msg tea.KeyPressMsg) (ScreenModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return s, func() tea.Msg { return QuickMenuCancelledMsg{} }
	case "j":
		return s, func() tea.Msg { return QuickMenuJoinServerMsg{} }
	case "b":
		return s, func() tea.Msg { return QuickMenuBookmarksMsg{} }
	case "t":
		return s, func() tea.Msg { return QuickMenuTrackerMsg{} }
	case "up", "k":
		s.selectedIndex = (s.selectedIndex - 1 + 3) % 3
		return s, nil
	case "down":
		s.selectedIndex = (s.selectedIndex + 1) % 3
		return s, nil
	case "enter":
		return s.selectCurrent()
	}
	return s, nil
}

// selectCurrent triggers the action for the currently selected item
func (s *QuickMenuScreen) selectCurrent() (ScreenModel, tea.Cmd) {
	switch s.selectedIndex {
	case 0:
		return s, func() tea.Msg { return QuickMenuJoinServerMsg{} }
	case 1:
		return s, func() tea.Msg { return QuickMenuBookmarksMsg{} }
	case 2:
		return s, func() tea.Msg { return QuickMenuTrackerMsg{} }
	}
	return s, nil
}

// renderItem renders a menu item with hotkey, optionally highlighted
func (s *QuickMenuScreen) renderItem(index int, hotkey, label string) string {
	hotkeyPart := style.HotkeyStyle.Render("(" + hotkey + ")")
	text := fmt.Sprintf("%s %s", hotkeyPart, label)

	if index == s.selectedIndex {
		// Highlight selected item
		return lipgloss.NewStyle().
			Background(style.CurrentTheme.Accent).
			Foreground(style.CurrentTheme.Admin).
			Render(text)
	}
	return text
}
