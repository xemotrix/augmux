package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	cardWidth = 42
	cardInner = cardWidth - 2 // minus border (2); lipgloss Width includes padding
	textWidth = cardInner - 2 // minus padding (2); actual chars per line
)

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

func agentBorderColor(a *AgentState) lipgloss.TerminalColor {
	if a.MergeCommit != "" {
		return colorCyan
	}
	if a.Resolving != "" {
		return colorYellow
	}
	return colorGreen
}

func renderAgentCard(a *AgentState, repoRoot, srcBranch string) string {
	statusRaw := agentStatusRaw(a)
	statusStyled := agentStatusStyled(a, statusRaw)

	idStr := fmt.Sprintf("Agent %d", a.Index)
	gap := textWidth - len(idStr) - len(statusRaw)
	if gap < 1 {
		gap = 1
	}
	header := lipgloss.NewStyle().Bold(true).Foreground(colorWhite).Render(idStr) +
		strings.Repeat(" ", gap) + statusStyled

	name := truncate(a.Description, textWidth)
	nameLine := valueStyle.Render(name)

	ahead := gitMust(repoRoot, "rev-list", "--count", srcBranch+".."+a.Branch)
	if ahead == "" {
		ahead = "?"
	}
	aheadStr := ahead + " ↑"
	iconWidth := 2 // branchIcon + space
	branch := truncate(a.Branch, textWidth-len(aheadStr)-1-iconWidth)
	branchGap := textWidth - len(branch) - iconWidth - len(aheadStr)
	if branchGap < 1 {
		branchGap = 1
	}
	branchLine := renderBranch(branch) + strings.Repeat(" ", branchGap) + aheadStyle.Render(aheadStr)

	content := header + "\n" + nameLine + "\n" + branchLine
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(agentBorderColor(a)).
		Padding(0, 1).
		Width(cardInner)
	return style.Render(content)
}

func renderStatusView(repoRoot string, termWidth int) string {
	srcBranch := sourceBranch(repoRoot)
	var b strings.Builder

	b.WriteString(titleStyle.Render("⚡ augmux session"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s %s   %s %s\n\n",
		headerKeyStyle.Render("Repo:"), headerValStyle.Render(repoRoot),
		headerKeyStyle.Render("Source:"), renderBranch(srcBranch)))

	agents := listAgents(repoRoot)
	if len(agents) == 0 {
		b.WriteString(labelStyle.Render("  No agents running.\n"))
		return b.String()
	}

	var cards []string
	for _, idx := range agents {
		a, err := readAgent(repoRoot, idx)
		if err != nil {
			continue
		}
		cards = append(cards, renderAgentCard(a, repoRoot, srcBranch))
	}

	cols := termWidth / (cardWidth + 1)
	if cols < 1 {
		cols = 1
	}
	if cols > len(cards) {
		cols = len(cards)
	}

	var rows []string
	for i := 0; i < len(cards); i += cols {
		end := i + cols
		if end > len(cards) {
			end = len(cards)
		}
		row := lipgloss.JoinHorizontal(lipgloss.Top, cards[i:end]...)
		rows = append(rows, row)
	}
	b.WriteString(lipgloss.JoinVertical(lipgloss.Left, rows...))
	b.WriteString("\n\n")
	b.WriteString(pickerHintStyle.Render("  spawn · merge · accept · reject · cancel · finish · clean"))
	return b.String()
}

// Watch mode

type statusWatchModel struct {
	repoRoot string
	width    int
	content  string
	quitting bool
}

type tickMsg time.Time

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m statusWatchModel) Init() tea.Cmd {
	return tickEvery(2 * time.Second)
}

func (m statusWatchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.content = renderStatusView(m.repoRoot, m.width)
	case tickMsg:
		m.content = renderStatusView(m.repoRoot, m.width)
		return m, tickEvery(2 * time.Second)
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "esc" || msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m statusWatchModel) View() string {
	if m.quitting {
		return ""
	}
	return m.content + "\n\n" + pickerHintStyle.Render("  q quit · auto-refreshes every 2s") + "\n"
}

func runStatusWatch(repoRoot string) {
	m := statusWatchModel{repoRoot: repoRoot, width: 100}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fatal("TUI error: %v", err)
	}
}
