package internal

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/jhalter/mobius/hotline"
	"gopkg.in/yaml.v3"
)

// Screen types
type Screen int

// ScreenModel is the interface that all screens must implement
type ScreenModel interface {
	Update(tea.Msg) (ScreenModel, tea.Cmd)
	View() string
}

const (
	ScreenHome Screen = iota
	ScreenJoinServer
	ScreenBookmarks
	ScreenTracker
	ScreenSettings
	ScreenServerUI
	ScreenNews
	ScreenNewsArticlePost
	ScreenNewsArticleView
	ScreenNewsBundleForm
	ScreenNewsCategoryForm
	ScreenLegacyNewsPost
	ScreenMessageBoard
	ScreenFiles
	ScreenLogs
	ScreenModal
	ScreenTasks
	ScreenAccounts
	ScreenAccountEdit
	ScreenComposeMessage
	ScreenFilePicker
	ScreenLoading
	ScreenQuickMenu
)

// Model
type Model struct {
	program *tea.Program

	// Configuration
	cfgPath     string
	prefs       *Settings
	logger      *slog.Logger
	debugBuffer *DebugBuffer
	soundPlayer *SoundPlayer

	msgHandlers map[reflect.Type]msgHandler

	// Screen state (for pre-connection/global screens)
	screenHistory []Screen // Stack of screens, current screen is last element

	width         int
	height        int
	welcomeBanner string // Randomly selected banner, loaded once at startup

	// Multi-server support
	sessions           []*ServerSession // Active server sessions (max 10)
	activeSessionIndex int              // Index of currently active session (-1 = no session)

	// Pending connection state (before session is created)
	pendingServerName string // Name to display when connection succeeds (from bookmark/tracker/address)
	pendingServerAddr string // Address being connected to

	// Global/pre-connection screens
	homeScreen       *HomeScreen
	joinServerScreen *JoinServerScreen
	bookmarkScreen   *BookmarkScreen
	trackerScreen    *TrackerScreen
	settingsScreen   *SettingsScreen
	tasksScreen      *TasksScreen
	logsScreen       *LogsScreen
	filePickerScreen *FilePickerScreen
	modalScreen      *ModalScreen
	loadingScreen    *LoadingScreen
	quickMenuScreen  *QuickMenuScreen

	// File picker state
	lastPickerLocation string // Remember last location

	// Task management for file downloads and uploads (global - spans all sessions)
	taskManager  *TaskManager
	downloadDir  string
	taskProgress map[string]progress.Model // task ID -> progress model
}

// CurrentScreen returns the current screen, or ScreenHome if history is empty
func (m *Model) CurrentScreen() Screen {
	if len(m.screenHistory) == 0 {
		return ScreenHome
	}
	return m.screenHistory[len(m.screenHistory)-1]
}

// PreviousScreen returns the previous screen, or ScreenHome if insufficient history
func (m *Model) PreviousScreen() Screen {
	if len(m.screenHistory) < 2 {
		return ScreenHome
	}
	return m.screenHistory[len(m.screenHistory)-2]
}

// PushScreen adds a new screen to history (modal/overlay pattern)
func (m *Model) PushScreen(screen Screen) {
	m.screenHistory = append(m.screenHistory, screen)
	// Also update session's screen history for server-specific screens
	if session := m.activeSession(); session != nil && isServerScreen(screen) {
		session.PushScreen(screen)
	}
}

// PopScreen removes current screen and returns to previous
// Returns the screen we're now on
func (m *Model) PopScreen() Screen {
	if len(m.screenHistory) <= 1 {
		m.screenHistory = []Screen{ScreenHome}
		return ScreenHome
	}
	popped := m.screenHistory[len(m.screenHistory)-1]
	m.screenHistory = m.screenHistory[:len(m.screenHistory)-1]
	// Also update session's screen history for server-specific screens
	if session := m.activeSession(); session != nil && isServerScreen(popped) {
		session.PopScreen()
	}
	return m.screenHistory[len(m.screenHistory)-1]
}

