package internal

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jhalter/mobius-hotline-client/internal/style"
	"github.com/jhalter/mobius/hotline"
)

type msgHandler = func(msg tea.Msg) (tea.Model, tea.Cmd)

// registerHandler registers a message handler for the given message type.
// The msgType parameter should be a zero-value instance of the message type.
func (m *Model) registerHandler(msgType tea.Msg, handler msgHandler) {
	t := reflect.TypeOf(msgType)
	m.msgHandlers[t] = handler
}

func (m *Model) handleWindowResize(msg tea.Msg) (tea.Model, tea.Cmd) {
	windowMsg := msg.(tea.WindowSizeMsg)
	m.width = windowMsg.Width
	m.height = windowMsg.Height
	m.resizeAllScreens(windowMsg.Width, windowMsg.Height)
	return m, nil
}

func (m *Model) resizeAllScreens(w, h int) {
	// Resize global/pre-connection screens
	if m.homeScreen != nil {
		m.homeScreen.SetSize(w, h)
	}
	if m.joinServerScreen != nil {
		m.joinServerScreen.SetSize(w, h)
	}
	if m.bookmarkScreen != nil {
		m.bookmarkScreen.SetSize(w, h)
	}
	if m.trackerScreen != nil {
		m.trackerScreen.SetSize(w, h)
	}
	if m.settingsScreen != nil {
		m.settingsScreen.SetSize(w, h)
	}
	if m.tasksScreen != nil {
		m.tasksScreen.SetSize(w, h)
	}
	if m.logsScreen != nil {
		m.logsScreen.SetSize(w, h)
	}
	if m.filePickerScreen != nil {
		m.filePickerScreen.SetSize(w, h)
	}
	if m.modalScreen != nil {
		m.modalScreen.SetSize(w, h)
	}
	if m.loadingScreen != nil {
		m.loadingScreen.SetSize(w, h)
	}

	// Resize all session-specific screens
	for _, session := range m.sessions {
		session.resizeAllScreens(w, h)
	}
}

func (m *Model) handleChatMsgfunc(msg tea.Msg) (tea.Model, tea.Cmd) {
	session := m.activeSession()
	if session == nil {
		return m, nil
	}

	chatMessage := msg.(chatMsg)

	var formattedMsg string

	// Check if this is a system message (already styled, starts with <<<)
	if strings.Contains(chatMessage.text, "<<<") {
		// System messages are already styled, use as-is
		formattedMsg = chatMessage.text
	} else {
		// Use regex to extract username (everything up to and including first colon)
		re := regexp.MustCompile(`^[^:]*:`)
		match := re.FindString(chatMessage.text)

		// Apply bold styling to username
		message := strings.TrimPrefix(chatMessage.text, match)
		formattedMsg = style.UsernameStyle.Render(match) + message
	}

	// Add to server screen if it exists
	if session.serverScreen != nil {
		session.serverScreen.AddChatMessage(formattedMsg)
	}

	return m, nil
}

func (m *Model) handleUserListMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	session := m.activeSession()
	if session == nil {
		return m, nil
	}

	userListMessage := msg.(userListMsg)
	session.userList = userListMessage.users

	// Update server screen if it exists
	if session.serverScreen != nil {
		session.serverScreen.SetUserList(userListMessage.users)
	}

	return m, nil
}

func (m *Model) handleMessageBoardMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	session := m.activeSession()
	if session == nil {
		return m, nil
	}
	messageBoardMessage := msg.(messageBoardMsg)
	session.messageBoardScreen = NewMessageBoardScreen(messageBoardMessage.text, m)
	m.PushScreen(ScreenMessageBoard)
	return m, nil
}

func (m *Model) handleErrorMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	errorMessage := msg.(errorMsg)
	m.logger.Error("Received error message", "text", errorMessage.text)
	m.soundPlayer.PlayAsync(SoundError)
	// Pop loading screen if it's active, so error modal replaces it properly
	if m.CurrentScreen() == ScreenLoading {
		m.PopScreen()
	}
	m.modalScreen = NewModalScreen(ModalTypeError, "⚠️  Error", errorMessage.text, []string{"Close"}, m)
	m.PushScreen(ScreenModal)
	return m, m.modalScreen.Init()
}

func (m *Model) handleServerMsgMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	session := m.activeSession()
	if session == nil {
		return m, nil
	}

	serverMessage := msg.(serverMsgMsg)

	// Add to private message stack
	pm := PrivateMessage{
		From:   serverMessage.from,
		UserID: serverMessage.userID,
		Text:   serverMessage.text,
		Time:   serverMessage.time,
	}
	session.privateMessages = append(session.privateMessages, pm)

	// Update or create the PM modal
	m.updatePrivateMessageModal()

	// Only push screen if not already showing a PM modal
	if !m.isShowingPrivateMessageModal() {
		m.PushScreen(ScreenModal)
	}

	return m, m.modalScreen.Init()
}

func (m *Model) handleAgreementMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Pop loading screen if it's active
	if m.CurrentScreen() == ScreenLoading {
		m.PopScreen()
	}
	agreementMessage := msg.(agreementMsg)
	m.modalScreen = NewModalScreen(
		ModalTypeAgreement,
		"Server Agreement",
		agreementMessage.text,
		[]string{"Disagree", "Agree"},
		m,
	)
	m.PushScreen(ScreenModal)
	return m, m.modalScreen.Init()
}

