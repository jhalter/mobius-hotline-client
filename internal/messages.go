package internal

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jhalter/mobius/hotline"
)

// Internal message types for BubbleTea communication

// SessionMsg wraps any tea.Msg with the originating session ID
// Used to route messages from server connections to the correct session
type SessionMsg struct {
	SessionID string
	Msg       tea.Msg
}

// disconnectSessionMsg is sent when a specific session disconnects
type disconnectSessionMsg struct {
	sessionID string
}

type chatMsg struct {
	text string
}

type userListMsg struct {
	users []hotline.User
}

type messageBoardMsg struct {
	text string
}

type errorMsg struct {
	text string
}

type serverMsgMsg struct {
	from   string
	userID [2]byte
	text   string
	time   string
}

// PrivateMessage represents a pending private message in the stack
type PrivateMessage struct {
	From   string
	UserID [2]byte
	Text   string
	Time   string
}

type agreementMsg struct {
	text string
}

type trackerListMsg struct {
	servers []hotline.ServerRecord
}

type serverConnectedMsg struct {
	name string
}

// serverConnectionAttemptMsg is sent after joinServer() completes (success or failure)
type serverConnectionAttemptMsg struct {
	err error
}

type filesMsg struct {
	files []hotline.FileNameWithInfo
}

type taskProgressMsg struct {
	taskID string
	bytes  int64
}

type disconnectMsg struct {
}

type taskStatusMsg struct {
	taskID string
	status TaskStatus
	err    error
}

type downloadReplyMsg struct {
	txID         [4]byte
	refNum       [4]byte
	transferSize uint32
	fileSize     uint32
}

type uploadReplyMsg struct {
	txID   [4]byte
	refNum [4]byte
}

type newsCategoriesMsg struct {
	categories []newsItem
}

type newsArticlesMsg struct {
	articles []newsArticleItem
}

type newsArticleDataMsg struct {
	article hotline.NewsArtData
}

type fileInfoMsg struct {
	fileName          string
	fileTypeString    string
	fileCreatorString string
	fileType          [4]byte
	createDate        hotline.Time
	modifyDate        hotline.Time
	comment           string
	fileSize          uint32
	hasFileSize       bool // Differentiates files from folders
}

// Account management types

type accountListMsg struct {
	accounts []accountItem
}

type accountItem struct {
	login   string
	name    string
	hasPass bool
	access  hotline.AccessBitmap
	index   int
}

func (a accountItem) FilterValue() string { return a.login }
func (a accountItem) Title() string       { return a.login }
func (a accountItem) Description() string {
	desc := a.name
	if a.hasPass {
		desc += " [password set]"
	}
	return desc
}

type selectedAccountData struct {
	login          string
	name           string
	originalAccess hotline.AccessBitmap
	hasPassword    bool
}

type accessBitInfo struct {
	bit  int
	name string
}
