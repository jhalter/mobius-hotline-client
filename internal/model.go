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

	"github.com/google/uuid"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jhalter/mobius-hotline-client/internal/style"
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
	ScreenNewsBundleForm
	ScreenNewsCategoryForm
	ScreenLegacyNewsPost
	ScreenMessageBoard
	ScreenFiles
	ScreenLogs
	ScreenModal
	ScreenTasks
	ScreenAccounts
	ScreenComposeMessage
	ScreenFilePicker
	ScreenLoading
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

	// Screen state
	screenHistory []Screen // Stack of screens, current screen is last element

	width         int
	height        int
	welcomeBanner string // Randomly selected banner, loaded once at startup

	// Hotline client
	hlClient          *hotline.Client
	serverName        string
	pendingServerName string // Name to display when connection succeeds (from bookmark/tracker/address)
	pendingServerAddr string // Address being connected to
	userAccess        hotline.AccessBitmap
	userList          []hotline.User

	// Connection management
	connectionCtx       context.Context
	connectionCtxCancel context.CancelFunc
	clientDisconnecting bool

	// Screens
	homeScreen             *HomeScreen
	joinServerScreen       *JoinServerScreen
	bookmarkScreen         *BookmarkScreen
	trackerScreen          *TrackerScreen
	settingsScreen         *SettingsScreen
	serverScreen           *ServerScreen
	newsScreen             *NewsScreen
	newsArticlePostScreen  *NewsArticlePostScreen
	newsBundleFormScreen   *NewsBundleFormScreen
	newsCategoryFormScreen *NewsCategoryFormScreen
	legacyNewsPostScreen   *LegacyNewsPostScreen
	accountsScreen         *AccountsScreen
	filesScreen            *FilesScreen
	tasksScreen            *TasksScreen
	logsScreen             *LogsScreen
	messageBoardScreen     *MessageBoardScreen
	filePickerScreen       *FilePickerScreen
	composeMessageScreen   *ComposeMessageScreen
	modalScreen            *ModalScreen
	loadingScreen          *LoadingScreen

	// File picker state
	lastPickerLocation string // Remember last location

	// Private message stack (for handling multiple incoming PMs)
	privateMessages []PrivateMessage

	// Task management for file downloads and uploads
	taskManager      *TaskManager
	downloadDir      string
	pendingDownloads map[[4]byte]string // transaction ID -> task ID
	pendingUploads   map[[4]byte]string // transaction ID -> task ID

	// Task widget
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
}

// PopScreen removes current screen and returns to previous
// Returns the screen we're now on
func (m *Model) PopScreen() Screen {
	if len(m.screenHistory) <= 1 {
		m.screenHistory = []Screen{ScreenHome}
		return ScreenHome
	}
	m.screenHistory = m.screenHistory[:len(m.screenHistory)-1]
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
}

// NavigateTo clears history and jumps to a screen (hard navigation)
// Used for disconnect, logout, or other full resets
func (m *Model) NavigateTo(screen Screen) {
	m.screenHistory = []Screen{screen}
}

