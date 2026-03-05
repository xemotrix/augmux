package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/xemotrix/augmux/internal/core"
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

	titleLine := styles.TitleStyle.Render(m.title)

	var items []string
	for i, opt := range m.options {
		cur := lipgloss.NewStyle().Width(2).Render("")
		style := styles.PickerNormalStyle
		if i == m.cursor {
			cur = styles.PickerCursorStyle.Render("▸ ")
			style = styles.PickerSelectedStyle
		}
		items = append(items, lipgloss.JoinHorizontal(lipgloss.Top, cur, style.Render(opt)))
	}
	optionsList := lipgloss.JoinVertical(lipgloss.Left, items...)

	hint := styles.PickerHintStyle.Render("j/k navigate · enter select · esc cancel")

	return lipgloss.JoinVertical(lipgloss.Left, titleLine, "", optionsList, "", hint, "")
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
		core.Fatal("TUI error: %v", err)
	}
	return final.(selectMenuModel).chosen
}
