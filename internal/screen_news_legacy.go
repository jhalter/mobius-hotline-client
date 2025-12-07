package internal

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/jhalter/mobius-hotline-client/internal/style"
)

// Messages sent from LegacyNewsPostScreen to parent
type LegacyNewsPostedMsg struct {
	Content string
}

type LegacyNewsPostCancelledMsg struct{}

// LegacyNewsPostScreen is a self-contained BubbleTea model for legacy messageboard-style news posts
type LegacyNewsPostScreen struct {
	form          *huh.Form
	width, height int
	model         *Model
}

// NewLegacyNewsPostScreen creates a new screen for posting legacy-style news
func NewLegacyNewsPostScreen(m *Model) (*LegacyNewsPostScreen, tea.Cmd) {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Key("newsPost").
				CharLimit(1000),

			huh.NewConfirm().
				Key("done").
				Value(func() *bool { t := true; return &t }()).
				Affirmative("Post").
				Negative("Cancel"),
		),
	).
		WithWidth(45).
		WithShowHelp(false).
		WithShowErrors(false).
		WithTheme(style.FormTheme)

	screen := &LegacyNewsPostScreen{
		form:   form,
		width:  m.width,
		height: m.height,
		model:  m,
	}

	return screen, form.Init()
}

// Init implements tea.Model
func (s *LegacyNewsPostScreen) Init() tea.Cmd {
	return s.form.Init()
}

// Update implements ScreenModel
func (s *LegacyNewsPostScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		return s, nil

	case LegacyNewsPostedMsg:
		s.model.handleLegacyNewsPostedMsg(msg)
		return s, nil

	case LegacyNewsPostCancelledMsg:
		s.model.PopScreen()
		return s, nil

	case tea.KeyMsg:
		if msg.String() == "esc" {
			return s, func() tea.Msg { return LegacyNewsPostCancelledMsg{} }
		}
	}

	// Update the form
	form, cmd := s.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		s.form = f
	}

	// Check if form is complete
	if s.form.State == huh.StateCompleted {
		newsText := s.form.GetString("newsPost")
		confirmed := s.form.GetBool("done")

		if newsText != "" && confirmed {
			return s, func() tea.Msg {
				return LegacyNewsPostedMsg{
					Content: newsText,
				}
			}
		}
		// Cancelled or empty - return cancel message
		return s, func() tea.Msg { return LegacyNewsPostCancelledMsg{} }
	}

	return s, cmd
}

// View implements tea.Model
func (s *LegacyNewsPostScreen) View() string {
	return style.RenderSubscreen(
		s.width,
		s.height,
		"New Messageboard Post",
		s.form.View(),
	)
}

// SetSize updates the screen dimensions
func (s *LegacyNewsPostScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}
