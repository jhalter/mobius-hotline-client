package internal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jhalter/mobius-hotline-client/internal/style"
	"github.com/jhalter/mobius/hotline"
)

// Messages sent from AccountsScreen to parent

// AccountsCancelledMsg signals user wants to close accounts screen
type AccountsCancelledMsg struct{}

// AccountsSaveMsg signals user wants to save account changes
type AccountsSaveMsg struct {
	Login           string
	Name            string
	Password        string
	PasswordChanged bool
	AccessBits      hotline.AccessBitmap
	IsNew           bool
}

// AccountsDeleteMsg signals user wants to delete an account
type AccountsDeleteMsg struct {
	Login string
}

// Access bit definitions organized by category
var accessBitsByCategory = []struct {
	category string
	bits     []accessBitInfo
}{
	{
		category: "File System Maintenance",
		bits: []accessBitInfo{
			{hotline.AccessDeleteFile, "Delete Files"},
			{hotline.AccessUploadFile, "Upload Files"},
			{hotline.AccessDownloadFile, "Download Files"},
			{hotline.AccessRenameFile, "Rename Files"},
			{hotline.AccessMoveFile, "Move Files"},
			{hotline.AccessCreateFolder, "Create Folders"},
			{hotline.AccessDeleteFolder, "Delete Folders"},
			{hotline.AccessRenameFolder, "Rename Folders"},
			{hotline.AccessMoveFolder, "Move Folders"},
			{hotline.AccessUploadAnywhere, "Upload Anywhere"},
			{hotline.AccessSetFileComment, "Comment Files"},
			{hotline.AccessSetFolderComment, "Comment Folders"},
			{hotline.AccessViewDropBoxes, "View Drop Boxes"},
			{hotline.AccessMakeAlias, "Make Aliases"},
			{hotline.AccessUploadFolder, "Upload Folders"},
			{hotline.AccessDownloadFolder, "Download Folders"},
		},
	},
	{
		category: "Chat",
		bits: []accessBitInfo{
			{hotline.AccessReadChat, "Read Chat"},
			{hotline.AccessSendChat, "Send Chat"},
			{hotline.AccessOpenChat, "Initiate Private Chat"},
		},
	},
	{
		category: "User Maintenance",
		bits: []accessBitInfo{
			{hotline.AccessCreateUser, "Create Accounts"},
			{hotline.AccessDeleteUser, "Delete Accounts"},
			{hotline.AccessOpenUser, "Read Accounts"},
			{hotline.AccessModifyUser, "Modify Accounts"},
			{hotline.AccessDisconUser, "Disconnect Users"},
			{hotline.AccessCannotBeDiscon, "Cannot Be Disconnected"},
			{hotline.AccessGetClientInfo, "Get User Info"},
		},
	},
	{
		category: "News",
		bits: []accessBitInfo{
			{hotline.AccessNewsReadArt, "Read Articles"},
			{hotline.AccessNewsPostArt, "Post Articles"},
			{hotline.AccessNewsDeleteArt, "Delete Articles"},
			{hotline.AccessNewsCreateCat, "Create Categories"},
			{hotline.AccessNewsDeleteCat, "Delete Categories"},
			{hotline.AccessNewsCreateFldr, "Create Bundles"},
			{hotline.AccessNewsDeleteFldr, "Delete Bundles"},
		},
	},
	{
		category: "Messaging",
		bits: []accessBitInfo{
			{hotline.AccessBroadcast, "Broadcast"},
			{hotline.AccessSendPrivMsg, "Send Messages"},
		},
	},
	{
		category: "Miscellaneous",
		bits: []accessBitInfo{
			{hotline.AccessAnyName, "Use Any Name"},
			{hotline.AccessNoAgreement, "No Agreement"},
		},
	},
}

// Focus indices for account editor (beyond checkboxes)
const (
	focusLogin = 41 // Login field
	focusName  = 42 // Display name field
	focusPass  = 43 // Password field
)

// AccountsScreen is a self-contained BubbleTea model for managing user accounts
type AccountsScreen struct {
	// Bubble Tea components
	list     list.Model
	viewport viewport.Model

	// Screen dimensions
	width, height int

	// Reference to parent model for callbacks
	model *Model

	// Screen-specific state
	allAccounts      []accountItem        // Complete account dataset
	selectedAccount  *selectedAccountData // Currently selected account
	detailFocused    bool                 // true = detail pane focused, false = list pane focused
	isNewAccount     bool                 // Creating new vs editing existing
	editedLogin      string               // Working copy of login name
	editedName       string               // Working copy of display name
	editedPassword   string               // Working copy of password
	editedAccessBits hotline.AccessBitmap // Working copy of permissions
	focusedAccessBit int                  // Currently focused checkbox (0-40, or 41+ for other fields)
	passwordChanged  bool                 // Track if password was modified

	// User permissions
	userAccess hotline.AccessBitmap
}

