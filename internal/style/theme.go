package style

import (
	"image/color"

	"github.com/charmbracelet/x/exp/charmtone"
)

// Theme defines the color palette for the application UI.
// Uses the CharmTone color palette from github.com/charmbracelet/x/exp/charmtone
type Theme struct {
	Name string

	// Text colors
	TextMuted color.Color // Muted/de-emphasized text (join/leave messages, stats, empty states)

	// Highlight/accent colors
	Highlight color.Color // Primary highlight (titles, active items, categories)
	Accent    color.Color // Secondary accent (hotkeys)

	// Status colors
	Success color.Color // Completed/success states
	Error   color.Color // Error/failed states

	// User styling
	Admin color.Color // Admin user names

	// Border colors
	BorderPrimary color.Color // Main borders (chat, panels)
	BorderMuted   color.Color // Inactive/scrollback borders

	// Background colors
	BackgroundPanel color.Color // Panel/subscreen backgrounds

	// Dialog/Modal
	DialogBorder color.Color // Modal border color

	// Gradient colors (for banner)
	GradientStart color.Color
	GradientEnd   color.Color

	// Whitespace/subtle background pattern
	Subtle color.Color
}

// CurrentTheme is the active theme used throughout the application.
// Uses the CharmTone color palette as the default and only theme.
var CurrentTheme = Theme{
	Name:            "CharmTone",
	TextMuted:       charmtone.Squid,
	Highlight:       charmtone.Charple,
	Accent:          charmtone.Dolly,
	Success:         charmtone.Guac,
	Error:           charmtone.Sriracha,
	Admin:           charmtone.Sriracha,
	BorderPrimary:   charmtone.Charple,
	BorderMuted:     charmtone.Charcoal,
	BackgroundPanel: charmtone.Pepper,
	DialogBorder:    charmtone.Charple,
	GradientStart:   charmtone.Coral,
	GradientEnd:     charmtone.Sriracha,
	Subtle:          charmtone.Charcoal,
}
