package internal

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/jhalter/mobius-hotline-client/internal/style"
)

// Tab bar styles
var (
	tabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Background(style.CurrentTheme.Accent).
			Padding(0, 1)

	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(style.CurrentTheme.TextMuted).
				Background(style.CurrentTheme.BorderMuted).
				Padding(0, 1)

	tabActivityStyle = lipgloss.NewStyle().
				Foreground(style.CurrentTheme.Highlight).
				Bold(true)

	tabBarStyle = lipgloss.NewStyle().
			Background(style.CurrentTheme.BackgroundPanel)
)

// RenderTabBar renders the horizontal tab bar showing all connected servers
func (m *Model) RenderTabBar() string {
	if len(m.sessions) == 0 {
		return ""
	}

	var tabs []string

	// Prepend app name with gradient styling
	appName := style.ApplyBoldForegroundGrad("Mobius", style.CurrentTheme.GradientStart, style.CurrentTheme.GradientEnd)
	appNameStyled := lipgloss.NewStyle().
		Background(style.CurrentTheme.BackgroundPanel).
		PaddingRight(2).
		Render(appName)
	tabs = append(tabs, appNameStyled)

	for i, session := range m.sessions {
		isActive := i == m.activeSessionIndex

		// Determine tab label
		tabLabel := session.DisplayName
		if tabLabel == "" {
			tabLabel = session.Address
		}
		// Truncate long names - active tabs get more space
		maxLen := 15
		if isActive {
			maxLen = 25
		}
		if len(tabLabel) > maxLen {
			tabLabel = tabLabel[:maxLen-3] + "..."
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
		if isActive {
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
		Background(style.CurrentTheme.Accent).
		Padding(0, 1)

	tabInactiveStyle = lipgloss.NewStyle().
		Foreground(style.CurrentTheme.TextMuted).
		Background(style.CurrentTheme.BorderMuted).
		Padding(0, 1)

	tabActivityStyle = lipgloss.NewStyle().
		Foreground(style.CurrentTheme.Highlight).
		Bold(true)

	tabBarStyle = lipgloss.NewStyle().
		Background(style.CurrentTheme.BackgroundPanel)
}
