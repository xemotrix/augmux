package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
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
	ActionFocus
)

// TUIResult holds the result of an interactive TUI session.
type TUIResult struct {
	Action   TUIAction
	AgentIdx int // selected agent index, or -1 if none
}

// ActionResult is returned by TUI action handlers. Implementations are
// ActionDone (simple output) and MenuRequest (inline menu needed).
type ActionResult interface {
	isActionResult()
}

// ActionDone signals that the action completed. Lines are shown as status.
type ActionDone struct {
	Lines []string
}

func (ActionDone) isActionResult() {}

// MenuRequest signals that the action needs user input via an inline menu.
type MenuRequest struct {
	Title    string
	Options  []string
	Lines    []string                    // status lines shown above the menu
	Callback func(choice int) ActionResult // called with selected index (-1 = cancelled)
}

func (MenuRequest) isActionResult() {}

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

func agentBorderColor(a *core.AgentState, commitsAhead int) lipgloss.TerminalColor {
	if a.MergeCommit != "" {
		return colorCyan // merged
	}
	if a.Resolving != "" {
		return colorRed // resolving conflicts
	}
	if a.HasConflicts {
		return colorRed // would conflict if merged
	}
	if a.Activity == core.ActivityWorking {
		return colorYellow // working
	}
	// idle
	if commitsAhead > 0 {
		return colorGreen // idle, some commits
	}
	return colorDimGray // idle, no commits
}

func activityRawStr(a *core.AgentState) string {
	if a.Activity == core.ActivityWorking {
		return "⠋ working"
	}
	return "○ idle"
}

func activityIndicator(a *core.AgentState, spinnerFrame string) string {
	if a.Activity == core.ActivityWorking {
		return lipgloss.NewStyle().Foreground(colorYellow).Render(spinnerFrame + " working")
	}
	return lipgloss.NewStyle().Foreground(colorDimGray).Render("○ idle")
}

func renderAgentCard(a *core.AgentState, repoRoot, srcBranch, spinnerFrame string, selected ...bool) string {
	sel := len(selected) > 0 && selected[0]

	// Compute commits ahead early — needed for border color and branch line
	ahead := core.GitMust(repoRoot, "rev-list", "--count", srcBranch+".."+a.Branch)
	if ahead == "" {
		ahead = "?"
	}
	aheadNum := 0
	fmt.Sscanf(ahead, "%d", &aheadNum)

	borderColor := agentBorderColor(a, aheadNum)
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
	fill := max(cardWidth-used, 1)
	topLine := bdr.Render(cTL+cH) + idStyled +
		bdr.Render(strings.Repeat(cH, fill)) +
		" " + statusStyled + " " +
		bdr.Render(cH+cTR)

	// Description line with activity indicator on the right
	activityStr := activityIndicator(a, spinnerFrame)
	activityRaw := activityRawStr(a)
	maxName := max(textWidth-lipgloss.Width(activityRaw)-1, 10)
	name := truncate(a.Description, maxName)
	namePad := max(textWidth-lipgloss.Width(name)-lipgloss.Width(activityRaw), 1)
	nameLine := bdr.Render(cV) + " " + valueStyle.Render(name) +
		strings.Repeat(" ", namePad) + activityStr + " " + bdr.Render(cV)

	// Branch line
	aheadStr := ahead + " ↑"

	// Count uncommitted changes in the worktree
	dirty := countUncommitted(a.Worktree)
	dirtyStr := fmt.Sprintf("%d ✎", dirty)

	rightInfo := aheadStr + "  " + dirtyStr
	iconWidth := 2 // branchIcon + space
	maxBranch := textWidth - lipgloss.Width(rightInfo) - 1 - iconWidth
	branch := truncate(a.Branch, maxBranch)
	branchGap := max(textWidth-lipgloss.Width(branch)-iconWidth-lipgloss.Width(rightInfo), 1)
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

	b.WriteString(titleStyle.Render("  \uebc8 augmux session"))
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
		if a.MergeCommit == "" && a.Resolving == "" && a.Branch != "" {
			a.HasConflicts = core.DetectConflicts(repoRoot, srcBranch, a.Branch)
		}
		cards = append(cards, renderAgentCard(a, repoRoot, srcBranch, "⠋"))
	}

	cols := min(max(termWidth/(cardWidth+1), 1), len(cards))

	var rows []string
	for i := 0; i < len(cards); i += cols {
		end := min(i+cols, len(cards))
		row := lipgloss.JoinHorizontal(lipgloss.Top, cards[i:end]...)
		rows = append(rows, row)
	}
	b.WriteString(lipgloss.JoinVertical(lipgloss.Left, rows...))
	b.WriteString("\n\n")
	b.WriteString(pickerHintStyle.Render("  spawn · merge · accept · reject · cancel"))
	return b.String()
}

