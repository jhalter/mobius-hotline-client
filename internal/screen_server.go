package internal

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jhalter/mobius-hotline-client/internal/style"
	"github.com/jhalter/mobius/hotline"
	"github.com/muesli/reflow/wordwrap"
)

// Messages sent from ServerScreen to parent

// ServerDisconnectRequestedMsg signals user wants to disconnect
type ServerDisconnectRequestedMsg struct{}

// ServerSendChatMsg signals user wants to send a chat message
type ServerSendChatMsg struct {
	Text string
}

// ServerOpenNewsMsg signals user wants to open news
type ServerOpenNewsMsg struct{}

// ServerOpenMessageBoardMsg signals user wants to open message board
type ServerOpenMessageBoardMsg struct{}

// ServerOpenFilesMsg signals user wants to open files
type ServerOpenFilesMsg struct{}

// ServerOpenAccountsMsg signals user wants to open accounts
type ServerOpenAccountsMsg struct{}

// ServerComposeMessageMsg signals user wants to compose a private message
type ServerComposeMessageMsg struct {
	TargetUserID [2]byte
}

// ServerOpenTasksMsg signals user wants to open tasks screen
type ServerOpenTasksMsg struct{}

// serverScreenKeyMap defines key bindings for the server UI help display
type serverScreenKeyMap struct {
	News         key.Binding
	MessageBoard key.Binding
	Files        key.Binding
	Logs         key.Binding
	Accounts     key.Binding
	Disconnect   key.Binding
	Send         key.Binding
}

func (k serverScreenKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.MessageBoard, k.News, k.Files, k.Logs, k.Accounts, k.Disconnect}
}

func (k serverScreenKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.MessageBoard, k.News, k.Files, k.Logs, k.Accounts, k.Disconnect},
	}
}

// ServerScreen represents the main server UI after connecting
type ServerScreen struct {
	// Bubble Tea components
	chatViewport viewport.Model
	chatInput    textinput.Model
	userViewport viewport.Model
	help         help.Model
	keys         serverScreenKeyMap

	// Screen dimensions
	width, height int

	// Reference to parent model for callbacks
	model *Model

	// Screen-specific state
	chatMessages    []string // Store original formatted messages for re-wrapping
	chatContent     string   // Rendered chat content
	chatWasAtBottom bool     // Track scroll position for smart auto-scroll
	focusOnUserList bool     // true = user list focused, false = chat input focused
	selectedUserIdx int      // index of selected user in userList
	serverName      string   // Connected server name
	userList        []hotline.User
}

// NewServerScreen creates a new server screen
func NewServerScreen(m *Model) *ServerScreen {
	chatInput := textinput.New()
	chatInput.Placeholder = "Type a message..."
	chatInput.Width = 80
	chatInput.Focus()

	keys := serverScreenKeyMap{
		MessageBoard: key.NewBinding(
			key.WithKeys("ctrl+b"),
			key.WithHelp("^B", "messageboard"),
		),
		News: key.NewBinding(
			key.WithKeys("ctrl+n"),
			key.WithHelp("^N", "news"),
		),
		Files: key.NewBinding(
			key.WithKeys("ctrl+f"),
			key.WithHelp("^F", "files"),
		),
		Logs: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("^L", "logs"),
		),
		Accounts: key.NewBinding(
			key.WithKeys("ctrl+a"),
			key.WithHelp("^A", "accounts"),
		),
		Disconnect: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "disconnect"),
		),
		Send: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send message"),
		),
	}

	return &ServerScreen{
		chatViewport: viewport.New(m.width-30, m.height-9),
		chatInput:    chatInput,
		userViewport: viewport.New(25, m.height-9),
		help:         help.New(),
		keys:         keys,
		width:        m.width,
		height:       m.height,
		model:        m,
	}
}

// Init returns initial commands
func (s *ServerScreen) Init() tea.Cmd {
	return nil
}

// Update handles messages and returns updated screen + commands
func (s *ServerScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.SetSize(msg.Width, msg.Height)
		return s, nil

	// Handle screen messages by delegating to parent methods
	case ServerDisconnectRequestedMsg:
		return s, s.model.handleServerDisconnectRequestedMsg()

	case ServerSendChatMsg:
		s.model.handleServerSendChatMsg(msg)
		return s, nil

	case ServerOpenNewsMsg:
		s.model.handleServerOpenNewsMsg()
		return s, nil

	case ServerOpenMessageBoardMsg:
		s.model.handleServerOpenMessageBoardMsg()
		return s, nil

	case ServerOpenFilesMsg:
		s.model.handleServerOpenFilesMsg()
		return s, nil

	case ServerOpenAccountsMsg:
		s.model.handleServerOpenAccountsMsg()
		return s, nil

	case ServerComposeMessageMsg:
		return s, s.model.handleServerComposeMessageMsg(msg)

	case ServerOpenTasksMsg:
		s.model.handleServerOpenTasksMsg()
		return s, nil

	case tea.KeyMsg:
		return s.handleKeys(msg)
	}

	// Delegate to internal components
	var cmd tea.Cmd
	s.chatInput, cmd = s.chatInput.Update(msg)
	return s, cmd
}

