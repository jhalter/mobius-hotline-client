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
	TextMuted lipgloss.AdaptiveColor // Muted/de-emphasized text (join/leave messages, stats, empty states)

	// Highlight/accent colors
	Highlight lipgloss.AdaptiveColor // Primary highlight (titles, active items, categories)
	Accent    lipgloss.AdaptiveColor // Secondary accent (hotkeys)

	// Status colors
	Success lipgloss.AdaptiveColor // Completed/success states
	Error   lipgloss.AdaptiveColor // Error/failed states

	// User styling
	Admin lipgloss.AdaptiveColor // Admin user names

	// Border colors
	BorderPrimary lipgloss.AdaptiveColor // Main borders (chat, panels)
	BorderMuted   lipgloss.AdaptiveColor // Inactive/scrollback borders

	// Background colors
	BackgroundPanel lipgloss.AdaptiveColor // Panel/subscreen backgrounds

	// Dialog/Modal
	DialogBorder lipgloss.AdaptiveColor // Modal border color

	// Gradient colors (for banner)
	GradientStart lipgloss.AdaptiveColor
	GradientEnd   lipgloss.AdaptiveColor

	// Whitespace/subtle background pattern
	Subtle lipgloss.AdaptiveColor
}

// CurrentTheme is the active theme used throughout the application.
// Uses the CharmTone color palette as the default and only theme.
var CurrentTheme = Theme{
	Name:            "CharmTone",
	TextMuted:       lipgloss.AdaptiveColor{Light: charmtone.Oyster.Hex(), Dark: charmtone.Squid.Hex()},
	Highlight:       lipgloss.AdaptiveColor{Light: charmtone.Jelly.Hex(), Dark: charmtone.Charple.Hex()},
	Accent:          lipgloss.AdaptiveColor{Light: charmtone.Macaron.Hex(), Dark: charmtone.Dolly.Hex()},
	Success:         lipgloss.AdaptiveColor{Light: charmtone.Pickle.Hex(), Dark: charmtone.Guac.Hex()},
	Error:           lipgloss.AdaptiveColor{Light: charmtone.Pom.Hex(), Dark: charmtone.Sriracha.Hex()},
	Admin:           lipgloss.AdaptiveColor{Light: charmtone.Pom.Hex(), Dark: charmtone.Sriracha.Hex()},
	BorderPrimary:   lipgloss.AdaptiveColor{Light: charmtone.Jelly.Hex(), Dark: charmtone.Charple.Hex()},
	BorderMuted:     lipgloss.AdaptiveColor{Light: charmtone.Smoke.Hex(), Dark: charmtone.Charcoal.Hex()},
	BackgroundPanel: lipgloss.AdaptiveColor{Light: charmtone.Butter.Hex(), Dark: charmtone.Pepper.Hex()},
	DialogBorder:    lipgloss.AdaptiveColor{Light: charmtone.Jelly.Hex(), Dark: charmtone.Charple.Hex()},
	GradientStart:   lipgloss.AdaptiveColor{Light: charmtone.Salmon.Hex(), Dark: charmtone.Coral.Hex()},
	GradientEnd:     lipgloss.AdaptiveColor{Light: charmtone.Pom.Hex(), Dark: charmtone.Sriracha.Hex()},
	Subtle:          lipgloss.AdaptiveColor{Light: charmtone.Ash.Hex(), Dark: charmtone.Charcoal.Hex()},
}
