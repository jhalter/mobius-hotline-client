package internal

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jhalter/mobius-hotline-client/internal/style"
)

// LoadingCancelledMsg is sent when the user cancels the loading screen
type LoadingCancelledMsg struct{}

// LoadingScreen displays a modal with an animated spinner
type LoadingScreen struct {
	spinner       spinner.Model
	message       string
	width, height int
	model         *Model
}

// NewLoadingScreen creates a new loading screen instance
func NewLoadingScreen(message string, m *Model) (*LoadingScreen, tea.Cmd) {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(style.CurrentTheme.DialogBorder)

	screen := &LoadingScreen{
		spinner: s,
		message: message,
		width:   m.width,
		height:  m.height,
		model:   m,
	}

	return screen, screen.spinner.Tick
}

// Init returns initial commands
func (s *LoadingScreen) Init() tea.Cmd {
	return s.spinner.Tick
}

// Update handles messages and returns updated screen + commands
func (s *LoadingScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.SetSize(msg.Width, msg.Height)
		return s, nil

	case tea.KeyPressMsg:
		if msg.String() == "esc" {
			return s, func() tea.Msg { return LoadingCancelledMsg{} }
		}
		return s, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		return s, cmd
	}

	return s, nil
}

// View renders the loading screen
func (s *LoadingScreen) View() string {
	title := lipgloss.NewStyle().Align(lipgloss.Center).Render(style.Rainbow(lipgloss.NewStyle(), "Loading", style.Blends))

	content := lipgloss.NewStyle().
		Padding(1).
		Width(50).
		Align(lipgloss.Center).
		Render(s.spinner.View() + " " + s.message)

	return lipgloss.Place(s.width, s.height,
		lipgloss.Center, lipgloss.Center,
		style.DialogBoxStyle.Render(lipgloss.JoinVertical(
			lipgloss.Center,
			title,
			content,
		)),
		lipgloss.WithWhitespaceChars("☃︎"),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(style.Subtle)),
	)
}

// SetSize updates dimensions
func (s *LoadingScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}
