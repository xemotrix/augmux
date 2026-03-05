package tui

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/xemotrix/augmux/internal/agent"
	"github.com/xemotrix/augmux/internal/core"
	"github.com/xemotrix/augmux/internal/ops"
	"github.com/xemotrix/augmux/internal/styles"
	// "github.com/xemotrix/augmux/internal/tui"
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
	Lines    []string                      // status lines shown above the menu
	Callback func(choice int) ActionResult // called with selected index (-1 = cancelled)
}

func (MenuRequest) isActionResult() {}

const (
	cardWidth = 42
	cardInner = cardWidth - 2 // minus border (2); lipgloss Width includes padding
	textWidth = cardInner - 2 // minus padding (2); actual chars per line
)

// Interactive TUI mode

type tuiMode int

const (
	modeNormal     tuiMode = iota
	modeSpawning           // text input for spawn task name
	modeMenu               // inline menu selection
	modeAgentSetup         // agent CLI picker shown on startup
)

type TUIModel struct {
	repoRoot      string
	width         int
	height        int
	cursor        int // index into the agents slice
	agents        []*core.AgentState
	quitting      bool
	mode          tuiMode
	spinner       spinner.Model
	textInput     textinput.Model                      // for spawn
	statusLines   []string                             // output from last action
	actionHandler func(TUIResult, string) ActionResult // returns action result
	menuTitle     string                               // inline menu title
	menuOptions   []string                             // inline menu options
	menuCursor    int                                  // inline menu cursor
	menuCallback  func(int) ActionResult               // inline menu callback
}

