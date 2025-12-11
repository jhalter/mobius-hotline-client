package internal

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jhalter/mobius-hotline-client/internal/style"
	"github.com/jhalter/mobius/hotline"
	"github.com/muesli/reflow/wordwrap"
)

// Messages sent from NewsArticleViewScreen to parent

// NewsArticleViewCancelledMsg signals user wants to close the article view
type NewsArticleViewCancelledMsg struct{}

// NewsArticleViewReplyMsg signals user wants to reply to the article
type NewsArticleViewReplyMsg struct {
	ArticleID uint32
	Subject   string
	Path      []string
}

// newsArticleViewScreenKeyMap defines key bindings for the article view screen
type newsArticleViewScreenKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Reply    key.Binding
	Back     key.Binding
}

func (k newsArticleViewScreenKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.PageUp, k.PageDown, k.Reply, k.Back}
}

func (k newsArticleViewScreenKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Reply, k.Back},
	}
}

// NewsArticleViewScreen is a self-contained BubbleTea model for viewing a news article
type NewsArticleViewScreen struct {
	viewport      viewport.Model
	width, height int
	model         *Model
	help          help.Model
	keys          newsArticleViewScreenKeyMap

	// Article data
	articleID uint32
	title     string
	poster    string
	date      [8]byte
	content   string
	path      []string // News path for reply functionality
}

// NewNewsArticleViewScreen creates a new article view screen
func NewNewsArticleViewScreen(
	articleID uint32,
	title string,
	poster string,
	date [8]byte,
	content string,
	path []string,
	m *Model,
) *NewsArticleViewScreen {
	keys := newsArticleViewScreenKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdn", "page down"),
		),
		Reply: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("^R", "reply"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
	}

	vp := viewport.New(viewport.WithWidth(m.width-10), viewport.WithHeight(m.height-10))

	screen := &NewsArticleViewScreen{
		viewport:  vp,
		width:     m.width,
		height:    m.height,
		model:     m,
		help:      help.New(),
		keys:      keys,
		articleID: articleID,
		title:     title,
		poster:    poster,
		date:      date,
		content:   content,
		path:      path,
	}

	screen.updateViewportContent()
	return screen
}

// updateViewportContent formats and sets the article content in the viewport
func (s *NewsArticleViewScreen) updateViewportContent() {
	// Format timestamp
	timestamp := hotline.Time(s.date).Format("Jan 2, 2006 at 3:04 PM")

	// Build header
	headerStyle := lipgloss.NewStyle().Bold(true)
	metaStyle := lipgloss.NewStyle().Faint(true)

	header := lipgloss.JoinVertical(
		lipgloss.Left,
		headerStyle.Render(s.title),
		metaStyle.Render(fmt.Sprintf("By %s on %s", s.poster, timestamp)),
	)

	// Wrap content to fit viewport width
	contentWidth := s.viewport.Width() - 2
	if contentWidth < 20 {
		contentWidth = 20
	}
	wrappedContent := wordwrap.String(s.content, contentWidth)

	s.viewport.SetContent(lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		" ",
		wrappedContent,
	))
}

// Init implements tea.Model
func (s *NewsArticleViewScreen) Init() tea.Cmd {
	return nil
}

// Update implements ScreenModel
func (s *NewsArticleViewScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.SetSize(msg.Width, msg.Height)
		return s, nil

	case NewsArticleViewCancelledMsg:
		s.model.PopScreen()
		return s, nil

	case NewsArticleViewReplyMsg:
		return s, s.model.handleNewsArticleViewReplyMsg(msg)

	case tea.KeyPressMsg:
		return s.handleKeys(msg)
	}

	// Delegate to viewport for scrolling
	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

// View implements ScreenModel
func (s *NewsArticleViewScreen) View() tea.View {
	content := lipgloss.Place(
		s.width,
		s.height-10,
		lipgloss.Left,
		lipgloss.Center,
		style.SubScreenStyle.Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				style.SubTitleStyle.Render("News Article"),
				s.viewport.View(),
				" ",
				lipgloss.JoinHorizontal(
					lipgloss.Left,
					s.help.View(s.keys),
					"  ",
					fmt.Sprintf("%3.f%%", s.viewport.ScrollPercent()*100),
				),
			),
		),
		lipgloss.WithWhitespaceChars(style.CurrentTheme.BackgroundChar),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(style.Subtle)),
	)
	return tea.NewView(content)
}

// SetSize updates dimensions
func (s *NewsArticleViewScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.viewport.SetWidth(width - 10)
	s.viewport.SetHeight(height - 16) // Account for title, help bar, padding
	s.updateViewportContent()
}

// handleKeys handles keyboard input
func (s *NewsArticleViewScreen) handleKeys(msg tea.KeyPressMsg) (ScreenModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return s, func() tea.Msg { return NewsArticleViewCancelledMsg{} }

	case "ctrl+r":
		// Create subject with "Re: " prefix if not already present
		subject := s.title
		if !strings.HasPrefix(subject, "Re: ") {
			subject = "Re: " + subject
		}
		articleID := s.articleID
		pathCopy := make([]string, len(s.path))
		copy(pathCopy, s.path)

		return s, func() tea.Msg {
			return NewsArticleViewReplyMsg{
				ArticleID: articleID,
				Subject:   subject,
				Path:      pathCopy,
			}
		}
	}

	// Pass all other keys to viewport for scrolling
	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

// SetUserAccess updates key bindings based on user permissions
func (s *NewsArticleViewScreen) SetUserAccess(access hotline.AccessBitmap) {
	s.keys.Reply.SetEnabled(access.IsSet(hotline.AccessNewsPostArt))
}