// Interactive TUI mode

type tuiMode int

const (
	modeNormal   tuiMode = iota
	modeSpawning         // text input for spawn task name
	modeMenu             // inline menu selection
)

type interactiveTUIModel struct {
	repoRoot      string
	width         int
	height        int
	cursor        int // index into the agents slice
	agents        []*core.AgentState
	quitting      bool
	mode          tuiMode
	spinner       spinner.Model
	textInput     textinput.Model                       // for spawn
	statusLines   []string                              // output from last action
	actionHandler func(TUIResult, string) ActionResult  // returns action result
	menuTitle     string                                // inline menu title
	menuOptions   []string                              // inline menu options
	menuCursor    int                                   // inline menu cursor
	menuCallback  func(int) ActionResult                // inline menu callback
}

type tickMsg time.Time

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *interactiveTUIModel) refreshAgents() {
	indices := core.ListAgents(m.repoRoot)
	srcBranch := core.SourceBranch(m.repoRoot)
	m.agents = nil
	for _, idx := range indices {
		a, err := core.ReadAgent(m.repoRoot, idx)
		if err != nil {
			continue
		}
		// Proactively detect merge conflicts for WIP agents.
		if a.MergeCommit == "" && a.Resolving == "" && a.Branch != "" {
			a.HasConflicts = core.DetectConflicts(m.repoRoot, srcBranch, a.Branch)
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
	cols := max(m.width/(cardWidth+1), 1)
	if len(m.agents) > 0 && cols > len(m.agents) {
		cols = len(m.agents)
	}
	return cols
}

// actionResultMsg wraps an ActionResult returned by the action handler.
type actionResultMsg struct {
	result ActionResult
}

func (m interactiveTUIModel) Init() tea.Cmd {
	return tea.Batch(tickEvery(500*time.Millisecond), m.spinner.Tick)
}

func (m interactiveTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.refreshAgents()

	case tickMsg:
		m.refreshAgents()
		return m, tickEvery(500 * time.Millisecond)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case actionResultMsg:
		switch v := msg.result.(type) {
		case ActionDone:
			m.statusLines = v.Lines
			m.refreshAgents()
		case MenuRequest:
			m.mode = modeMenu
			m.menuTitle = v.Title
			m.menuOptions = v.Options
			m.menuCursor = 0
			m.menuCallback = v.Callback
			m.statusLines = v.Lines
			m.refreshAgents()
		}
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeSpawning:
			return m.updateSpawning(msg)
		case modeMenu:
			return m.updateMenu(msg)
		default:
			// Normal mode — clear status on any key if showing status
			if len(m.statusLines) > 0 {
				m.statusLines = nil
			}
			return m.updateNormal(msg)
		}
	}

	// Forward to text input if in spawning mode
	if m.mode == modeSpawning {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m interactiveTUIModel) updateSpawning(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		name := strings.TrimSpace(m.textInput.Value())
		m.mode = modeNormal
		if name == "" {
			m.statusLines = []string{"Empty name — spawn aborted."}
			return m, nil
		}
		// Run spawn with captured output
		agentIdx := -1
		result := TUIResult{Action: ActionSpawn, AgentIdx: agentIdx}
		handler := m.actionHandler
		return m, func() tea.Msg {
			return actionResultMsg{result: handler(result, name)}
		}
	case "esc", "ctrl+c":
		m.mode = modeNormal
		m.statusLines = []string{"Spawn cancelled."}
		return m, nil
	}
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m interactiveTUIModel) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down", "j":
		if m.menuCursor < len(m.menuOptions)-1 {
			m.menuCursor++
		}
	case "enter":
		m.mode = modeNormal
		cb := m.menuCallback
		cursor := m.menuCursor
		m.statusLines = nil
		return m, func() tea.Msg {
			return actionResultMsg{result: cb(cursor)}
		}
	case "esc", "ctrl+c":
		m.mode = modeNormal
		cb := m.menuCallback
		m.statusLines = nil
		return m, func() tea.Msg {
			return actionResultMsg{result: cb(-1)}
		}
	}
	return m, nil
}

