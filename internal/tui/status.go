package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/xemotrix/augmux/internal/core"
)

// TUIAction represents an action the user triggered from the interactive TUI.
type TUIAction int

const (
	ActionNone TUIAction = iota
	ActionSpawn
	ActionMerge
	ActionAccept
	ActionReject
	ActionCancel
	ActionFinish
)

// TUIResult holds the result of an interactive TUI session.
type TUIResult struct {
	Action   TUIAction
	AgentIdx int // selected agent index, or -1 if none
}

const (
	cardWidth = 42
	cardInner = cardWidth - 2 // minus border (2); lipgloss Width includes padding
	textWidth = cardInner - 2 // minus padding (2); actual chars per line
)

func countUncommitted(worktree string) int {
	out := core.GitMust(worktree, "status", "--porcelain")
	if out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
}

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

func activityRawStr(a *core.AgentState) string {
	if a.Activity == core.ActivityWorking {
		return "⠋ working"
	}
	return "● idle"
}

func activityIndicator(a *core.AgentState, spinnerFrame string) string {
	if a.Activity == core.ActivityWorking {
		return lipgloss.NewStyle().Foreground(colorYellow).Render(spinnerFrame + " working")
	}
	return lipgloss.NewStyle().Foreground(colorDimGray).Render("● idle")
}

func renderAgentCard(a *core.AgentState, repoRoot, srcBranch, spinnerFrame string, selected ...bool) string {
	sel := len(selected) > 0 && selected[0]
	borderColor := agentBorderColor(a)
	bdr := lipgloss.NewStyle().Foreground(borderColor)
	if sel {
		bdr = bdr.Bold(true)
	}

	// Border characters: thicker (double-line) when selected
	var cTL, cTR, cBL, cBR, cH, cV string
	if sel {
		cTL, cTR, cBL, cBR, cH, cV = "╔", "╗", "╚", "╝", "═", "║"
	} else {
		cTL, cTR, cBL, cBR, cH, cV = "╭", "╮", "╰", "╯", "─", "│"
	}

	statusRaw := AgentStatusRaw(a)
	statusStyled := AgentStatusStyled(a, statusRaw)

	// Top border: ╭─❮3❯──────────────────────── ● wip ─╮
	idLabel := fmt.Sprintf("❮%d❯", a.Index)
	idStyled := lipgloss.NewStyle().Bold(true).Foreground(colorWhite).Render(idLabel)

	// cardWidth - "╭─"(2) - idLabel - fill - " status "(1+statusRaw+1) - "─╮"(2)
	used := 2 + lipgloss.Width(idLabel) + 1 + lipgloss.Width(statusRaw) + 1 + 2
	fill := cardWidth - used
	if fill < 1 {
		fill = 1
	}
	topLine := bdr.Render(cTL+cH) + idStyled +
		bdr.Render(strings.Repeat(cH, fill)) +
		" " + statusStyled + " " +
		bdr.Render(cH+cTR)

	// Description line with activity indicator on the right
	activityStr := activityIndicator(a, spinnerFrame)
	activityRaw := activityRawStr(a)
	maxName := textWidth - lipgloss.Width(activityRaw) - 1
	if maxName < 10 {
		maxName = 10
	}
	name := truncate(a.Description, maxName)
	namePad := textWidth - lipgloss.Width(name) - lipgloss.Width(activityRaw)
	if namePad < 1 {
		namePad = 1
	}
	nameLine := bdr.Render(cV) + " " + valueStyle.Render(name) +
		strings.Repeat(" ", namePad) + activityStr + " " + bdr.Render(cV)

	// Branch line
	ahead := core.GitMust(repoRoot, "rev-list", "--count", srcBranch+".."+a.Branch)
	if ahead == "" {
		ahead = "?"
	}
	aheadStr := ahead + " ↑"

	// Count uncommitted changes in the worktree
	dirty := countUncommitted(a.Worktree)
	dirtyStr := fmt.Sprintf("%d ✎", dirty)

	rightInfo := aheadStr + "  " + dirtyStr
	iconWidth := 2 // branchIcon + space
	maxBranch := textWidth - lipgloss.Width(rightInfo) - 1 - iconWidth
	branch := truncate(a.Branch, maxBranch)
	branchGap := textWidth - lipgloss.Width(branch) - iconWidth - lipgloss.Width(rightInfo)
	if branchGap < 1 {
		branchGap = 1
	}
	branchLine := bdr.Render(cV) + " " + RenderBranch(branch) +
		strings.Repeat(" ", branchGap) +
		aheadStyle.Render(aheadStr) + "  " + dirtyStyle.Render(dirtyStr) +
		" " + bdr.Render(cV)

	// Bottom border
	bottomLine := bdr.Render(cBL + strings.Repeat(cH, cardWidth-2) + cBR)

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
		cards = append(cards, renderAgentCard(a, repoRoot, srcBranch, "⠋"))
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
	b.WriteString(pickerHintStyle.Render("  spawn · merge · accept · reject · cancel · finish"))
	return b.String()
}

// Interactive TUI mode

type interactiveTUIModel struct {
	repoRoot string
	width    int
	cursor   int // index into the agents slice
	agents   []*core.AgentState
	quitting bool
	action   TUIAction // pending action to execute after TUI exits
	spinner  spinner.Model
}

