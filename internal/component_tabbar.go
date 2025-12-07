package internal

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/jhalter/mobius-hotline-client/internal/style"
)

// Tab bar styles
var (
	tabActiveStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#5555FF")).
		Padding(0, 1)

	tabInactiveStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#AAAAAA")).
		Background(lipgloss.Color("#333333")).
		Padding(0, 1)

	tabActivityStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFF00")).
		Bold(true)

	tabBarStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#222222"))
)

// RenderTabBar renders the horizontal tab bar showing all connected servers
func (m *Model) RenderTabBar() string {
	if len(m.sessions) == 0 {
		return ""
	}

	var tabs []string
	for i, session := range m.sessions {
		// Determine tab label
		tabLabel := session.DisplayName
		if tabLabel == "" {
			tabLabel = session.Address
		}
		// Truncate long names
		if len(tabLabel) > 15 {
			tabLabel = tabLabel[:12] + "..."
		}

		// Keyboard shortcut indicator
		shortcut := fmt.Sprintf("F%d", i+1) // F1-F9, F10 for 10th

		// Activity indicator
		indicator := ""
		if session.hasUnreadActivity && i != m.activeSessionIndex {
			indicator = tabActivityStyle.Render("*")
		}

		// Build tab content
		tabContent := fmt.Sprintf("%s %s%s", shortcut, tabLabel, indicator)

		// Style based on active state
		var styledTab string
		if i == m.activeSessionIndex {
			styledTab = tabActiveStyle.Render(tabContent)
		} else {
			styledTab = tabInactiveStyle.Render(tabContent)
		}

		tabs = append(tabs, styledTab)
	}

	// Add "+" tab for new connections
	plusContent := "ctrl+k +"
	plusTab := tabInactiveStyle.Render(plusContent)
	tabs = append(tabs, plusTab)

	// Join tabs horizontally
	tabLine := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	// Add padding to fill width
	return tabBarStyle.Width(m.width).Render(tabLine)
}

// TabBarHeight returns the height of the tab bar (1 line when visible, 0 otherwise)
func (m *Model) TabBarHeight() int {
	if len(m.sessions) > 0 {
		return 1
	}
	return 0
}

// updateTabBarStyles updates tab bar colors based on current theme
func updateTabBarStyles() {
	tabActiveStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(style.CurrentTheme.Accent).
		Padding(0, 1)

	tabInactiveStyle = lipgloss.NewStyle().
		Foreground(style.CurrentTheme.TextMuted).
		Background(lipgloss.Color("#333333")).
		Padding(0, 1)

	tabActivityStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFF00")).
		Bold(true)

	tabBarStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#222222"))
}