// NewAccountsScreen creates a new accounts screen with the given account list
func NewAccountsScreen(accounts []accountItem, userAccess hotline.AccessBitmap, m *Model) *AccountsScreen {
	items := make([]list.Item, len(accounts))
	for i, acct := range accounts {
		items[i] = acct
	}

	l := list.New(items, newAccountDelegate(), m.width/2, m.height-10)
	l.Title = "User Accounts"
	l.SetFilteringEnabled(true)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()

	vp := viewport.New(m.width/2-4, m.height-12)

	return &AccountsScreen{
		list:        l,
		viewport:    vp,
		width:       m.width,
		height:      m.height,
		model:       m,
		allAccounts: accounts,
		userAccess:  userAccess,
	}
}

func newAccountDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles = style.ListItemStyles
	d.ShowDescription = true
	return d
}

// Init implements tea.Model
func (s *AccountsScreen) Init() tea.Cmd {
	return nil
}

// Update handles messages and returns updated screen + commands
func (s *AccountsScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.SetSize(msg.Width, msg.Height)
		return s, nil

	// Handle screen messages by delegating to parent methods
	case AccountsCancelledMsg:
		s.model.PopScreen()
		return s, nil

	case AccountsSaveMsg:
		cmd := s.model.handleAccountsSaveMsg(msg)
		return s, cmd

	case AccountsDeleteMsg:
		cmd := s.model.handleAccountsDeleteMsg(msg)
		return s, cmd

	case tea.KeyMsg:
		return s.handleKeys(msg)
	}

	// Delegate to internal components when not editing
	if s.selectedAccount == nil && !s.isNewAccount {
		var cmd tea.Cmd
		s.list, cmd = s.list.Update(msg)
		return s, cmd
	}

	return s, nil
}

// View renders the screen
func (s *AccountsScreen) View() string {
	if s.selectedAccount != nil || s.isNewAccount {
		return s.renderSplitView()
	}
	return s.renderListOnly()
}

// SetSize updates dimensions
func (s *AccountsScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.list.SetSize(width/2, height-10)
	s.viewport.Width = width/2 - 4
	s.viewport.Height = height - 12
}

// renderListOnly renders the accounts list without the detail pane
func (s *AccountsScreen) renderListOnly() string {
	content := s.list.View()

	// Add help text
	help := "\n\n"
	if s.userAccess.IsSet(hotline.AccessCreateUser) {
		help += "n: new account  "
	}
	help += "enter: view/edit  esc: close"

	return style.SubScreenStyle.Render(content + help)
}