func (m *Model) handleServerConnectedMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Pop loading screen if it's active
	if m.CurrentScreen() == ScreenLoading {
		m.PopScreen()
	}

	session := m.activeSession()
	if session == nil {
		return m, nil
	}

	serverConnected := msg.(serverConnectedMsg)
	session.serverName = serverConnected.name
	session.DisplayName = serverConnected.name

	// Create and initialize ServerScreen for this session
	session.serverScreen = NewServerScreen(m)
	session.serverScreen.SetServerName(serverConnected.name)
	session.serverScreen.SetSize(m.width, m.height)
	session.serverScreen.SetUserAccess(session.userAccess)
	session.serverScreen.FocusChatInput()

	m.NavigateTo(ScreenServerUI)
	m.soundPlayer.PlayAsync(SoundLoggedIn)

	// Add initial join message to chat viewport
	joinStyle := lipgloss.NewStyle().Bold(true).Foreground(style.CurrentTheme.TextMuted)
	timestamp := time.Now().Format("1/2/06 3:04:05 PM")
	session.serverScreen.AddChatMessage(joinStyle.Render(fmt.Sprintf(" <<<   %s has joined   >>>", m.prefs.Username)))
	session.serverScreen.AddChatMessage(joinStyle.Render(fmt.Sprintf(" <<<   %s    >>>", timestamp)))

	return m, nil
}

func (m *Model) handleTrackerListMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Pop loading screen if it's active
	if m.CurrentScreen() == ScreenLoading {
		m.PopScreen()
	}
	trackerMessage := msg.(trackerListMsg)
	m.trackerScreen = NewTrackerScreen(trackerMessage.servers, m)
	m.ReplaceScreen(ScreenTracker)
	return m, nil
}

func (m *Model) handleTrackerServerSelectedMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	serverMsg := msg.(TrackerServerSelectedMsg)
	srv := serverMsg.Server

	// Create join server screen with tracker server details
	var cmd tea.Cmd
	m.joinServerScreen, cmd = NewJoinServerScreenForConnect(
		srv.Addr(),
		hotline.GuestAccount,
		"",
		false,
		ScreenTracker,
		m,
	)
	m.pendingServerName = string(srv.Name)
	m.ReplaceScreen(ScreenJoinServer)

	return m, cmd
}

func (m *Model) handleTrackerCancelledMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.NavigateTo(ScreenHome)
	return m, nil
}

func (m *Model) handleSettingsSavedMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	settingsMsg := msg.(SettingsSavedMsg)

	// Update preferences
	m.prefs.Username = settingsMsg.Username
	m.prefs.IconID = settingsMsg.IconID
	m.prefs.Tracker = settingsMsg.Tracker
	m.prefs.DownloadDir = settingsMsg.DownloadDir
	m.prefs.EnableBell = settingsMsg.EnableBell
	m.prefs.EnableSounds = settingsMsg.EnableSounds

	// Update the active download directory
	m.downloadDir = m.prefs.DownloadDir

	// Update sound player enabled state
	if m.soundPlayer != nil {
		m.soundPlayer.SetEnabled(m.prefs.EnableSounds)
	}

	// Save to file
	if err := m.savePreferences(); err != nil {
		m.logger.Error("Failed to save preferences", "err", err)
	}

	m.PopScreen()
	return m, nil
}

func (m *Model) handleSettingsCancelledMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.PopScreen()
	return m, nil
}

func (m *Model) handleBookmarkSelectedMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	bookmarkMsg := msg.(BookmarkSelectedMsg)
	bm := bookmarkMsg.Bookmark

	// Create join server screen with bookmark details
	var cmd tea.Cmd
	m.joinServerScreen, cmd = NewJoinServerScreenForConnect(
		bm.Addr,
		bm.Login,
		bm.Password,
		bm.TLS,
		ScreenBookmarks,
		m,
	)
	m.pendingServerName = bm.Name
	m.ReplaceScreen(ScreenJoinServer)

	return m, cmd
}

func (m *Model) handleBookmarkEditMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	bookmarkMsg := msg.(BookmarkEditMsg)
	bm := bookmarkMsg.Bookmark

	// Create join server screen for editing bookmark
	var cmd tea.Cmd
	m.joinServerScreen, cmd = NewJoinServerScreenForEdit(bm, bookmarkMsg.Index, m)
	m.ReplaceScreen(ScreenJoinServer)

	return m, cmd
}

func (m *Model) handleBookmarkCreateMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Create join server screen for creating new bookmark
	var cmd tea.Cmd
	m.joinServerScreen, cmd = NewJoinServerScreenForCreate(m)
	m.ReplaceScreen(ScreenJoinServer)

	return m, cmd
}

func (m *Model) handleBookmarkCancelledMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.NavigateTo(ScreenHome)
	return m, nil
}

// JoinServerScreen message handlers

