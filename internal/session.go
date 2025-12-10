package internal

import (
	"context"

	"github.com/jhalter/mobius/hotline"
)

// MaxSessions is the maximum number of concurrent server connections
const MaxSessions = 10

// ServerSession encapsulates all state for a single server connection
type ServerSession struct {
	// Identity
	ID          string // UUID for session identification
	DisplayName string // Server name for tab display
	Address     string // Connection address

	// Hotline client
	hlClient *hotline.Client

	// Connection lifecycle
	connectionCtx       context.Context
	connectionCtxCancel context.CancelFunc
	clientDisconnecting bool

	// Server state
	serverName      string
	userList        []hotline.User
	userAccess      hotline.AccessBitmap
	privateMessages []PrivateMessage

	// Per-session screen stack
	screenHistory []Screen

	// Per-session screens (server-specific)
	serverScreen           *ServerScreen
	newsScreen             *NewsScreen
	newsArticlePostScreen  *NewsArticlePostScreen
	newsArticleViewScreen  *NewsArticleViewScreen
	newsBundleFormScreen   *NewsBundleFormScreen
	newsCategoryFormScreen *NewsCategoryFormScreen
	legacyNewsPostScreen   *LegacyNewsPostScreen
	accountsScreen         *AccountsScreen
	accountEditScreen      *AccountEditScreen
	filesScreen            *FilesScreen
	messageBoardScreen     *MessageBoardScreen
	composeMessageScreen   *ComposeMessageScreen

	// File transfer state (per-session)
	pendingDownloads map[[4]byte]string // transaction ID -> task ID
	pendingUploads   map[[4]byte]string // transaction ID -> task ID

	// Activity indicator for tab display
	hasUnreadActivity bool
}

// CurrentScreen returns the current screen for this session
func (s *ServerSession) CurrentScreen() Screen {
	if len(s.screenHistory) == 0 {
		return ScreenServerUI
	}
	return s.screenHistory[len(s.screenHistory)-1]
}

// PreviousScreen returns the previous screen for this session
func (s *ServerSession) PreviousScreen() Screen {
	if len(s.screenHistory) < 2 {
		return ScreenServerUI
	}
	return s.screenHistory[len(s.screenHistory)-2]
}

// PushScreen adds a new screen to this session's history
func (s *ServerSession) PushScreen(screen Screen) {
	s.screenHistory = append(s.screenHistory, screen)
}

// PopScreen removes current screen and returns to previous
func (s *ServerSession) PopScreen() Screen {
	if len(s.screenHistory) <= 1 {
		s.screenHistory = []Screen{ScreenServerUI}
		return ScreenServerUI
	}
	s.screenHistory = s.screenHistory[:len(s.screenHistory)-1]
	return s.screenHistory[len(s.screenHistory)-1]
}

// ReplaceScreen replaces the current screen without adding to history
func (s *ServerSession) ReplaceScreen(screen Screen) {
	if len(s.screenHistory) == 0 {
		s.screenHistory = []Screen{screen}
	} else {
		s.screenHistory[len(s.screenHistory)-1] = screen
	}
}

// NavigateTo clears history and jumps to a screen
func (s *ServerSession) NavigateTo(screen Screen) {
	s.screenHistory = []Screen{screen}
}

// currentScreenModel returns the current screen as a ScreenModel interface
func (s *ServerSession) currentScreenModel() ScreenModel {
	switch s.CurrentScreen() {
	case ScreenServerUI:
		return s.serverScreen
	case ScreenNews:
		return s.newsScreen
	case ScreenNewsArticlePost:
		return s.newsArticlePostScreen
	case ScreenNewsArticleView:
		return s.newsArticleViewScreen
	case ScreenNewsBundleForm:
		return s.newsBundleFormScreen
	case ScreenNewsCategoryForm:
		return s.newsCategoryFormScreen
	case ScreenLegacyNewsPost:
		return s.legacyNewsPostScreen
	case ScreenAccounts:
		return s.accountsScreen
	case ScreenAccountEdit:
		return s.accountEditScreen
	case ScreenFiles:
		return s.filesScreen
	case ScreenMessageBoard:
		return s.messageBoardScreen
	case ScreenComposeMessage:
		return s.composeMessageScreen
	}
	return nil
}

// resizeAllScreens resizes all session-specific screens
func (s *ServerSession) resizeAllScreens(w, h int) {
	if s.serverScreen != nil {
		s.serverScreen.SetSize(w, h)
	}
	if s.newsScreen != nil {
		s.newsScreen.SetSize(w, h)
	}
	if s.newsArticlePostScreen != nil {
		s.newsArticlePostScreen.SetSize(w, h)
	}
	if s.newsArticleViewScreen != nil {
		s.newsArticleViewScreen.SetSize(w, h)
	}
	if s.newsBundleFormScreen != nil {
		s.newsBundleFormScreen.SetSize(w, h)
	}
	if s.newsCategoryFormScreen != nil {
		s.newsCategoryFormScreen.SetSize(w, h)
	}
	if s.legacyNewsPostScreen != nil {
		s.legacyNewsPostScreen.SetSize(w, h)
	}
	if s.accountsScreen != nil {
		s.accountsScreen.SetSize(w, h)
	}
	if s.accountEditScreen != nil {
		s.accountEditScreen.SetSize(w, h)
	}
	if s.filesScreen != nil {
		s.filesScreen.SetSize(w, h)
	}
	if s.messageBoardScreen != nil {
		s.messageBoardScreen.SetSize(w, h)
	}
	if s.composeMessageScreen != nil {
		s.composeMessageScreen.SetSize(w, h)
	}
}

// Disconnect cleans up the session connection
func (s *ServerSession) Disconnect() {
	s.clientDisconnecting = true
	if s.connectionCtxCancel != nil {
		s.connectionCtxCancel()
		s.connectionCtxCancel = nil
	}
	s.connectionCtx = nil
	if s.hlClient != nil {
		_ = s.hlClient.Disconnect()
	}
}

// isServerScreen returns true if the given screen is a server-specific screen
// (as opposed to a global/pre-connection screen)
func isServerScreen(screen Screen) bool {
	switch screen {
	case ScreenServerUI,
		ScreenNews,
		ScreenNewsArticlePost,
		ScreenNewsArticleView,
		ScreenNewsBundleForm,
		ScreenNewsCategoryForm,
		ScreenLegacyNewsPost,
		ScreenMessageBoard,
		ScreenFiles,
		ScreenAccounts,
		ScreenAccountEdit,
		ScreenComposeMessage:
		return true
	}
	return false
}
