package internal

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/jhalter/mobius-hotline-client/internal/style"
)

// Messages sent from ComposeMessageScreen to parent

// ComposeMessageSentMsg signals user sent a private message
type ComposeMessageSentMsg struct {
	TargetID  [2]byte
	Text      string
	QuoteText string
}

// ComposeMessageCancelledMsg signals user cancelled composing
type ComposeMessageCancelledMsg struct{}

// ComposeMessageScreen is a self-contained BubbleTea model for composing private messages
type ComposeMessageScreen struct {
	form          *huh.Form
	width, height int
	model         *Model

	targetID   [2]byte
	targetName string
	quoteText  string
}

// NewComposeMessageScreen creates a new compose message screen
func NewComposeMessageScreen(targetID [2]byte, targetName string, quoteText string, m *Model) (*ComposeMessageScreen, tea.Cmd) {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Key("message").
				Placeholder("Type your message...").
				CharLimit(1000).
				Validate(func(str string) error {
					if len(strings.TrimSpace(str)) == 0 {
						return fmt.Errorf("message cannot be empty")
					}
					return nil
				}),

			huh.NewConfirm().
				Key("confirm").
				Affirmative("Send").
				Negative("Cancel"),
		),
	).
		WithWidth(60).
		WithShowHelp(true).
		WithShowErrors(true).
		WithTheme(style.FormTheme)

	screen := &ComposeMessageScreen{
		form:       form,
		width:      m.width,
		height:     m.height,
		model:      m,
		targetID:   targetID,
		targetName: targetName,
		quoteText:  quoteText,
	}

	cmd := form.Init()
	return screen, cmd
}

// Init implements tea.Model
func (s *ComposeMessageScreen) Init() tea.Cmd {
	cmd := s.form.Init()
	return cmd
}

// Update implements ScreenModel
func (s *ComposeMessageScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		return s, nil

	// Handle screen messages by delegating to parent methods
	case ComposeMessageSentMsg:
		return s, s.model.handleComposeMessageSentMsg(msg)

	case ComposeMessageCancelledMsg:
		return s, s.model.handleComposeMessageCancelledMsg()

	case tea.KeyPressMsg:
		if msg.String() == "esc" {
			return s, func() tea.Msg { return ComposeMessageCancelledMsg{} }
		}
	}

	// Update the form
	form, cmd := s.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		s.form = f
	}

	// Check if form is complete
	if s.form.State == huh.StateCompleted {
		messageText := s.form.GetString("message")
		confirmed := s.form.GetBool("confirm")

		if confirmed && strings.TrimSpace(messageText) != "" {
			targetID := s.targetID
			quoteText := s.quoteText

			return s, func() tea.Msg {
				return ComposeMessageSentMsg{
					TargetID:  targetID,
					Text:      messageText,
					QuoteText: quoteText,
				}
			}
		}
		// Cancelled or invalid - return cancel message
		return s, func() tea.Msg { return ComposeMessageCancelledMsg{} }
	}

	return s, cmd
}

// View implements ScreenModel
func (s *ComposeMessageScreen) View() tea.View {
	var content strings.Builder

	// Show quoted message if this is a reply
	if s.quoteText != "" {
		quotedStyle := lipgloss.NewStyle().
			Foreground(style.CurrentTheme.TextMuted).
			Italic(true)
		content.WriteString(quotedStyle.Render("> " + s.quoteText))
		content.WriteString("\n\n")
	}

	content.WriteString(s.form.View())

	return tea.NewView(style.RenderSubscreen(
		s.width,
		s.height,
		"Send Private Message to "+s.targetName,
		content.String(),
	))
}

// SetSize updates dimensions
func (s *ComposeMessageScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}