func (m *Model) handleJoinServerConnectMsg(msg JoinServerConnectMsg) tea.Cmd {
	// Save bookmark if requested
	if msg.Save {
		m.prefs.AddBookmark(msg.Name, msg.Addr, msg.Login, msg.Password, msg.TLS)
		_ = m.savePreferences()
	}

	// Store the address for display (used as fallback if no name was set)
	m.pendingServerAddr = msg.Addr
	// If no server name was set (e.g., connecting directly), use the address as the name
	if m.pendingServerName == "" {
		m.pendingServerName = msg.Addr
	}

	// Show loading screen while connecting
	var loadingCmd tea.Cmd
	m.loadingScreen, loadingCmd = NewLoadingScreen("Connecting to server...", m)
	m.PushScreen(ScreenLoading)

	// Connect to server asynchronously
	connectCmd := func() tea.Msg {
		err := m.joinServer(msg.Addr, msg.Login, msg.Password, msg.TLS)
		return serverConnectionAttemptMsg{err: err}
	}

	return tea.Batch(loadingCmd, connectCmd)
}

func (m *Model) handleServerConnectionAttemptMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	attemptMsg := msg.(serverConnectionAttemptMsg)
	if attemptMsg.err != nil {
		// Connection failed - pop loading screen and show error
		if m.CurrentScreen() == ScreenLoading {
			m.PopScreen()
		}
		m.modalScreen = NewModalScreen(ModalTypeError, "Connection Error", attemptMsg.err.Error(), []string{"OK"}, m)
		m.PushScreen(ScreenModal)
		return m, m.modalScreen.Init()
	}
	// Connection succeeded - keep loading screen until serverConnectedMsg arrives
	return m, nil
}

func (m *Model) handleJoinServerBookmarkSavedMsg(msg JoinServerBookmarkSavedMsg) {
	// Update existing bookmark
	if msg.Index >= 0 && msg.Index < len(m.prefs.Bookmarks) {
		m.prefs.Bookmarks[msg.Index].Name = msg.Name
		m.prefs.Bookmarks[msg.Index].Addr = msg.Addr
		m.prefs.Bookmarks[msg.Index].Login = msg.Login
		m.prefs.Bookmarks[msg.Index].Password = msg.Password
		m.prefs.Bookmarks[msg.Index].TLS = msg.TLS
		_ = m.savePreferences()
	}
	m.bookmarkScreen = NewBookmarkScreen(m.prefs.Bookmarks, m)
	m.PopScreen()
}

func (m *Model) handleJoinServerBookmarkCreatedMsg(msg JoinServerBookmarkCreatedMsg) {
	m.prefs.AddBookmark(msg.Name, msg.Addr, msg.Login, msg.Password, msg.TLS)
	_ = m.savePreferences()
	m.bookmarkScreen = NewBookmarkScreen(m.prefs.Bookmarks, m)
	m.PopScreen()
}

func (m *Model) handleJoinServerCancelledMsg(msg JoinServerCancelledMsg) {
	// Return to the page we came from
	if msg.BackPage != ScreenHome {
		// Refresh bookmark screen if returning there
		if msg.BackPage == ScreenBookmarks {
			m.bookmarkScreen = NewBookmarkScreen(m.prefs.Bookmarks, m)
		}
		m.NavigateTo(msg.BackPage)
	} else {
		m.NavigateTo(ScreenHome)
	}
}

func (m *Model) handleBookmarkDeletedMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	bookmarkMsg := msg.(BookmarkDeletedMsg)
	bm := bookmarkMsg.Bookmark

	// Remove from prefs by finding matching bookmark
	for i, b := range m.prefs.Bookmarks {
		if b.Name == bm.Name && b.Addr == bm.Addr {
			m.prefs.Bookmarks = append(m.prefs.Bookmarks[:i], m.prefs.Bookmarks[i+1:]...)
			break
		}
	}
	_ = m.savePreferences()

	return m, nil
}

func (m *Model) handleFilesMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	session := m.activeSession()
	if session == nil {
		return m, nil
	}
	filesMessage := msg.(filesMsg)

	// Create screen if needed
	if session.filesScreen == nil {
		session.filesScreen = NewFilesScreen(m)
	}

	// Update files in screen
	session.filesScreen.SetFiles(filesMessage.files)

	// Only push if not already in Files screen
	// This preserves the correct screen to return to when closing Files
	if m.CurrentScreen() != ScreenFiles {
		m.PushScreen(ScreenFiles)
	}
	return m, nil
}

func (m *Model) handleNewsCategoriesMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	session := m.activeSession()
	if session == nil {
		return m, nil
	}
	newsCategoriesMessage := msg.(newsCategoriesMsg)
	if session.newsScreen == nil {
		session.newsScreen = NewNewsScreen(m)
	}
	session.newsScreen.SetCategories(newsCategoriesMessage.categories)
	if m.CurrentScreen() != ScreenNews {
		m.PushScreen(ScreenNews)
	}
	return m, nil
}

func (m *Model) handleNewsArticlesMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	session := m.activeSession()
	if session == nil {
		return m, nil
	}
	newsArticlesMessage := msg.(newsArticlesMsg)
	if session.newsScreen == nil {
		session.newsScreen = NewNewsScreen(m)
	}
	session.newsScreen.SetArticles(newsArticlesMessage.articles)
	if m.CurrentScreen() != ScreenNews {
		m.PushScreen(ScreenNews)
	}
	return m, nil
}

