package internal

import (
	"charm.land/bubbles/v2/filepicker"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jhalter/mobius-hotline-client/internal/style"
)

// Messages sent from FilePickerScreen to parent

// FilePickerFileSelectedMsg signals user selected a file for upload
type FilePickerFileSelectedMsg struct {
	Path string
}

// FilePickerCancelledMsg signals user cancelled the file picker
type FilePickerCancelledMsg struct{}

// FilePickerScreen is a self-contained BubbleTea model for selecting files to upload
type FilePickerScreen struct {
	filePicker    filepicker.Model
	width, height int
	model         *Model

	// Remember last location for next time
	lastLocation string
}

// NewFilePickerScreen creates a new file picker screen
func NewFilePickerScreen(startDir string, m *Model) *FilePickerScreen {
	fp := filepicker.New()
	fp.AllowedTypes = []string{} // Allow all file types
	fp.CurrentDirectory = startDir
	fp.ShowHidden = false
	fp.ShowPermissions = false
	fp.SetHeight(20)

	return &FilePickerScreen{
		filePicker:   fp,
		width:        m.width,
		height:       m.height,
		model:        m,
		lastLocation: startDir,
	}
}

// Init implements tea.Model
func (s *FilePickerScreen) Init() tea.Cmd {
	cmd := s.filePicker.Init()
	return cmd
}

// Update implements ScreenModel
func (s *FilePickerScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.SetSize(msg.Width, msg.Height)
		return s, nil

	// Handle screen messages by delegating to parent methods
	case FilePickerFileSelectedMsg:
		return s, s.model.handleFilePickerFileSelectedMsg(msg)

	case FilePickerCancelledMsg:
		s.model.handleFilePickerCancelledMsg()
		return s, nil

	case tea.KeyPressMsg:
		return s.handleKeys(msg)
	}

	// Delegate to internal component
	var cmd tea.Cmd
	s.filePicker, cmd = s.filePicker.Update(msg)
	return s, cmd
}

// View implements tea.Model
func (s *FilePickerScreen) View() string {

	return lipgloss.Place(
		s.width,
		s.height,
		lipgloss.Center,
		lipgloss.Center,
		style.SubScreenStyle.Width(s.width-20).Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				style.SubTitleStyle.Render("Select file to upload"),
				s.filePicker.View(),
			),
		),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(style.CurrentTheme.BorderMuted)),
	)
}

// SetSize updates dimensions
func (s *FilePickerScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.filePicker.SetHeight(height - 10)
}

// handleKeys handles keyboard input
func (s *FilePickerScreen) handleKeys(msg tea.KeyPressMsg) (ScreenModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return s, func() tea.Msg { return FilePickerCancelledMsg{} }
	}

	// Update file picker with key
	var cmd tea.Cmd
	s.filePicker, cmd = s.filePicker.Update(msg)

	// Check if file was selected
	if didSelect, path := s.filePicker.DidSelectFile(msg); didSelect {
		// Remember location for next time
		s.lastLocation = s.filePicker.CurrentDirectory
		selectedPath := path
		return s, func() tea.Msg {
			return FilePickerFileSelectedMsg{Path: selectedPath}
		}
	}

	// Check for disabled file (means user hit 'q' or similar to quit)
	if didSelect, _ := s.filePicker.DidSelectDisabledFile(msg); didSelect {
		return s, func() tea.Msg { return FilePickerCancelledMsg{} }
	}

	return s, cmd
}

// GetLastLocation returns the last directory location
func (s *FilePickerScreen) GetLastLocation() string {
	return s.lastLocation
}
