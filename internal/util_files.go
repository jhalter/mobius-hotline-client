package internal

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/jhalter/mobius-hotline-client/internal/style"
)

// Files screen
type fileItem struct {
	name     string
	isFolder bool
	size     uint32 // size in KB
	fileType [4]byte
	creator  [4]byte
}

// fileTypeEmoji returns the appropriate emoji for a file type code
func fileTypeEmoji(typeCode [4]byte) string {
	tc := string(typeCode[:])

	switch tc {
	case "JPEG", "GIFf", "PNGf", "TIFF", "BMPf":
		return "🖼️" // Image files
	case "PDF ":
		return "📄" // PDF documents
	case "MooV", "MPEG":
		return "🎬" // Video files
	case "SIT!", "ZIP ", "Gzip":
		return "📦" // Archive files
	case "TEXT":
		return "📝" // Text files
	case "HTft":
		return "⏳" // Incomplete file uploads
	case "rohd":
		return "💾" // Disk images
	default:
		return "📄" // Default document emoji
	}
}

func (i fileItem) FilterValue() string { return i.name }
func (i fileItem) Title() string {
	if i.isFolder {
		return "📁 " + i.name
	}
	emoji := fileTypeEmoji(i.fileType)
	return emoji + " " + i.name
}
func (i fileItem) Description() string {
	if i.isFolder {
		return "Folder"
	}
	return fmt.Sprintf("%d KB", i.size)
}

// newFileDelegate creates a custom delegate for file list items
func newFileDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles = style.ListItemStyles

	// Add custom help text for file-specific keys
	d.ShortHelpFunc = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "select"),
			),
			key.NewBinding(
				key.WithKeys("tab"),
				key.WithHelp("tab", "file info"),
			),
			key.NewBinding(
				key.WithKeys("ctrl+u"),
				key.WithHelp("^u", "upload file"),
			),
		}
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{
			{
				key.NewBinding(
					key.WithKeys("enter"),
					key.WithHelp("enter", "select"),
				),
				key.NewBinding(
					key.WithKeys("tab"),
					key.WithHelp("tab", "file info"),
				),
				key.NewBinding(
					key.WithKeys("ctrl+u"),
					key.WithHelp("^u", "upload file"),
				),
			},
		}
	}

	return d
}
