package style

import (
	"image/color"

	"charm.land/bubbles/v2/list"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/gamut"
)

const Background1 = "☖"

var ColorHotlineRed = lipgloss.Color("196")

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
	Subtle             color.Color
	Blends             []color.Color
	FormTheme          huh.Theme
	ListItemStyles     list.DefaultItemStyles
)

func init() {
	regenerateStyles()
}

// regenerateStyles rebuilds all styles from the current theme.
func regenerateStyles() {
	t := CurrentTheme

	// Base app style
	AppStyle = lipgloss.NewStyle().Padding(1, 2)

	// Banner styles
	HotkeyStyle = lipgloss.NewStyle().
		Foreground(t.Highlight).
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
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderPrimary).
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

	// Form theme for huh forms - uses a ThemeFunc to implement the Theme interface
	FormTheme = huh.ThemeFunc(func(isDark bool) *huh.Styles {
		styles := huh.ThemeCharm(isDark)
		styles.Focused.Title = styles.Focused.Title.Foreground(t.Highlight)
		styles.Focused.SelectSelector = styles.Focused.SelectSelector.Foreground(t.Highlight)
		styles.Focused.SelectedOption = styles.Focused.SelectedOption.Foreground(t.Highlight)
		styles.Focused.FocusedButton = styles.Focused.FocusedButton.
			Foreground(lipgloss.Color("0")).
			Background(t.Highlight)
		styles.Focused.BlurredButton = styles.Focused.BlurredButton.
			Foreground(t.TextMuted).
			Background(t.BackgroundPanel)
		styles.Focused.TextInput.Cursor = styles.Focused.TextInput.Cursor.Foreground(t.Highlight)
		styles.Focused.TextInput.Prompt = styles.Focused.TextInput.Prompt.Foreground(t.Accent)
		styles.Blurred.Title = styles.Blurred.Title.Foreground(t.TextMuted)
		styles.Blurred.TextInput.Prompt = styles.Blurred.TextInput.Prompt.Foreground(t.TextMuted)
		styles.Blurred.SelectedOption = styles.Blurred.SelectedOption.Foreground(t.Highlight)
		styles.Blurred.FocusedButton = styles.Blurred.FocusedButton.
			Foreground(lipgloss.Color("0")).
			Background(t.Highlight)
		styles.Blurred.BlurredButton = styles.Blurred.BlurredButton.
			Foreground(t.TextMuted).
			Background(t.BackgroundPanel)
		return styles
	})

	// List item styles for bubbles/list component
	ListItemStyles = list.NewDefaultItemStyles(compat.HasDarkBackground)
	ListItemStyles.SelectedTitle = ListItemStyles.SelectedTitle.
		Foreground(t.Highlight).
		BorderForeground(t.Highlight)
	ListItemStyles.SelectedDesc = ListItemStyles.SelectedDesc.
		Foreground(t.Highlight).
		BorderForeground(t.Highlight)
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
		lipgloss.WithWhitespaceChars(CurrentTheme.BackgroundChar),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(Subtle)),
	)

}
