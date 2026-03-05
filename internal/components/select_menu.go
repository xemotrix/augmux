package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/xemotrix/augmux/internal/styles"
)

type selectMenuModel struct {
	title   string
	options []string
	cursor  int
	chosen  int // -1 = cancelled
	quit    bool
}

func newSelectMenuModel(title string, options []string) selectMenuModel {
	return selectMenuModel{title: title, options: options, cursor: 0, chosen: -1}
}

func (m selectMenuModel) Init() tea.Cmd { return nil }

func (m selectMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "g":
			m.cursor = 0
		case "G":
			m.cursor = len(m.options) - 1
		case "enter":
			m.chosen = m.cursor
			m.quit = true
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			m.quit = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m selectMenuModel) View() string {
	if m.quit {
		return ""
	}
	var b strings.Builder
	b.WriteString(styles.TitleStyle.Render(m.title))
	b.WriteString("\n\n")

	for i, opt := range m.options {
		cursor := "  "
		style := styles.PickerNormalStyle
		if i == m.cursor {
			cursor = styles.PickerCursorStyle.Render("▸ ")
			style = styles.PickerSelectedStyle
		}
		b.WriteString(cursor + style.Render(opt) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.PickerHintStyle.Render("j/k navigate · enter select · esc cancel"))
	b.WriteString("\n")
	return b.String()
}

// RunSelectMenu shows a single-select menu and returns the chosen index, or -1 if cancelled.
func RunSelectMenu(title string, options []string) int {
	if len(options) == 1 {
		return 0
	}
	m := newSelectMenuModel(title, options)
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		fmt.Printf("TUI error: %v\n", err)
		return -1
	}
	return final.(selectMenuModel).chosen
}
