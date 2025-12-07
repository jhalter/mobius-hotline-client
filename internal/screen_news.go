package internal

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/jhalter/mobius-hotline-client/internal/style"
	"github.com/jhalter/mobius/hotline"
	"github.com/muesli/reflow/wordwrap"
)

// Messages sent from NewsScreen to parent
type NewsCancelledMsg struct{}

type NewsNavigateToCategoryMsg struct {
	Path []string
}

type NewsNavigateToBundleMsg struct {
	Path []string
}

type NewsRequestArticleMsg struct {
	Path      []string
	ArticleID uint32
}

type NewsPostArticleMsg struct {
	Subject  string
	ParentID uint32
}

type NewsCreateBundleMsg struct{}

type NewsCreateCategoryMsg struct{}

// newsItem represents a category or bundle in the news hierarchy
type newsItem struct {
	name     string
	isBundle bool // true = bundle (container), false = category (contains articles)
}

func (i newsItem) FilterValue() string { return i.name }
func (i newsItem) Title() string {
	if i.name == "<- Back" {
		return i.name
	}
	if i.isBundle {
		return "📦 " + i.name
	}
	return "📰 " + i.name
}
func (i newsItem) Description() string {
	if i.name == "<- Back" {
		return ""
	}
	if i.isBundle {
		return "Bundle"
	}
	return "Category"
}

// newsArticleItem represents an article in a category
type newsArticleItem struct {
	id          uint32
	title       string
	poster      string
	date        [8]byte
	parentID    uint32 // Parent article ID (0 = root)
	depth       int    // Nesting level
	isExpanded  bool   // Are children shown?
	hasChildren bool   // Has replies?
}

func (i newsArticleItem) FilterValue() string { return i.title }
func (i newsArticleItem) Title() string {
	indent := strings.Repeat("  ", i.depth)

	var indicator string
	if i.hasChildren {
		if i.isExpanded {
			indicator = "∨ "
		} else {
			indicator = "› "
		}
	} else {
		indicator = "  "
	}

	return indent + indicator + i.title
}
func (i newsArticleItem) Description() string {
	// Convert and format timestamp
	timestamp := hotline.Time(i.date).Format("(Jan 2, 2006 at 3:04 PM)")

	// Use lipgloss for faint styling
	timestampStyle := lipgloss.NewStyle().Faint(true)

	return i.poster + "   " + timestampStyle.Render(timestamp)
}

// selectedArticleData holds the full article data for display
type selectedArticleData struct {
	id      uint32
	title   string
	poster  string
	date    [8]byte
	content string
}

// NewsScreen is a self-contained BubbleTea model for browsing threaded news
type NewsScreen struct {
	list            list.Model
	articleViewport viewport.Model
	width, height   int
	model           *Model

	// News state
	newsPath          []string             // Track current location in news hierarchy
	isViewingCategory bool                 // true = viewing category (articles), false = viewing bundle/root
	selectedArticle   *selectedArticleData // Currently selected article
	pendingArticleID  uint32               // Article ID of pending request for tracking
	allArticles       []newsArticleItem    // Complete article set
	expandedArticles  map[uint32]bool      // Track expanded state

	// Form state for posting
	articlePostForm      *huh.Form // For threaded news article posting
	bundleForm           *huh.Form // For creating news bundles
	categoryForm         *huh.Form // For creating news categories
	replyParentArticleID uint32    // Parent article ID when replying (0 for new posts)
}

// NewNewsScreen creates a new news screen
func NewNewsScreen(m *Model) *NewsScreen {
	l := list.New([]list.Item{}, newNewsBundleDelegate(), m.width, m.height)
	l.SetShowTitle(false)
	l.SetFilteringEnabled(true)
	l.SetShowStatusBar(true)
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()

	return &NewsScreen{
		list:             l,
		articleViewport:  viewport.New(0, 0),
		width:            m.width,
		height:           m.height,
		model:            m,
		newsPath:         []string{},
		expandedArticles: make(map[uint32]bool),
		allArticles:      []newsArticleItem{},
	}
}

// SetCategories initializes the screen with news categories
func (s *NewsScreen) SetCategories(categories []newsItem) {
	var items []list.Item
	for _, cat := range categories {
		items = append(items, cat)
	}

	s.list = list.New(items, newNewsBundleDelegate(), s.width, s.height)
	s.list.SetShowTitle(false)
	s.list.SetFilteringEnabled(true)
	s.list.SetShowStatusBar(true)
	s.list.SetShowHelp(true)
	s.list.DisableQuitKeybindings()
	s.isViewingCategory = false
	s.selectedArticle = nil
}

