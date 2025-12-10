package internal

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/jhalter/mobius-hotline-client/internal/style"
	"github.com/jhalter/mobius/hotline"
)

// AccountEditCancelledMsg signals user cancelled account editing
type AccountEditCancelledMsg struct{}

// accountEditScreenKeyMap defines key bindings for the account edit screen
type accountEditScreenKeyMap struct {
	Enter  key.Binding
	Delete key.Binding
	Esc    key.Binding
}

func (k accountEditScreenKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Delete, k.Esc}
}

func (k accountEditScreenKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Enter, k.Delete, k.Esc},
	}
}

// AccountEditScreen is a full-screen view for editing a single user account
type AccountEditScreen struct {
	width, height int
	model         *Model

	// Account data
	isNewAccount   bool
	originalLogin  string
	editedLogin    string
	editedName     string
	editedPassword string
	selectedPerms  []int // Selected permission bit numbers for MultiSelect

	// UI state
	form       *huh.Form
	userAccess hotline.AccessBitmap
	help       help.Model
	keys       accountEditScreenKeyMap
}

// buildAccountForm creates a huh form for Login, Name, Password fields and permissions MultiSelects
func buildAccountForm(login, name, password *string, selectedPerms *[]int, accessBits hotline.AccessBitmap, formHeight int) *huh.Form {
	fields := []huh.Field{
		huh.NewInput().
			Key("login").
			Title("Login").
			Value(login),

		huh.NewInput().
			Key("name").
			Title("Name").
			Value(name),

		huh.NewInput().
			Key("password").
			Title("Password").
			EchoMode(huh.EchoModePassword).
			Value(password),
	}

	// Add a MultiSelect for each permission category
	for _, category := range accessBitsByCategory {
		var permOptions []huh.Option[int]
		for _, bit := range category.bits {
			opt := huh.NewOption(bit.name, bit.bit)
			if accessBits.IsSet(bit.bit) {
				opt = opt.Selected(true)
			}
			permOptions = append(permOptions, opt)
		}

		height := len(category.bits) + 1
		if height > 17 {
			height = 17
		}

		fields = append(fields,
			huh.NewMultiSelect[int]().
				Key("perms_"+category.category).
				Title(category.category).
				Options(permOptions...).
				Value(selectedPerms).
				Height(height),
		)
	}

	return huh.NewForm(huh.NewGroup(fields...)).
		WithWidth(60).
		WithHeight(formHeight).
		WithShowHelp(true).
		WithShowErrors(true).
		WithKeyMap(enterSubmitsKeyMap()).
		WithTheme(style.FormTheme)
}

// NewAccountEditScreen creates a new account edit screen
func NewAccountEditScreen(account *accountItem, userAccess hotline.AccessBitmap, m *Model) (*AccountEditScreen, tea.Cmd) {
	isNewAccount := account == nil
	canEdit := userAccess.IsSet(hotline.AccessModifyUser) || isNewAccount

	keys := accountEditScreenKeyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "save"),
		),
		Delete: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("^D", "delete"),
		),
		Esc: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
	}

	// Configure key availability based on permissions
	keys.Enter.SetEnabled(canEdit)
	keys.Delete.SetEnabled(!isNewAccount && userAccess.IsSet(hotline.AccessDeleteUser))

	screen := &AccountEditScreen{
		width:      m.width,
		height:     m.height,
		model:      m,
		userAccess: userAccess,
		help:       help.New(),
		keys:       keys,
	}

	var accessBits hotline.AccessBitmap
	if account != nil {
		// Editing existing account
		screen.isNewAccount = false
		screen.originalLogin = account.login
		screen.editedLogin = account.login
		screen.editedName = account.name
		screen.editedPassword = ""
		accessBits = account.access
	} else {
		// Creating new account
		screen.isNewAccount = true
		screen.editedLogin = ""
		screen.editedName = ""
		screen.editedPassword = ""
		accessBits = hotline.AccessBitmap{}
	}

	// Initialize selectedPerms from accessBits
	screen.selectedPerms = []int{}
	for _, category := range accessBitsByCategory {
		for _, bit := range category.bits {
			if accessBits.IsSet(bit.bit) {
				screen.selectedPerms = append(screen.selectedPerms, bit.bit)
			}
		}
	}

	// Build form with pointers to edited values
	// Reserve space for: title (1), borders (2), padding (2), help bar (1)
	formHeight := m.height - 10
	screen.form = buildAccountForm(&screen.editedLogin, &screen.editedName, &screen.editedPassword,
		&screen.selectedPerms, accessBits, formHeight)

	return screen, screen.form.Init()
}

// Init implements tea.Model
func (s *AccountEditScreen) Init() tea.Cmd {
	return s.form.Init()
}

// Update handles messages and returns updated screen + commands
func (s *AccountEditScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		// Pass to form so it can adjust its internal layout
		form, cmd := s.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			s.form = f
		}
		return s, cmd

	case tea.KeyMsg:
		return s.handleKeys(msg)
	}

	// Delegate other messages to the form
	form, cmd := s.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		s.form = f
	}

	return s, cmd
}

// View renders the screen
func (s *AccountEditScreen) View() string {
	var title string
	if s.isNewAccount {
		title = "New Account"
	} else {
		title = fmt.Sprintf("Edit Account: %s", s.originalLogin)
	}

	return style.RenderSubscreen(s.width, s.height, title,
		s.form.View()+"\n"+s.help.View(s.keys))
}

// SetSize updates dimensions
func (s *AccountEditScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// handleKeys handles keyboard input
func (s *AccountEditScreen) handleKeys(msg tea.KeyMsg) (ScreenModel, tea.Cmd) {
	canEdit := s.userAccess.IsSet(hotline.AccessModifyUser) || s.isNewAccount

	// ESC always cancels
	if msg.String() == "esc" {
		return s, func() tea.Msg { return AccountEditCancelledMsg{} }
	}

	// Enter always saves (when editable)
	if msg.String() == "enter" && canEdit {
		// Update form to commit current field value
		form, _ := s.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			s.form = f
		}
		// Convert selectedPerms slice to AccessBitmap
		accessBits := hotline.AccessBitmap{}
		for _, bit := range s.selectedPerms {
			accessBits.Set(bit)
		}
		// Check if password was changed (non-empty means changed for existing accounts)
		passwordChanged := s.editedPassword != ""
		return s, func() tea.Msg {
			return AccountsSaveMsg{
				Login:           s.editedLogin,
				Name:            s.editedName,
				Password:        s.editedPassword,
				PasswordChanged: passwordChanged,
				AccessBits:      accessBits,
				IsNew:           s.isNewAccount,
			}
		}
	}

	// Delete account
	if msg.String() == "ctrl+d" {
		if !s.isNewAccount && s.userAccess.IsSet(hotline.AccessDeleteUser) {
			return s, func() tea.Msg {
				return AccountsDeleteMsg{Login: s.originalLogin}
			}
		}
		return s, nil
	}

	// Delegate all other keys to the form
	form, cmd := s.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		s.form = f
	}

	return s, cmd
}
