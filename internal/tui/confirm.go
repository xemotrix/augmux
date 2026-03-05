package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/xemotrix/augmux/internal/core"
	"github.com/xemotrix/augmux/internal/styles"
)

var (
	confirmActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(styles.ColorWhite).
				Background(styles.ColorAccent).
				Padding(0, 2)

	confirmInactiveStyle = lipgloss.NewStyle().
				Foreground(styles.ColorGray).
				Padding(0, 2)

	confirmWarningStyle = lipgloss.NewStyle().
				Foreground(styles.ColorRed).
				Bold(true)
)

type confirmModel struct {
	message     string
	yesSelected bool
	confirmed   bool
	quitting    bool
}

func (m confirmModel) Init() tea.Cmd { return nil }

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h", "right", "l", "tab":
			m.yesSelected = !m.yesSelected
		case "y", "Y":
			m.yesSelected = true
			m.confirmed = true
			m.quitting = true
			return m, tea.Quit
		case "n", "N":
			m.yesSelected = false
			m.quitting = true
			return m, tea.Quit
		case "enter":
			m.confirmed = m.yesSelected
			m.quitting = true
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m confirmModel) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder

	b.WriteString(confirmWarningStyle.Render("⚠ " + m.message))
	b.WriteString("\n\n")

	yes := confirmInactiveStyle.Render("Yes")
	no := confirmInactiveStyle.Render("No")
	if m.yesSelected {
		yes = confirmActiveStyle.Render("Yes")
	} else {
		no = confirmActiveStyle.Render("No")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, yes, "    ", no)
	b.WriteString(lipgloss.PlaceHorizontal(60, lipgloss.Center, buttons))
	b.WriteString("\n\n")
	b.WriteString(styles.PickerHintStyle.Render("h/l switch · y/n · enter confirm · q/esc cancel"))
	b.WriteString("\n")

	return b.String()
}

// RunConfirm shows a yes/no confirmation dialog. Returns true if confirmed.
func RunConfirm(message string) bool {
	m := confirmModel{message: message, yesSelected: false}
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		core.Fatal("TUI error: %v", err)
	}
	return final.(confirmModel).confirmed
}