// updatePrivateMessageModal creates or updates the modal for the current PM stack
// Shows the top message with count indicator if multiple messages are pending
func (m *Model) updatePrivateMessageModal() {
	if len(m.privateMessages) == 0 {
		return
	}

	// Get current message (top of stack - most recent)
	current := m.privateMessages[len(m.privateMessages)-1]

	// Build title with count if multiple messages
	title := "Private Message from " + current.From
	if len(m.privateMessages) > 1 {
		title += fmt.Sprintf(" (1 of %d)", len(m.privateMessages))
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
	switch m.CurrentScreen() {
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
	case ScreenServerUI:
		return m.serverScreen
	case ScreenNews:
		return m.newsScreen
	case ScreenNewsArticlePost:
		return m.newsArticlePostScreen
	case ScreenNewsBundleForm:
		return m.newsBundleFormScreen
	case ScreenNewsCategoryForm:
		return m.newsCategoryFormScreen
	case ScreenLegacyNewsPost:
		return m.legacyNewsPostScreen
	case ScreenAccounts:
		return m.accountsScreen
	case ScreenFiles:
		return m.filesScreen
	case ScreenTasks:
		return m.tasksScreen
	case ScreenLogs:
		return m.logsScreen
	case ScreenMessageBoard:
		return m.messageBoardScreen
	case ScreenFilePicker:
		return m.filePickerScreen
	case ScreenComposeMessage:
		return m.composeMessageScreen
	case ScreenModal:
		return m.modalScreen
	case ScreenLoading:
		return m.loadingScreen
	}
	return nil
}

func NewModel(cfgPath string, logger *slog.Logger, db *DebugBuffer) *Model {
	prefs, err := readConfig(cfgPath)
	if err != nil {
		logger.Error(fmt.Sprintf("unable to read config file %s\n", cfgPath))
		os.Exit(1)
	}

	// Apply saved theme (defaults to Classic if not set)
	style.SetTheme(style.GetThemeByName(prefs.Theme))

	hlClient := hotline.NewClient(prefs.Username, logger)

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
		hlClient:           hlClient,
		taskManager:        NewTaskManager(),
		downloadDir:        downloadDir,
		pendingDownloads:   make(map[[4]byte]string),
		pendingUploads:     make(map[[4]byte]string),
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
		if keyMsg.String() == "ctrl+q" {
			return m, tea.Quit
		}
		if keyMsg.String() == "ctrl+l" {
			m.logsScreen = NewLogsScreen(m.debugBuffer, m)
			m.PushScreen(ScreenLogs)
			return m, nil
		}
	}

	if _, ok := msg.(disconnectMsg); ok {
		m.NavigateTo(ScreenHome)
		_ = m.hlClient.Disconnect()

		// Only send error if client didn't initiate disconnect
		var cmd tea.Cmd
		if !m.clientDisconnecting {
			cmd = func() tea.Msg {
				return errorMsg{text: "Server connection closed."}
			}
		}

		// Clean up connection state
		if m.connectionCtxCancel != nil {
			m.connectionCtxCancel()
			m.connectionCtxCancel = nil
		}
		m.connectionCtx = nil
		m.clientDisconnecting = false

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
	if m.isShowingPrivateMessageModal() && len(m.privateMessages) > 0 {
		m.privateMessages = m.privateMessages[:len(m.privateMessages)-1]

		// Check if there are more PMs pending
		if len(m.privateMessages) > 0 {
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
	// Handle different modal types
	switch msg.Type {
	case ModalTypeAgreement:
		if msg.ButtonClicked == "Agree" {
			// User agreed - send TranAgreed asynchronously
			return func() tea.Msg {
				_ = m.hlClient.Send(hotline.NewTransaction(
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
		m.clientDisconnecting = true
		if m.connectionCtxCancel != nil {
			m.connectionCtxCancel()
		}
		m.NavigateTo(ScreenHome)
		return func() tea.Msg {
			_ = m.hlClient.Disconnect()
			return nil
		}

	case ModalTypeDisconnect:
		if msg.ButtonClicked == "Exit" {
			// Signal that client is initiating disconnect
			m.clientDisconnecting = true

			// Cancel context to unblock HandleTransactions
			if m.connectionCtxCancel != nil {
				m.connectionCtxCancel()
			}

			m.NavigateTo(ScreenHome)
			// Close the connection asynchronously
			return func() tea.Msg {
				_ = m.hlClient.Disconnect()
				return nil
			}
		}
		// Cancel - return to previous screen
		m.PopScreen()

	case ModalTypePrivateMessage:
		// Get and pop current message from stack
		if len(m.privateMessages) > 0 {
			current := m.privateMessages[len(m.privateMessages)-1]
			m.privateMessages = m.privateMessages[:len(m.privateMessages)-1]

			if msg.ButtonClicked == "Reply" {
				// Reply button clicked - open compose screen with current message data
				var cmd tea.Cmd
				m.composeMessageScreen, cmd = NewComposeMessageScreen(
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
		if len(m.privateMessages) > 0 {
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
	if screen := m.currentScreen(); screen != nil {
		return screen.View()
	}
	return ""
}

func (m *Model) initiateFileUpload(localPath string) tea.Cmd {
	return func() tea.Msg {
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
		if m.filesScreen != nil {
			filePath = m.filesScreen.GetFilePath()
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

		// Map transaction ID to task ID
		m.pendingUploads[t.ID] = task.ID

		// Send transaction
		if err := m.hlClient.Send(t); err != nil {
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

	// Register transaction handlers
	m.hlClient.HandleFunc(hotline.TranAgreed, m.HandleTranAgreed)
	m.hlClient.HandleFunc(hotline.TranChatMsg, m.HandleClientChatMsg)
	m.hlClient.HandleFunc(hotline.TranDownloadFile, m.HandleDownloadFile)
	m.hlClient.HandleFunc(hotline.TranGetFileInfo, m.HandleGetFileInfo)
	m.hlClient.HandleFunc(hotline.TranGetFileNameList, m.HandleGetFileNameList)
	m.hlClient.HandleFunc(hotline.TranGetMsgs, m.TranGetMsgs)
	m.hlClient.HandleFunc(hotline.TranGetNewsArtData, m.HandleGetNewsArtData)
	m.hlClient.HandleFunc(hotline.TranGetNewsArtNameList, m.HandleGetNewsArtNameList)
	m.hlClient.HandleFunc(hotline.TranGetNewsCatNameList, m.HandleGetNewsCatNameList)
	m.hlClient.HandleFunc(hotline.TranGetUserNameList, m.HandleClientGetUserNameList)
	m.hlClient.HandleFunc(hotline.TranKeepAlive, m.HandleKeepAlive)
	m.hlClient.HandleFunc(hotline.TranListUsers, m.HandleListUsers)
	m.hlClient.HandleFunc(hotline.TranLogin, m.HandleClientTranLogin)
	m.hlClient.HandleFunc(hotline.TranNewMsg, m.HandleNewMsg)
	m.hlClient.HandleFunc(hotline.TranNewNewsCat, m.HandleNewNewsCat)
	m.hlClient.HandleFunc(hotline.TranNewNewsFldr, m.HandleNewNewsFldr)
	m.hlClient.HandleFunc(hotline.TranNotifyChangeUser, m.HandleNotifyChangeUser)
	m.hlClient.HandleFunc(hotline.TranNotifyChatDeleteUser, m.HandleNotifyDeleteUser)
	m.hlClient.HandleFunc(hotline.TranNotifyDeleteUser, m.HandleNotifyDeleteUser)
	m.hlClient.HandleFunc(hotline.TranPostNewsArt, m.HandlePostNewsArt)
	m.hlClient.HandleFunc(hotline.TranServerMsg, m.HandleTranServerMsg)
	m.hlClient.HandleFunc(hotline.TranShowAgreement, m.HandleClientTranShowAgreement)
	m.hlClient.HandleFunc(hotline.TranUploadFile, m.HandleUploadFile)
	m.hlClient.HandleFunc(hotline.TranUserAccess, m.HandleClientTranUserAccess)

	_, err := m.program.Run()
	return err
}

func (m *Model) joinServer(addr, login, password string, useTLS bool) error {
	// Create cancellable context for this connection
	m.connectionCtx, m.connectionCtxCancel = context.WithCancel(context.Background())
	m.clientDisconnecting = false

	// Append default port to address if no port supplied
	if len(strings.Split(addr, ":")) == 1 {
		if useTLS {
			addr += ":5600"
		} else {
			addr += ":5500"
		}
	}

	var err error
	if useTLS {
		// Create TLS connection
		m.hlClient.Connection, err = tls.Dial("tcp", addr, &tls.Config{
			InsecureSkipVerify: true,
		})
		if err != nil {
			if m.connectionCtxCancel != nil {
				m.connectionCtxCancel()
			}
			return fmt.Errorf("TLS connection error: %v", err)
		}

		// Perform handshake
		if err := m.hlClient.Handshake(); err != nil {
			if m.connectionCtxCancel != nil {
				m.connectionCtxCancel()
			}
			return fmt.Errorf("handshake error: %v", err)
		}

		// Send login transaction
		err = m.hlClient.Send(
			hotline.NewTransaction(
				hotline.TranLogin, [2]byte{0, 0},
				hotline.NewField(hotline.FieldUserName, []byte(m.prefs.Username)),
				hotline.NewField(hotline.FieldUserIconID, m.prefs.IconBytes()),
				hotline.NewField(hotline.FieldUserLogin, hotline.EncodeString([]byte(login))),
				hotline.NewField(hotline.FieldUserPassword, hotline.EncodeString([]byte(password))),
			),
		)
		if err != nil {
			if m.connectionCtxCancel != nil {
				m.connectionCtxCancel()
			}
			return fmt.Errorf("login error: %v", err)
		}
	} else {
		if err := m.hlClient.Connect(addr, login, password); err != nil {
			if m.connectionCtxCancel != nil {
				m.connectionCtxCancel()
			}
			return fmt.Errorf("error joining server: %v", err)
		}
	}

	go func() {
		// Use the cancellable context
		err = m.hlClient.HandleTransactions(m.connectionCtx)
		m.logger.Error("Transaction scanning failed", "err", err)

		m.program.Send(disconnectMsg{})
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