func (m *Model) handleNewsArticleDataMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	session := m.activeSession()
	if session == nil {
		return m, nil
	}
	newsArticleData := msg.(newsArticleDataMsg)
	if session.newsScreen != nil {
		// Get pending article ID and path from news screen
		articleID := session.newsScreen.GetPendingArticleID()
		path := session.newsScreen.GetPath()
		session.newsScreen.ClearPendingArticleID()

		// Create and push the article view screen
		session.newsArticleViewScreen = NewNewsArticleViewScreen(
			articleID,
			newsArticleData.article.Title,
			newsArticleData.article.Poster,
			newsArticleData.article.Date,
			newsArticleData.article.Data,
			path,
			m,
		)
		if session.userAccess.IsSet(hotline.AccessNewsPostArt) {
			session.newsArticleViewScreen.SetUserAccess(session.userAccess)
		}
		m.PushScreen(ScreenNewsArticleView)
	}
	return m, nil
}

func (m *Model) handleNewsNavigateToCategoryMsg(msg NewsNavigateToCategoryMsg) {
	session := m.activeSession()
	if session == nil {
		return
	}
	pathBytes := encodeNewsPath(msg.Path)
	if err := session.hlClient.Send(hotline.NewTransaction(
		hotline.TranGetNewsArtNameList,
		[2]byte{},
		hotline.NewField(hotline.FieldNewsPath, pathBytes),
	)); err != nil {
		m.logger.Error("Error requesting news articles", "err", err)
	}
}

func (m *Model) handleNewsNavigateToBundleMsg(msg NewsNavigateToBundleMsg) {
	session := m.activeSession()
	if session == nil {
		return
	}
	var fields []hotline.Field
	if len(msg.Path) > 0 {
		pathBytes := encodeNewsPath(msg.Path)
		fields = append(fields, hotline.NewField(hotline.FieldNewsPath, pathBytes))
	}
	if err := session.hlClient.Send(hotline.NewTransaction(hotline.TranGetNewsCatNameList, [2]byte{}, fields...)); err != nil {
		m.logger.Error("Error requesting news categories", "err", err)
	}
}

func (m *Model) handleNewsRequestArticleMsg(msg NewsRequestArticleMsg) {
	session := m.activeSession()
	if session == nil {
		return
	}
	pathBytes := encodeNewsPath(msg.Path)

	articleIDBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(articleIDBytes, msg.ArticleID)

	if err := session.hlClient.Send(hotline.NewTransaction(
		hotline.TranGetNewsArtData,
		[2]byte{},
		hotline.NewField(hotline.FieldNewsPath, pathBytes),
		hotline.NewField(hotline.FieldNewsArtID, articleIDBytes),
	)); err != nil {
		m.logger.Error("Error requesting article data", "err", err)
	}
}

func (m *Model) handleNewsPostArticleMsg(msg NewsPostArticleMsg) tea.Cmd {
	session := m.activeSession()
	if session == nil {
		return nil
	}
	screen, cmd := NewNewsArticlePostScreen(session.newsScreen.GetPath(), msg.ParentID, msg.Subject, m)
	session.newsArticlePostScreen = screen
	m.PushScreen(ScreenNewsArticlePost)
	return cmd
}

func (m *Model) handleNewsArticleViewReplyMsg(msg NewsArticleViewReplyMsg) tea.Cmd {
	session := m.activeSession()
	if session == nil {
		return nil
	}
	screen, cmd := NewNewsArticlePostScreen(msg.Path, msg.ArticleID, msg.Subject, m)
	session.newsArticlePostScreen = screen
	m.PushScreen(ScreenNewsArticlePost)
	return cmd
}

func (m *Model) handleNewsCreateBundleMsg() tea.Cmd {
	session := m.activeSession()
	if session == nil {
		return nil
	}
	screen, cmd := NewNewsBundleFormScreen(session.newsScreen.GetPath(), m)
	session.newsBundleFormScreen = screen
	m.PushScreen(ScreenNewsBundleForm)
	return cmd
}

func (m *Model) handleNewsCreateCategoryMsg() tea.Cmd {
	session := m.activeSession()
	if session == nil {
		return nil
	}
	screen, cmd := NewNewsCategoryFormScreen(session.newsScreen.GetPath(), m)
	session.newsCategoryFormScreen = screen
	m.PushScreen(ScreenNewsCategoryForm)
	return cmd
}