func (m interactiveTUIModel) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

	// Check if agent has commits ahead of source branch
	hasCommits := false
	if sel != nil {
		srcBranch := core.SourceBranch(m.repoRoot)
		ahead := core.GitMust(m.repoRoot, "rev-list", "--count", srcBranch+".."+sel.Branch)
		aheadNum := 0
		fmt.Sscanf(ahead, "%d", &aheadNum)
		hasCommits = aheadNum > 0
	}

	agentIdx := -1
	if sel != nil {
		agentIdx = sel.Index
	}

	switch msg.String() {
	case "q", "ctrl+c":
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
		ti := textinput.New()
		ti.Placeholder = "e.g. fix auth bug"
		ti.Focus()
		ti.CharLimit = 80
		ti.Width = 50
		ti.PromptStyle = pickerCursorStyle
		ti.TextStyle = pickerSelectedStyle
		m.textInput = ti
		m.mode = modeSpawning
		m.statusLines = nil
		return m, textinput.Blink
	case "m":
		if isWip && hasCommits {
			return m.runInlineAction(ActionMerge, agentIdx)
		}
	case "a":
		if isMerged {
			return m.runInlineAction(ActionAccept, agentIdx)
		}
	case "r":
		if isMerged || isResolving {
			return m.runInlineAction(ActionReject, agentIdx)
		}
	case "c":
		if isWip || isResolving {
			return m.runInlineAction(ActionCancel, agentIdx)
		}
	case "enter":
		if sel != nil {
			return m.runInlineAction(ActionFocus, agentIdx)
		}
	}
	return m, nil
}

// runInlineAction runs an action in a goroutine. The handler may return
// ActionDone (status lines) or MenuRequest (inline menu).
func (m interactiveTUIModel) runInlineAction(action TUIAction, agentIdx int) (tea.Model, tea.Cmd) {
	result := TUIResult{Action: action, AgentIdx: agentIdx}
	handler := m.actionHandler
	return m, func() tea.Msg {
		return actionResultMsg{result: handler(result, "")}
	}
}

