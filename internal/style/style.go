package style

import (
	"image/color"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/gamut"
)

// Legacy color constants - these are updated when theme changes for backward compatibility.
var (
	ColorCyan      lipgloss.Color
	ColorBrightRed lipgloss.Color
	ColorFuscia    lipgloss.Color
	ColorGrey3     lipgloss.Color
)

const Background1 = "☖"

// Style variables - regenerated when theme changes.
var (
	AppStyle           lipgloss.Style
	HotkeyStyle        lipgloss.Style
	ServerTitleStyle   lipgloss.Style
	SubTitleStyle      lipgloss.Style
	BoxStyle           lipgloss.Style
	AdminUserStyle     lipgloss.Style
	AwayUserStyle      lipgloss.Style
	AwayAdminUserStyle lipgloss.Style
	UsernameStyle      lipgloss.Style
	SubScreenStyle     lipgloss.Style
	TaskWidgetStyle    lipgloss.Style
	TaskActiveStyle    lipgloss.Style
	TaskCompleteStyle  lipgloss.Style
	TaskFailedStyle    lipgloss.Style
	CategoryStyle      lipgloss.Style
	TitleStyle         lipgloss.Style
	DialogBoxStyle     lipgloss.Style
	Subtle             lipgloss.AdaptiveColor
	Blends             []color.Color
)

func init() {
	regenerateStyles()
}

// regenerateStyles rebuilds all styles from the current theme.
func regenerateStyles() {
	t := CurrentTheme

	// Update legacy color aliases for backward compatibility
	ColorCyan = t.BorderPrimary
	ColorBrightRed = t.Admin
	ColorFuscia = t.Highlight
	ColorGrey3 = t.BorderMuted

	// Base app style
	AppStyle = lipgloss.NewStyle().Padding(1, 2)

	// Banner styles
	HotkeyStyle = lipgloss.NewStyle().
		Foreground(t.Accent).
		Bold(true)

	// Styling for the main server screen title.
	ServerTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Highlight)

	SubTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Padding(0, 0, 1).
		Foreground(t.Highlight)

	BoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderPrimary).
		Height(2).
		Padding(0, 1)

	AdminUserStyle = lipgloss.NewStyle().
		Foreground(t.Admin).
		Bold(true)

	AwayUserStyle = lipgloss.NewStyle().
		Faint(true)

	AwayAdminUserStyle = lipgloss.NewStyle().
		Foreground(t.Admin).
		Bold(true).
		Faint(true)

	UsernameStyle = lipgloss.NewStyle().Bold(true)

	SubScreenStyle = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(t.BorderPrimary).
		Background(t.BackgroundPanel).
		Padding(1, 1)

	TaskWidgetStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderPrimary).
		Padding(0, 1).
		Width(25)

	TaskActiveStyle = lipgloss.NewStyle().
		Foreground(t.Highlight).
		Bold(true)

	TaskCompleteStyle = lipgloss.NewStyle().
		Foreground(t.Success)

	TaskFailedStyle = lipgloss.NewStyle().
		Foreground(t.Error)

	CategoryStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Highlight)

	TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Highlight)

	DialogBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.DialogBorder).
		Padding(1, 0).
		BorderTop(true).
		BorderLeft(true).
		BorderRight(true).
		BorderBottom(true)

	Subtle = t.Subtle

	// Rainbow gradient colors
	Blends = gamut.Blends(lipgloss.Color("#F25D94"), lipgloss.Color("#EDFF82"), 50)
}

func Rainbow(base lipgloss.Style, s string, colors []color.Color) string {
	var str string
	for i, ss := range s {
		c, _ := colorful.MakeColor(colors[i%len(colors)])
		str += base.Foreground(lipgloss.Color(c.Hex())).Render(string(ss))
	}
	return str
}

func RenderSubscreen(w, h int, title, content string) string {
	return lipgloss.Place(
		w,
		h,
		lipgloss.Center,
		lipgloss.Center,
		SubScreenStyle.Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				TitleStyle.Render(title),
				content,
			),
		),
		lipgloss.WithWhitespaceChars(Background1),
		lipgloss.WithWhitespaceForeground(Subtle),
	)

}
