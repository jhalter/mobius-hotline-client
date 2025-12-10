package internal

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/jhalter/mobius-hotline-client/internal/style"
)

func (m *Model) renderTaskWidget() string {
	// Get active tasks first, then completed to fill remaining slots
	activeTasks := m.taskManager.GetActive()
	completedTasks := m.taskManager.GetCompleted(3)

	// Combine: active tasks first, then completed
	var displayTasks []*Task
	displayTasks = append(displayTasks, activeTasks...)

	// Fill remaining slots with completed tasks
	remaining := 3 - len(activeTasks)
	if remaining > 0 && len(completedTasks) > 0 {
		for i := 0; i < remaining && i < len(completedTasks); i++ {
			displayTasks = append(displayTasks, completedTasks[i])
		}
	}

	// Limit to 3 tasks maximum
	if len(displayTasks) > 3 {
		displayTasks = displayTasks[:3]
	}

	// Build widget content
	var content strings.Builder

	// Title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(style.CurrentTheme.Highlight).
		Render("Recent Transfers")
	content.WriteString(title + "\n")

	if len(displayTasks) == 0 {
		// Empty state
		emptyMsg := lipgloss.NewStyle().
			Foreground(style.CurrentTheme.TextMuted).
			Render("No transfers")
		content.WriteString(emptyMsg)
	} else {
		// Render each task
		for i, task := range displayTasks {
			if i > 0 {
				content.WriteString("\n") // Spacing between tasks
			}
			content.WriteString(m.renderCompactTask(task))
		}
	}

	return style.TaskWidgetStyle.Render(content.String())
}

func (m *Model) renderCompactTask(task *Task) string {
	var lines []string

	// Line 1: Filename (truncated) + status/percentage
	fileName := task.FileName
	if len(fileName) > 18 {
		fileName = fileName[:15] + "..."
	}

	var statusStr string
	switch task.Status {
	case TaskActive:
		percent := 0
		if task.TotalBytes > 0 {
			percent = int((float64(task.TransferredBytes) / float64(task.TotalBytes)) * 100)
		}
		statusStr = style.TaskActiveStyle.Render(fmt.Sprintf("%3d%%", percent))
	case TaskCompleted:
		statusStr = style.TaskCompleteStyle.Render("Done")
	case TaskFailed:
		statusStr = style.TaskFailedStyle.Render("Fail")
	case TaskPending:
		statusStr = lipgloss.NewStyle().Foreground(style.CurrentTheme.TextMuted).Render("Wait")
	}

	line1 := fmt.Sprintf("%-18s %4s", fileName, statusStr)
	lines = append(lines, line1)

	// Line 2: Progress bar (only for active tasks)
	if task.Status == TaskActive {
		if prog, ok := m.taskProgress[task.ID]; ok {
			lines = append(lines, prog.View())
		}
	}

	// Line 3: Speed (only for active tasks)
	if task.Status == TaskActive && task.Speed > 0 {
		speedStr := formatSpeed(task.Speed)
		lines = append(lines, lipgloss.NewStyle().
			Foreground(style.CurrentTheme.TextMuted).
			Render(speedStr))
	}

	return strings.Join(lines, "\n")
}

// encodeNewsPath encodes a news path into bytes for the FieldNewsPath field
func encodeNewsPath(path []string) []byte {
	if len(path) == 0 {
		return []byte{}
	}

	// Format: [2 bytes count][1 byte len][name]...
	var buf []byte
	countBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(countBytes, uint16(len(path)))
	buf = append(buf, countBytes...)

	for _, name := range path {
		buf = append(buf, 0, 0)
		// Add length byte
		buf = append(buf, byte(len(name)))
		// Add name
		buf = append(buf, []byte(name)...)
	}

	return buf
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
