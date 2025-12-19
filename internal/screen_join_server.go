package internal

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/jhalter/mobius-hotline-client/internal/style"
	"github.com/jhalter/mobius/hotline"
)

// JoinServerMode represents the mode the join server screen is in
type JoinServerMode int

// joinServerKeyMap defines the keybindings for the join server screen
type joinServerKeyMap struct {
	Tab    key.Binding
	Enter  key.Binding
	Escape key.Binding
}

func (k joinServerKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Enter, k.Escape}
}

func (k joinServerKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Tab, k.Enter, k.Escape}}
}

func newJoinServerKeyMap() joinServerKeyMap {
	return joinServerKeyMap{
		Tab:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next")),
		Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "connect")),
		Escape: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

const (
	JoinServerModeConnect JoinServerMode = iota
	JoinServerModeEditBookmark
	JoinServerModeCreateBookmark
)

// Messages sent from JoinServerScreen to parent
type JoinServerConnectMsg struct {
	Name     string
	Addr     string
	Login    string
	Password string
	TLS      bool
	Save     bool // Save as bookmark
}

type JoinServerBookmarkSavedMsg struct {
	Name     string
	Addr     string
	Login    string
	Password string
	TLS      bool
	Index    int // Index of bookmark being edited
}

type JoinServerBookmarkCreatedMsg struct {
	Name     string
	Addr     string
	Login    string
	Password string
	TLS      bool
}

type JoinServerCancelledMsg struct {
	BackPage Screen
}

// JoinServerScreen is a self-contained BubbleTea model for the join server form
type JoinServerScreen struct {
	form                 *huh.Form
	mode                 JoinServerMode
	editingBookmarkIndex int
	backPage             Screen
	width, height        int
	model                *Model
	help                 help.Model
	keys                 joinServerKeyMap

	// Form field values (bound to form inputs)
	name         string
	server       string
	login        string
	password     string
	useTLS       bool
	saveBookmark bool
}

// enterSubmitsKeyMap creates a keymap where Enter submits the form immediately
// instead of tabbing through fields.
func enterSubmitsKeyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	// Remove enter from Next so it only navigates with tab
	km.Input.Next = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next"))
	km.Confirm.Next = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next"))
	// Add enter to submit so it shows in help
	km.Input.Submit = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "connect"))
	km.Confirm.Submit = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "connect"))
	return km
}

// buildJoinServerForm creates a Huh form based on the mode and initial values
func buildJoinServerForm(mode JoinServerMode, name, server, login, password *string, useTLS, saveBookmark *bool) *huh.Form {
	var groups []*huh.Group

	if mode == JoinServerModeEditBookmark || mode == JoinServerModeCreateBookmark {
		// Edit/Create mode: name, server, login, password, TLS
		groups = append(groups, huh.NewGroup(
			huh.NewInput().
				Key("name").
				Title("Name").
				Placeholder("Bookmark Name").
				Value(name),

			huh.NewInput().
				Key("server").
				Title("Server").
				Placeholder("hostname:port").
				Value(server),

			huh.NewInput().
				Key("login").
				Title("Login").
				Placeholder("guest").
				Value(login),

			huh.NewInput().
				Key("password").
				Title("Password").
				Placeholder("password").
				EchoMode(huh.EchoModePassword).
				Value(password),

			huh.NewConfirm().
				Key("tls").
				Title("Use TLS").
				Affirmative("Yes").
				Negative("No").
				Value(useTLS),
		))
	} else {
		// Connect mode: server, login, password, TLS, Save
		groups = append(groups, huh.NewGroup(
			huh.NewInput().
				Key("server").
				Title("Server").
				Placeholder("server:port").
				Value(server),

			huh.NewInput().
				Key("login").
				Title("Login").
				Placeholder("guest").
				Value(login),

			huh.NewInput().
				Key("password").
				Title("Password").
				Placeholder("password").
				EchoMode(huh.EchoModePassword).
				Value(password),

			huh.NewConfirm().
				Key("tls").
				Title("Use TLS").
				Affirmative("Yes").
				Negative("No").
				Value(useTLS),

			huh.NewConfirm().
				Key("save").
				Title("Save as Bookmark").
				Affirmative("Yes").
				Negative("No").
				Value(saveBookmark),
		))
	}

	return huh.NewForm(groups...).
		WithWidth(50).
		WithShowHelp(false).
		WithShowErrors(true).
		WithKeyMap(enterSubmitsKeyMap()).
		WithTheme(style.FormTheme)
}

// NewJoinServerScreen creates a new join server screen for connecting to a server
func NewJoinServerScreen(m *Model) (*JoinServerScreen, tea.Cmd) {
	screen := &JoinServerScreen{
		mode:                 JoinServerModeConnect,
		editingBookmarkIndex: -1,
		backPage:             ScreenHome,
		width:                m.width,
		height:               m.height,
		model:                m,
		help:                 style.NewHelp(),
		keys:                 newJoinServerKeyMap(),
	}

	screen.form = buildJoinServerForm(JoinServerModeConnect, &screen.name, &screen.server, &screen.login, &screen.password, &screen.useTLS, &screen.saveBookmark)

	cmd := screen.form.Init()
	return screen, cmd
}

