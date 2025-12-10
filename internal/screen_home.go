package internal

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jhalter/mobius-hotline-client/internal/style"
)

// Messages sent from HomeScreen to parent
type HomeJoinServerMsg struct{}

type HomeBookmarksMsg struct{}

type HomeTrackerMsg struct{}

type HomeSettingsMsg struct{}

type HomeQuitMsg struct{}

type HomeRefreshBannerMsg struct{}

// HomeScreen is a self-contained BubbleTea model for the home screen
type HomeScreen struct {
	width, height int
	welcomeBanner string
	model         *Model
}

// NewHomeScreen creates a new home screen
func NewHomeScreen(m *Model) *HomeScreen {
	return &HomeScreen{
		width:         m.width,
		height:        m.height,
		welcomeBanner: m.welcomeBanner,
		model:         m,
	}
}

// Init implements tea.Model
func (s *HomeScreen) Init() tea.Cmd {
	return nil
}

// Update implements ScreenModel
func (s *HomeScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.SetSize(msg.Width, msg.Height)
		return s, nil

	case HomeJoinServerMsg:
		return s, s.model.handleHomeJoinServerMsg()
	case HomeBookmarksMsg:
		s.model.handleHomeBookmarksMsg()
		return s, nil
	case HomeTrackerMsg:
		return s, s.model.handleHomeTrackerMsg()
	case HomeSettingsMsg:
		return s, s.model.handleHomeSettingsMsg()
	case HomeQuitMsg:
		return s, tea.Quit
	case HomeRefreshBannerMsg:
		s.welcomeBanner = randomBanner()
		s.model.welcomeBanner = s.welcomeBanner
		return s, nil

	case tea.KeyPressMsg:
		return s.handleKeys(msg)
	}

	return s, nil
}

// View implements ScreenModel
func (s *HomeScreen) View() tea.View {
	content := lipgloss.Place(
		s.width,
		s.height,
		lipgloss.Center,
		lipgloss.Center,
		lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(style.ColorHotlineRed).
			Padding(1, 3).
			Render(
				lipgloss.NewStyle().
					Render(
						lipgloss.JoinVertical(
							lipgloss.Left,
							lipgloss.NewStyle().
								Foreground(style.ColorHotlineRed).
								Render(s.welcomeBanner),
							lipgloss.NewStyle().
								Render(strings.Join(
									[]string{
										fmt.Sprintf("%s Join Server", style.HotkeyStyle.Render("(j)")),
										fmt.Sprintf("%s Bookmarks", style.HotkeyStyle.Render("(b)")),
										fmt.Sprintf("%s Browse Tracker", style.HotkeyStyle.Render("(t)")),
										fmt.Sprintf("%s Settings", style.HotkeyStyle.Render("(s)")),
										fmt.Sprintf("%s Quit", style.HotkeyStyle.Render("(q)")),
									},
									"\n",
								)),
						),
					),
			),
		lipgloss.WithWhitespaceChars("⌘"),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(style.Subtle)),
	)
	t := tea.NewView(content)
	return t
}

// SetSize updates the screen dimensions
func (s *HomeScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// handleKeys handles key input for the home screen
func (s *HomeScreen) handleKeys(msg tea.KeyPressMsg) (ScreenModel, tea.Cmd) {
	switch msg.String() {
	case "j":
		return s, func() tea.Msg { return HomeJoinServerMsg{} }
	case "b":
		return s, func() tea.Msg { return HomeBookmarksMsg{} }
	case "t":
		return s, func() tea.Msg { return HomeTrackerMsg{} }
	case "s":
		return s, func() tea.Msg { return HomeSettingsMsg{} }
	case "ctrl+r":
		return s, func() tea.Msg { return HomeRefreshBannerMsg{} }
	case "q":
		return s, func() tea.Msg { return HomeQuitMsg{} }
	}
	return s, nil
}
