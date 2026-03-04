package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type inputModel struct {
	prompt    string
	textInput textinput.Model
	confirmed bool
	quitting  bool
}

func newInputModel(prompt string) inputModel {
	ti := textinput.New()
	ti.Placeholder = "e.g. fix auth bug"
	ti.Focus()
	ti.CharLimit = 80
	ti.Width = 50
	ti.PromptStyle = pickerCursorStyle
	ti.TextStyle = pickerSelectedStyle
	return inputModel{prompt: prompt, textInput: ti}
}

func (m inputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.confirmed = true
			m.quitting = true
			return m, tea.Quit
		case "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m inputModel) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render(m.prompt))
	b.WriteString("\n\n")
	b.WriteString("  " + m.textInput.View())
	b.WriteString("\n\n")
	b.WriteString(pickerHintStyle.Render("  enter confirm · esc/ctrl+c cancel"))
	b.WriteString("\n")
	return b.String()
}

// RunTextInput shows a text input prompt and returns the entered value, or "" if cancelled.
func RunTextInput(prompt string) string {
	m := newInputModel(prompt)
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		fmt.Printf("TUI error: %v\n", err)
		return ""
	}
	fm := final.(inputModel)
	if !fm.confirmed {
		return ""
	}
	return strings.TrimSpace(fm.textInput.Value())
}