// SetArticles initializes the screen with news articles
func (s *NewsScreen) SetArticles(articles []newsArticleItem) {
	// Build thread tree and store articles
	articles = buildThreadTree(articles)
	s.allArticles = articles

	// Filter for visible articles
	visibleArticles := s.filterVisibleArticles(articles)

	var items []list.Item
	for _, art := range visibleArticles {
		art.isExpanded = s.expandedArticles[art.id]
		items = append(items, art)
	}

	s.list = list.New(items, newNewsArticleDelegate(), s.width, s.height)

	// Build title with category name
	title := "Articles"
	if len(s.newsPath) > 0 {
		title += " - " + s.newsPath[len(s.newsPath)-1]
	}
	s.list.Title = title

	s.list.SetFilteringEnabled(false)
	s.list.SetShowStatusBar(false)
	s.list.SetShowHelp(true)
	s.list.DisableQuitKeybindings()
	s.isViewingCategory = true
	s.selectedArticle = nil
}

// SetArticleData sets the currently selected article's full data
func (s *NewsScreen) SetArticleData(article hotline.NewsArtData) {
	s.selectedArticle = &selectedArticleData{
		id:      s.pendingArticleID,
		title:   article.Title,
		poster:  article.Poster,
		date:    article.Date,
		content: article.Data,
	}
	s.pendingArticleID = 0
}

// Init implements tea.Model
func (s *NewsScreen) Init() tea.Cmd {
	return nil
}

// Update implements ScreenModel
func (s *NewsScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		s.list.SetSize(msg.Width-10, msg.Height-10)
		return s, nil

	// Handle screen messages by delegating to parent
	case NewsCancelledMsg:
		s.model.PopScreen()
		return s, nil
	case NewsNavigateToCategoryMsg:
		s.model.handleNewsNavigateToCategoryMsg(msg)
		return s, nil
	case NewsNavigateToBundleMsg:
		s.model.handleNewsNavigateToBundleMsg(msg)
		return s, nil
	case NewsRequestArticleMsg:
		s.model.handleNewsRequestArticleMsg(msg)
		return s, nil
	case NewsPostArticleMsg:
		cmd := s.model.handleNewsPostArticleMsg(msg)
		return s, cmd
	case NewsCreateBundleMsg:
		cmd := s.model.handleNewsCreateBundleMsg()
		return s, cmd
	case NewsCreateCategoryMsg:
		cmd := s.model.handleNewsCreateCategoryMsg()
		return s, cmd

	case tea.KeyMsg:
		return s.handleKeys(msg)
	}

	// Delegate all other messages to the list
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