// renderSplitView renders the accounts list alongside the detail pane
func (s *AccountsScreen) renderSplitView() string {
	canEdit := s.userAccess.IsSet(hotline.AccessModifyUser) || s.isNewAccount

	// Determine border styles based on focus
	leftBorderStyle := lipgloss.NormalBorder()
	rightBorderStyle := lipgloss.NormalBorder()
	if s.detailFocused {
		rightBorderStyle = lipgloss.DoubleBorder()
	} else {
		leftBorderStyle = lipgloss.DoubleBorder()
	}

	// Left pane: account list
	leftPane := lipgloss.NewStyle().
		Width(s.width / 2).
		Height(s.height - 10).
		BorderStyle(leftBorderStyle).
		BorderRight(true).
		Render(s.list.View())

	// Right pane: account details
	var rightContent strings.Builder

	if s.isNewAccount {
		rightContent.WriteString(style.TitleStyle.Render("New Account"))
	} else {
		rightContent.WriteString(style.TitleStyle.Render("Account: " + s.selectedAccount.login))
	}
	rightContent.WriteString("\n\n")

	// Account fields
	loginLabel := "Login: "
	if s.focusedAccessBit == focusLogin {
		loginLabel = "> " + loginLabel
	} else {
		loginLabel = "  " + loginLabel
	}
	rightContent.WriteString(loginLabel + s.editedLogin + "\n")

	nameLabel := "Name: "
	if s.focusedAccessBit == focusName {
		nameLabel = "> " + nameLabel
	} else {
		nameLabel = "  " + nameLabel
	}
	rightContent.WriteString(nameLabel + s.editedName + "\n")

	passLabel := "Password: "
	if s.focusedAccessBit == focusPass {
		passLabel = "> " + passLabel
	} else {
		passLabel = "  " + passLabel
	}
	passDisplay := s.editedPassword
	if len(passDisplay) == 0 {
		passDisplay = "(not set)"
	} else {
		passDisplay = strings.Repeat("*", len(passDisplay))
	}
	rightContent.WriteString(passLabel + passDisplay + "\n\n")

	// Access permissions by category
	focusIndex := 0
	for _, category := range accessBitsByCategory {
		rightContent.WriteString(style.CategoryStyle.Render(category.category))
		rightContent.WriteString("\n")

		for _, bit := range category.bits {
			checkbox := "[ ]"
			if s.editedAccessBits.IsSet(bit.bit) {
				checkbox = "[x]"
			}

			prefix := "  "
			if focusIndex == s.focusedAccessBit {
				prefix = "> "
			}

			itemStyle := lipgloss.NewStyle()
			if !canEdit {
				itemStyle = itemStyle.Foreground(style.CurrentTheme.TextDisabled)
			} else if focusIndex == s.focusedAccessBit {
				itemStyle = itemStyle.Bold(true)
			}

			rightContent.WriteString(itemStyle.Render(prefix + checkbox + " " + bit.name))
			rightContent.WriteString("\n")

			focusIndex++
		}
		rightContent.WriteString("\n")
	}

	// Set viewport content
	s.viewport.SetContent(rightContent.String())

	// Help text
	helpText := "\n"
	if canEdit {
		helpText += "tab: toggle focus  up/down: navigate  space: toggle  enter: save  pgup/pgdn: scroll"
		if !s.isNewAccount && s.userAccess.IsSet(hotline.AccessDeleteUser) {
			helpText += "  ctrl+d: delete"
		}
		helpText += "  esc: cancel"
	} else {
		helpText += "tab: toggle focus  esc: close (read-only)"
	}

	// Render right pane with viewport and border
	rightPane := lipgloss.NewStyle().
		Width(s.width/2 - 2).
		Height(s.height - 10).
		BorderStyle(rightBorderStyle).
		BorderLeft(true).
		Padding(1).
		Render(s.viewport.View())

	splitView := lipgloss.JoinHorizontal(
		lipgloss.Left,
		leftPane,
		rightPane,
	)

	return style.SubScreenStyle.Render(splitView + helpText)
}