func (m *Model) handleFileInfoMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	fileInfoMessage := msg.(fileInfoMsg)
	// Format file info for display
	var contentBuilder strings.Builder
	labelStyle := lipgloss.NewStyle().Bold(true)

	contentBuilder.WriteString(fmt.Sprintf("%s %s\n\n", labelStyle.Render("File:"), fileInfoMessage.fileName))

	if fileInfoMessage.fileTypeString != "" {
		contentBuilder.WriteString(fmt.Sprintf("%s     %s\n", labelStyle.Render("Type:"), fileInfoMessage.fileTypeString))
	}
	if fileInfoMessage.fileCreatorString != "" {
		contentBuilder.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Creator:"), fileInfoMessage.fileCreatorString))
	}
	if fileInfoMessage.fileTypeString != "" || fileInfoMessage.fileCreatorString != "" {
		contentBuilder.WriteString("\n")
	}

	contentBuilder.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Created:"), fileInfoMessage.createDate.Format("Jan 2, 2006 3:04 PM")))
	contentBuilder.WriteString(fmt.Sprintf("%s %s\n\n", labelStyle.Render("Modified:"), fileInfoMessage.modifyDate.Format("Jan 2, 2006 3:04 PM")))

	if fileInfoMessage.hasFileSize {
		sizeKB := float64(fileInfoMessage.fileSize) / 1024.0
		contentBuilder.WriteString(fmt.Sprintf("%s     %.2f KB (%d bytes)\n\n", labelStyle.Render("Size:"), sizeKB, fileInfoMessage.fileSize))
	}

	if fileInfoMessage.comment != "" {
		contentBuilder.WriteString(fmt.Sprintf("%s\n%s\n", labelStyle.Render("Comment:"), fileInfoMessage.comment))
	}

	// Display modal
	m.modalScreen = NewModalScreen(ModalTypeGeneric, "File Information", contentBuilder.String(), []string{"Close"}, m)
	m.PushScreen(ScreenModal)

	return m, m.modalScreen.Init()
}

func (m *Model) handleAccountListMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	session := m.activeSession()
	if session == nil {
		return m, nil
	}
	accountListMessage := msg.(accountListMsg)

	// If accounts screen is currently visible (or modal on top), just refresh it
	if (m.CurrentScreen() == ScreenAccounts || m.CurrentScreen() == ScreenModal) && session.accountsScreen != nil {
		session.accountsScreen.UpdateAccounts(accountListMessage.accounts)
		return m, nil
	}

	session.accountsScreen = NewAccountsScreen(accountListMessage.accounts, session.userAccess, m)
	m.PushScreen(ScreenAccounts)
	return m, nil
}

func (m *Model) handleTaskProgressMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	taskProgressMessage := msg.(taskProgressMsg)
	task := m.taskManager.Get(taskProgressMessage.taskID)
	if task != nil {
		now := time.Now()

		// Calculate speed if we have previous data
		if !task.LastUpdate.IsZero() {
			duration := now.Sub(task.LastUpdate).Seconds()
			if duration > 0 {
				bytesSinceLast := taskProgressMessage.bytes - task.LastBytes
				task.Speed = float64(bytesSinceLast) / duration
			}
		}

		task.TransferredBytes = taskProgressMessage.bytes
		task.LastBytes = taskProgressMessage.bytes
		task.LastUpdate = now

		// Update or create progress model for active tasks
		if task.Status == TaskActive {
			prog, exists := m.taskProgress[taskProgressMessage.taskID]
			if !exists {
				prog = progress.New(progress.WithDefaultBlend(), progress.WithWidth(20))
				m.taskProgress[taskProgressMessage.taskID] = prog
			}

			percent := 0.0
			if task.TotalBytes > 0 {
				percent = float64(task.TransferredBytes) / float64(task.TotalBytes)
			}
			cmd := prog.SetPercent(percent)
			m.taskProgress[taskProgressMessage.taskID] = prog
			return m, cmd // Return progress animation command
		}
	}
	return m, nil
}

func (m *Model) handleTaskStatusMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	taskStatusMessage := msg.(taskStatusMsg)
	task := m.taskManager.Get(taskStatusMessage.taskID)
	if task != nil {
		task.Status = taskStatusMessage.status
		task.Error = taskStatusMessage.err
		if taskStatusMessage.status == TaskCompleted || taskStatusMessage.status == TaskFailed {
			task.EndTime = time.Now()
			// Remove progress model when task completes
			delete(m.taskProgress, taskStatusMessage.taskID)
			if taskStatusMessage.status == TaskCompleted {
				m.soundPlayer.PlayAsync(SoundTransferComplete)
			}
		}
	}
	return m, nil
}

func (m *Model) handleDownloadReplyMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	session := m.activeSession()
	if session == nil {
		return m, nil
	}
	downloadReply := msg.(downloadReplyMsg)
	taskID := session.pendingDownloads[downloadReply.txID]
	delete(session.pendingDownloads, downloadReply.txID)

	task := m.taskManager.Get(taskID)
	if task != nil {
		task.TotalBytes = int64(downloadReply.transferSize)
		task.Status = TaskActive

		// Launch file transfer in background
		go m.performFileTransfer(session, task, downloadReply.refNum, downloadReply.transferSize)
	}
	return m, nil
}

func (m *Model) handleUploadReplyMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	session := m.activeSession()
	if session == nil {
		return m, nil
	}
	uploadReply := msg.(uploadReplyMsg)
	taskID, ok := session.pendingUploads[uploadReply.txID]
	if !ok {
		return m, nil
	}
	delete(session.pendingUploads, uploadReply.txID)

	task := m.taskManager.Get(taskID)
	if task == nil {
		return m, nil
	}

	task.Status = TaskActive

	// Start file transfer in goroutine
	go m.performFileUpload(session, task, uploadReply.refNum)

	return m, nil
}