type tickMsg time.Time

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *TUIModel) refreshAgents() {
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

func (m TUIModel) cols() int {
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

func (m TUIModel) Init() tea.Cmd {
	return tea.Batch(tickEvery(500*time.Millisecond), m.spinner.Tick)
}

func (m TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		case modeAgentSetup:
			return m.updateAgentSetup(msg)
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

func (m TUIModel) updateSpawning(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (m TUIModel) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (m TUIModel) updateAgentSetup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		defs := agent.KnownAgentDefs()
		if m.menuCursor >= 0 && m.menuCursor < len(defs) {
			if err := agent.SaveAgentChoice(defs[m.menuCursor].ID); err != nil {
				m.statusLines = []string{fmt.Sprintf("Failed to save config: %v", err)}
			} else {
				m.statusLines = []string{fmt.Sprintf("  ✓ Configured to use %s", defs[m.menuCursor].DisplayName)}
			}
		}
		m.mode = modeNormal
		return m, nil
	case "esc", "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m TUIModel) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		ti.PromptStyle = styles.PickerCursorStyle
		ti.TextStyle = styles.PickerSelectedStyle
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
func (m TUIModel) runInlineAction(action TUIAction, agentIdx int) (tea.Model, tea.Cmd) {
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

	accentStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.ColorAccent)
	enabledStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.ColorWhite)
	disabledStyle := lipgloss.NewStyle().Foreground(styles.ColorDimGray)
	sepStyle := lipgloss.NewStyle().Foreground(styles.ColorDimGray)

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

func (m TUIModel) View() string {
	if m.quitting {
		return ""
	}

	srcBranch := core.SourceBranch(m.repoRoot)
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("  \uebc8 augmux session"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s %s   %s %s\n\n",
		styles.HeaderKeyStyle.Render("Repo:"), styles.HeaderValStyle.Render(m.repoRoot),
		styles.HeaderKeyStyle.Render("Source:"), styles.RenderBranch(srcBranch)))

	if m.mode == modeAgentSetup {
		b.WriteString(styles.TitleStyle.Render(m.menuTitle))
		b.WriteString("\n\n")
		for i, opt := range m.menuOptions {
			cursor := "  "
			style := styles.PickerNormalStyle
			if i == m.menuCursor {
				cursor = styles.PickerCursorStyle.Render("▸ ")
				style = styles.PickerSelectedStyle
			}
			b.WriteString(cursor + style.Render(opt) + "\n")
		}
		b.WriteString("\n")
		b.WriteString(styles.PickerHintStyle.Render("  j/k navigate · enter select · esc quit"))
		b.WriteString("\n")
		return b.String()
	}

	if len(m.agents) == 0 {
		b.WriteString(styles.LabelStyle.Render("  No agents running.\n"))
	} else {
		spinnerFrame := m.spinner.View()
		var cards []string
		for i, a := range m.agents {
			cards = append(cards, RunAgentCard(a, m.repoRoot, srcBranch, spinnerFrame, i == m.cursor))
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
		statusStyle := lipgloss.NewStyle().Foreground(styles.ColorGreen)
		for _, line := range m.statusLines {
			b.WriteString(statusStyle.Render(line))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Show text input for spawn mode
	if m.mode == modeSpawning {
		b.WriteString(styles.TitleStyle.Render("Task name for new agent:"))
		b.WriteString("\n\n")
		b.WriteString("  " + m.textInput.View())
		b.WriteString("\n\n")
		b.WriteString(styles.PickerHintStyle.Render("  enter confirm · esc cancel"))
		b.WriteString("\n")
		return b.String()
	}

	// Show inline menu
	if m.mode == modeMenu {
		b.WriteString(styles.TitleStyle.Render(m.menuTitle))
		b.WriteString("\n\n")
		for i, opt := range m.menuOptions {
			cursor := "  "
			style := styles.PickerNormalStyle
			if i == m.menuCursor {
				cursor = styles.PickerCursorStyle.Render("▸ ")
				style = styles.PickerSelectedStyle
			}
			b.WriteString(cursor + style.Render(opt) + "\n")
		}
		b.WriteString("\n")
		b.WriteString(styles.PickerHintStyle.Render("  j/k navigate · enter select · esc cancel"))
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
func RunInteractiveTUI(
	repoRoot string,
) {
	s := spinner.New(
		spinner.WithSpinner(spinner.Spinner{
			Frames: []string{"✶", "✸", "✹", "✺", "✹", "✷"},
			FPS:    time.Second / 10,
		}),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(styles.ColorYellow)),
	)
	m := TUIModel{
		repoRoot:      repoRoot,
		width:         100,
		spinner:       s,
		actionHandler: tuiActionHandler(repoRoot),
	}
	if !agent.IsConfigured() {
		defs := agent.KnownAgentDefs()
		var options []string
		for _, d := range defs {
			options = append(options, d.DisplayName)
		}
		m.mode = modeAgentSetup
		m.menuTitle = "No agent CLI configured — select one:"
		m.menuOptions = options
		m.menuCursor = 0
	}
	m.refreshAgents()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		core.Fatal("TUI error: %v", err)
	}
}

func tuiActionHandler(repoRoot string) func(TUIResult, string) ActionResult {
	return func(result TUIResult, spawnName string) ActionResult {
		// All ops output is discarded — the TUI refreshes agent state
		// automatically, so no feedback text is needed.
		var sink bytes.Buffer
		idx := result.AgentIdx

		switch result.Action {
		case ActionMerge:
			if idx >= 0 {
				err := ops.MergeOne(&sink, repoRoot, idx, ops.MergeTUI)
				if conflictErr, ok := err.(*ops.MergeConflictErr); ok {
					return MenuRequest{
						Title: fmt.Sprintf("Conflict merging agent %d — how to resolve?", idx),
						Options: []string{
							"Continue — leave conflicts, resolve manually",
							"Abort — discard merge and reset",
						},
						Callback: func(choice int) ActionResult {
							var sink2 bytes.Buffer
							// Remap choice: 0=continue, 1=abort (maps to default in ResolveConflict)
							if choice == 1 {
								choice = -1
							}
							ops.ResolveConflict(&sink2, conflictErr, choice)
							return ActionDone{}
						},
					}
				}
			}

		case ActionSpawn:
			ops.SpawnByName(&sink, repoRoot, spawnName)

		case ActionAccept:
			if idx >= 0 {
				ops.AcceptOne(&sink, repoRoot, idx)
			}

		case ActionReject:
			if idx >= 0 {
				ops.RejectOne(&sink, repoRoot, idx)
			}

		case ActionCancel:
			if idx >= 0 {
				ops.CancelOne(&sink, repoRoot, idx)
			}

		case ActionFocus:
			if idx >= 0 {
				ag, err := core.ReadAgent(repoRoot, idx)
				if err == nil {
					core.TmuxRun("select-window", "-t", ag.Window)
				}
			}
		}

		return ActionDone{}
	}
}
