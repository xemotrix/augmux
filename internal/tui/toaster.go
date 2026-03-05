package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/xemotrix/augmux/internal/styles"
)

type ToastLevel int

const (
	ToastInfo ToastLevel = iota
	ToastSuccess
	ToastWarning
	ToastError
)

const defaultToastDuration = 4 * time.Second

type toast struct {
	id       int
	message  string
	level    ToastLevel
	created  time.Time
	duration time.Duration
}

type addToastMsg struct {
	Message  string
	Level    ToastLevel
	Duration time.Duration // zero means defaultToastDuration
}

type toastExpiredMsg struct {
	id int
}

type toaster struct {
	toasts []toast
	nextID int
}

func newToaster() toaster {
	return toaster{}
}

// AddToast returns a tea.Cmd that sends an addToastMsg.
func AddToast(message string, level ToastLevel) tea.Cmd {
	return func() tea.Msg {
		return addToastMsg{Message: message, Level: level}
	}
}

func (t toaster) Update(msg tea.Msg) (toaster, tea.Cmd) {
	switch msg := msg.(type) {
	case addToastMsg:
		dur := msg.Duration
		if dur == 0 {
			dur = defaultToastDuration
		}
		tt := toast{
			id:       t.nextID,
			message:  msg.Message,
			level:    msg.Level,
			created:  time.Now(),
			duration: dur,
		}
		t.nextID++
		t.toasts = append(t.toasts, tt)
		id := tt.id
		return t, tea.Tick(dur, func(time.Time) tea.Msg {
			return toastExpiredMsg{id: id}
		})

	case toastExpiredMsg:
		for i, tt := range t.toasts {
			if tt.id == msg.id {
				t.toasts = append(t.toasts[:i], t.toasts[i+1:]...)
				break
			}
		}
	}
	return t, nil
}

func (t toaster) View() string {
	if len(t.toasts) == 0 {
		return ""
	}
	var lines []string
	for _, tt := range t.toasts {
		lines = append(lines, renderToast(tt))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func renderToast(t toast) string {
	var icon string
	var style lipgloss.Style

	switch t.level {
	case ToastSuccess:
		icon = "✓"
		style = styles.ToastSuccess
	case ToastWarning:
		icon = "⚠"
		style = styles.ToastWarning
	case ToastError:
		icon = "✗"
		style = styles.ToastError
	default:
		icon = "●"
		style = styles.ToastInfo
	}

	return style.Render(icon + " " + t.message)
}