func (s *NewsScreen) handleKeys(msg tea.KeyMsg) (ScreenModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// If an article is selected, deselect it first
		if s.selectedArticle != nil {
			s.selectedArticle = nil
			return s, nil
		}

		// Check if we're viewing articles (not categories)
		items := s.list.Items()
		isViewingArticles := false
		if len(items) > 0 {
			for _, item := range items {
				if _, ok := item.(newsArticleItem); ok {
					isViewingArticles = true
					break
				}
			}
		}

		// If viewing articles and not at root, go back one level
		if isViewingArticles && len(s.newsPath) > 0 {
			// Go up one level
			s.newsPath = s.newsPath[:len(s.newsPath)-1]
			pathCopy := make([]string, len(s.newsPath))
			copy(pathCopy, s.newsPath)
			return s, func() tea.Msg {
				return NewsNavigateToBundleMsg{Path: pathCopy}
			}
		}

		// Otherwise close the news modal
		s.newsPath = []string{}
		s.isViewingCategory = false
		s.selectedArticle = nil
		return s, func() tea.Msg { return NewsCancelledMsg{} }

	case "ctrl+p":
		// Only allow posting when viewing a category (not bundles/root)
		if s.isViewingCategory {
			parentID := uint32(0)
			return s, func() tea.Msg {
				return NewsPostArticleMsg{Subject: "", ParentID: parentID}
			}
		}
		return s, nil

	case "ctrl+r":
		// Only allow replying when viewing an article
		if s.selectedArticle == nil {
			return s, nil
		}

		// Create subject with "Re: " prefix
		subject := s.selectedArticle.title
		if !strings.HasPrefix(subject, "Re: ") {
			subject = "Re: " + subject
		}
		parentID := s.selectedArticle.id

		return s, func() tea.Msg {
			return NewsPostArticleMsg{Subject: subject, ParentID: parentID}
		}

	case "ctrl+b":
		// Only allow creating bundles when viewing bundle/root (not categories)
		if !s.isViewingCategory {
			return s, func() tea.Msg { return NewsCreateBundleMsg{} }
		}
		return s, nil

	case "ctrl+c":
		// Only allow creating categories when viewing bundle/root (not articles)
		if !s.isViewingCategory {
			return s, func() tea.Msg { return NewsCreateCategoryMsg{} }
		}
		return s, nil

	case " ":
		// Toggle expand/collapse for articles with children
		selectedItem := s.list.SelectedItem()

		if article, ok := selectedItem.(newsArticleItem); ok {
			if article.hasChildren {
				if s.expandedArticles[article.id] {
					delete(s.expandedArticles, article.id)
				} else {
					s.expandedArticles[article.id] = true
				}

				s.refreshArticleList()
			}
		}
		return s, nil

	case "pgup", "pgdown", "up", "down":
		// If we have an article selected, handle viewport scrolling
		if s.selectedArticle != nil {
			switch msg.String() {
			case "pgup":
				s.articleViewport.PageUp()
				return s, nil
			case "pgdown":
				s.articleViewport.PageDown()
				return s, nil
			}
		}
		// Otherwise pass to list
		var cmd tea.Cmd
		s.list, cmd = s.list.Update(msg)
		return s, cmd

	case "enter":
		// Get selected item
		selectedItem := s.list.SelectedItem()

		// Handle newsItem (category/bundle)
		if item, ok := selectedItem.(newsItem); ok {
			// Handle "<- Back" option
			if item.name == "<- Back" {
				// Go up one level
				if len(s.newsPath) > 0 {
					s.newsPath = s.newsPath[:len(s.newsPath)-1]
				}
				s.selectedArticle = nil

				pathCopy := make([]string, len(s.newsPath))
				copy(pathCopy, s.newsPath)
				return s, func() tea.Msg {
					return NewsNavigateToBundleMsg{Path: pathCopy}
				}
			}

			// Navigate into bundle or category
			s.newsPath = append(s.newsPath, item.name)
			s.selectedArticle = nil

			pathCopy := make([]string, len(s.newsPath))
			copy(pathCopy, s.newsPath)

			if item.isBundle {
				return s, func() tea.Msg {
					return NewsNavigateToBundleMsg{Path: pathCopy}
				}
			}
			return s, func() tea.Msg {
				return NewsNavigateToCategoryMsg{Path: pathCopy}
			}
		}

		// Handle newsArticleItem (article)
		if item, ok := selectedItem.(newsArticleItem); ok {
			// Auto-expand parent chain if this is a child article
			if item.parentID != 0 {
				// Walk up parent chain and expand all parents
				parentMap := make(map[uint32]uint32)
				for _, art := range s.allArticles {
					parentMap[art.id] = art.parentID
				}

				currentID := item.parentID
				for currentID != 0 {
					s.expandedArticles[currentID] = true
					currentID = parentMap[currentID]
				}

				// Refresh the list to show the expanded chain
				s.refreshArticleList()
			}

			// Store pending article ID and request full article data
			s.pendingArticleID = item.id

			pathCopy := make([]string, len(s.newsPath))
			copy(pathCopy, s.newsPath)
			articleID := item.id

			return s, func() tea.Msg {
				return NewsRequestArticleMsg{Path: pathCopy, ArticleID: articleID}
			}
		}

		return s, nil
	}

	// Pass other keys to the list
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

// View implements tea.Model
func (s *NewsScreen) View() string {
	// Check if we have an article selected for split view
	if s.selectedArticle != nil {
		return s.renderSplitView()
	}

	// Otherwise, render full modal with list only
	return s.renderListOnly()
}

func (s *NewsScreen) renderListOnly() string {
	// Set news list dimensions
	s.list.SetSize(s.width-10, s.height-10)

	// Place modal centered with dim gray background
	return lipgloss.Place(
		s.width,
		s.height,
		lipgloss.Center,
		lipgloss.Center,
		style.SubScreenStyle.Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				style.SubTitleStyle.Render("News"),
				s.list.View(),
			),
		),
		lipgloss.WithWhitespaceBackground(style.ColorGrey3),
	)
}

