package internal

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/jhalter/mobius-hotline-client/internal/style"
	"github.com/jhalter/mobius/hotline"
)

// Messages sent from TrackerScreen to parent
type TrackerServerSelectedMsg struct {
	Server hotline.ServerRecord
}

type TrackerCancelledMsg struct{}

// TrackerScreen is a self-contained BubbleTea model for browsing tracker servers
type TrackerScreen struct {
	list          list.Model
	width, height int
	model         *Model
}

// NewTrackerScreen creates a new tracker screen with the given server list
func NewTrackerScreen(servers []hotline.ServerRecord, m *Model) *TrackerScreen {
	items := make([]list.Item, len(servers))
	for i, srv := range servers {
		items[i] = trackerItem{server: srv}
	}

	h, v := style.AppStyle.GetFrameSize()

	l := list.New(items, newTrackerDelegate(), h, m.height-v)
	l.SetFilteringEnabled(true)
	l.SetShowTitle(false)
	l.SetShowStatusBar(true)
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()

	return &TrackerScreen{
		list:   l,
		width:  m.width,
		height: m.height,
		model:  m,
	}
}

// Init implements tea.Model
func (s *TrackerScreen) Init() tea.Cmd {
	return nil
}

// Update implements ScreenModel
func (s *TrackerScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		s.list.SetSize(msg.Width, msg.Height)
		return s, nil
	case TrackerServerSelectedMsg:
		s.model.handleTrackerServerSelectedMsg(msg)
	case TrackerCancelledMsg:
		s.model.handleTrackerCancelledMsg(msg)
	case tea.KeyPressMsg:
		// Handle custom keys when NOT actively filtering
		if s.list.FilterState() != list.Filtering {
			switch msg.String() {
			case "esc":
				return s, func() tea.Msg { return TrackerCancelledMsg{} }

			case "enter":
				if item, ok := s.list.SelectedItem().(trackerItem); ok {
					return s, func() tea.Msg {
						return TrackerServerSelectedMsg{Server: item.server}
					}
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
func (s *TrackerScreen) View() tea.View {
	s.list.SetSize(s.width, s.height-6)

	return tea.NewView(style.RenderSubscreen(s.width, s.height, "Tracker Servers", s.list.View()))
}

// SetSize updates the screen dimensions
func (s *TrackerScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.list.SetSize(width, height)
}

// trackerItem represents a server in the tracker list
type trackerItem struct {
	server hotline.ServerRecord
}

func (i trackerItem) FilterValue() string { return string(i.server.Name) }

func (i trackerItem) Title() string {
	name := string(i.server.Name)
	if i.server.TLSPort != [2]byte{0, 0} {
		name = "🔒 " + name
	}

	// Append user count if > 0
	numUsers := binary.BigEndian.Uint16(i.server.NumUsers[:])
	if numUsers > 0 {
		name += fmt.Sprintf(" (%d 🟢)", numUsers)
	}

	return name
}

func (i trackerItem) Description() string { return string(i.server.Description) }

// newTrackerDelegate creates a custom delegate for tracker list items
func newTrackerDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles = style.ListItemStyles

	d.ShortHelpFunc = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "choose"),
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
					key.WithKeys("/"),
					key.WithHelp("/", "filter"),
				),
			},
		}
	}

	return d
}

// fetchTrackerList fetches the server list from the configured tracker
func fetchTrackerList(tracker string) tea.Cmd {
	return func() tea.Msg {
		conn, err := net.DialTimeout("tcp", tracker, 5*time.Second)
		if err != nil {
			return errorMsg{text: fmt.Sprintf("Error connecting to tracker:\n%v", err)}
		}
		defer func() { _ = conn.Close() }()

		// Set read deadline to prevent hanging
		if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
			return errorMsg{text: fmt.Sprintf("Error setting connection deadline:\n%v", err)}
		}

		listing, err := hotline.GetListing(conn)
		if err != nil {
			return errorMsg{text: fmt.Sprintf("Error fetching tracker results:\n%v", err)}
		}

		return trackerListMsg{servers: listing}
	}
}
