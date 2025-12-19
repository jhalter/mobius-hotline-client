package internal

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"slices"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

// accountSaveSuccessMsg signals account was saved successfully
type accountSaveSuccessMsg struct{}

// accountDeleteSuccessMsg signals account was deleted successfully
type accountDeleteSuccessMsg struct{}

// AccountsDeleteMsg signals user wants to delete an account
type AccountsDeleteMsg struct {
	Login string
}

// AccountEditRequestMsg signals user wants to edit an existing account
type AccountEditRequestMsg struct {
	Account accountItem
}

// AccountNewRequestMsg signals user wants to create a new account
type AccountNewRequestMsg struct{}

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

// accountsScreenKeyMap defines key bindings for the accounts screen
type accountsScreenKeyMap struct {
	New   key.Binding
	Enter key.Binding
	Esc   key.Binding
}

func (k accountsScreenKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.New, k.Enter, k.Esc}
}

func (k accountsScreenKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.New, k.Enter, k.Esc},
	}
}

// AccountsScreen is a self-contained BubbleTea model for managing user accounts
type AccountsScreen struct {
	// Bubble Tea components
	list list.Model

	// Screen dimensions
	width, height int

	// Reference to parent model for callbacks
	model *Model

	// Screen-specific state
	allAccounts []accountItem // Complete account dataset

	// User permissions
	userAccess hotline.AccessBitmap

	// Help system
	help help.Model
	keys accountsScreenKeyMap
}

// NewAccountsScreen creates a new accounts screen with the given account list
func NewAccountsScreen(accounts []accountItem, userAccess hotline.AccessBitmap, m *Model) *AccountsScreen {
	slices.SortFunc(accounts, func(a, b accountItem) int {
		return cmp.Compare(strings.ToLower(a.name), strings.ToLower(b.name))
	})

	items := make([]list.Item, len(accounts))
	for i, acct := range accounts {
		items[i] = acct
	}

	l := list.New(items, newAccountDelegate(), m.width-4, m.height-10)
	l.SetShowTitle(false)
	l.SetFilteringEnabled(true)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()

	keys := accountsScreenKeyMap{
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new account"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "view/edit"),
		),
		Esc: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "close"),
		),
	}

	// Configure key availability based on permissions
	keys.New.SetEnabled(userAccess.IsSet(hotline.AccessCreateUser))

	return &AccountsScreen{
		list:        l,
		width:       m.width,
		height:      m.height,
		model:       m,
		allAccounts: accounts,
		userAccess:  userAccess,
		help:        style.NewHelp(),
		keys:        keys,
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

	case AccountsCancelledMsg:
		s.model.PopScreen()
		return s, nil

	case tea.KeyPressMsg:
		return s.handleKeys(msg)
	}

	// Delegate to list component
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

// View renders the screen
func (s *AccountsScreen) View() tea.View {
	content := style.RenderSubscreen(s.width, s.height, "Accounts",
		lipgloss.JoinVertical(
			lipgloss.Left,
			s.list.View(),
			" ",
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				s.help.View(s.keys),
			),
		),
	)
	return tea.NewView(content)
}

// SetSize updates dimensions
func (s *AccountsScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.list.SetSize(width-4, height-10)
}

// UpdateAccounts refreshes the account list with new data
func (s *AccountsScreen) UpdateAccounts(accounts []accountItem) {
	slices.SortFunc(accounts, func(a, b accountItem) int {
		return cmp.Compare(strings.ToLower(a.name), strings.ToLower(b.name))
	})

	items := make([]list.Item, len(accounts))
	for i, acct := range accounts {
		items[i] = acct
	}
	s.list.SetItems(items)
	s.allAccounts = accounts
}

// handleKeys handles keyboard input
func (s *AccountsScreen) handleKeys(msg tea.KeyPressMsg) (ScreenModel, tea.Cmd) {
	switch msg.String() {
	case "n":
		if s.userAccess.IsSet(hotline.AccessCreateUser) {
			return s, func() tea.Msg { return AccountNewRequestMsg{} }
		}
	case "enter":
		if item, ok := s.list.SelectedItem().(accountItem); ok {
			return s, func() tea.Msg { return AccountEditRequestMsg{Account: item} }
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

		return accountSaveSuccessMsg{}
	}
}

// deleteAccount deletes the specified account from the server
func (m *Model) deleteAccount(login string) tea.Cmd {
	return func() tea.Msg {
		session := m.activeSession()
		if session == nil {
			return errorMsg{text: "No active server connection"}
		}
		if err := session.hlClient.Send(hotline.NewTransaction(
			hotline.TranDeleteUser,
			[2]byte{},
			hotline.NewField(hotline.FieldUserLogin, hotline.EncodeString([]byte(login))),
		)); err != nil {
			m.logger.Error("Error deleting account", "err", err)
			return errorMsg{text: fmt.Sprintf("Error deleting account: %v", err)}
		}

		m.logger.Info("Account deleted successfully")

		return accountDeleteSuccessMsg{}
	}
}
