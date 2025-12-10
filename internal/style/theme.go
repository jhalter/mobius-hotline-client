package style

import (
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"github.com/charmbracelet/x/exp/charmtone"
)

// Theme defines the color palette for the application UI.
// Uses the CharmTone color palette from github.com/charmbracelet/x/exp/charmtone
type Theme struct {
	Name string

	// Text colors
	TextMuted compat.AdaptiveColor // Muted/de-emphasized text (join/leave messages, stats, empty states)

	// Highlight/accent colors
	Highlight compat.AdaptiveColor // Primary highlight (titles, active items, categories)
	Accent    compat.AdaptiveColor // Secondary accent (hotkeys)

	// Status colors
	Success compat.AdaptiveColor // Completed/success states
	Error   compat.AdaptiveColor // Error/failed states

	// User styling
	Admin compat.AdaptiveColor // Admin user names

	// Border colors
	BorderPrimary compat.AdaptiveColor // Main borders (chat, panels)
	BorderMuted   compat.AdaptiveColor // Inactive/scrollback borders

	// Background colors
	BackgroundPanel compat.AdaptiveColor // Panel/subscreen backgrounds

	// Dialog/Modal
	DialogBorder compat.AdaptiveColor // Modal border color

	// Gradient colors (for banner)
	GradientStart compat.AdaptiveColor
	GradientEnd   compat.AdaptiveColor

	// Whitespace/subtle background pattern
	Subtle compat.AdaptiveColor
}

// CurrentTheme is the active theme used throughout the application.
// Uses the CharmTone color palette as the default and only theme.
var CurrentTheme = Theme{
	Name:            "CharmTone",
	TextMuted:       compat.AdaptiveColor{Light: lipgloss.Color(charmtone.Oyster.Hex()), Dark: lipgloss.Color(charmtone.Squid.Hex())},
	Highlight:       compat.AdaptiveColor{Light: lipgloss.Color(charmtone.Jelly.Hex()), Dark: lipgloss.Color(charmtone.Charple.Hex())},
	Accent:          compat.AdaptiveColor{Light: lipgloss.Color(charmtone.Macaron.Hex()), Dark: lipgloss.Color(charmtone.Dolly.Hex())},
	Success:         compat.AdaptiveColor{Light: lipgloss.Color(charmtone.Pickle.Hex()), Dark: lipgloss.Color(charmtone.Guac.Hex())},
	Error:           compat.AdaptiveColor{Light: lipgloss.Color(charmtone.Pom.Hex()), Dark: lipgloss.Color(charmtone.Sriracha.Hex())},
	Admin:           compat.AdaptiveColor{Light: lipgloss.Color(charmtone.Pom.Hex()), Dark: lipgloss.Color(charmtone.Sriracha.Hex())},
	BorderPrimary:   compat.AdaptiveColor{Light: lipgloss.Color(charmtone.Jelly.Hex()), Dark: lipgloss.Color(charmtone.Charple.Hex())},
	BorderMuted:     compat.AdaptiveColor{Light: lipgloss.Color(charmtone.Smoke.Hex()), Dark: lipgloss.Color(charmtone.Charcoal.Hex())},
	BackgroundPanel: compat.AdaptiveColor{Light: lipgloss.Color(charmtone.Butter.Hex()), Dark: lipgloss.Color(charmtone.Pepper.Hex())},
	DialogBorder:    compat.AdaptiveColor{Light: lipgloss.Color(charmtone.Jelly.Hex()), Dark: lipgloss.Color(charmtone.Charple.Hex())},
	GradientStart:   compat.AdaptiveColor{Light: lipgloss.Color(charmtone.Salmon.Hex()), Dark: lipgloss.Color(charmtone.Coral.Hex())},
	GradientEnd:     compat.AdaptiveColor{Light: lipgloss.Color(charmtone.Pom.Hex()), Dark: lipgloss.Color(charmtone.Sriracha.Hex())},
	Subtle:          compat.AdaptiveColor{Light: lipgloss.Color(charmtone.Ash.Hex()), Dark: lipgloss.Color(charmtone.Charcoal.Hex())},
}
