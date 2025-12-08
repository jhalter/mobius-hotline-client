package style

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/charmtone"
)

// Theme defines the color palette for the application UI.
// Uses the CharmTone color palette from github.com/charmbracelet/x/exp/charmtone
type Theme struct {
	Name string

	// Text colors
	TextMuted lipgloss.Color // Muted/de-emphasized text (join/leave messages, stats, empty states)

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
	BackgroundPanel lipgloss.AdaptiveColor // Panel/subscreen backgrounds

	// Dialog/Modal
	DialogBorder lipgloss.Color // Modal border color

	// Gradient colors (for banner)
	GradientStart lipgloss.Color
	GradientEnd   lipgloss.Color

	// Whitespace/subtle background pattern
	Subtle lipgloss.AdaptiveColor
}

// CurrentTheme is the active theme used throughout the application.
// Uses the CharmTone color palette as the default and only theme.
var CurrentTheme = Theme{
	Name:            "CharmTone",
	TextMuted:       lipgloss.Color(charmtone.Squid.Hex()),
	Highlight:       lipgloss.Color(charmtone.Charple.Hex()),
	Accent:          lipgloss.Color(charmtone.Dolly.Hex()),
	Success:         lipgloss.Color(charmtone.Guac.Hex()),
	Error:           lipgloss.Color(charmtone.Sriracha.Hex()),
	Admin:           lipgloss.Color(charmtone.Sriracha.Hex()),
	BorderPrimary:   lipgloss.Color(charmtone.Charple.Hex()),
	BorderMuted:     lipgloss.Color(charmtone.Charcoal.Hex()),
	BackgroundPanel: lipgloss.AdaptiveColor{Light: charmtone.Butter.Hex(), Dark: charmtone.Pepper.Hex()},
	DialogBorder:    lipgloss.Color(charmtone.Charple.Hex()),
	GradientStart:   lipgloss.Color(charmtone.Coral.Hex()),
	GradientEnd:     lipgloss.Color(charmtone.Sriracha.Hex()),
	Subtle:          lipgloss.AdaptiveColor{Light: charmtone.Ash.Hex(), Dark: charmtone.Charcoal.Hex()},
}
