package style

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme defines the color palette for the application UI.
// Use semantic color names that describe the purpose, not the color itself.
type Theme struct {
	Name string

	// Text colors
	TextMuted    lipgloss.Color // Muted/de-emphasized text (join/leave messages, stats, empty states)
	TextDisabled lipgloss.Color // Disabled items

	// Highlight/accent colors
	Highlight lipgloss.Color // Primary highlight (titles, active items, categories)
	Accent    lipgloss.Color // Secondary accent (hotkeys)

	// Status colors
	Success lipgloss.Color // Completed/success states
	Error   lipgloss.Color // Error/failed states

	// User styling
	Admin lipgloss.Color // Admin user names

	// Border colors
	BorderPrimary lipgloss.Color // Main borders (chat, panels)
	BorderMuted   lipgloss.Color // Inactive/scrollback borders

	// Background colors
	BackgroundPanel lipgloss.Color // Panel/subscreen backgrounds

	// Dialog/Modal
	DialogBorder lipgloss.Color // Modal border color

	// Gradient colors (for banner)
	GradientStart lipgloss.Color
	GradientEnd   lipgloss.Color

	// Whitespace/subtle background pattern
	Subtle lipgloss.AdaptiveColor
}

// CurrentTheme is the active theme used throughout the application.
var CurrentTheme = DefaultTheme()

// DefaultTheme returns the classic Mobius theme with the original color scheme.
func DefaultTheme() Theme {
	return Theme{
		Name:            "Classic",
		TextMuted:       lipgloss.Color("241"),
		TextDisabled:    lipgloss.Color("240"),
		Highlight:       lipgloss.Color("170"), // Fuchsia
		Accent:          lipgloss.Color("214"), // Orange
		Success:         lipgloss.Color("2"),   // Green
		Error:           lipgloss.Color("1"),   // Red
		Admin:           lipgloss.Color("196"), // Bright red
		BorderPrimary:   lipgloss.Color("63"),  // Cyan
		BorderMuted:     lipgloss.Color("236"), // Dark grey
		BackgroundPanel: lipgloss.Color("236"),
		DialogBorder:    lipgloss.Color("#874BFD"),
		GradientStart:   lipgloss.Color("196"),
		GradientEnd:     lipgloss.Color("#BF281B"),
		Subtle:          lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"},
	}
}

// SetTheme updates the current theme and regenerates all styles.
func SetTheme(t Theme) {
	CurrentTheme = t
	regenerateStyles()
}
