package components

import tea "github.com/charmbracelet/bubbletea"

// MenuNavResult indicates what happened after a key press.
type MenuNavResult int

const (
	NavNone   MenuNavResult = iota // no actionable event
	NavSelect                      // user pressed enter
	NavCancel                      // user pressed esc/q/ctrl+c
)

// MenuNav encapsulates cursor + options state and handles j/k/enter/esc
// navigation. Shared between inline TUI menus and the standalone select menu.
type MenuNav struct {
	Options []string
	Cursor  int
}

// HandleKey processes a key message and returns the navigation result.
func (m *MenuNav) HandleKey(msg tea.KeyMsg) MenuNavResult {
	switch msg.String() {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
		}
	case "down", "j":
		if m.Cursor < len(m.Options)-1 {
			m.Cursor++
		}
	case "g":
		m.Cursor = 0
	case "G":
		m.Cursor = len(m.Options) - 1
	case "enter":
		return NavSelect
	case "esc", "ctrl+c", "q":
		return NavCancel
	}
	return NavNone
}