// View renders the screen
func (s *ServerScreen) View() string {
	// Shortcuts
	shortcuts := s.help.View(s.keys)

	// User list
	var userListContent strings.Builder
	for i, u := range s.userList {
		flags := binary.BigEndian.Uint16(u.Flags)
		userName := u.Name

		// Highlight selected user when user list is focused
		if s.focusOnUserList && i == s.selectedUserIdx {
			userName = "> " + userName
		} else {
			userName = "  " + userName
		}

		// Check both admin and away flags
		isAdmin := (flags & (1 << hotline.UserFlagAdmin)) != 0
		isAway := (flags & (1 << hotline.UserFlagAway)) != 0

		if isAdmin && isAway {
			userListContent.WriteString(style.AwayAdminUserStyle.Render(userName))
		} else if isAdmin {
			userListContent.WriteString(style.AdminUserStyle.Render(userName))
		} else if isAway {
			userListContent.WriteString(style.AwayUserStyle.Render(userName))
		} else {
			userListContent.WriteString(userName)
		}
		userListContent.WriteString("\n")
	}
	s.userViewport.SetContent(userListContent.String())

	// Chat area - use double border when focused, grey border when scrolled up
	chatBorder := lipgloss.RoundedBorder()
	chatBorderColor := style.ColorCyan // Default cyan

	if !s.focusOnUserList {
		chatBorder = lipgloss.DoubleBorder()

		// Change to grey when in scrollback mode (not at bottom)
		if !s.chatViewport.AtBottom() {
			chatBorderColor = style.ColorGrey3
		}
	}

	chatView := lipgloss.NewStyle().
		PaddingLeft(1).
		Border(chatBorder).
		BorderForeground(chatBorderColor).
		Render(s.chatViewport.View())

	// User list area - use double border when focused
	userBorder := lipgloss.RoundedBorder()
	if s.focusOnUserList {
		userBorder = lipgloss.DoubleBorder()
	}
	userView := lipgloss.NewStyle().
		Border(userBorder).
		BorderForeground(style.ColorCyan).
		Render(s.userViewport.View())

	return lipgloss.JoinVertical(
		lipgloss.Left,
		style.ServerTitleStyle.Render(fmt.Sprintf("Mobius - Connected to %s", s.serverName)),
		shortcuts,
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			lipgloss.JoinVertical(lipgloss.Left, chatView, style.BoxStyle.Render(s.chatInput.View())),
			lipgloss.JoinVertical(lipgloss.Left, userView, s.model.renderTaskWidget()),
		),
	)
}

// SetSize updates dimensions
func (s *ServerScreen) SetSize(width, height int) {
	s.width = width
	s.height = height

	chatWidth := width - 30
	chatHeight := height - 9

	s.chatViewport.Width = chatWidth
	s.chatViewport.Height = chatHeight

	// Update chat input width to match chat viewport
	// Subtract additional padding for the input box border
	s.chatInput.Width = chatWidth - 4

	s.userViewport.Width = 25
	s.userViewport.Height = height - 9

	// Rebuild chat content with new width
	s.rebuildChatContent()
}

