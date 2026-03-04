package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type menuModel struct {
	title   string
	options []string
	cursor  int
	chosen  int // -1 = cancelled
	quit    bool
}

func newMenuModel(title string, options []string) menuModel {
	return menuModel{title: title, options: options, cursor: 0, chosen: -1}
}

func (m menuModel) Init() tea.Cmd { return nil }

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m menuModel) View() string {
	if m.quit {
		return ""
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render(m.title))
	b.WriteString("\n\n")

	for i, opt := range m.options {
		cursor := "  "
		style := pickerNormalStyle
		if i == m.cursor {
			cursor = pickerCursorStyle.Render("▸ ")
			style = pickerSelectedStyle
		}
		b.WriteString(cursor + style.Render(opt) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(pickerHintStyle.Render("j/k navigate · enter select · esc cancel"))
	b.WriteString("\n")
	return b.String()
}

// runMenu shows a single-select menu and returns the chosen index, or -1 if cancelled.
func runMenu(title string, options []string) int {
	m := newMenuModel(title, options)
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		fmt.Printf("TUI error: %v\n", err)
		return -1
	}
	return final.(menuModel).chosen
}