// News form screen handlers

func (m *Model) handleNewsArticlePostedMsg(msg NewsArticlePostedMsg) {
	session := m.activeSession()
	if session == nil {
		return
	}
	// Create transaction with all required fields
	pathBytes := encodeNewsPath(msg.Path)

	// Parent article ID: 0 for new post, or stored ID for replies
	parentIDBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(parentIDBytes, msg.ParentID)

	t := hotline.NewTransaction(
		hotline.TranPostNewsArt,
		[2]byte{},
		hotline.NewField(hotline.FieldNewsPath, pathBytes),
		hotline.NewField(hotline.FieldNewsArtID, parentIDBytes),
		hotline.NewField(hotline.FieldNewsArtTitle, []byte(msg.Subject)),
		hotline.NewField(hotline.FieldNewsArtData, []byte(msg.Body)),
	)

	if err := session.hlClient.Send(t); err != nil {
		m.logger.Error("Error posting news article", "err", err)
	}

	m.PopScreen()

	// Refetch the article list to show the new post
	refetchPathBytes := encodeNewsPath(session.newsScreen.GetPath())
	if err := session.hlClient.Send(hotline.NewTransaction(
		hotline.TranGetNewsArtNameList,
		[2]byte{},
		hotline.NewField(hotline.FieldNewsPath, refetchPathBytes),
	)); err != nil {
		m.logger.Error("Error refetching articles", "err", err)
	}
}

func (m *Model) handleNewsBundleCreatedMsg(msg NewsBundleCreatedMsg) {
	session := m.activeSession()
	if session == nil {
		return
	}
	// Create bundle at current location
	pathBytes := encodeNewsPath(msg.Path)

	t := hotline.NewTransaction(
		hotline.TranNewNewsFldr,
		[2]byte{},
		hotline.NewField(hotline.FieldNewsPath, pathBytes),
		hotline.NewField(hotline.FieldFileName, []byte(msg.Name)),
	)

	if err := session.hlClient.Send(t); err != nil {
		m.logger.Error("Error creating news bundle", "err", err)
	}

	m.PopScreen()

	// Refetch current location
	refetchPathBytes := encodeNewsPath(session.newsScreen.GetPath())
	if err := session.hlClient.Send(hotline.NewTransaction(
		hotline.TranGetNewsCatNameList,
		[2]byte{},
		hotline.NewField(hotline.FieldNewsPath, refetchPathBytes),
	)); err != nil {
		m.logger.Error("Error refetching categories", "err", err)
	}
}

func (m *Model) handleNewsCategoryCreatedMsg(msg NewsCategoryCreatedMsg) {
	session := m.activeSession()
	if session == nil {
		return
	}
	// Create category at current location
	pathBytes := encodeNewsPath(msg.Path)

	t := hotline.NewTransaction(
		hotline.TranNewNewsCat,
		[2]byte{},
		hotline.NewField(hotline.FieldNewsPath, pathBytes),
		hotline.NewField(hotline.FieldNewsCatName, []byte(msg.Name)),
	)

	if err := session.hlClient.Send(t); err != nil {
		m.logger.Error("Error creating news category", "err", err)
	}

	m.PopScreen()

	// Refetch current location
	refetchPathBytes := encodeNewsPath(session.newsScreen.GetPath())
	if err := session.hlClient.Send(hotline.NewTransaction(
		hotline.TranGetNewsCatNameList,
		[2]byte{},
		hotline.NewField(hotline.FieldNewsPath, refetchPathBytes),
	)); err != nil {
		m.logger.Error("Error refetching categories", "err", err)
	}
}

func (m *Model) handleLegacyNewsPostedMsg(msg LegacyNewsPostedMsg) {
	session := m.activeSession()
	if session == nil {
		return
	}
	// Create and send the transaction
	t := hotline.NewTransaction(
		hotline.TranOldPostNews,
		[2]byte{},
		hotline.NewField(hotline.FieldData, []byte(msg.Content)),
	)

	if err := session.hlClient.Send(t); err != nil {
		m.logger.Error("Error posting news", "err", err)
	}

	// Refresh the messageboard content
	if err := session.hlClient.Send(hotline.NewTransaction(hotline.TranGetMsgs, [2]byte{})); err != nil {
		m.logger.Error("Error refreshing messageboard", "err", err)
	}

	m.PopScreen()
}

// ServerScreen message handlers

func (m *Model) handleServerDisconnectRequestedMsg() tea.Cmd {
	m.modalScreen = NewModalScreen(ModalTypeDisconnect, "Disconnect from the server?", "", []string{"Cancel", "Exit"}, m)
	m.PushScreen(ScreenModal)
	return m.modalScreen.Init()
}

func (m *Model) handleServerSendChatMsg(msg ServerSendChatMsg) {
	session := m.activeSession()
	if session == nil {
		return
	}
	t := hotline.NewTransaction(hotline.TranChatSend, [2]byte{},
		hotline.NewField(hotline.FieldData, []byte(msg.Text)),
	)
	_ = session.hlClient.Send(t)
}