// handleKeys handles keyboard input
func (s *AccountsScreen) handleKeys(msg tea.KeyMsg) (ScreenModel, tea.Cmd) {
	// List-only view
	if s.selectedAccount == nil && !s.isNewAccount {
		switch msg.String() {
		case "n":
			if s.userAccess.IsSet(hotline.AccessCreateUser) {
				s.isNewAccount = true
				s.editedLogin = ""
				s.editedName = ""
				s.editedPassword = ""
				s.editedAccessBits = hotline.AccessBitmap{}
				s.focusedAccessBit = focusLogin
				s.detailFocused = true
				return s, nil
			}
		case "enter":
			if item, ok := s.list.SelectedItem().(accountItem); ok {
				s.selectedAccount = &selectedAccountData{
					login:          item.login,
					name:           item.name,
					originalAccess: item.access,
					hasPassword:    item.hasPass,
				}
				s.editedLogin = item.login
				s.editedName = item.name
				s.editedPassword = ""
				s.editedAccessBits = item.access
				s.passwordChanged = false
				s.focusedAccessBit = 0
				s.detailFocused = true
				return s, nil
			}
		case "esc":
			return s, func() tea.Msg { return AccountsCancelledMsg{} }
		default:
			var cmd tea.Cmd
			s.list, cmd = s.list.Update(msg)
			return s, cmd
		}
		return s, nil
	}

	// Split view (editing account)
	canEdit := s.userAccess.IsSet(hotline.AccessModifyUser) || s.isNewAccount

	switch msg.String() {
	case "tab":
		// Toggle focus between list and detail panes
		s.detailFocused = !s.detailFocused
		return s, nil

	case "esc":
		s.selectedAccount = nil
		s.isNewAccount = false
		s.detailFocused = false
		return s, nil

	case "up":
		// Route based on focus
		if !s.detailFocused {
			// List focused - pass to list
			var cmd tea.Cmd
			s.list, cmd = s.list.Update(msg)
			return s, cmd
		}
		// Detail focused - navigate checkboxes/fields
		if canEdit && s.focusedAccessBit > 0 {
			s.focusedAccessBit--
			s.scrollToFocusedCheckbox()
		}
		return s, nil

	case "down":
		// Route based on focus
		if !s.detailFocused {
			// List focused - pass to list
			var cmd tea.Cmd
			s.list, cmd = s.list.Update(msg)
			return s, cmd
		}
		// Detail focused - navigate checkboxes/fields
		if canEdit && s.focusedAccessBit < focusPass {
			s.focusedAccessBit++
			s.scrollToFocusedCheckbox()
		}
		return s, nil

	case "pgup":
		// Manual viewport scrolling when detail pane focused
		if s.detailFocused {
			s.viewport.HalfPageUp()
		}
		return s, nil

	case "pgdown":
		// Manual viewport scrolling when detail pane focused
		if s.detailFocused {
			s.viewport.HalfPageDown()
		}
		return s, nil

	case " ", "space":
		if canEdit && s.detailFocused {
			// Map focus index to actual access bit
			focusIndex := 0
			for _, category := range accessBitsByCategory {
				for _, bit := range category.bits {
					if focusIndex == s.focusedAccessBit {
						// Toggle the checkbox
						if s.editedAccessBits.IsSet(bit.bit) {
							s.editedAccessBits[bit.bit/8] &^= 1 << uint(7-bit.bit%8)
						} else {
							s.editedAccessBits.Set(bit.bit)
						}
						return s, nil
					}
					focusIndex++
				}
			}
		}
		return s, nil

	case "enter":
		// If list focused, select account
		if !s.detailFocused {
			if item, ok := s.list.SelectedItem().(accountItem); ok {
				s.selectedAccount = &selectedAccountData{
					login:          item.login,
					name:           item.name,
					originalAccess: item.access,
					hasPassword:    item.hasPass,
				}
				s.editedLogin = item.login
				s.editedName = item.name
				s.editedPassword = ""
				s.editedAccessBits = item.access
				s.passwordChanged = false
				s.focusedAccessBit = 0
				s.detailFocused = true
				return s, nil
			}
		}
		// If detail focused and can edit, submit changes
		if canEdit && s.detailFocused {
			return s, func() tea.Msg {
				return AccountsSaveMsg{
					Login:           s.editedLogin,
					Name:            s.editedName,
					Password:        s.editedPassword,
					PasswordChanged: s.passwordChanged,
					AccessBits:      s.editedAccessBits,
					IsNew:           s.isNewAccount,
				}
			}
		}
		return s, nil

	case "ctrl+d":
		if !s.isNewAccount && s.userAccess.IsSet(hotline.AccessDeleteUser) && s.detailFocused {
			return s, func() tea.Msg {
				return AccountsDeleteMsg{Login: s.selectedAccount.login}
			}
		}
		return s, nil

	default:
		// Handle text input for focused fields (only when detail pane focused)
		if canEdit && s.detailFocused {
			switch s.focusedAccessBit {
			case focusLogin:
				if msg.Type == tea.KeyRunes {
					s.editedLogin += string(msg.Runes)
				} else if msg.Type == tea.KeyBackspace && len(s.editedLogin) > 0 {
					s.editedLogin = s.editedLogin[:len(s.editedLogin)-1]
				}
			case focusName:
				if msg.Type == tea.KeyRunes {
					s.editedName += string(msg.Runes)
				} else if msg.Type == tea.KeyBackspace && len(s.editedName) > 0 {
					s.editedName = s.editedName[:len(s.editedName)-1]
				}
			case focusPass:
				if msg.Type == tea.KeyRunes {
					s.editedPassword += string(msg.Runes)
					s.passwordChanged = true
				} else if msg.Type == tea.KeyBackspace && len(s.editedPassword) > 0 {
					s.editedPassword = s.editedPassword[:len(s.editedPassword)-1]
					s.passwordChanged = true
				}
			}
		}
	}

	return s, nil
}

// scrollToFocusedCheckbox scrolls the viewport to keep the focused item visible
func (s *AccountsScreen) scrollToFocusedCheckbox() {
	// Calculate line position of focused item
	// Account for header (3 lines), Login/Name/Password fields (5 lines including blank)
	const headerLines = 3
	const accountFieldLines = 5

	// Count lines up to focused item
	linePos := headerLines + accountFieldLines

	// If focused on checkbox (0-40), calculate its position
	if s.focusedAccessBit < 41 {
		// Count through categories to find line position
		currentBit := 0
		for _, category := range accessBitsByCategory {
			linePos++ // Category header line
			for range category.bits {
				if currentBit == s.focusedAccessBit {
					// Center the focused item in viewport
					centerOffset := s.viewport.Height / 2
					targetYOffset := linePos - centerOffset
					if targetYOffset < 0 {
						targetYOffset = 0
					}
					s.viewport.SetYOffset(targetYOffset)
					return
				}
				linePos++ // Checkbox line
				currentBit++
			}
			linePos++ // Blank line after category
		}
	} else {
		// Focused on text fields at top - scroll to top
		s.viewport.SetYOffset(0)
	}
}

