package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/xemotrix/augmux/internal/core"
	"github.com/xemotrix/augmux/internal/styles"
)

type taskPickerItem struct {
	agent *core.AgentState
	cols  [5]string // name, status, commits, dirty, branch
}

type taskPickerModel struct {
	title     string
	items     []taskPickerItem
	cursor    int
	checked   map[int]bool
	quitting  bool
	confirmed bool
	colWidths [5]int
}

func newTaskPickerModel(
	title, repoRoot string,
	agents []*core.AgentState,
	filter func(*core.AgentState) bool,
) taskPickerModel {
	srcBranch := core.SourceBranch(repoRoot)
	var items []taskPickerItem
	var maxW [5]int

	for _, a := range agents {
		if filter != nil && !filter(a) {
			continue
		}
		ahead := core.GitMust(repoRoot, "rev-list", "--count", srcBranch+".."+a.Branch)
		if ahead == "" {
			ahead = "?"
		}
		dirty := countUncommitted(a.Worktree)
		cols := [5]string{
			fmt.Sprintf("Agent %d: %s", a.Index, a.Description),
			AgentStatusRaw(a),
			ahead + " commits",
			fmt.Sprintf("%d ✎", dirty),
			a.Branch,
		}
		for i, c := range cols {
			if w := lipgloss.Width(c); w > maxW[i] {
				maxW[i] = w
			}
		}
		items = append(items, taskPickerItem{agent: a, cols: cols})
	}
	return taskPickerModel{title: title, items: items, cursor: 0, checked: make(map[int]bool), colWidths: maxW}
}

func (m taskPickerModel) Init() tea.Cmd { return nil }

func (m taskPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m taskPickerModel) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString(styles.TitleStyle.Render(m.title))
	b.WriteString("\n\n")

	for i, item := range m.items {
		isCursor := i == m.cursor
		isChecked := m.checked[i]

		cursor := "  "
		if isCursor {
			cursor = styles.PickerCursorStyle.Render("▸ ")
		}
		check := "[ ] "
		if isChecked {
			check = styles.PickerCursorStyle.Render("[x] ")
		}

		nameStyle := styles.PickerNormalStyle
		if isCursor {
			nameStyle = styles.PickerSelectedStyle
		}
		name := nameStyle.Width(m.colWidths[0]).Render(item.cols[0])
		status := AgentStatusStyled(item.agent, lipgloss.NewStyle().Width(m.colWidths[1]).Render(item.cols[1]))
		commits := styles.AheadStyle.Width(m.colWidths[2]).Render(item.cols[2])
		dirty := styles.DirtyStyle.Width(m.colWidths[3]).Render(item.cols[3])
		branch := styles.RenderBranch(item.cols[4])

		sep := styles.SeparatorStyle.Render(" │ ")
		b.WriteString(cursor + check + name + sep + status + sep + commits + sep + dirty + sep + branch + "\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.PickerHintStyle.Render("j/k navigate · g/G top/bottom · space/x toggle · a all · enter confirm · esc cancel"))
	b.WriteString("\n")
	return b.String()
}

// RunPicker shows a multi-select picker and returns selected agent indices (nil if cancelled).
func RunPicker(title, repoRoot string, filter func(*core.AgentState) bool) []int {
	agents := core.ListAgents(repoRoot)
	if len(agents) == 0 {
		return nil
	}
	srcBranch := core.SourceBranch(repoRoot)
	var states []*core.AgentState
	for _, idx := range agents {
		a, err := core.ReadAgent(repoRoot, idx)
		if err == nil {
			if a.MergeCommit == "" && a.Resolving == "" && a.Branch != "" {
				a.HasConflicts = core.DetectConflicts(repoRoot, srcBranch, a.Branch)
			}
			states = append(states, a)
		}
	}
	m := newTaskPickerModel(title, repoRoot, states, filter)
	if len(m.items) == 0 {
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
	fm := final.(taskPickerModel)
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
	if a.HasConflicts {
		return "● conflicts"
	}
	return "● wip"
}

func AgentStatusStyled(a *core.AgentState, text string) string {
	if a.MergeCommit != "" {
		return styles.BadgeMerged.Render(text)
	}
	if a.Resolving != "" {
		return styles.BadgeResolving.Render(text)
	}
	if a.HasConflicts {
		return styles.BadgeConflicts.Render(text)
	}
	return styles.BadgeWip.Render(text)
}