// handleKeys handles keyboard input
func (s *ServerScreen) handleKeys(msg tea.KeyMsg) (ScreenModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return s, func() tea.Msg { return ServerDisconnectRequestedMsg{} }

	case "tab":
		// Toggle focus between chat input and user list
		s.focusOnUserList = !s.focusOnUserList
		if s.focusOnUserList {
			// Blur chat input when switching to user list
			s.chatInput.Blur()
		} else {
			// Focus chat input when switching back
			s.chatInput.Focus()
		}
		return s, nil

	case "ctrl+n":
		return s, func() tea.Msg { return ServerOpenNewsMsg{} }

	case "ctrl+b":
		return s, func() tea.Msg { return ServerOpenMessageBoardMsg{} }

	case "ctrl+f":
		return s, func() tea.Msg { return ServerOpenFilesMsg{} }

	case "ctrl+a":
		return s, func() tea.Msg { return ServerOpenAccountsMsg{} }

	case "ctrl+t":
		return s, func() tea.Msg { return ServerOpenTasksMsg{} }

	case "up":
		if s.focusOnUserList && s.selectedUserIdx > 0 {
			s.selectedUserIdx--
		} else if !s.focusOnUserList {
			// Scroll chat viewport up when chat input is focused
			s.chatViewport.ScrollUp(1)
		}
		return s, nil

	case "down":
		if s.focusOnUserList && s.selectedUserIdx < len(s.userList)-1 {
			s.selectedUserIdx++
		} else if !s.focusOnUserList {
			// Scroll chat viewport down when chat input is focused
			s.chatViewport.ScrollDown(1)
		}
		return s, nil

	case "pgup":
		if !s.focusOnUserList {
			// Page up in chat viewport
			s.chatViewport.PageUp()
		}
		return s, nil

	case "pgdown":
		if !s.focusOnUserList {
			// Page down in chat viewport
			s.chatViewport.PageDown()
		}
		return s, nil

	case "home":
		if !s.focusOnUserList {
			// Jump to top of chat
			s.chatViewport.GotoTop()
		}
		return s, nil

	case "end":
		if !s.focusOnUserList {
			// Jump to bottom of chat
			s.chatViewport.GotoBottom()
		}
		return s, nil

	case "enter":
		if s.focusOnUserList {
			// Open compose message modal for selected user
			if s.selectedUserIdx >= 0 && s.selectedUserIdx < len(s.userList) {
				targetID := s.userList[s.selectedUserIdx].ID
				return s, func() tea.Msg {
					return ServerComposeMessageMsg{TargetUserID: targetID}
				}
			}
		} else {
			// Send chat message
			text := s.chatInput.Value()
			if text != "" {
				s.chatInput.SetValue("")
				return s, func() tea.Msg {
					return ServerSendChatMsg{Text: text}
				}
			}
		}
		return s, nil
	}

	// Pass all other keys to the chat input if it's focused
	if !s.focusOnUserList {
		var cmd tea.Cmd
		s.chatInput, cmd = s.chatInput.Update(msg)
		return s, cmd
	}

	return s, nil
}

// SetServerName sets the connected server name
func (s *ServerScreen) SetServerName(name string) {
	s.serverName = name
}

// SetUserList updates the user list
func (s *ServerScreen) SetUserList(users []hotline.User) {
	s.userList = users
	// Ensure selected index is still valid
	if s.selectedUserIdx >= len(s.userList) {
		s.selectedUserIdx = len(s.userList) - 1
	}
	if s.selectedUserIdx < 0 {
		s.selectedUserIdx = 0
	}
}

// AddChatMessage adds a new chat message and updates the viewport
func (s *ServerScreen) AddChatMessage(formattedMsg string) {
	// Track if viewport was at bottom before adding
	s.chatWasAtBottom = s.chatViewport.AtBottom()

	s.chatMessages = append(s.chatMessages, formattedMsg)
	s.rebuildChatContent()

	// Auto-scroll to bottom if user was already at bottom
	if s.chatWasAtBottom {
		s.chatViewport.GotoBottom()
	}
}

// FocusChatInput sets focus to the chat input
func (s *ServerScreen) FocusChatInput() {
	s.focusOnUserList = false
	s.chatInput.Focus()
}

// wrapChatMessage wraps a chat message to fit within the chat viewport width
func (s *ServerScreen) wrapChatMessage(formattedMsg string) string {
	const borderWidth = 2
	const paddingLeft = 1 // From chatView rendering (PaddingLeft)

	chatWidth := s.width - 30 - borderWidth
	if chatWidth < 10 {
		chatWidth = 10
	}

	// Subtract padding to get usable content width
	wrapWidth := chatWidth - paddingLeft
	if wrapWidth < 5 {
		wrapWidth = 5 // Minimum for edge cases
	}

	// wordwrap.String handles ANSI codes correctly
	return wordwrap.String(formattedMsg, wrapWidth)
}

// rebuildChatContent rebuilds entire chat content from stored messages,
// applying word-wrapping based on current viewport width
func (s *ServerScreen) rebuildChatContent() {
	var content strings.Builder
	for _, msg := range s.chatMessages {
		content.WriteString(fmt.Sprintf("%s\n", s.wrapChatMessage(msg)))
	}

	s.chatContent = content.String()
	s.chatViewport.SetContent(s.chatContent)
}

// SetUserAccess updates keybindings based on user access permissions
func (s *ServerScreen) SetUserAccess(access hotline.AccessBitmap) {
	s.keys.News.SetEnabled(access.IsSet(hotline.AccessNewsReadArt))
	s.keys.Accounts.SetEnabled(access.IsSet(hotline.AccessModifyUser))
}
