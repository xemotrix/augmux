package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/xemotrix/augmux/internal/core"
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

func agentBorderColor(a *core.AgentState) lipgloss.TerminalColor {
	if a.MergeCommit != "" {
		return colorCyan
	}
	if a.Resolving != "" {
		return colorYellow
	}
	return colorGreen
}

func renderAgentCard(a *core.AgentState, repoRoot, srcBranch string) string {
	borderColor := agentBorderColor(a)
	bdr := lipgloss.NewStyle().Foreground(borderColor)

	statusRaw := AgentStatusRaw(a)
	statusStyled := AgentStatusStyled(a, statusRaw)

	// Top border: ╭─❮3❯──────────────────────── ● active ─╮
	idLabel := fmt.Sprintf("❮%d❯", a.Index)
	idStyled := lipgloss.NewStyle().Bold(true).Foreground(colorWhite).Render(idLabel)

	// cardWidth - "╭─"(2) - idLabel - fill - " status "(1+statusRaw+1) - "─╮"(2)
	used := 2 + lipgloss.Width(idLabel) + 1 + lipgloss.Width(statusRaw) + 1 + 2
	fill := cardWidth - used
	if fill < 1 {
		fill = 1
	}
	topLine := bdr.Render("╭─") + idStyled +
		bdr.Render(strings.Repeat("─", fill)) +
		" " + statusStyled + " " +
		bdr.Render("─╮")

	// Description line
	name := truncate(a.Description, textWidth)
	namePad := textWidth - lipgloss.Width(name)
	if namePad < 0 {
		namePad = 0
	}
	nameLine := bdr.Render("│") + " " + valueStyle.Render(name) +
		strings.Repeat(" ", namePad) + " " + bdr.Render("│")

	// Branch line
	ahead := core.GitMust(repoRoot, "rev-list", "--count", srcBranch+".."+a.Branch)
	if ahead == "" {
		ahead = "?"
	}
	aheadStr := ahead + " ↑"
	iconWidth := 2 // branchIcon + space
	maxBranch := textWidth - lipgloss.Width(aheadStr) - 1 - iconWidth
	branch := truncate(a.Branch, maxBranch)
	branchGap := textWidth - lipgloss.Width(branch) - iconWidth - lipgloss.Width(aheadStr)
	if branchGap < 1 {
		branchGap = 1
	}
	branchLine := bdr.Render("│") + " " + RenderBranch(branch) +
		strings.Repeat(" ", branchGap) + aheadStyle.Render(aheadStr) +
		" " + bdr.Render("│")

	// Bottom border
	bottomLine := bdr.Render("╰" + strings.Repeat("─", cardWidth-2) + "╯")

	return topLine + "\n" + nameLine + "\n" + branchLine + "\n" + bottomLine
}

func RenderStatusView(repoRoot string, termWidth int) string {
	srcBranch := core.SourceBranch(repoRoot)
	var b strings.Builder

	b.WriteString(titleStyle.Render("⚡ augmux session"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s %s   %s %s\n\n",
		headerKeyStyle.Render("Repo:"), headerValStyle.Render(repoRoot),
		headerKeyStyle.Render("Source:"), RenderBranch(srcBranch)))

	agents := core.ListAgents(repoRoot)
	if len(agents) == 0 {
		b.WriteString(labelStyle.Render("  No agents running.\n"))
		return b.String()
	}

	var cards []string
	for _, idx := range agents {
		a, err := core.ReadAgent(repoRoot, idx)
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
	return tickEvery(500 * time.Millisecond)
}

func (m statusWatchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.content = RenderStatusView(m.repoRoot, m.width)
	case tickMsg:
		m.content = RenderStatusView(m.repoRoot, m.width)
		return m, tickEvery(500 * time.Millisecond)
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
	return m.content + "\n\n" + pickerHintStyle.Render("  q quit") + "\n"
}

func RunStatusWatch(repoRoot string) {
	m := statusWatchModel{repoRoot: repoRoot, width: 100}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		core.Fatal("TUI error: %v", err)
	}
}
