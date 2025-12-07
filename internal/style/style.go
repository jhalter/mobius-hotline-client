package style

import (
	"image/color"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/gamut"
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
	FormTheme          *huh.Theme
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
		Border(lipgloss.RoundedBorder()).
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

	// Form theme for huh forms
	FormTheme = huh.ThemeCharm()
	FormTheme.Focused.Title = FormTheme.Focused.Title.Foreground(t.Highlight)
	FormTheme.Focused.SelectSelector = FormTheme.Focused.SelectSelector.Foreground(t.Highlight)
	FormTheme.Focused.SelectedOption = FormTheme.Focused.SelectedOption.Foreground(t.Highlight)
	FormTheme.Focused.FocusedButton = FormTheme.Focused.FocusedButton.
		Foreground(lipgloss.Color("0")).
		Background(t.Highlight)
	FormTheme.Focused.BlurredButton = FormTheme.Focused.BlurredButton.
		Foreground(t.TextMuted).
		Background(t.BackgroundPanel)
	FormTheme.Focused.TextInput.Cursor = FormTheme.Focused.TextInput.Cursor.Foreground(t.Highlight)
	FormTheme.Focused.TextInput.Prompt = FormTheme.Focused.TextInput.Prompt.Foreground(t.Accent)
	FormTheme.Blurred.Title = FormTheme.Blurred.Title.Foreground(t.TextMuted)
	FormTheme.Blurred.TextInput.Prompt = FormTheme.Blurred.TextInput.Prompt.Foreground(t.TextMuted)
	FormTheme.Blurred.SelectedOption = FormTheme.Blurred.SelectedOption.Foreground(t.Highlight)

	// TEST
	FormTheme.Blurred.FocusedButton = FormTheme.Blurred.FocusedButton.
		Foreground(lipgloss.Color("0")).
		Background(t.Highlight)

	FormTheme.Blurred.BlurredButton = FormTheme.Blurred.BlurredButton.
		Foreground(t.TextMuted).
		Background(t.BackgroundPanel)

	// List item styles for bubbles/list component
	ListItemStyles = list.NewDefaultItemStyles()
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
		lipgloss.WithWhitespaceChars(Background1),
		lipgloss.WithWhitespaceForeground(Subtle),
	)

}