func (s *NewsScreen) renderSplitView() string {
	// Calculate available width for article content
	const borderWidth = 2
	const padding = 2 // Padding from Padding(0, 1) on both sides

	articleWidth := (s.width - 10) / 2 // Half of modal width
	wrapWidth := articleWidth - borderWidth - padding
	if wrapWidth < 20 {
		wrapWidth = 20 // Minimum for edge cases
	}

	// Wrap article content to fit width
	wrappedContent := wordwrap.String(s.selectedArticle.content, wrapWidth)

	s.articleViewport.SetContent(wrappedContent)

	// Update list dimensions for top half
	s.list.SetSize(s.width-10, s.height-10)

	// Build top half (article list)
	articleList := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(style.CurrentTheme.BorderPrimary).
		Render(s.list.View())

	// Build bottom half (article content)
	var articleHeader strings.Builder
	articleHeader.WriteString(lipgloss.NewStyle().Bold(true).Render(s.selectedArticle.title))
	articleHeader.WriteString("\n")

	posterInfo := s.selectedArticle.poster

	timestamp := hotline.Time(s.selectedArticle.date).Format("Jan 2, 2006 at 3:04 PM")
	posterInfo += " - " + timestamp

	articleHeader.WriteString(lipgloss.NewStyle().Faint(true).Render(posterInfo))
	articleHeader.WriteString("\n\n")

	renderedArticle := lipgloss.NewStyle().
		Padding(0, 1).
		Render(articleHeader.String() + wrappedContent)

	// Combine top and bottom
	splitView := lipgloss.JoinHorizontal(
		lipgloss.Left,
		articleList,
		renderedArticle,
	)

	return lipgloss.Place(
		s.width,
		s.height,
		lipgloss.Center,
		lipgloss.Center,
		style.SubScreenStyle.Render(
			splitView,
		),
		lipgloss.WithWhitespaceBackground(style.ColorGrey3),
	)
}

// SetSize updates the screen dimensions
func (s *NewsScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.list.SetSize(width-10, height-10)
}

// GetPath returns the current news path
func (s *NewsScreen) GetPath() []string {
	return s.newsPath
}

// SetPath sets the current news path
func (s *NewsScreen) SetPath(path []string) {
	s.newsPath = path
}

// IsViewingCategory returns true if viewing articles in a category
func (s *NewsScreen) IsViewingCategory() bool {
	return s.isViewingCategory
}

// GetSelectedArticle returns the currently selected article
func (s *NewsScreen) GetSelectedArticle() *selectedArticleData {
	return s.selectedArticle
}

// Helper functions

// buildThreadTree calculates depth and hasChildren for each article
func buildThreadTree(articles []newsArticleItem) []newsArticleItem {
	// Build article ID set
	validIDs := make(map[uint32]bool)
	for _, art := range articles {
		validIDs[art.id] = true
	}

	// Build parent -> children map
	childrenMap := make(map[uint32][]int)

	for i, art := range articles {
		if art.parentID != 0 {
			// Orphaned articles become roots
			if !validIDs[art.parentID] {
				articles[i].parentID = 0
			} else {
				childrenMap[art.parentID] = append(childrenMap[art.parentID], i)
			}
		}
	}

	// Set hasChildren flag
	for i := range articles {
		articles[i].hasChildren = len(childrenMap[articles[i].id]) > 0
	}

	// Calculate depths recursively
	var calculateDepth func(articleID uint32, currentDepth int)
	calculateDepth = func(articleID uint32, currentDepth int) {
		for _, childIdx := range childrenMap[articleID] {
			articles[childIdx].depth = currentDepth
			calculateDepth(articles[childIdx].id, currentDepth+1)
		}
	}

	// Start from roots
	for i := range articles {
		if articles[i].parentID == 0 {
			articles[i].depth = 0
			calculateDepth(articles[i].id, 1)
		}
	}

	// Reverse the slice of articles so that newest are ordered first.
	slices.Reverse(articles)

	return articles
}

