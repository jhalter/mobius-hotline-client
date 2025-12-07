package internal

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/jhalter/mobius-hotline-client/internal/style"
)

// Messages sent from NewsBundleFormScreen to parent
type NewsBundleCreatedMsg struct {
	Name string
	Path []string
}

type NewsBundleFormCancelledMsg struct{}

// NewsBundleFormScreen is a self-contained BubbleTea model for creating news bundles
type NewsBundleFormScreen struct {
	form          *huh.Form
	path          []string
	width, height int
	model         *Model
}

// NewNewsBundleFormScreen creates a new screen for creating a news bundle
func NewNewsBundleFormScreen(path []string, m *Model) (*NewsBundleFormScreen, tea.Cmd) {
	// Copy path to avoid mutation
	pathCopy := make([]string, len(path))
	copy(pathCopy, path)

	bundleName := ""

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("bundleName").
				Title("Bundle Name").
				Placeholder("Enter bundle name").
				Value(&bundleName).
				CharLimit(255).
				Validate(func(str string) error {
					if len(strings.TrimSpace(str)) == 0 {
						return fmt.Errorf("bundle name cannot be empty")
					}
					return nil
				}),

			huh.NewConfirm().
				Key("confirm").
				Title("Create this bundle?").
				Affirmative("Create").
				Negative("Cancel"),
		),
	).
		WithWidth(60).
		WithShowHelp(true).
		WithShowErrors(true).
		WithTheme(style.FormTheme)

	screen := &NewsBundleFormScreen{
		form:   form,
		path:   pathCopy,
		width:  m.width,
		height: m.height,
		model:  m,
	}

	return screen, form.Init()
}

// Init implements tea.Model
func (s *NewsBundleFormScreen) Init() tea.Cmd {
	return s.form.Init()
}

// Update implements ScreenModel
func (s *NewsBundleFormScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		return s, nil

	case NewsBundleCreatedMsg:
		s.model.handleNewsBundleCreatedMsg(msg)
		return s, nil

	case NewsBundleFormCancelledMsg:
		s.model.PopScreen()
		return s, nil

	case tea.KeyMsg:
		if msg.String() == "esc" {
			return s, func() tea.Msg { return NewsBundleFormCancelledMsg{} }
		}
	}

	// Update the form
	form, cmd := s.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		s.form = f
	}

	// Check if form is complete
	if s.form.State == huh.StateCompleted {
		bundleName := s.form.GetString("bundleName")
		confirmed := s.form.GetBool("confirm")

		if confirmed && strings.TrimSpace(bundleName) != "" {
			pathCopy := make([]string, len(s.path))
			copy(pathCopy, s.path)

			return s, func() tea.Msg {
				return NewsBundleCreatedMsg{
					Name: bundleName,
					Path: pathCopy,
				}
			}
		}
		// Cancelled or invalid - return cancel message
		return s, func() tea.Msg { return NewsBundleFormCancelledMsg{} }
	}

	return s, cmd
}

// View implements tea.Model
func (s *NewsBundleFormScreen) View() string {
	return style.RenderSubscreen(
		s.width,
		s.height,
		"New News Bundle",
		s.form.View(),
	)
}

// SetSize updates the screen dimensions
func (s *NewsBundleFormScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}
