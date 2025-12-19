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
	TextColor  color.Color // Default color of text
	TextMuted  color.Color // Muted/de-emphasized text (join/leave messages, stats, empty states)
	TextSubtle color.Color // Subtle text (help descriptions)

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

	BackgroundChar string
}

// CurrentTheme is the active theme used throughout the application.
// Uses the CharmTone color palette as the default and only theme.
var CurrentTheme = Theme{
	Name:            "CharmTone",
	TextColor:       charmtone.Salt,
	TextMuted:       charmtone.Squid,
	TextSubtle:      charmtone.Oyster,
	Highlight:       charmtone.Charple,
	Accent:          charmtone.Dolly,
	Success:         charmtone.Guac,
	Error:           charmtone.Sriracha,
	Admin:           charmtone.Sriracha,
	BorderPrimary:   charmtone.Charple,
	BorderMuted:     charmtone.Charcoal,
	BackgroundPanel: charmtone.Pepper,
	DialogBorder:    charmtone.Sapphire,
	GradientStart:   charmtone.Coral,
	GradientEnd:     charmtone.Sriracha,
	Subtle:          charmtone.Charcoal,
	BackgroundChar:  "☃",
}
