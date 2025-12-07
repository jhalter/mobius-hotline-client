package internal

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/jhalter/mobius-hotline-client/internal/style"
)

// Messages sent from NewsArticlePostScreen to parent
type NewsArticlePostedMsg struct {
	Subject  string
	Body     string
	ParentID uint32
	Path     []string
}

type NewsArticlePostCancelledMsg struct{}

// NewsArticlePostScreen is a self-contained BubbleTea model for posting/replying to news articles
type NewsArticlePostScreen struct {
	form          *huh.Form
	parentID      uint32
	path          []string
	width, height int
	model         *Model
}

// NewNewsArticlePostScreen creates a new screen for posting a news article
func NewNewsArticlePostScreen(path []string, parentID uint32, prefillSubject string, m *Model) (*NewsArticlePostScreen, tea.Cmd) {
	// Copy path to avoid mutation
	pathCopy := make([]string, len(path))
	copy(pathCopy, path)

	subject := prefillSubject

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("subject").
				Title("Subject").
				Placeholder("Enter article subject").
				Value(&subject).
				CharLimit(255).
				Validate(func(str string) error {
					if len(strings.TrimSpace(str)) == 0 {
						return fmt.Errorf("subject cannot be empty")
					}
					return nil
				}),

			huh.NewText().
				Key("body").
				Title("Body").
				CharLimit(4000).
				Validate(func(str string) error {
					if len(strings.TrimSpace(str)) == 0 {
						return fmt.Errorf("body cannot be empty")
					}
					return nil
				}),

			huh.NewConfirm().
				Key("confirm").
				Title("Post this article?").
				Affirmative("Post").
				Negative("Cancel"),
		),
	).
		WithWidth(60).
		WithShowHelp(true).
		WithShowErrors(true).
		WithTheme(style.FormTheme)

	screen := &NewsArticlePostScreen{
		form:     form,
		parentID: parentID,
		path:     pathCopy,
		width:    m.width,
		height:   m.height,
		model:    m,
	}

	return screen, form.Init()
}

// Init implements tea.Model
func (s *NewsArticlePostScreen) Init() tea.Cmd {
	return s.form.Init()
}

// Update implements ScreenModel
func (s *NewsArticlePostScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		return s, nil

	case NewsArticlePostedMsg:
		s.model.handleNewsArticlePostedMsg(msg)
		return s, nil

	case NewsArticlePostCancelledMsg:
		s.model.PopScreen()
		return s, nil

	case tea.KeyMsg:
		if msg.String() == "esc" {
			return s, func() tea.Msg { return NewsArticlePostCancelledMsg{} }
		}
	}

	// Update the form
	form, cmd := s.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		s.form = f
	}

	// Check if form is complete
	if s.form.State == huh.StateCompleted {
		subject := s.form.GetString("subject")
		body := s.form.GetString("body")
		confirmed := s.form.GetBool("confirm")

		if confirmed && subject != "" && body != "" {
			pathCopy := make([]string, len(s.path))
			copy(pathCopy, s.path)
			parentID := s.parentID

			return s, func() tea.Msg {
				return NewsArticlePostedMsg{
					Subject:  subject,
					Body:     body,
					ParentID: parentID,
					Path:     pathCopy,
				}
			}
		}
		// Cancelled or invalid - return cancel message
		return s, func() tea.Msg { return NewsArticlePostCancelledMsg{} }
	}

	return s, cmd
}

// View implements tea.Model
func (s *NewsArticlePostScreen) View() string {
	title := "New Article"
	if s.parentID != 0 {
		title = "Reply to Article"
	}

	return style.RenderSubscreen(
		s.width,
		s.height,
		title,
		s.form.View(),
	)
}

// SetSize updates the screen dimensions
func (s *NewsArticlePostScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}
