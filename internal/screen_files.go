package internal

import (
	"bytes"
	"encoding/binary"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"
	"github.com/jhalter/mobius-hotline-client/internal/style"
	"github.com/jhalter/mobius/hotline"
)

// Messages sent from FilesScreen to parent

// FilesCancelledMsg signals user wants to close files
type FilesCancelledMsg struct{}

// FilesDownloadMsg signals user wants to download a file
type FilesDownloadMsg struct {
	FileName string
	FilePath []string
}

// FilesGetInfoMsg signals user wants file info
type FilesGetInfoMsg struct {
	FileName string
	FilePath []string
}

// FilesUploadMsg signals user wants to upload a file
type FilesUploadMsg struct{}

// FilesNavigateMsg signals user wants to navigate to a folder
type FilesNavigateMsg struct {
	Path []string
}

// FilesScreen is a self-contained BubbleTea model for browsing files
type FilesScreen struct {
	list          list.Model
	width, height int
	model         *Model

	// Screen-specific state
	filePath []string // Current folder path for navigation
}

// NewFilesScreen creates a new files screen
func NewFilesScreen(m *Model) *FilesScreen {
	// Create empty list initially
	delegate := newFileDelegate()
	h, v := style.AppStyle.GetFrameSize()
	l := list.New([]list.Item{}, delegate, m.width-h, m.height-v)
	l.SetFilteringEnabled(true)
	l.SetShowTitle(false)
	l.SetShowStatusBar(true)
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()

	return &FilesScreen{
		list:     l,
		width:    m.width,
		height:   m.height,
		model:    m,
		filePath: []string{},
	}
}

// Init implements tea.Model
func (s *FilesScreen) Init() tea.Cmd {
	return nil
}

// Update implements ScreenModel
func (s *FilesScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.SetSize(msg.Width, msg.Height)
		return s, nil

	// Handle screen messages by delegating to parent methods
	case FilesCancelledMsg:
		s.model.PopScreen()
		return s, nil

	case FilesDownloadMsg:
		return s, s.model.handleFilesDownloadMsg(msg)

	case FilesGetInfoMsg:
		s.model.handleFilesGetInfoMsg(msg)
		return s, nil

	case FilesUploadMsg:
		return s, s.model.handleFilesUploadMsg()

	case FilesNavigateMsg:
		s.model.handleFilesNavigateMsg(msg)
		return s, nil

	case tea.KeyPressMsg:
		return s.handleKeys(msg)
	}

	// Delegate to internal components
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

// View implements tea.Model
func (s *FilesScreen) View() string {
	s.list.SetSize(s.width-10, s.height-10)
	return style.RenderSubscreen(s.width, s.height, "Files", s.list.View())
}

// SetSize updates dimensions
func (s *FilesScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
	h, v := style.AppStyle.GetFrameSize()
	s.list.SetSize(width-h, height-v)
}

// handleKeys handles keyboard input
func (s *FilesScreen) handleKeys(msg tea.KeyPressMsg) (ScreenModel, tea.Cmd) {
	// Check for filtering state - let list handle most keys during filter
	if s.list.FilterState() == list.Filtering {
		if msg.String() == "esc" {
			// Let list handle esc to exit filtering
			var cmd tea.Cmd
			s.list, cmd = s.list.Update(msg)
			return s, cmd
		}
		// Pass all other keys to list during filtering
		var cmd tea.Cmd
		s.list, cmd = s.list.Update(msg)
		return s, cmd
	}

	switch msg.String() {
	case "esc":
		// Reset file path when closing
		s.filePath = []string{}
		return s, func() tea.Msg { return FilesCancelledMsg{} }

	case "tab":
		// Get selected file for info
		if item, ok := s.list.SelectedItem().(fileItem); ok {
			// Don't allow file info on the "<- Back" option
			if item.name == "<- Back" {
				return s, nil
			}

			path := make([]string, len(s.filePath))
			copy(path, s.filePath)
			name := item.name

			return s, func() tea.Msg {
				return FilesGetInfoMsg{
					FileName: name,
					FilePath: path,
				}
			}
		}
		return s, nil

	case "ctrl+u":
		return s, func() tea.Msg { return FilesUploadMsg{} }

	case "enter":
		if item, ok := s.list.SelectedItem().(fileItem); ok {
			// Handle "<- Back" option
			if item.name == "<- Back" {
				if len(s.filePath) > 0 {
					s.filePath = s.filePath[:len(s.filePath)-1]
				}
				path := make([]string, len(s.filePath))
				copy(path, s.filePath)
				return s, func() tea.Msg {
					return FilesNavigateMsg{Path: path}
				}
			}

			// Handle folder navigation
			if item.isFolder {
				s.filePath = append(s.filePath, item.name)
				path := make([]string, len(s.filePath))
				copy(path, s.filePath)
				return s, func() tea.Msg {
					return FilesNavigateMsg{Path: path}
				}
			}

			// Handle file selection - initiate download
			path := make([]string, len(s.filePath))
			copy(path, s.filePath)
			name := item.name

			return s, func() tea.Msg {
				return FilesDownloadMsg{
					FileName: name,
					FilePath: path,
				}
			}
		}
		return s, nil
	}

	// Pass other keys to the list for navigation
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

// SetFiles updates the file list with new files
func (s *FilesScreen) SetFiles(files []hotline.FileNameWithInfo) {
	var items []list.Item

	// Add "<- Back" option if we're in a subfolder
	if len(s.filePath) > 0 {
		items = append(items, fileItem{
			name:     "<- Back",
			isFolder: true,
			size:     0,
		})
	}

	// Add the files
	for _, f := range files {
		items = append(items, fileItem{
			name:     string(f.Name),
			isFolder: bytes.Equal(f.Type[:], []byte("fldr")),
			size:     binary.BigEndian.Uint32(f.FileSize[:]) / 1024,
			fileType: f.Type,
			creator:  f.Creator,
		})
	}

	s.list.SetItems(items)
}

// GetFilePath returns the current file path
func (s *FilesScreen) GetFilePath() []string {
	return s.filePath
}

// SetFilePath sets the current file path
func (s *FilesScreen) SetFilePath(path []string) {
	s.filePath = path
}

// InitiateDownload creates a download task and returns the download command
func (s *FilesScreen) InitiateDownload(fileName string, filePath []string) tea.Cmd {
	return func() tea.Msg {
		// Create task
		taskID := uuid.New().String()
		task := &Task{
			ID:        taskID,
			FileName:  fileName,
			FilePath:  filePath,
			Status:    TaskPending,
			StartTime: time.Now(),
		}
		s.model.taskManager.Add(task)

		// Send download transaction
		t := hotline.NewTransaction(
			hotline.TranDownloadFile,
			[2]byte{},
			hotline.NewField(hotline.FieldFileName, []byte(fileName)),
		)

		// Add file path if in subdirectory
		if len(filePath) > 0 {
			pathStr := strings.Join(filePath, "/")
			t.Fields = append(t.Fields, hotline.NewField(hotline.FieldFilePath, hotline.EncodeFilePath(pathStr)))
		}

		// Map transaction ID to task ID (on the active session)
		session := s.model.activeSession()
		if session == nil {
			return errorMsg{text: "No active server connection"}
		}
		session.pendingDownloads[t.ID] = taskID

		if err := session.hlClient.Send(t); err != nil {
			s.model.logger.Error("Error sending download transaction", "err", err)
		}

		return nil
	}
}