func (m *Model) handleServerOpenNewsMsg() {
	session := m.activeSession()
	if session == nil {
		return
	}
	// Request threaded news - create fresh screen (handler will initialize when response arrives)
	session.newsScreen = NewNewsScreen(m)
	if err := session.hlClient.Send(hotline.NewTransaction(hotline.TranGetNewsCatNameList, [2]byte{})); err != nil {
		m.logger.Error("Error requesting news categories", "err", err)
	}
}

func (m *Model) handleServerOpenMessageBoardMsg() {
	session := m.activeSession()
	if session == nil {
		return
	}
	// Request messageboard
	if err := session.hlClient.Send(hotline.NewTransaction(hotline.TranGetMsgs, [2]byte{})); err != nil {
		m.logger.Error("Error requesting messageboard", "err", err)
	}
}

func (m *Model) handleServerOpenFilesMsg() {
	session := m.activeSession()
	if session == nil {
		return
	}
	// Create fresh files screen
	session.filesScreen = NewFilesScreen(m)
	// Request file list (handler will push screen when response arrives)
	if err := session.hlClient.Send(hotline.NewTransaction(hotline.TranGetFileNameList, [2]byte{})); err != nil {
		m.logger.Error("Error requesting files", "err", err)
	}
}

func (m *Model) handleServerOpenAccountsMsg() {
	session := m.activeSession()
	if session == nil {
		return
	}
	// Request user accounts list
	if err := session.hlClient.Send(hotline.NewTransaction(hotline.TranListUsers, [2]byte{})); err != nil {
		m.logger.Error("Error requesting account list", "err", err)
	}
}

func (m *Model) handleServerComposeMessageMsg(msg ServerComposeMessageMsg) tea.Cmd {
	session := m.activeSession()
	if session == nil {
		return nil
	}
	// Look up target username
	targetName := ""
	for _, u := range session.userList {
		if u.ID == msg.TargetUserID {
			targetName = u.Name
			break
		}
	}

	var cmd tea.Cmd
	session.composeMessageScreen, cmd = NewComposeMessageScreen(
		msg.TargetUserID,
		targetName,
		"", // No quote when composing new message
		m,
	)
	m.PushScreen(ScreenComposeMessage)
	return cmd
}

// ComposeMessageScreen message handlers

func (m *Model) handleComposeMessageSentMsg(msg ComposeMessageSentMsg) tea.Cmd {
	session := m.activeSession()
	if session == nil {
		return nil
	}
	// Build transaction fields
	fields := []hotline.Field{
		hotline.NewField(hotline.FieldData, []byte(msg.Text)),
		hotline.NewField(hotline.FieldUserID, msg.TargetID[:]),
	}

	// Add quoted message if replying
	if msg.QuoteText != "" {
		fields = append(fields, hotline.NewField(hotline.FieldQuotingMsg, []byte(msg.QuoteText)))
	}

	t := hotline.NewTransaction(hotline.TranSendInstantMsg, [2]byte{}, fields...)
	if err := session.hlClient.Send(t); err != nil {
		m.logger.Error("Error sending private message", "err", err)
	}

	// Check if there are more pending private messages
	if len(session.privateMessages) > 0 {
		m.updatePrivateMessageModal()
		m.ReplaceScreen(ScreenModal)
		return m.modalScreen.Init()
	}

	m.PopScreen()
	return nil
}

func (m *Model) handleComposeMessageCancelledMsg() tea.Cmd {
	session := m.activeSession()
	// Check if there are more pending private messages
	if session != nil && len(session.privateMessages) > 0 {
		m.updatePrivateMessageModal()
		m.ReplaceScreen(ScreenModal)
		return m.modalScreen.Init()
	}

	m.PopScreen()
	return nil
}

func (m *Model) handleServerOpenTasksMsg() {
	m.tasksScreen = NewTasksScreen(m)
	m.PushScreen(ScreenTasks)
}

// AccountsScreen message handlers

func (m *Model) handleAccountEditRequestMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	session := m.activeSession()
	if session == nil {
		return m, nil
	}
	editMsg := msg.(AccountEditRequestMsg)
	var cmd tea.Cmd
	session.accountEditScreen, cmd = NewAccountEditScreen(&editMsg.Account, session.userAccess, m)
	m.PushScreen(ScreenAccountEdit)
	return m, cmd
}

func (m *Model) handleAccountNewRequestMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	session := m.activeSession()
	if session == nil {
		return m, nil
	}
	var cmd tea.Cmd
	session.accountEditScreen, cmd = NewAccountEditScreen(nil, session.userAccess, m)
	m.PushScreen(ScreenAccountEdit)
	return m, cmd
}

func (m *Model) handleAccountEditCancelledMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.PopScreen()
	return m, nil
}

func (m *Model) handleAccountsSaveMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	saveMsg := msg.(AccountsSaveMsg)
	m.PopScreen()
	return m, m.submitAccountChanges(saveMsg)
}

func (m *Model) handleAccountSaveSuccessMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.modalScreen = NewModalScreen(ModalTypeGeneric, "Account Saved", "", []string{"Close"}, m)
	m.PushScreen(ScreenModal)
	return m, tea.Batch(m.modalScreen.Init(), m.refreshAccountList())
}