// ReplaceScreen replaces the current screen without adding to history
// Used when switching between peer screens (e.g., News -> MessageBoard from ServerUI)
func (m *Model) ReplaceScreen(screen Screen) {
	if len(m.screenHistory) == 0 {
		m.screenHistory = []Screen{screen}
	} else {
		m.screenHistory[len(m.screenHistory)-1] = screen
	}
	// Also update session's screen history for server-specific screens
	if session := m.activeSession(); session != nil && isServerScreen(screen) {
		session.ReplaceScreen(screen)
	}
}

// NavigateTo clears history and jumps to a screen (hard navigation)
// Used for disconnect, logout, or other full resets
func (m *Model) NavigateTo(screen Screen) {
	m.screenHistory = []Screen{screen}
	// Also update session's screen history for server-specific screens
	if session := m.activeSession(); session != nil && isServerScreen(screen) {
		session.NavigateTo(screen)
	}
}

// activeSession returns the currently active server session, or nil if none
func (m *Model) activeSession() *ServerSession {
	if m.activeSessionIndex < 0 || m.activeSessionIndex >= len(m.sessions) {
		return nil
	}
	return m.sessions[m.activeSessionIndex]
}

// getSessionByID returns the session with the given ID, or nil if not found
func (m *Model) getSessionByID(id string) *ServerSession {
	for _, s := range m.sessions {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// getSessionIndex returns the index of the session with the given ID, or -1 if not found
func (m *Model) getSessionIndex(id string) int {
	for i, s := range m.sessions {
		if s.ID == id {
			return i
		}
	}
	return -1
}

// switchToSession switches to the session at the given index
func (m *Model) switchToSession(index int) {
	if index < 0 || index >= len(m.sessions) {
		return
	}
	m.activeSessionIndex = index
	// Clear unread indicator for the session we're switching to
	m.sessions[index].hasUnreadActivity = false
}

// switchToNextSession cycles to the next session
func (m *Model) switchToNextSession() {
	if len(m.sessions) == 0 {
		return
	}
	m.activeSessionIndex = (m.activeSessionIndex + 1) % len(m.sessions)
	m.sessions[m.activeSessionIndex].hasUnreadActivity = false
}

// switchToPreviousSession cycles to the previous session
func (m *Model) switchToPreviousSession() {
	if len(m.sessions) == 0 {
		return
	}
	m.activeSessionIndex--
	if m.activeSessionIndex < 0 {
		m.activeSessionIndex = len(m.sessions) - 1
	}
	m.sessions[m.activeSessionIndex].hasUnreadActivity = false
}

// removeSession removes the session at the given index and adjusts activeSessionIndex
func (m *Model) removeSession(index int) {
	if index < 0 || index >= len(m.sessions) {
		return
	}

	// Clean up the session
	m.sessions[index].Disconnect()

	// Remove from slice
	m.sessions = append(m.sessions[:index], m.sessions[index+1:]...)

	// Adjust active index
	if len(m.sessions) == 0 {
		m.activeSessionIndex = -1
	} else if m.activeSessionIndex >= len(m.sessions) {
		m.activeSessionIndex = len(m.sessions) - 1
	} else if m.activeSessionIndex > index {
		m.activeSessionIndex--
	}
}

// hasActiveSessions returns true if there is at least one active session
func (m *Model) hasActiveSessions() bool {
	return len(m.sessions) > 0
}

// updatePrivateMessageModal creates or updates the modal for the current PM stack
// Shows the top message with count indicator if multiple messages are pending
func (m *Model) updatePrivateMessageModal() {
	session := m.activeSession()
	if session == nil || len(session.privateMessages) == 0 {
		return
	}

	// Get current message (top of stack - most recent)
	current := session.privateMessages[len(session.privateMessages)-1]

	// Build title with count if multiple messages
	title := "Private Message from " + current.From
	if len(session.privateMessages) > 1 {
		title += fmt.Sprintf(" (1 of %d)", len(session.privateMessages))
	}

	content := current.Text + "\n\nAt " + current.Time

	m.modalScreen = NewModalScreen(ModalTypePrivateMessage, title, content, []string{"Close", "Reply"}, m)
}

// isShowingPrivateMessageModal returns true if the current screen is a PM modal
func (m *Model) isShowingPrivateMessageModal() bool {
	return m.CurrentScreen() == ScreenModal &&
		m.modalScreen != nil &&
		m.modalScreen.modalType == ModalTypePrivateMessage
}

// currentScreen returns the current screen as a ScreenModel interface
func (m *Model) currentScreen() ScreenModel {
	screen := m.CurrentScreen()

	// Check if this is a server-specific screen that belongs to the active session
	if session := m.activeSession(); session != nil && isServerScreen(screen) {
		return session.currentScreenModel()
	}

	// Global/pre-connection screens
	switch screen {
	case ScreenHome:
		return m.homeScreen
	case ScreenJoinServer:
		return m.joinServerScreen
	case ScreenTracker:
		return m.trackerScreen
	case ScreenBookmarks:
		return m.bookmarkScreen
	case ScreenSettings:
		return m.settingsScreen
	case ScreenTasks:
		return m.tasksScreen
	case ScreenLogs:
		return m.logsScreen
	case ScreenFilePicker:
		return m.filePickerScreen
	case ScreenModal:
		return m.modalScreen
	case ScreenLoading:
		return m.loadingScreen
	case ScreenQuickMenu:
		return m.quickMenuScreen
	}
	return nil
}

func NewModel(cfgPath string, logger *slog.Logger, db *DebugBuffer) *Model {
	prefs, err := readConfig(cfgPath)
	if err != nil {
		logger.Error(fmt.Sprintf("unable to read config file %s\n", cfgPath))
		os.Exit(1)
	}

	// Initialize download directory
	downloadDir := prefs.DownloadDir
	if downloadDir == "" {
		home, _ := os.UserHomeDir()
		downloadDir = home + "/Downloads/Hotline"
	}

	// Initialize last picker location
	startDir, _ := os.UserHomeDir()

	// Initialize sound player
	soundPlayer, err := NewSoundPlayer(prefs.EnableSounds)
	if err != nil {
		logger.Error("Failed to initialize sound player", "err", err)
	}

	return &Model{
		msgHandlers:        make(map[reflect.Type]msgHandler),
		cfgPath:            cfgPath,
		prefs:              prefs,
		logger:             logger,
		debugBuffer:        db,
		soundPlayer:        soundPlayer,
		welcomeBanner:      randomBanner(), // Load banner once at startup
		sessions:           make([]*ServerSession, 0, MaxSessions),
		activeSessionIndex: -1, // No active session initially
		taskManager:        NewTaskManager(),
		downloadDir:        downloadDir,
		lastPickerLocation: startDir,
		taskProgress:       make(map[string]progress.Model),
		screenHistory:      []Screen{ScreenHome},
	}
}

func readConfig(cfgPath string) (*Settings, error) {
	fh, err := os.Open(cfgPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = fh.Close()
	}()

	var prefs Settings
	decoder := yaml.NewDecoder(fh)
	if err := decoder.Decode(&prefs); err != nil {
		return nil, err
	}
	return &prefs, nil
}

func (m *Model) Init() tea.Cmd {
	// Initialize home screen
	m.homeScreen = NewHomeScreen(m)

	m.registerHandler(tea.WindowSizeMsg{}, m.handleWindowResize)
	m.registerHandler(chatMsg{}, m.handleChatMsgfunc)
	m.registerHandler(userListMsg{}, m.handleUserListMsg)
	m.registerHandler(messageBoardMsg{}, m.handleMessageBoardMsg)
	m.registerHandler(errorMsg{}, m.handleErrorMsg)
	m.registerHandler(serverMsgMsg{}, m.handleServerMsgMsg)
	m.registerHandler(agreementMsg{}, m.handleAgreementMsg)
	m.registerHandler(serverConnectedMsg{}, m.handleServerConnectedMsg)
	m.registerHandler(serverConnectionAttemptMsg{}, m.handleServerConnectionAttemptMsg)
	m.registerHandler(trackerListMsg{}, m.handleTrackerListMsg)
	m.registerHandler(SettingsSavedMsg{}, m.handleSettingsSavedMsg)
	m.registerHandler(SettingsCancelledMsg{}, m.handleSettingsCancelledMsg)

	m.registerHandler(filesMsg{}, m.handleFilesMsg)
	m.registerHandler(newsCategoriesMsg{}, m.handleNewsCategoriesMsg)
	m.registerHandler(newsArticlesMsg{}, m.handleNewsArticlesMsg)
	m.registerHandler(newsArticleDataMsg{}, m.handleNewsArticleDataMsg)
	m.registerHandler(fileInfoMsg{}, m.handleFileInfoMsg)
	m.registerHandler(accountListMsg{}, m.handleAccountListMsg)
	m.registerHandler(AccountEditRequestMsg{}, m.handleAccountEditRequestMsg)
	m.registerHandler(AccountNewRequestMsg{}, m.handleAccountNewRequestMsg)
	m.registerHandler(AccountEditCancelledMsg{}, m.handleAccountEditCancelledMsg)
	m.registerHandler(AccountsSaveMsg{}, m.handleAccountsSaveMsg)
	m.registerHandler(accountSaveSuccessMsg{}, m.handleAccountSaveSuccessMsg)
	m.registerHandler(AccountsDeleteMsg{}, m.handleAccountsDeleteMsg)
	m.registerHandler(accountDeleteSuccessMsg{}, m.handleAccountDeleteSuccessMsg)
	m.registerHandler(taskProgressMsg{}, m.handleTaskProgressMsg)
	m.registerHandler(taskStatusMsg{}, m.handleTaskStatusMsg)
	m.registerHandler(downloadReplyMsg{}, m.handleDownloadReplyMsg)
	m.registerHandler(uploadReplyMsg{}, m.handleUploadReplyMsg)
	m.registerHandler(ModalButtonClickedMsg{}, m.handleModalButtonClickedMsgHandler)
	m.registerHandler(ModalCancelledMsg{}, m.handleModalCancelledMsgHandler)
	m.registerHandler(LoadingCancelledMsg{}, m.handleLoadingCancelledMsgHandler)

	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.logger.Debug("Update UI", "tea.Msg", fmt.Sprintf("%v", msg), "currentScreen", m.CurrentScreen())

	// Handle global keybindings
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+q":
			return m, tea.Quit
		case "ctrl+l":
			m.logsScreen = NewLogsScreen(m.debugBuffer, m)
			m.PushScreen(ScreenLogs)
			return m, nil
		// Tab switching shortcuts (F1 through F9, F10 for 10th)
		case "f1":
			m.switchToSession(0)
			return m, nil
		case "f2":
			m.switchToSession(1)
			return m, nil
		case "f3":
			m.switchToSession(2)
			return m, nil
		case "f4":
			m.switchToSession(3)
			return m, nil
		case "f5":
			m.switchToSession(4)
			return m, nil
		case "f6":
			m.switchToSession(5)
			return m, nil
		case "f7":
			m.switchToSession(6)
			return m, nil
		case "f8":
			m.switchToSession(7)
			return m, nil
		case "f9":
			m.switchToSession(8)
			return m, nil
		case "f10":
			m.switchToSession(9)
			return m, nil
		case "ctrl+k":
			// Show quick menu only when NOT on a server screen (which uses ctrl+n for News)
			m.quickMenuScreen = NewQuickMenuScreen(m)
			m.PushScreen(ScreenQuickMenu)
			return m, nil

		case "ctrl+tab":
			m.switchToNextSession()
			return m, nil
		case "ctrl+shift+tab":
			m.switchToPreviousSession()
			return m, nil
		}
	}

	// Handle session disconnect
	if disconnectMsg, ok := msg.(disconnectSessionMsg); ok {
		session := m.getSessionByID(disconnectMsg.sessionID)
		if session == nil {
			return m, nil
		}

		// Only show error if client didn't initiate disconnect
		var cmd tea.Cmd
		if !session.clientDisconnecting {
			cmd = func() tea.Msg {
				return errorMsg{text: "Server connection closed."}
			}
		}

		// Remove the session
		sessionIndex := m.getSessionIndex(disconnectMsg.sessionID)
		if sessionIndex >= 0 {
			m.removeSession(sessionIndex)
		}

		// If no more sessions, navigate to home
		if len(m.sessions) == 0 {
			m.NavigateTo(ScreenHome)
		}

		return m, cmd
	}

	// Check if we have a registered handler for this message type
	msgType := reflect.TypeOf(msg)
	if handler, ok := m.msgHandlers[msgType]; ok {
		return handler(msg)
	}

	if screen := m.currentScreen(); screen != nil {
		_, cmd := screen.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleModalCancelledMsgHandler wraps handleModalCancelledMsg for the msgHandler signature
func (m *Model) handleModalCancelledMsgHandler(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, m.handleModalCancelledMsg()
}

// handleModalButtonClickedMsgHandler wraps handleModalButtonClickedMsg for the msgHandler signature
func (m *Model) handleModalButtonClickedMsgHandler(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, m.handleModalButtonClickedMsg(msg.(ModalButtonClickedMsg))
}

// handleModalCancelledMsg handles when the modal is cancelled (ESC pressed)
func (m *Model) handleModalCancelledMsg() tea.Cmd {
	// If this is a PM modal, pop from the PM stack
	if session := m.activeSession(); session != nil && m.isShowingPrivateMessageModal() && len(session.privateMessages) > 0 {
		session.privateMessages = session.privateMessages[:len(session.privateMessages)-1]

		// Check if there are more PMs pending
		if len(session.privateMessages) > 0 {
			m.updatePrivateMessageModal()
			return m.modalScreen.Init()
		}
	}

	m.PopScreen()
	return nil
}

// handleLoadingCancelledMsgHandler handles when the loading screen is cancelled (ESC pressed)
func (m *Model) handleLoadingCancelledMsgHandler(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.PopScreen()
	return m, nil
}

// handleModalButtonClickedMsg handles modal button clicks
func (m *Model) handleModalButtonClickedMsg(msg ModalButtonClickedMsg) tea.Cmd {
	session := m.activeSession()

	// Handle different modal types
	switch msg.Type {
	case ModalTypeAgreement:
		if session == nil {
			m.PopScreen()
			return nil
		}
		if msg.ButtonClicked == "Agree" {
			// User agreed - send TranAgreed asynchronously
			return func() tea.Msg {
				_ = session.hlClient.Send(hotline.NewTransaction(
					hotline.TranAgreed,
					[2]byte{},
					hotline.NewField(hotline.FieldUserName, []byte(m.prefs.Username)),
					hotline.NewField(hotline.FieldUserIconID, m.prefs.IconBytes()),
					hotline.NewField(hotline.FieldUserFlags, []byte{0x00, 0x00}),
					hotline.NewField(hotline.FieldOptions, []byte{0x00, 0x00}),
				))
				return nil
			}
		}
		// User disagreed - disconnect and return to home
		session.clientDisconnecting = true
		sessionIndex := m.getSessionIndex(session.ID)
		if sessionIndex >= 0 {
			m.removeSession(sessionIndex)
		}
		if len(m.sessions) == 0 {
			m.NavigateTo(ScreenHome)
		} else {
			m.NavigateTo(ScreenServerUI)
		}
		return nil

	case ModalTypeDisconnect:
		if session == nil {
			m.PopScreen()
			return nil
		}
		if msg.ButtonClicked == "Exit" {
			// Signal that client is initiating disconnect
			session.clientDisconnecting = true
			sessionIndex := m.getSessionIndex(session.ID)
			if sessionIndex >= 0 {
				m.removeSession(sessionIndex)
			}
			if len(m.sessions) == 0 {
				m.NavigateTo(ScreenHome)
			} else {
				m.NavigateTo(ScreenServerUI)
			}
			return nil
		}
		// Cancel - return to previous screen
		m.PopScreen()

	case ModalTypePrivateMessage:
		if session == nil {
			m.PopScreen()
			return nil
		}
		// Get and pop current message from stack
		if len(session.privateMessages) > 0 {
			current := session.privateMessages[len(session.privateMessages)-1]
			session.privateMessages = session.privateMessages[:len(session.privateMessages)-1]

			if msg.ButtonClicked == "Reply" {
				// Reply button clicked - open compose screen with current message data
				var cmd tea.Cmd
				session.composeMessageScreen, cmd = NewComposeMessageScreen(
					current.UserID,
					current.From,
					current.Text,
					m,
				)
				m.ReplaceScreen(ScreenComposeMessage)
				return cmd
			}
		}

		// Close button: check if more PMs pending
		if len(session.privateMessages) > 0 {
			m.updatePrivateMessageModal()
			return m.modalScreen.Init()
		}
		m.PopScreen()

	default:
		// Generic modal (errors, file info, etc.) - return to previous screen
		m.PopScreen()
	}

	return nil
}

func (m *Model) View() string {
	screen := m.currentScreen()
	if screen == nil {
		return ""
	}

	screenContent := lipgloss.NewStyle().
		//Background(style.CurrentTheme.BackgroundPanel).
		Render(screen.View())

	// Show tab bar when connected to servers and on a server-related screen
	if len(m.sessions) > 0 && isServerScreen(m.CurrentScreen()) {
		tabBar := m.RenderTabBar()
		return lipgloss.JoinVertical(lipgloss.Left, tabBar, screenContent)
	}

	return screenContent
}

func (m *Model) initiateFileUpload(localPath string) tea.Cmd {
	return func() tea.Msg {
		session := m.activeSession()
		if session == nil {
			return errorMsg{text: "No active server connection"}
		}

		// Get file info
		fileInfo, err := os.Stat(localPath)
		if err != nil {
			return errorMsg{text: fmt.Sprintf("Failed to access file: %v", err)}
		}

		if fileInfo.IsDir() {
			return errorMsg{text: "Folder uploads not yet supported"}
		}

		fileName := filepath.Base(localPath)

		// Get file path from files screen
		var filePath []string
		if session.filesScreen != nil {
			filePath = session.filesScreen.GetFilePath()
		}

		// Create task
		task := &Task{
			ID:         uuid.New().String(),
			FileName:   fileName,
			FilePath:   filePath, // Upload to current directory in Files screen
			Status:     TaskPending,
			TotalBytes: fileInfo.Size(),
			StartTime:  time.Now(),
			LocalPath:  localPath,
		}
		m.taskManager.Add(task)

		// Create upload transaction
		sizeBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(sizeBytes, uint32(fileInfo.Size()))

		fields := []hotline.Field{
			hotline.NewField(hotline.FieldFileName, []byte(fileName)),
			hotline.NewField(hotline.FieldTransferSize, sizeBytes),
		}

		// Add file path if uploading to subfolder
		if len(filePath) > 0 {
			pathStr := strings.Join(filePath, "/")
			pathBytes := hotline.EncodeFilePath(pathStr)
			fields = append(fields, hotline.NewField(hotline.FieldFilePath, pathBytes))
		}

		t := hotline.NewTransaction(hotline.TranUploadFile, [2]byte{}, fields...)

		// Map transaction ID to task ID (on session)
		session.pendingUploads[t.ID] = task.ID

		// Send transaction
		if err := session.hlClient.Send(t); err != nil {
			m.logger.Error("Failed to send upload transaction", "err", err)
			return errorMsg{text: fmt.Sprintf("Failed to initiate upload: %v", err)}
		}

		m.logger.Info("Upload initiated", "file", fileName, "size", fileInfo.Size())

		return taskStatusMsg{
			taskID: task.ID,
			status: TaskPending,
		}
	}
}

func (m *Model) Start() error {
	// Store program reference for sending messages from transaction handlers
	m.program = tea.NewProgram(m, tea.WithAltScreen())

	// Transaction handlers are now registered per-session in registerSessionHandlers()

	_, err := m.program.Run()
	return err
}

// registerSessionHandlers registers all transaction handlers for a session's client
func (m *Model) registerSessionHandlers(session *ServerSession) {
	c := session.hlClient
	sid := session.ID

	c.HandleFunc(hotline.TranAgreed, m.makeSessionHandler(sid, m.HandleTranAgreed))
	c.HandleFunc(hotline.TranChatMsg, m.makeSessionHandler(sid, m.HandleClientChatMsg))
	c.HandleFunc(hotline.TranDownloadFile, m.makeSessionHandler(sid, m.HandleDownloadFile))
	c.HandleFunc(hotline.TranGetFileInfo, m.makeSessionHandler(sid, m.HandleGetFileInfo))
	c.HandleFunc(hotline.TranGetFileNameList, m.makeSessionHandler(sid, m.HandleGetFileNameList))
	c.HandleFunc(hotline.TranGetMsgs, m.makeSessionHandler(sid, m.TranGetMsgs))
	c.HandleFunc(hotline.TranGetNewsArtData, m.makeSessionHandler(sid, m.HandleGetNewsArtData))
	c.HandleFunc(hotline.TranGetNewsArtNameList, m.makeSessionHandler(sid, m.HandleGetNewsArtNameList))
	c.HandleFunc(hotline.TranGetNewsCatNameList, m.makeSessionHandler(sid, m.HandleGetNewsCatNameList))
	c.HandleFunc(hotline.TranGetUserNameList, m.makeSessionHandler(sid, m.HandleClientGetUserNameList))
	c.HandleFunc(hotline.TranKeepAlive, m.makeSessionHandler(sid, m.HandleKeepAlive))
	c.HandleFunc(hotline.TranListUsers, m.makeSessionHandler(sid, m.HandleListUsers))
	c.HandleFunc(hotline.TranLogin, m.makeSessionHandler(sid, m.HandleClientTranLogin))
	c.HandleFunc(hotline.TranNewMsg, m.makeSessionHandler(sid, m.HandleNewMsg))
	c.HandleFunc(hotline.TranNewNewsCat, m.makeSessionHandler(sid, m.HandleNewNewsCat))
	c.HandleFunc(hotline.TranNewNewsFldr, m.makeSessionHandler(sid, m.HandleNewNewsFldr))
	c.HandleFunc(hotline.TranNotifyChangeUser, m.makeSessionHandler(sid, m.HandleNotifyChangeUser))
	c.HandleFunc(hotline.TranNotifyChatDeleteUser, m.makeSessionHandler(sid, m.HandleNotifyDeleteUser))
	c.HandleFunc(hotline.TranNotifyDeleteUser, m.makeSessionHandler(sid, m.HandleNotifyDeleteUser))
	c.HandleFunc(hotline.TranPostNewsArt, m.makeSessionHandler(sid, m.HandlePostNewsArt))
	c.HandleFunc(hotline.TranServerMsg, m.makeSessionHandler(sid, m.HandleTranServerMsg))
	c.HandleFunc(hotline.TranShowAgreement, m.makeSessionHandler(sid, m.HandleClientTranShowAgreement))
	c.HandleFunc(hotline.TranUploadFile, m.makeSessionHandler(sid, m.HandleUploadFile))
	c.HandleFunc(hotline.TranUserAccess, m.makeSessionHandler(sid, m.HandleClientTranUserAccess))
}

// makeSessionHandler wraps a transaction handler to include session context
// The handler will receive the session ID in the context
func (m *Model) makeSessionHandler(sessionID string, handler func(context.Context, *hotline.Client, *hotline.Transaction) ([]hotline.Transaction, error)) func(context.Context, *hotline.Client, *hotline.Transaction) ([]hotline.Transaction, error) {
	return func(ctx context.Context, c *hotline.Client, t *hotline.Transaction) ([]hotline.Transaction, error) {
		// Store session ID in context for handlers to access
		ctx = context.WithValue(ctx, sessionIDKey, sessionID)
		return handler(ctx, c, t)
	}
}

// sessionIDKey is the context key for session ID
type sessionIDKeyType struct{}

var sessionIDKey = sessionIDKeyType{}

// getSessionIDFromContext extracts the session ID from context
func getSessionIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(sessionIDKey).(string); ok {
		return id
	}
	return ""
}

func (m *Model) joinServer(addr, login, password string, useTLS bool) error {
	// Check connection limit
	if len(m.sessions) >= MaxSessions {
		return fmt.Errorf("maximum of %d server connections reached", MaxSessions)
	}

	// Append default port to address if no port supplied
	if len(strings.Split(addr, ":")) == 1 {
		if useTLS {
			addr += ":5600"
		} else {
			addr += ":5500"
		}
	}

	// Create new session
	session := &ServerSession{
		ID:               uuid.New().String(),
		DisplayName:      m.pendingServerName,
		Address:          addr,
		hlClient:         hotline.NewClient(m.prefs.Username, m.logger),
		screenHistory:    []Screen{ScreenServerUI},
		pendingDownloads: make(map[[4]byte]string),
		pendingUploads:   make(map[[4]byte]string),
	}

	// Create cancellable context for this connection
	session.connectionCtx, session.connectionCtxCancel = context.WithCancel(context.Background())
	session.clientDisconnecting = false

	// Register transaction handlers for this session
	m.registerSessionHandlers(session)

	var err error
	if useTLS {
		// Create TLS connection
		session.hlClient.Connection, err = tls.Dial("tcp", addr, &tls.Config{
			InsecureSkipVerify: true,
		})
		if err != nil {
			if session.connectionCtxCancel != nil {
				session.connectionCtxCancel()
			}
			return fmt.Errorf("TLS connection error: %v", err)
		}

		// Perform handshake
		if err := session.hlClient.Handshake(); err != nil {
			if session.connectionCtxCancel != nil {
				session.connectionCtxCancel()
			}
			return fmt.Errorf("handshake error: %v", err)
		}

		// Send login transaction
		err = session.hlClient.Send(
			hotline.NewTransaction(
				hotline.TranLogin, [2]byte{0, 0},
				hotline.NewField(hotline.FieldUserName, []byte(m.prefs.Username)),
				hotline.NewField(hotline.FieldUserIconID, m.prefs.IconBytes()),
				hotline.NewField(hotline.FieldUserLogin, hotline.EncodeString([]byte(login))),
				hotline.NewField(hotline.FieldUserPassword, hotline.EncodeString([]byte(password))),
			),
		)
		if err != nil {
			if session.connectionCtxCancel != nil {
				session.connectionCtxCancel()
			}
			return fmt.Errorf("login error: %v", err)
		}
	} else {
		if err := session.hlClient.Connect(addr, login, password); err != nil {
			if session.connectionCtxCancel != nil {
				session.connectionCtxCancel()
			}
			return fmt.Errorf("error joining server: %v", err)
		}
	}

	// Add session to list and make it active
	m.sessions = append(m.sessions, session)
	m.activeSessionIndex = len(m.sessions) - 1

	// Start transaction handler goroutine
	go func() {
		sessionID := session.ID
		err := session.hlClient.HandleTransactions(session.connectionCtx)
		m.logger.Error("Transaction scanning failed", "err", err, "sessionID", sessionID)

		// Send session-specific disconnect message
		m.program.Send(disconnectSessionMsg{sessionID: sessionID})
	}()

	return nil
}

func (m *Model) savePreferences() error {
	out, err := yaml.Marshal(m.prefs)
	if err != nil {
		return err
	}
	return os.WriteFile(m.cfgPath, out, 0666)
}