// ResetEditState resets the editing state after save/delete operations
func (s *AccountsScreen) ResetEditState() {
	s.selectedAccount = nil
	s.isNewAccount = false
	s.detailFocused = false
}

// HandleListUsers handles the transaction response for listing user accounts
func (m *Model) HandleListUsers(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if m.checkTransactionError(t) {
		return nil, nil
	}

	var accounts []accountItem

	// Each FieldData contains one account
	for i, field := range t.Fields {
		if field.Type != hotline.FieldData {
			continue
		}

		var acct accountItem
		acct.index = i

		// Parse sub-fields from FieldData using scanner
		scanner := bufio.NewScanner(bytes.NewReader(field.Data[2:]))
		scanner.Split(hotline.FieldScanner)

		fieldCount := int(binary.BigEndian.Uint16(field.Data[0:2]))

		// Read each sub-field
		for j := 0; j < fieldCount; j++ {
			if !scanner.Scan() {
				break
			}

			var subField hotline.Field
			if _, err := subField.Write(scanner.Bytes()); err != nil {
				m.logger.Error("Error reading sub-field", "err", err)
				break
			}

			switch subField.Type {
			case hotline.FieldUserLogin:
				acct.login = string(hotline.EncodeString(subField.Data))
			case hotline.FieldUserName:
				acct.name = string(subField.Data)
			case hotline.FieldUserAccess:
				if len(subField.Data) >= 8 {
					copy(acct.access[:], subField.Data)
				}
			case hotline.FieldUserPassword:
				acct.hasPass = len(subField.Data) > 0
			}
		}

		accounts = append(accounts, acct)
	}

	m.program.Send(accountListMsg{accounts: accounts})
	return res, err
}

// submitAccountChanges submits account updates to the server
func (m *Model) submitAccountChanges(msg AccountsSaveMsg) tea.Cmd {
	return func() tea.Msg {
		// Build sub-fields
		subFields := []hotline.Field{
			hotline.NewField(hotline.FieldUserLogin,
				hotline.EncodeString([]byte(msg.Login))),
			hotline.NewField(hotline.FieldUserName, []byte(msg.Name)),
			hotline.NewField(hotline.FieldUserAccess, msg.AccessBits[:]),
		}

		// Handle password
		if msg.PasswordChanged {
			if len(msg.Password) > 0 {
				subFields = append(subFields,
					hotline.NewField(hotline.FieldUserPassword, []byte(msg.Password)))
			}
			// If password is empty and changed, don't include field (removes password)
		} else {
			// Keep existing password
			subFields = append(subFields,
				hotline.NewField(hotline.FieldUserPassword, []byte{0}))
		}

		// Serialize sub-fields
		var fieldData []byte
		subFieldCount := make([]byte, 2)
		binary.BigEndian.PutUint16(subFieldCount, uint16(len(subFields)))
		fieldData = append(fieldData, subFieldCount...)

		for _, field := range subFields {
			b, _ := io.ReadAll(&field)
			fieldData = append(fieldData, b...)
		}

		// Send transaction
		session := m.activeSession()
		if session == nil {
			return errorMsg{text: "No active server connection"}
		}
		if err := session.hlClient.Send(hotline.NewTransaction(
			hotline.TranUpdateUser,
			[2]byte{},
			hotline.NewField(hotline.FieldData, fieldData),
		)); err != nil {
			m.logger.Error("Error updating account", "err", err)
			return errorMsg{text: fmt.Sprintf("Error updating account: %v", err)}
		}

		m.logger.Info("Account updated successfully")

		return nil
	}
}

// deleteAccount deletes the specified account from the server
func (m *Model) deleteAccount(login string) tea.Cmd {
	return func() tea.Msg {
		session := m.activeSession()
		if session == nil {
			return errorMsg{text: "No active server connection"}
		}
		// For delete, send only FieldData with the login
		loginData := hotline.EncodeString([]byte(login))

		if err := session.hlClient.Send(hotline.NewTransaction(
			hotline.TranUpdateUser,
			[2]byte{},
			hotline.NewField(hotline.FieldData, loginData),
		)); err != nil {
			m.logger.Error("Error deleting account", "err", err)
			return errorMsg{text: fmt.Sprintf("Error deleting account: %v", err)}
		}

		m.logger.Info("Account deleted successfully")

		return nil
	}
}
