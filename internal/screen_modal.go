package internal

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/jhalter/mobius-hotline-client/internal/style"
	"github.com/muesli/reflow/wordwrap"
)

// ModalType identifies the type of modal for proper handling
type ModalType int

const (
	ModalTypeGeneric ModalType = iota
	ModalTypePrivateMessage
	ModalTypeAgreement
	ModalTypeDisconnect
	ModalTypeError
)

// Messages sent from ModalScreen to parent
type ModalCancelledMsg struct{}

type ModalButtonClickedMsg struct {
	Title         string    // Modal title for context
	ButtonClicked string    // Which button was clicked
	Type          ModalType // Modal type for proper handling
}

// ModalScreen encapsulates the modal dialog component
type ModalScreen struct {
	// Bubble Tea components
	form *huh.Form

	// Screen dimensions
	width, height int

	// Reference to parent model
	model *Model

	// Modal state
	modalType ModalType
	title     string
	content   string
	buttons   []string
}

// NewModalScreen creates a new modal screen instance
func NewModalScreen(modalType ModalType, title, content string, buttons []string, m *Model) *ModalScreen {
	if len(buttons) == 0 {
		buttons = []string{"OK"}
	}

	s := &ModalScreen{
		modalType: modalType,
		title:     title,
		content:   content,
		buttons:   buttons,
		width:     m.width,
		height:    m.height,
		model:     m,
	}

	s.initForm()
	return s
}

// initForm creates the huh form for the modal buttons
func (s *ModalScreen) initForm() {
	var confirmField *huh.Confirm
	if len(s.buttons) == 1 {
		// Single button modal - just shows one button (always returns true)
		confirmField = huh.NewConfirm().
			Key("confirm").
			Affirmative(s.buttons[0]).
			Negative("")
	} else if len(s.buttons) >= 2 {
		// Two button modal - second button is affirmative (default), first is negative
		confirmField = huh.NewConfirm().
			Key("confirm").
			Value(func() *bool { t := true; return &t }()).
			Affirmative(s.buttons[1]).
			Negative(s.buttons[0])
	}

	// Get default key map and add Tab to toggle binding
	keyMap := huh.NewDefaultKeyMap()
	keyMap.Confirm.Toggle.SetKeys("left", "right", "h", "l", "tab")

	// Create theme based on app theme, without left border
	theme := *style.FormTheme
	theme.Focused.Base = theme.Focused.Base.
		UnsetBorderLeft().
		UnsetBorderStyle()

	s.form = huh.NewForm(
		huh.NewGroup(confirmField),
	).
		WithWidth(60).
		WithShowHelp(false).
		WithShowErrors(false).
		WithKeyMap(keyMap).
		WithTheme(&theme)
}

// Init returns initial commands
func (s *ModalScreen) Init() tea.Cmd {
	if s.form != nil {
		return s.form.Init()
	}
	return nil
}

// Update handles messages and returns updated screen + commands
func (s *ModalScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.SetSize(msg.Width, msg.Height)
		return s, nil

	case tea.KeyMsg:
		return s.handleKeys(msg)
	}

	// Delegate to form component
	if s.form != nil {
		form, cmd := s.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			s.form = f
		}

		// Check if form is complete
		if s.form.State == huh.StateCompleted {
			return s, s.handleFormComplete()
		}

		return s, cmd
	}

	return s, nil
}

// handleKeys handles keyboard input
func (s *ModalScreen) handleKeys(msg tea.KeyMsg) (ScreenModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return s, func() tea.Msg { return ModalCancelledMsg{} }
	}

	// Pass to form component
	if s.form != nil {
		form, cmd := s.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			s.form = f
		}

		// Check if form is complete after key handling
		if s.form.State == huh.StateCompleted {
			return s, s.handleFormComplete()
		}

		return s, cmd
	}

	return s, nil
}

// handleFormComplete processes form completion and sends appropriate message
func (s *ModalScreen) handleFormComplete() tea.Cmd {
	// Get boolean from Confirm field and map to button label
	confirmed := s.form.GetBool("confirm")

	var buttonClicked string
	if confirmed {
		buttonClicked = s.buttons[len(s.buttons)-1] // Affirmative is last button
	} else if len(s.buttons) > 1 {
		buttonClicked = s.buttons[0] // Negative is first button
	}

	return func() tea.Msg {
		return ModalButtonClickedMsg{
			Title:         s.title,
			ButtonClicked: buttonClicked,
			Type:          s.modalType,
		}
	}
}

// View renders the modal screen
func (s *ModalScreen) View() string {
	title := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Render(style.Rainbow(lipgloss.NewStyle(), s.title, style.Blends))

	var body string
	if s.content != "" {
		body = lipgloss.NewStyle().
			Padding(1).
			Render(wordwrap.String(s.content, 56))
	}

	// Render the huh form buttons
	var buttons string
	if s.form != nil {
		buttons = lipgloss.NewStyle().
			Width(50).
			Align(lipgloss.Center).
			Render(s.form.View())
	}

	return lipgloss.Place(s.width, s.height,
		lipgloss.Center, lipgloss.Center,
		style.DialogBoxStyle.Render(lipgloss.JoinVertical(
			lipgloss.Center,
			title,
			body,
			buttons,
		)),
		lipgloss.WithWhitespaceChars("☃︎"),
		lipgloss.WithWhitespaceForeground(style.Subtle),
	)
}

// SetSize updates dimensions
func (s *ModalScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// GetTitle returns the modal title (for parent context)
func (s *ModalScreen) GetTitle() string {
	return s.title
}