// NewJoinServerScreenForConnect creates a new screen pre-populated for connecting
func NewJoinServerScreenForConnect(serverAddr, login, password string, useTLS bool, backPage Screen, m *Model) (*JoinServerScreen, tea.Cmd) {
	screen := &JoinServerScreen{
		mode:                 JoinServerModeConnect,
		editingBookmarkIndex: -1,
		backPage:             backPage,
		width:                m.width,
		height:               m.height,
		model:                m,
		help:                 style.NewHelp(),
		keys:                 newJoinServerKeyMap(),
		server:               serverAddr,
		login:                login,
		password:             password,
		useTLS:               useTLS,
	}

	screen.form = buildJoinServerForm(JoinServerModeConnect, &screen.name, &screen.server, &screen.login, &screen.password, &screen.useTLS, &screen.saveBookmark)

	cmd := screen.form.Init()
	return screen, cmd
}

// NewJoinServerScreenForEdit creates a new screen for editing a bookmark
func NewJoinServerScreenForEdit(bm Bookmark, index int, m *Model) (*JoinServerScreen, tea.Cmd) {
	screen := &JoinServerScreen{
		mode:                 JoinServerModeEditBookmark,
		editingBookmarkIndex: index,
		backPage:             ScreenBookmarks,
		width:                m.width,
		height:               m.height,
		model:                m,
		help:                 style.NewHelp(),
		keys:                 newJoinServerKeyMap(),
		name:                 bm.Name,
		server:               bm.Addr,
		login:                bm.Login,
		password:             bm.Password,
		useTLS:               bm.TLS,
	}

	screen.form = buildJoinServerForm(JoinServerModeEditBookmark, &screen.name, &screen.server, &screen.login, &screen.password, &screen.useTLS, &screen.saveBookmark)

	cmd := screen.form.Init()
	return screen, cmd
}

// NewJoinServerScreenForCreate creates a new screen for creating a bookmark
func NewJoinServerScreenForCreate(m *Model) (*JoinServerScreen, tea.Cmd) {
	screen := &JoinServerScreen{
		mode:                 JoinServerModeCreateBookmark,
		editingBookmarkIndex: -1,
		backPage:             ScreenBookmarks,
		width:                m.width,
		height:               m.height,
		model:                m,
		help:                 style.NewHelp(),
		keys:                 newJoinServerKeyMap(),
	}

	screen.form = buildJoinServerForm(JoinServerModeCreateBookmark, &screen.name, &screen.server, &screen.login, &screen.password, &screen.useTLS, &screen.saveBookmark)

	cmd := screen.form.Init()
	return screen, cmd
}

// Init implements tea.Model
func (s *JoinServerScreen) Init() tea.Cmd {
	cmd := s.form.Init()
	return cmd
}

// Update implements ScreenModel
func (s *JoinServerScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.SetSize(msg.Width, msg.Height)
		return s, nil

	case JoinServerConnectMsg:
		return s, s.model.handleJoinServerConnectMsg(msg)
	case JoinServerBookmarkSavedMsg:
		s.model.handleJoinServerBookmarkSavedMsg(msg)
		return s, nil
	case JoinServerBookmarkCreatedMsg:
		s.model.handleJoinServerBookmarkCreatedMsg(msg)
		return s, nil
	case JoinServerCancelledMsg:
		s.model.handleJoinServerCancelledMsg(msg)
		return s, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			backPage := s.backPage
			return s, func() tea.Msg { return JoinServerCancelledMsg{BackPage: backPage} }
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
func (s *JoinServerScreen) handleSubmit() tea.Cmd {
	// Read values from struct fields (bound to form inputs)
	name := s.name
	addr := s.server
	login := s.login
	if login == "" {
		login = hotline.GuestAccount
	}
	password := s.password
	useTLS := s.useTLS
	saveBookmark := s.saveBookmark

	switch s.mode {
	case JoinServerModeEditBookmark:
		index := s.editingBookmarkIndex
		return func() tea.Msg {
			return JoinServerBookmarkSavedMsg{
				Name:     name,
				Addr:     addr,
				Login:    login,
				Password: password,
				TLS:      useTLS,
				Index:    index,
			}
		}

	case JoinServerModeCreateBookmark:
		return func() tea.Msg {
			return JoinServerBookmarkCreatedMsg{
				Name:     name,
				Addr:     addr,
				Login:    login,
				Password: password,
				TLS:      useTLS,
			}
		}

	default:
		// Connect mode
		return func() tea.Msg {
			return JoinServerConnectMsg{
				Name:     name,
				Addr:     addr,
				Login:    login,
				Password: password,
				TLS:      useTLS,
				Save:     saveBookmark,
			}
		}
	}
}

// View implements ScreenModel
func (s *JoinServerScreen) View() tea.View {
	var title string
	switch s.mode {
	case JoinServerModeEditBookmark:
		title = "Edit Bookmark"
	case JoinServerModeCreateBookmark:
		title = "New Bookmark"
	default:
		title = "Connect to Server"
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		s.form.View(),
		"",
		s.help.View(s.keys),
	)

	return tea.NewView(style.RenderSubscreen(s.width, s.height, title, content))
}

// SetSize updates the screen dimensions
func (s *JoinServerScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// SetPendingServerName sets the server name to display when connection succeeds
func (s *JoinServerScreen) SetPendingServerName(name string) {
	s.model.pendingServerName = name
}
