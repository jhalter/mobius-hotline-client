package internal

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/jhalter/mobius-hotline-client/internal/style"
)

type Bookmark struct {
	Name     string `yaml:"Name"`
	Addr     string `yaml:"Addr"`
	Login    string `yaml:"Login"`
	Password string `yaml:"Password"`
	TLS      bool   `yaml:"TLS"`
}

// Messages sent from BookmarkScreen to parent
type BookmarkSelectedMsg struct {
	Bookmark Bookmark
}

type BookmarkEditMsg struct {
	Bookmark Bookmark
	Index    int
}

type BookmarkCreateMsg struct{}

type BookmarkCancelledMsg struct{}

type BookmarkDeletedMsg struct {
	Bookmark Bookmark
}

// BookmarkScreen is a self-contained BubbleTea model for browsing bookmarks
type BookmarkScreen struct {
	list          list.Model
	width, height int
	model         *Model
}

// NewBookmarkScreen creates a new bookmark screen with the given bookmarks
func NewBookmarkScreen(bookmarks []Bookmark, m *Model) *BookmarkScreen {
	items := make([]list.Item, len(bookmarks))
	for i, bm := range bookmarks {
		items[i] = bookmarkItem{bookmark: bm, index: i}
	}

	h, v := style.AppStyle.GetFrameSize()
	l := list.New(items, newBookmarkDelegate(), m.width-h, m.height-v)
	l.SetFilteringEnabled(true)
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()

	return &BookmarkScreen{
		list:   l,
		width:  m.width,
		height: m.height,
		model:  m,
	}
}

// Init implements tea.Model
func (s *BookmarkScreen) Init() tea.Cmd {
	return nil
}

// Update implements ScreenModel
func (s *BookmarkScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		h, v := style.AppStyle.GetFrameSize()
		s.list.SetSize(msg.Width-h, msg.Height-v)
		return s, nil

	case BookmarkSelectedMsg:
		s.model.handleBookmarkSelectedMsg(msg)
	case BookmarkEditMsg:
		s.model.handleBookmarkEditMsg(msg)
	case BookmarkCreateMsg:
		s.model.handleBookmarkCreateMsg(msg)
	case BookmarkCancelledMsg:
		s.model.handleBookmarkCancelledMsg(msg)
	case BookmarkDeletedMsg:
		s.model.handleBookmarkDeletedMsg(msg)

	case tea.KeyPressMsg:
		// Handle custom keys when NOT actively filtering
		if s.list.FilterState() != list.Filtering {
			switch msg.String() {
			case "esc":
				return s, func() tea.Msg { return BookmarkCancelledMsg{} }

			case "enter":
				if item, ok := s.list.SelectedItem().(bookmarkItem); ok {
					return s, func() tea.Msg {
						return BookmarkSelectedMsg{Bookmark: item.bookmark}
					}
				}
				return s, nil

			case "e":
				if item, ok := s.list.SelectedItem().(bookmarkItem); ok {
					bm := item.bookmark
					idx := item.index
					return s, func() tea.Msg {
						return BookmarkEditMsg{Bookmark: bm, Index: idx}
					}
				}
				return s, nil

			case "n":
				return s, func() tea.Msg { return BookmarkCreateMsg{} }

			case "x":
				if item, ok := s.list.SelectedItem().(bookmarkItem); ok {
					bm := item.bookmark
					// Remove from list UI
					index := s.list.Index()
					if index >= 0 && index < len(s.list.Items()) {
						s.list.RemoveItem(index)
					}
					// Emit message for parent to handle persistence
					var statusCmd tea.Cmd
					if len(s.list.Items()) == 0 {
						statusCmd = s.list.NewStatusMessage("All bookmarks deleted")
					} else {
						statusCmd = s.list.NewStatusMessage("Deleted bookmark")
					}
					return s, tea.Batch(
						statusCmd,
						func() tea.Msg { return BookmarkDeletedMsg{Bookmark: bm} },
					)
				}
				return s, nil
			}
		}
	}

	// Delegate all other messages to the list
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

// View implements ScreenModel
func (s *BookmarkScreen) View() tea.View {
	s.list.SetSize(s.width-10, s.height-10)

	return tea.NewView(style.RenderSubscreen(s.width, s.height, "Bookmarks", s.list.View()))
}

// SetSize updates the screen dimensions
func (s *BookmarkScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
	h, v := style.AppStyle.GetFrameSize()
	s.list.SetSize(width-h, height-v)
}

// bookmarkItem represents a bookmark in the list
type bookmarkItem struct {
	bookmark Bookmark
	index    int
}

func (i bookmarkItem) FilterValue() string {
	// Include both name and address for filtering
	return i.bookmark.Name + " " + i.bookmark.Addr
}
func (i bookmarkItem) Title() string       { return i.bookmark.Name }
func (i bookmarkItem) Description() string { return i.bookmark.Addr }

// newBookmarkDelegate creates a custom delegate for bookmark list items
func newBookmarkDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles = style.ListItemStyles

	// Add custom help text for bookmark-specific keys
	d.ShortHelpFunc = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "choose"),
			),
			key.NewBinding(
				key.WithKeys("e"),
				key.WithHelp("e", "edit"),
			),
			key.NewBinding(
				key.WithKeys("n"),
				key.WithHelp("n", "new"),
			),
			key.NewBinding(
				key.WithKeys("x"),
				key.WithHelp("x", "delete"),
			),
		}
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{
			{
				key.NewBinding(
					key.WithKeys("enter"),
					key.WithHelp("enter", "choose"),
				),
				key.NewBinding(
					key.WithKeys("e"),
					key.WithHelp("e", "edit"),
				),
				key.NewBinding(
					key.WithKeys("n"),
					key.WithHelp("n", "new"),
				),
				key.NewBinding(
					key.WithKeys("/"),
					key.WithHelp("/", "filter"),
				),
			},
		}
	}

	return d
}
