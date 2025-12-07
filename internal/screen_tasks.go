package internal

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jhalter/mobius-hotline-client/internal/style"
)

// Messages sent from TasksScreen to parent

// TasksCancelledMsg signals user wants to close tasks screen
type TasksCancelledMsg struct{}

// tasksScreenKeyMap defines key bindings for the tasks screen help display
type tasksScreenKeyMap struct {
	Back key.Binding
}

func (k tasksScreenKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Back}
}

func (k tasksScreenKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Back}}
}

// TasksScreen is a self-contained BubbleTea model for viewing download/upload tasks
type TasksScreen struct {
	width, height int
	model         *Model
	help          help.Model
	keys          tasksScreenKeyMap
}

// NewTasksScreen creates a new tasks screen
func NewTasksScreen(m *Model) *TasksScreen {
	keys := tasksScreenKeyMap{
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "close"),
		),
	}

	return &TasksScreen{
		width:  m.width,
		height: m.height,
		model:  m,
		help:   help.New(),
		keys:   keys,
	}
}

// Init implements tea.Model
func (s *TasksScreen) Init() tea.Cmd {
	return nil
}

// Update implements ScreenModel
func (s *TasksScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.SetSize(msg.Width, msg.Height)
		return s, nil

	case TasksCancelledMsg:
		s.model.PopScreen()
		return s, nil

	case tea.KeyMsg:
		return s.handleKeys(msg)

	case progress.FrameMsg:
		// Handle progress animation updates
		var cmds []tea.Cmd
		for taskID, prog := range s.model.taskProgress {
			model, cmd := prog.Update(msg)
			s.model.taskProgress[taskID] = model.(progress.Model)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return s, tea.Batch(cmds...)
	}

	return s, nil
}

// View implements tea.Model
func (s *TasksScreen) View() string {
	activeTasks := s.model.taskManager.GetActive()
	completedTasks := s.model.taskManager.GetCompleted(10)

	var b strings.Builder

	// Active section
	if len(activeTasks) > 0 {
		sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(style.CurrentTheme.Highlight)
		b.WriteString(sectionStyle.Render("Active Downloads"))
		b.WriteString("\n\n")

		for _, task := range activeTasks {
			b.WriteString(s.renderTask(task))
			b.WriteString("\n\n")
		}
	} else {
		mutedStyle := lipgloss.NewStyle().Foreground(style.CurrentTheme.TextMuted)
		b.WriteString(mutedStyle.Render("No active downloads"))
		b.WriteString("\n\n")
	}

	// Completed section
	if len(completedTasks) > 0 {
		b.WriteString("\n")
		sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(style.CurrentTheme.Highlight)
		b.WriteString(sectionStyle.Render("Recent Completed"))
		b.WriteString("\n\n")

		for _, task := range completedTasks {
			b.WriteString(s.renderCompletedTask(task))
			b.WriteString("\n")
		}
	}

	// Help
	b.WriteString("\n")
	b.WriteString(s.help.View(s.keys))

	return style.RenderSubscreen(s.width, s.height, "Tasks", b.String())
}

// SetSize updates dimensions
func (s *TasksScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// handleKeys handles keyboard input
func (s *TasksScreen) handleKeys(msg tea.KeyMsg) (ScreenModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return s, func() tea.Msg { return TasksCancelledMsg{} }
	}
	return s, nil
}

// renderTask renders an active task with progress bar
func (s *TasksScreen) renderTask(task *Task) string {
	var b strings.Builder

	// File name
	highlightStyle := lipgloss.NewStyle().Bold(true).Foreground(style.CurrentTheme.Highlight)
	b.WriteString(highlightStyle.Render(task.FileName))
	b.WriteString("\n")

	// Progress bar (40 chars)
	prog := float64(task.TransferredBytes) / float64(task.TotalBytes)
	if task.TotalBytes == 0 {
		prog = 0
	}
	filled := int(prog * 40)
	if filled > 40 {
		filled = 40
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 40-filled)
	b.WriteString(bar)
	b.WriteString("\n")

	// Stats
	pct := int(prog * 100)
	if pct > 100 {
		pct = 100
	}
	transferred := formatBytes(task.TransferredBytes)
	total := formatBytes(task.TotalBytes)

	// Calculate speed and ETA
	now := time.Now()
	elapsed := now.Sub(task.LastUpdate).Seconds()
	if elapsed > 0 {
		deltaBytes := task.TransferredBytes - task.LastBytes
		task.Speed = float64(deltaBytes) / elapsed
		task.LastBytes = task.TransferredBytes
		task.LastUpdate = now
	}

	speed := formatBytes(int64(task.Speed)) + "/s"

	var eta string
	if task.Speed > 0 {
		remaining := task.TotalBytes - task.TransferredBytes
		etaSeconds := float64(remaining) / task.Speed
		eta = formatDuration(time.Duration(etaSeconds) * time.Second)
	} else {
		eta = "--:--"
	}

	mutedStyle := lipgloss.NewStyle().Foreground(style.CurrentTheme.TextMuted)
	stats := fmt.Sprintf("%d%% • %s / %s • %s • ETA: %s", pct, transferred, total, speed, eta)
	b.WriteString(mutedStyle.Render(stats))

	return b.String()
}

// renderCompletedTask renders a completed task
func (s *TasksScreen) renderCompletedTask(task *Task) string {
	var icon, status string

	successStyle := lipgloss.NewStyle().Foreground(style.CurrentTheme.Success)
	errorStyle := lipgloss.NewStyle().Foreground(style.CurrentTheme.Error)
	mutedStyle := lipgloss.NewStyle().Foreground(style.CurrentTheme.TextMuted)

	if task.Status == TaskCompleted {
		icon = successStyle.Render("✓")
		duration := task.EndTime.Sub(task.StartTime)
		status = fmt.Sprintf("%s • %s", formatBytes(task.TotalBytes), formatDuration(duration))
	} else {
		icon = errorStyle.Render("✗")
		if task.Error != nil {
			status = task.Error.Error()
		} else {
			status = "Failed"
		}
	}

	return fmt.Sprintf("%s %s  %s", icon, task.FileName, mutedStyle.Render(status))
}