// renderActionBar renders the bottom action bar with context-sensitive styling.
func renderActionBar(a *core.AgentState, repoRoot string) string {
	type action struct {
		name    string
		enabled bool
	}

	isWip := a != nil && a.MergeCommit == "" && a.Resolving == ""
	isMerged := a != nil && a.MergeCommit != ""
	isResolving := a != nil && a.Resolving != ""

	// Check if agent has commits ahead of source branch
	hasCommits := false
	if a != nil && repoRoot != "" {
		srcBranch := core.SourceBranch(repoRoot)
		ahead := core.GitMust(repoRoot, "rev-list", "--count", srcBranch+".."+a.Branch)
		aheadNum := 0
		fmt.Sscanf(ahead, "%d", &aheadNum)
		hasCommits = aheadNum > 0
	}

	hasAgent := a != nil

	actions := []action{
		{"enter:focus", hasAgent},           // focus agent window
		{"spawn", true},                     // always available
		{"merge", isWip && hasCommits},      // wip agents with commits only
		{"accept", isMerged},                // merged agents only
		{"reject", isMerged || isResolving}, // merged or resolving
		{"cancel", isWip || isResolving},    // wip or resolving
	}

	accentStyle := lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	enabledStyle := lipgloss.NewStyle().Bold(true).Foreground(colorWhite)
	disabledStyle := lipgloss.NewStyle().Foreground(colorDimGray)
	sepStyle := lipgloss.NewStyle().Foreground(colorDimGray)

	var parts []string
	for _, act := range actions {
		if act.enabled {
			if idx := strings.Index(act.name, ":"); idx >= 0 {
				// "key:label" format — highlight the key portion
				key := act.name[:idx]
				label := act.name[idx+1:]
				parts = append(parts, accentStyle.Render(key)+" "+enabledStyle.Render(label))
			} else {
				first := accentStyle.Render(string(act.name[0]))
				rest := enabledStyle.Render(act.name[1:])
				parts = append(parts, first+rest)
			}
		} else {
			name := act.name
			if idx := strings.Index(name, ":"); idx >= 0 {
				name = name[:idx] + " " + name[idx+1:]
			}
			parts = append(parts, disabledStyle.Render(name))
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

	b.WriteString(titleStyle.Render("  \uebc8 augmux session"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s %s   %s %s\n\n",
		headerKeyStyle.Render("Repo:"), headerValStyle.Render(m.repoRoot),
		headerKeyStyle.Render("Source:"), RenderBranch(srcBranch)))

	if len(m.agents) == 0 {
		b.WriteString(labelStyle.Render("  No agents running.\n"))
	} else {
		spinnerFrame := m.spinner.View()
		var cards []string
		for i, a := range m.agents {
			cards = append(cards, renderAgentCard(a, m.repoRoot, srcBranch, spinnerFrame, i == m.cursor))
		}

		cols := m.cols()

		var rows []string
		for i := 0; i < len(cards); i += cols {
			end := min(i+cols, len(cards))
			row := lipgloss.JoinHorizontal(lipgloss.Top, cards[i:end]...)
			rows = append(rows, row)
		}
		b.WriteString(lipgloss.JoinVertical(lipgloss.Left, rows...))
	}
	b.WriteString("\n\n")

	// Show status lines from last action
	if len(m.statusLines) > 0 {
		statusStyle := lipgloss.NewStyle().Foreground(colorGreen)
		for _, line := range m.statusLines {
			b.WriteString(statusStyle.Render(line))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Show text input for spawn mode
	if m.mode == modeSpawning {
		b.WriteString(titleStyle.Render("Task name for new agent:"))
		b.WriteString("\n\n")
		b.WriteString("  " + m.textInput.View())
		b.WriteString("\n\n")
		b.WriteString(pickerHintStyle.Render("  enter confirm · esc cancel"))
		b.WriteString("\n")
		return b.String()
	}

	// Show inline menu
	if m.mode == modeMenu {
		b.WriteString(titleStyle.Render(m.menuTitle))
		b.WriteString("\n\n")
		for i, opt := range m.menuOptions {
			cursor := "  "
			style := pickerNormalStyle
			if i == m.menuCursor {
				cursor = pickerCursorStyle.Render("▸ ")
				style = pickerSelectedStyle
			}
			b.WriteString(cursor + style.Render(opt) + "\n")
		}
		b.WriteString("\n")
		b.WriteString(pickerHintStyle.Render("  j/k navigate · enter select · esc cancel"))
		b.WriteString("\n")
		return b.String()
	}

	// Action bar based on selected agent
	var selected *core.AgentState
	if m.cursor >= 0 && m.cursor < len(m.agents) {
		selected = m.agents[m.cursor]
	}
	b.WriteString(renderActionBar(selected, m.repoRoot))
	b.WriteString("\n")

	return b.String()
}

// RunInteractiveTUI runs the interactive TUI. actionHandler is called when the
// user triggers an action. It returns an ActionResult: ActionDone for simple
// status output, or MenuRequest to show an inline menu within the TUI.
//
// The actionHandler receives a TUIResult and an optional spawn name (non-empty
// only for ActionSpawn).
func RunInteractiveTUI(repoRoot string, actionHandler func(TUIResult, string) ActionResult) {
	s := spinner.New(
		spinner.WithSpinner(spinner.Spinner{
			Frames: []string{"✶", "✸", "✹", "✺", "✹", "✷"},
			FPS:    time.Second / 10,
		}),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(colorYellow)),
	)
	m := interactiveTUIModel{
		repoRoot:      repoRoot,
		width:         100,
		spinner:       s,
		actionHandler: actionHandler,
	}
	m.refreshAgents()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		core.Fatal("TUI error: %v", err)
	}
}