// filterVisibleArticles returns articles that should be shown based on expansion state
func (s *NewsScreen) filterVisibleArticles(articles []newsArticleItem) []newsArticleItem {
	var visible []newsArticleItem

	// Build article lookup and children map
	articleMap := make(map[uint32]newsArticleItem)
	childrenMap := make(map[uint32][]newsArticleItem)

	for _, art := range articles {
		articleMap[art.id] = art
		if art.parentID != 0 {
			childrenMap[art.parentID] = append(childrenMap[art.parentID], art)
		}
	}

	// Recursively add article and its visible children
	var addArticleAndChildren func(art newsArticleItem)
	addArticleAndChildren = func(art newsArticleItem) {
		visible = append(visible, art)

		// If this article is expanded, add its children
		if s.expandedArticles[art.id] {
			for _, child := range childrenMap[art.id] {
				addArticleAndChildren(child)
			}
		}
	}

	// Start with root articles (in original order)
	for _, art := range articles {
		if art.parentID == 0 {
			addArticleAndChildren(art)
		}
	}

	return visible
}

// refreshArticleList rebuilds the news list with current visibility state
func (s *NewsScreen) refreshArticleList() {
	visibleArticles := s.filterVisibleArticles(s.allArticles)

	var items []list.Item

	for _, art := range visibleArticles {
		art.isExpanded = s.expandedArticles[art.id]
		items = append(items, art)
	}

	currentIndex := s.list.Index()
	s.list.SetItems(items)

	if currentIndex >= len(items) {
		currentIndex = len(items) - 1
	}
	if currentIndex >= 0 {
		s.list.Select(currentIndex)
	}
}

// newNewsBundleDelegate creates a delegate for browsing bundles/categories
func newNewsBundleDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles = style.ListItemStyles

	d.ShortHelpFunc = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "select"),
			),
			key.NewBinding(
				key.WithKeys("^B"),
				key.WithHelp("^B", "new bundle"),
			),
			key.NewBinding(
				key.WithKeys("^C"),
				key.WithHelp("^C", "new category"),
			),
			key.NewBinding(
				key.WithKeys("esc"),
				key.WithHelp("esc", "back"),
			),
		}
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{
			{
				key.NewBinding(
					key.WithKeys("enter"),
					key.WithHelp("enter", "select"),
				),
				key.NewBinding(
					key.WithKeys("^B"),
					key.WithHelp("^B", "new bundle"),
				),
				key.NewBinding(
					key.WithKeys("^C"),
					key.WithHelp("^C", "new category"),
				),
				key.NewBinding(
					key.WithKeys("esc"),
					key.WithHelp("esc", "back"),
				),
			},
		}
	}

	return d
}

// newNewsArticleDelegate creates a delegate for viewing articles in a category
func newNewsArticleDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles = style.ListItemStyles

	d.ShortHelpFunc = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "select"),
			),
			key.NewBinding(
				key.WithKeys("space"),
				key.WithHelp("space", "expand/collapse"),
			),
			key.NewBinding(
				key.WithKeys("^P"),
				key.WithHelp("^P", "new article"),
			),
			key.NewBinding(
				key.WithKeys("^R"),
				key.WithHelp("^R", "reply"),
			),
			key.NewBinding(
				key.WithKeys("esc"),
				key.WithHelp("esc", "back"),
			),
		}
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{
			{
				key.NewBinding(
					key.WithKeys("enter"),
					key.WithHelp("enter", "select"),
				),
				key.NewBinding(
					key.WithKeys("space"),
					key.WithHelp("space", "expand/collapse"),
				),
				key.NewBinding(
					key.WithKeys("^P"),
					key.WithHelp("^P", "new article"),
				),
				key.NewBinding(
					key.WithKeys("^R"),
					key.WithHelp("^R", "reply"),
				),
				key.NewBinding(
					key.WithKeys("esc"),
					key.WithHelp("esc", "back"),
				),
			},
		}
	}

	return d
}

// Form initialization methods

// InitArticlePostForm creates a Huh form for posting threaded news articles
func (s *NewsScreen) InitArticlePostForm(prefillSubject string, parentArticleID uint32) tea.Cmd {
	s.replyParentArticleID = parentArticleID

	subject := prefillSubject

	s.articlePostForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("subject").
				Title("Subject").
				Placeholder("Enter article subject").
				Value(&subject).
				CharLimit(255).
				Validate(func(str string) error {
					if len(strings.TrimSpace(str)) == 0 {
						return fmt.Errorf("subject cannot be empty")
					}
					return nil
				}),

			huh.NewText().
				Key("body").
				Title("Body").
				CharLimit(4000).
				Validate(func(str string) error {
					if len(strings.TrimSpace(str)) == 0 {
						return fmt.Errorf("body cannot be empty")
					}
					return nil
				}),

			huh.NewConfirm().
				Key("confirm").
				Title("Post this article?").
				Affirmative("Post").
				Negative("Cancel"),
		),
	).
		WithWidth(60).
		WithShowHelp(true).
		WithShowErrors(true).
		WithTheme(style.FormTheme)

	return s.articlePostForm.Init()
}

