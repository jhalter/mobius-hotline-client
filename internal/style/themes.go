package style

import "github.com/charmbracelet/lipgloss"

// Pre-defined themes
var (
	ThemeClassic = DefaultTheme()

	ThemeOcean = Theme{
		Name:            "Ocean",
		TextMuted:       lipgloss.Color("244"),
		TextDisabled:    lipgloss.Color("240"),
		Highlight:       lipgloss.Color("39"),  // Deep sky blue
		Accent:          lipgloss.Color("51"),  // Cyan
		Success:         lipgloss.Color("48"),  // Sea green
		Error:           lipgloss.Color("196"), // Red
		Admin:           lipgloss.Color("208"), // Orange
		BorderPrimary:   lipgloss.Color("33"),  // Blue
		BorderMuted:     lipgloss.Color("238"),
		BackgroundPanel: lipgloss.Color("235"),
		DialogBorder:    lipgloss.Color("39"),
		GradientStart:   lipgloss.Color("33"),
		GradientEnd:     lipgloss.Color("#1E90FF"),
		Subtle:          lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"},
	}

	ThemeForest = Theme{
		Name:            "Forest",
		TextMuted:       lipgloss.Color("243"),
		TextDisabled:    lipgloss.Color("240"),
		Highlight:       lipgloss.Color("34"),  // Green
		Accent:          lipgloss.Color("178"), // Gold
		Success:         lipgloss.Color("40"),  // Bright green
		Error:           lipgloss.Color("160"), // Dark red
		Admin:           lipgloss.Color("166"), // Orange
		BorderPrimary:   lipgloss.Color("29"),  // Dark green
		BorderMuted:     lipgloss.Color("238"),
		BackgroundPanel: lipgloss.Color("235"),
		DialogBorder:    lipgloss.Color("34"),
		GradientStart:   lipgloss.Color("34"),
		GradientEnd:     lipgloss.Color("#228B22"),
		Subtle:          lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"},
	}

	ThemeMonochrome = Theme{
		Name:            "Monochrome",
		TextMuted:       lipgloss.Color("245"),
		TextDisabled:    lipgloss.Color("240"),
		Highlight:       lipgloss.Color("255"),
		Accent:          lipgloss.Color("250"),
		Success:         lipgloss.Color("255"),
		Error:           lipgloss.Color("255"),
		Admin:           lipgloss.Color("255"),
		BorderPrimary:   lipgloss.Color("248"),
		BorderMuted:     lipgloss.Color("240"),
		BackgroundPanel: lipgloss.Color("236"),
		DialogBorder:    lipgloss.Color("248"),
		GradientStart:   lipgloss.Color("255"),
		GradientEnd:     lipgloss.Color("245"),
		Subtle:          lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"},
	}
)

// AvailableThemes returns all pre-defined themes.
func AvailableThemes() []Theme {
	return []Theme{ThemeClassic, ThemeOcean, ThemeForest, ThemeMonochrome}
}

// ThemeNames returns the names of all available themes.
func ThemeNames() []string {
	themes := AvailableThemes()
	names := make([]string, len(themes))
	for i, t := range themes {
		names[i] = t.Name
	}
	return names
}

// GetThemeByName returns a theme by name, or DefaultTheme if not found.
func GetThemeByName(name string) Theme {
	for _, t := range AvailableThemes() {
		if t.Name == name {
			return t
		}
	}
	return DefaultTheme()
}