func (m *Model) handleAccountsDeleteMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	deleteMsg := msg.(AccountsDeleteMsg)
	m.PopScreen()
	return m, m.deleteAccount(deleteMsg.Login)
}

func (m *Model) handleAccountDeleteSuccessMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.modalScreen = NewModalScreen(ModalTypeGeneric, "Account Deleted", "", []string{"Close"}, m)
	m.PushScreen(ScreenModal)
	return m, tea.Batch(m.modalScreen.Init(), m.refreshAccountList())
}

func (m *Model) refreshAccountList() tea.Cmd {
	return func() tea.Msg {
		session := m.activeSession()
		if session == nil {
			return nil
		}
		session.hlClient.Send(hotline.NewTransaction(hotline.TranListUsers, [2]byte{}))
		return nil
	}
}

// FilesScreen message handlers

func (m *Model) handleFilesDownloadMsg(msg FilesDownloadMsg) tea.Cmd {
	session := m.activeSession()
	if session == nil {
		return nil
	}
	// Pop back to previous screen
	m.PopScreen()
	// Initiate download
	return session.filesScreen.InitiateDownload(msg.FileName, msg.FilePath)
}

func (m *Model) handleFilesGetInfoMsg(msg FilesGetInfoMsg) {
	session := m.activeSession()
	if session == nil {
		return
	}
	// Create transaction with file name
	t := hotline.NewTransaction(
		hotline.TranGetFileInfo,
		[2]byte{},
		hotline.NewField(hotline.FieldFileName, []byte(msg.FileName)),
	)

	// Add file path if in subdirectory
	if len(msg.FilePath) > 0 {
		pathStr := strings.Join(msg.FilePath, "/")
		t.Fields = append(t.Fields, hotline.NewField(hotline.FieldFilePath, hotline.EncodeFilePath(pathStr)))
	}

	if err := session.hlClient.Send(t); err != nil {
		m.logger.Error("Error sending file info request", "err", err)
	}
}

func (m *Model) handleFilesUploadMsg() tea.Cmd {
	// Create file picker screen
	m.filePickerScreen = NewFilePickerScreen(m.lastPickerLocation, m)
	m.PushScreen(ScreenFilePicker)
	return m.filePickerScreen.Init()
}

func (m *Model) handleFilesNavigateMsg(msg FilesNavigateMsg) {
	session := m.activeSession()
	if session == nil {
		return
	}
	// Update files screen path
	if session.filesScreen != nil {
		session.filesScreen.SetFilePath(msg.Path)
	}
	// Request new file list for this path
	f := hotline.NewField(hotline.FieldFilePath, hotline.EncodeFilePath(strings.Join(msg.Path, "/")))
	if err := session.hlClient.Send(hotline.NewTransaction(hotline.TranGetFileNameList, [2]byte{}, f)); err != nil {
		m.logger.Error("Error requesting file list", "err", err)
	}
}

// MessageBoardScreen message handlers

func (m *Model) handleMessageBoardPostRequestedMsg() tea.Cmd {
	session := m.activeSession()
	if session == nil {
		return nil
	}
	screen, cmd := NewLegacyNewsPostScreen(m)
	session.legacyNewsPostScreen = screen
	m.PushScreen(ScreenLegacyNewsPost)
	return cmd
}

// HomeScreen message handlers

func (m *Model) handleHomeJoinServerMsg() tea.Cmd {
	var cmd tea.Cmd
	m.joinServerScreen, cmd = NewJoinServerScreen(m)
	m.PushScreen(ScreenJoinServer)
	return cmd
}

func (m *Model) handleHomeBookmarksMsg() {
	m.bookmarkScreen = NewBookmarkScreen(m.prefs.Bookmarks, m)
	m.PushScreen(ScreenBookmarks)
}

func (m *Model) handleHomeTrackerMsg() tea.Cmd {
	m.logger.Info("Browse tracker key pressed")
	var cmd tea.Cmd
	m.loadingScreen, cmd = NewLoadingScreen("Connecting to tracker...", m)
	m.PushScreen(ScreenLoading)
	return tea.Batch(cmd, fetchTrackerList(m.prefs.Tracker))
}

func (m *Model) handleHomeSettingsMsg() tea.Cmd {
	var cmd tea.Cmd
	m.settingsScreen, cmd = NewSettingsScreen(m.prefs, m)
	m.PushScreen(ScreenSettings)
	return cmd
}

// FilePickerScreen message handlers

func (m *Model) handleFilePickerFileSelectedMsg(msg FilePickerFileSelectedMsg) tea.Cmd {
	// Remember location for next time
	if m.filePickerScreen != nil {
		m.lastPickerLocation = m.filePickerScreen.GetLastLocation()
	}
	m.PopScreen()
	// Initiate upload
	return m.initiateFileUpload(msg.Path)
}

func (m *Model) handleFilePickerCancelledMsg() {
	// Remember location for next time
	if m.filePickerScreen != nil {
		m.lastPickerLocation = m.filePickerScreen.GetLastLocation()
	}
	m.PopScreen()
}
