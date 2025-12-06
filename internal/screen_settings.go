package internal

import (
	"encoding/binary"
	"strconv"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/jhalter/mobius-hotline-client/internal/style"
)

// settingsKeyMap defines the keybindings for the settings screen
type settingsKeyMap struct {
	Tab    key.Binding
	Enter  key.Binding
	Escape key.Binding
}

func (k settingsKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Enter, k.Escape}
}

func (k settingsKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Tab, k.Enter, k.Escape}}
}

func newSettingsKeyMap() settingsKeyMap {
	return settingsKeyMap{
		Tab:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next")),
		Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "save")),
		Escape: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

type Settings struct {
	Username     string     `yaml:"Username"`
	IconID       int        `yaml:"IconID"`
	Bookmarks    []Bookmark `yaml:"Bookmarks"`
	Tracker      string     `yaml:"Tracker"`
	EnableBell   bool       `yaml:"EnableBell"`
	EnableSounds bool       `yaml:"EnableSounds"`
	DownloadDir  string     `yaml:"DownloadDir"`
	Theme        string     `yaml:"Theme"`
}

func (cp *Settings) IconBytes() []byte {
	iconBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(iconBytes, uint16(cp.IconID))
	return iconBytes
}

func (cp *Settings) AddBookmark(name, addr, login, pass string, useTLS bool) {
	cp.Bookmarks = append(cp.Bookmarks, Bookmark{Name: name, Addr: addr, Login: login, Password: pass, TLS: useTLS})
}

// Messages sent from SettingsScreen to parent
type SettingsSavedMsg struct {
	Username     string
	IconID       int
	Tracker      string
	DownloadDir  string
	EnableBell   bool
	EnableSounds bool
	Theme        string
}

type SettingsCancelledMsg struct{}

// SettingsScreen is a self-contained BubbleTea model for editing settings
type SettingsScreen struct {
	form          *huh.Form
	width, height int
	model         *Model
	help          help.Model
	keys          settingsKeyMap

	// Form field values (bound to form inputs)
	username     string
	iconID       string
	tracker      string
	downloadDir  string
	enableBell   bool
	enableSounds bool
	theme        string
}

// buildSettingsForm creates a Huh form for editing settings
func buildSettingsForm(username, iconID, tracker, downloadDir, theme *string, enableBell, enableSounds *bool) *huh.Form {
	// Build theme options from available themes
	themeOptions := make([]huh.Option[string], 0)
	for _, name := range style.ThemeNames() {
		themeOptions = append(themeOptions, huh.NewOption(name, name))
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("username").
				Title("Your Name").
				Placeholder("Your Name").
				Value(username),

			huh.NewInput().
				Key("iconID").
				Title("Icon ID").
				Placeholder("Icon ID").
				Value(iconID),

			huh.NewInput().
				Key("tracker").
				Title("Tracker").
				Placeholder("Tracker URL").
				Value(tracker),

			huh.NewInput().
				Key("downloadDir").
				Title("Download Directory").
				Placeholder("Download Directory").
				Value(downloadDir),

			huh.NewSelect[string]().
				Key("theme").
				Title("Theme").
				Options(themeOptions...).
				Value(theme),

			huh.NewConfirm().
				Key("enableBell").
				Title("Terminal Bell").
				Affirmative("On").
				Negative("Off").
				Value(enableBell),

			huh.NewConfirm().
				Key("enableSounds").
				Title("Sounds").
				Affirmative("On").
				Negative("Off").
				Value(enableSounds),
		),
	).
		WithWidth(50).
		WithShowHelp(false).
		WithShowErrors(true).
		WithKeyMap(enterSubmitsKeyMap())
}

// NewSettingsScreen creates a new settings screen with current settings values
func NewSettingsScreen(prefs *Settings, m *Model) (*SettingsScreen, tea.Cmd) {
	// Default to Classic theme if not set
	theme := prefs.Theme
	if theme == "" {
		theme = "Classic"
	}

	screen := &SettingsScreen{
		width:        m.width,
		height:       m.height,
		model:        m,
		help:         help.New(),
		keys:         newSettingsKeyMap(),
		username:     prefs.Username,
		iconID:       strconv.Itoa(prefs.IconID),
		tracker:      prefs.Tracker,
		downloadDir:  prefs.DownloadDir,
		enableBell:   prefs.EnableBell,
		enableSounds: prefs.EnableSounds,
		theme:        theme,
	}

	screen.form = buildSettingsForm(&screen.username, &screen.iconID, &screen.tracker, &screen.downloadDir, &screen.theme, &screen.enableBell, &screen.enableSounds)

	return screen, screen.form.Init()
}

// Init implements tea.Model
func (s *SettingsScreen) Init() tea.Cmd {
	return s.form.Init()
}

// Update implements ScreenModel
func (s *SettingsScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.SetSize(msg.Width, msg.Height)
		return s, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return s, func() tea.Msg { return SettingsCancelledMsg{} }
		case "enter":
			// First update the form to commit the current field's value
			form, _ := s.form.Update(msg)
			if f, ok := form.(*huh.Form); ok {
				s.form = f
			}
			// Then submit the form immediately
			s.form.NextGroup()
			if s.form.State == huh.StateCompleted {
				return s, s.handleSubmit()
			}
			return s, nil
		}
	}

	// Update the form
	form, cmd := s.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		s.form = f
	}

	// Check if form is complete
	if s.form.State == huh.StateCompleted {
		return s, s.handleSubmit()
	}

	return s, cmd
}

// handleSubmit processes the form submission
func (s *SettingsScreen) handleSubmit() tea.Cmd {
	// Read values from struct fields (bound to form inputs)
	username := s.username
	tracker := s.tracker
	downloadDir := s.downloadDir
	enableBell := s.enableBell
	enableSounds := s.enableSounds
	theme := s.theme

	iconID := 0
	if id, err := strconv.Atoi(s.iconID); err == nil {
		iconID = id
	}

	return func() tea.Msg {
		return SettingsSavedMsg{
			Username:     username,
			IconID:       iconID,
			Tracker:      tracker,
			DownloadDir:  downloadDir,
			EnableBell:   enableBell,
			EnableSounds: enableSounds,
			Theme:        theme,
		}
	}
}

// View implements tea.Model
func (s *SettingsScreen) View() string {
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		s.form.View(),
		"",
		s.help.View(s.keys),
	)
	return style.RenderSubscreen(s.width, s.height, "Settings", content)
}

// SetSize updates the screen dimensions
func (s *SettingsScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}