type tickMsg time.Time

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *interactiveTUIModel) refreshAgents() {
	indices := core.ListAgents(m.repoRoot)
	m.agents = nil
	for _, idx := range indices {
		a, err := core.ReadAgent(m.repoRoot, idx)
		if err != nil {
			continue
		}
		m.agents = append(m.agents, a)
	}
	if m.cursor >= len(m.agents) {
		m.cursor = len(m.agents) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m interactiveTUIModel) cols() int {
	cols := m.width / (cardWidth + 1)
	if cols < 1 {
		cols = 1
	}
	if len(m.agents) > 0 && cols > len(m.agents) {
		cols = len(m.agents)
	}
	return cols
}

func (m interactiveTUIModel) Init() tea.Cmd {
	return tea.Batch(tickEvery(500*time.Millisecond), m.spinner.Tick)
}

func (m interactiveTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.refreshAgents()
	case tickMsg:
		m.refreshAgents()
		return m, tickEvery(500 * time.Millisecond)
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		n := len(m.agents)
		cols := m.cols()

		// Resolve selected agent for action guards
		var sel *core.AgentState
		if m.cursor >= 0 && m.cursor < n {
			sel = m.agents[m.cursor]
		}
		isWip := sel != nil && sel.MergeCommit == "" && sel.Resolving == ""
		isMerged := sel != nil && sel.MergeCommit != ""
		isResolving := sel != nil && sel.Resolving != ""

		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "h", "left":
			if m.cursor > 0 {
				m.cursor--
			}
		case "l", "right":
			if m.cursor < n-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor-cols >= 0 {
				m.cursor -= cols
			}
		case "j", "down":
			if m.cursor+cols < n {
				m.cursor += cols
			}
		// Action keys
		case "s":
			m.action = ActionSpawn
			return m, tea.Quit
		case "m":
			if isWip {
				m.action = ActionMerge
				return m, tea.Quit
			}
		case "a":
			if isMerged {
				m.action = ActionAccept
				return m, tea.Quit
			}
		case "r":
			if isMerged || isResolving {
				m.action = ActionReject
				return m, tea.Quit
			}
		case "c":
			if isWip || isResolving {
				m.action = ActionCancel
				return m, tea.Quit
			}
		case "f":
			m.action = ActionFinish
			return m, tea.Quit
		}
	}
	return m, nil
}

// renderActionBar renders the bottom action bar with context-sensitive styling.
func renderActionBar(a *core.AgentState) string {
	type action struct {
		name    string
		enabled bool
	}

	isWip := a != nil && a.MergeCommit == "" && a.Resolving == ""
	isMerged := a != nil && a.MergeCommit != ""
	isResolving := a != nil && a.Resolving != ""

	actions := []action{
		{"spawn", true},                              // always available
		{"merge", isWip},                              // wip agents only
		{"accept", isMerged},                          // merged agents only
		{"reject", isMerged || isResolving},            // merged or resolving
		{"cancel", isWip || isResolving},               // wip or resolving
		{"finish", true},                              // always available
	}

	accentStyle := lipgloss.NewStyle().Bold(true).Foreground(colorPurple)
	enabledStyle := lipgloss.NewStyle().Bold(true).Foreground(colorWhite)
	disabledStyle := lipgloss.NewStyle().Foreground(colorDimGray)
	sepStyle := lipgloss.NewStyle().Foreground(colorDimGray)

	var parts []string
	for _, act := range actions {
		if act.enabled {
			first := accentStyle.Render(string(act.name[0]))
			rest := enabledStyle.Render(act.name[1:])
			parts = append(parts, first+rest)
		} else {
			parts = append(parts, disabledStyle.Render(act.name))
		}
	}

	return "  " + strings.Join(parts, sepStyle.Render(" · "))
}

func (m interactiveTUIModel) View() string {
	if m.quitting {
		return ""
	}

	srcBranch := core.SourceBranch(m.repoRoot)
	var b strings.Builder

	b.WriteString(titleStyle.Render("⚡ augmux session"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s %s   %s %s\n\n",
		headerKeyStyle.Render("Repo:"), headerValStyle.Render(m.repoRoot),
		headerKeyStyle.Render("Source:"), RenderBranch(srcBranch)))

	if len(m.agents) == 0 {
		b.WriteString(labelStyle.Render("  No agents running.\n"))
		b.WriteString("\n")
		b.WriteString(renderActionBar(nil))
		b.WriteString("\n")
		return b.String()
	}

	spinnerFrame := m.spinner.View()
	var cards []string
	for i, a := range m.agents {
		cards = append(cards, renderAgentCard(a, m.repoRoot, srcBranch, spinnerFrame, i == m.cursor))
	}

	cols := m.cols()

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

	// Action bar based on selected agent
	var selected *core.AgentState
	if m.cursor >= 0 && m.cursor < len(m.agents) {
		selected = m.agents[m.cursor]
	}
	b.WriteString(renderActionBar(selected))
	b.WriteString("\n")

	return b.String()
}

// runInteractiveTUIOnce runs one iteration of the interactive TUI and returns the result.
func runInteractiveTUIOnce(repoRoot string) TUIResult {
	s := spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(colorYellow)),
	)
	m := interactiveTUIModel{repoRoot: repoRoot, width: 100, spinner: s}
	m.refreshAgents()
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		core.Fatal("TUI error: %v", err)
	}
	fm := final.(interactiveTUIModel)

	agentIdx := -1
	if fm.cursor >= 0 && fm.cursor < len(fm.agents) {
		agentIdx = fm.agents[fm.cursor].Index
	}
	return TUIResult{Action: fm.action, AgentIdx: agentIdx}
}

// RunInteractiveTUI runs the interactive TUI. actionHandler is called when the
// user triggers an action; after it returns the TUI is re-launched.
func RunInteractiveTUI(repoRoot string, actionHandler func(TUIResult)) {
	for {
		result := runInteractiveTUIOnce(repoRoot)
		if result.Action == ActionNone {
			return
		}
		actionHandler(result)
		// Pause so user can read output before TUI re-launches
		fmt.Println()
		fmt.Println("Press Enter to return to TUI...")
		fmt.Scanln()
	}
}
