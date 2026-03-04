package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/xemotrix/augmux/internal/core"
)

type pickerItem struct {
	agent    *core.AgentState
	cols     [4]string // name, status, commits, branch
	nameLen  int
}

type pickerModel struct {
	title    string
	items    []pickerItem
	cursor   int
	checked  map[int]bool
	quitting bool
	confirmed bool
	colWidths [4]int
}

func newPickerModel(title, repoRoot string, agents []*core.AgentState, filter func(*core.AgentState) bool) pickerModel {
	srcBranch := core.SourceBranch(repoRoot)
	var items []pickerItem
	var maxW [4]int

	for _, a := range agents {
		if filter != nil && !filter(a) {
			continue
		}
		ahead := core.GitMust(repoRoot, "rev-list", "--count", srcBranch+".."+a.Branch)
		if ahead == "" {
			ahead = "?"
		}
		cols := [4]string{
			fmt.Sprintf("Agent %d: %s", a.Index, a.Description),
			AgentStatusRaw(a),
			ahead + " commits",
			a.Branch,
		}
		for i, c := range cols {
			if len(c) > maxW[i] {
				maxW[i] = len(c)
			}
		}
		items = append(items, pickerItem{agent: a, cols: cols})
	}
	return pickerModel{title: title, items: items, cursor: 0, checked: make(map[int]bool), colWidths: maxW}
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "g":
			// gg — jump to top (single g goes to top)
			m.cursor = 0
		case "G":
			// G — jump to bottom
			m.cursor = len(m.items) - 1
		case "ctrl+u":
			// Half page up
			m.cursor -= len(m.items) / 2
			if m.cursor < 0 {
				m.cursor = 0
			}
		case "ctrl+d":
			// Half page down
			m.cursor += len(m.items) / 2
			if m.cursor >= len(m.items) {
				m.cursor = len(m.items) - 1
			}
		case " ", "x":
			// Toggle selection
			if m.checked[m.cursor] {
				delete(m.checked, m.cursor)
			} else {
				m.checked[m.cursor] = true
			}
		case "a":
			// Toggle all
			if len(m.checked) == len(m.items) {
				m.checked = make(map[int]bool)
			} else {
				for i := range m.items {
					m.checked[i] = true
				}
			}
		case "enter":
			// If nothing explicitly checked, select item under cursor
			if len(m.checked) == 0 {
				m.checked[m.cursor] = true
			}
			m.confirmed = true
			m.quitting = true
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m pickerModel) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render(m.title))
	b.WriteString("\n\n")

	for i, item := range m.items {
		isCursor := i == m.cursor
		isChecked := m.checked[i]

		// Cursor + checkbox
		cursor := "  "
		if isCursor {
			cursor = pickerCursorStyle.Render("▸ ")
		}
		check := "[ ] "
		if isChecked {
			check = pickerCursorStyle.Render("[x] ")
		}

		// Build padded columns
		name := pad(item.cols[0], m.colWidths[0])
		status := pad(item.cols[1], m.colWidths[1])
		commits := pad(item.cols[2], m.colWidths[2])
		branch := item.cols[3]

		// Apply styles
		if isCursor {
			name = pickerSelectedStyle.Render(name)
		} else {
			name = pickerNormalStyle.Render(name)
		}
		status = AgentStatusStyled(item.agent, status)
		commits = aheadStyle.Render(commits)
		branch = RenderBranch(branch)

		sep := separatorStyle.Render(" │ ")
		b.WriteString(cursor + check + name + sep + status + sep + commits + sep + branch + "\n")
	}

	b.WriteString("\n")
	b.WriteString(pickerHintStyle.Render("j/k navigate · g/G top/bottom · space/x toggle · a all · enter confirm · esc cancel"))
	b.WriteString("\n")
	return b.String()
}

func pad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// RunPicker shows a multi-select picker and returns selected agent indices (nil if cancelled).
func RunPicker(title, repoRoot string, filter func(*core.AgentState) bool) []int {
	agents := core.ListAgents(repoRoot)
	if len(agents) == 0 {
		fmt.Println("No agents found.")
		return nil
	}
	var states []*core.AgentState
	for _, idx := range agents {
		a, err := core.ReadAgent(repoRoot, idx)
		if err == nil {
			states = append(states, a)
		}
	}
	m := newPickerModel(title, repoRoot, states, filter)
	if len(m.items) == 0 {
		fmt.Println("No matching agents found.")
		return nil
	}
	if len(m.items) == 1 {
		return []int{m.items[0].agent.Index}
	}
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		core.Fatal("TUI error: %v", err)
	}
	fm := final.(pickerModel)
	if !fm.confirmed {
		return nil
	}
	var result []int
	for i, item := range fm.items {
		if fm.checked[i] {
			result = append(result, item.agent.Index)
		}
	}
	return result
}

func AgentStatusRaw(a *core.AgentState) string {
	if a.MergeCommit != "" {
		return "● merged"
	}
	if a.Resolving != "" {
		return "● resolving"
	}
	return "● active"
}

func AgentStatusStyled(a *core.AgentState, text string) string {
	if a.MergeCommit != "" {
		return badgeMerged.Render(text)
	}
	if a.Resolving != "" {
		return badgeResolving.Render(text)
	}
	return badgeActive.Render(text)
}

func AgentStatusLabel(a *core.AgentState) string {
	return AgentStatusStyled(a, AgentStatusRaw(a))
}