// InitBundleForm creates a Huh form for creating a new News Bundle
func (s *NewsScreen) InitBundleForm() tea.Cmd {
	bundleName := ""

	s.bundleForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("bundleName").
				Title("Bundle Name").
				Placeholder("Enter bundle name").
				Value(&bundleName).
				CharLimit(255).
				Validate(func(str string) error {
					if len(strings.TrimSpace(str)) == 0 {
						return fmt.Errorf("bundle name cannot be empty")
					}
					return nil
				}),

			huh.NewConfirm().
				Key("confirm").
				Title("Create this bundle?").
				Affirmative("Create").
				Negative("Cancel"),
		),
	).
		WithWidth(60).
		WithShowHelp(true).
		WithShowErrors(true).
		WithTheme(style.FormTheme)

	return s.bundleForm.Init()
}

// InitCategoryForm creates a Huh form for creating a new News Category
func (s *NewsScreen) InitCategoryForm() tea.Cmd {
	categoryName := ""

	s.categoryForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("categoryName").
				Title("Category Name").
				Placeholder("Enter category name").
				Value(&categoryName).
				CharLimit(255).
				Validate(func(str string) error {
					if len(strings.TrimSpace(str)) == 0 {
						return fmt.Errorf("category name cannot be empty")
					}
					return nil
				}),

			huh.NewConfirm().
				Key("confirm").
				Title("Create this category?").
				Affirmative("Create").
				Negative("Cancel"),
		),
	).
		WithWidth(60).
		WithShowHelp(true).
		WithShowErrors(true).
		WithTheme(style.FormTheme)

	return s.categoryForm.Init()
}

// IsFormActive returns true if any form is currently being displayed
func (s *NewsScreen) IsFormActive() bool {
	return s.articlePostForm != nil || s.bundleForm != nil || s.categoryForm != nil
}

// CancelForm cancels any active form
func (s *NewsScreen) CancelForm() {
	s.articlePostForm = nil
	s.bundleForm = nil
	s.categoryForm = nil
	s.replyParentArticleID = 0
}

// GetReplyParentArticleID returns the parent article ID for replies
func (s *NewsScreen) GetReplyParentArticleID() uint32 {
	return s.replyParentArticleID
}

// GetArticlePostForm returns the article post form if active
func (s *NewsScreen) GetArticlePostForm() *huh.Form {
	return s.articlePostForm
}

// GetBundleForm returns the bundle form if active
func (s *NewsScreen) GetBundleForm() *huh.Form {
	return s.bundleForm
}

// GetCategoryForm returns the category form if active
func (s *NewsScreen) GetCategoryForm() *huh.Form {
	return s.categoryForm
}

// ClearArticlePostForm clears the article post form
func (s *NewsScreen) ClearArticlePostForm() {
	s.articlePostForm = nil
	s.replyParentArticleID = 0
}

// ClearBundleForm clears the bundle form
func (s *NewsScreen) ClearBundleForm() {
	s.bundleForm = nil
}

// ClearCategoryForm clears the category form
func (s *NewsScreen) ClearCategoryForm() {
	s.categoryForm = nil
}

// RenderForm renders the currently active form
func (s *NewsScreen) RenderForm() string {
	var formView string
	var title string

	if s.articlePostForm != nil {
		formView = s.articlePostForm.View()
		title = "New Article"
	} else if s.bundleForm != nil {
		formView = s.bundleForm.View()
		title = "New News Bundle"
	} else if s.categoryForm != nil {
		formView = s.categoryForm.View()
		title = "New News Category"
	}

	return style.RenderSubscreen(
		s.width,
		s.height,
		title,
		formView,
	)
}

// UpdateForm updates the active form and returns a command
func (s *NewsScreen) UpdateForm(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	if s.articlePostForm != nil {
		form, cmd := s.articlePostForm.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			s.articlePostForm = f
			cmds = append(cmds, cmd)
		}
	} else if s.bundleForm != nil {
		form, cmd := s.bundleForm.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			s.bundleForm = f
			cmds = append(cmds, cmd)
		}
	} else if s.categoryForm != nil {
		form, cmd := s.categoryForm.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			s.categoryForm = f
			cmds = append(cmds, cmd)
		}
	}

	return tea.Batch(cmds...)
}
