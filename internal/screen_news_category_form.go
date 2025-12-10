package internal

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/jhalter/mobius-hotline-client/internal/style"
)

// Messages sent from NewsCategoryFormScreen to parent
type NewsCategoryCreatedMsg struct {
	Name string
	Path []string
}

type NewsCategoryFormCancelledMsg struct{}

// NewsCategoryFormScreen is a self-contained BubbleTea model for creating news categories
type NewsCategoryFormScreen struct {
	form          *huh.Form
	path          []string
	width, height int
	model         *Model
}

// NewNewsCategoryFormScreen creates a new screen for creating a news category
func NewNewsCategoryFormScreen(path []string, m *Model) (*NewsCategoryFormScreen, tea.Cmd) {
	// Copy path to avoid mutation
	pathCopy := make([]string, len(path))
	copy(pathCopy, path)

	categoryName := ""

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("categoryName").
				Title("Category Name").
				Placeholder("Enter category name").
				Value(&categoryName).
				CharLimit(255).
				Validate(func(str string) error {
					if len(strings.TrimSpace(str)) == 0 {
						return fmt.Errorf("category name cannot be empty")
					}
					return nil
				}),

			huh.NewConfirm().
				Key("confirm").
				Title("Create this category?").
				Affirmative("Create").
				Negative("Cancel"),
		),
	).
		WithWidth(60).
		WithShowHelp(true).
		WithShowErrors(true).
		WithTheme(style.FormTheme)

	screen := &NewsCategoryFormScreen{
		form:   form,
		path:   pathCopy,
		width:  m.width,
		height: m.height,
		model:  m,
	}

	cmd := form.Init()
	return screen, cmd
}

// Init implements tea.Model
func (s *NewsCategoryFormScreen) Init() tea.Cmd {
	cmd := s.form.Init()
	return cmd
}

// Update implements ScreenModel
func (s *NewsCategoryFormScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		return s, nil

	case NewsCategoryCreatedMsg:
		s.model.handleNewsCategoryCreatedMsg(msg)
		return s, nil

	case NewsCategoryFormCancelledMsg:
		s.model.PopScreen()
		return s, nil

	case tea.KeyPressMsg:
		if msg.String() == "esc" {
			return s, func() tea.Msg { return NewsCategoryFormCancelledMsg{} }
		}
	}

	// Update the form
	form, cmd := s.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		s.form = f
	}

	// Check if form is complete
	if s.form.State == huh.StateCompleted {
		categoryName := s.form.GetString("categoryName")
		confirmed := s.form.GetBool("confirm")

		if confirmed && strings.TrimSpace(categoryName) != "" {
			pathCopy := make([]string, len(s.path))
			copy(pathCopy, s.path)

			return s, func() tea.Msg {
				return NewsCategoryCreatedMsg{
					Name: categoryName,
					Path: pathCopy,
				}
			}
		}
		// Cancelled or invalid - return cancel message
		return s, func() tea.Msg { return NewsCategoryFormCancelledMsg{} }
	}

	return s, cmd
}

// View implements tea.Model
func (s *NewsCategoryFormScreen) View() string {
	return style.RenderSubscreen(
		s.width,
		s.height,
		"New News Category",
		s.form.View(),
	)
}

// SetSize updates the screen dimensions
func (s *NewsCategoryFormScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}
