package tui

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/xemotrix/augmux/internal/agent"
	"github.com/xemotrix/augmux/internal/core"
	"github.com/xemotrix/augmux/internal/ops"
	"github.com/xemotrix/augmux/internal/styles"
)

// TUIAction represents an action the user triggered from the interactive TUI.
type TUIAction int

const (
	ActionSpawn TUIAction = iota
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
	Level ToastLevel
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
	rebaseSpinner spinner.Model
	textInput     textinput.Model                      // for spawn
	toaster       toaster                              // toast notifications
	actionHandler func(TUIResult, string) ActionResult // returns action result
	menuTitle     string                               // inline menu title
	menuOptions   []string                             // inline menu options
	menuCursor    int                                  // inline menu cursor
	menuCallback  func(int) ActionResult               // inline menu callback

	rebasingAgentIdx int // agent index being rebased, -1 = none
}

type tickMsg time.Time
type rebaseDoneMsg struct{ err error }

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
		if m.rebasingAgentIdx == a.Index {
			a.Rebasing = true
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
	return tea.Batch(tickEvery(500*time.Millisecond), m.spinner.Tick, m.rebaseSpinner.Tick)
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
		var cmd1, cmd2 tea.Cmd
		m.spinner, cmd1 = m.spinner.Update(msg)
		m.rebaseSpinner, cmd2 = m.rebaseSpinner.Update(msg)
		return m, tea.Batch(cmd1, cmd2)

	case addToastMsg, toastExpiredMsg:
		var cmd tea.Cmd
		m.toaster, cmd = m.toaster.Update(msg)
		return m, cmd

	case rebaseDoneMsg:
		m.rebasingAgentIdx = -1
		m.refreshAgents()
		if msg.err != nil {
			return m, AddToast(fmt.Sprintf("Rebase failed: %s", msg.err), ToastError)
		}
		return m, AddToast("Rebase completed", ToastSuccess)

	case actionResultMsg:
		switch v := msg.result.(type) {
		case ActionDone:
			m.refreshAgents()
			var cmds []tea.Cmd
			for _, line := range v.Lines {
				cmds = append(cmds, AddToast(line, v.Level))
			}
			return m, tea.Batch(cmds...)
		case MenuRequest:
			m.mode = modeMenu
			m.menuTitle = v.Title
			m.menuOptions = v.Options
			m.menuCursor = 0
			m.menuCallback = v.Callback
			m.refreshAgents()
			var cmds []tea.Cmd
			for _, line := range v.Lines {
				cmds = append(cmds, AddToast(line, ToastInfo))
			}
			return m, tea.Batch(cmds...)
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

func validateAgentName(s string) error {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != ' ' {
			return fmt.Errorf("only letters, numbers, and spaces are allowed")
		}
	}
	return nil
}

func (m TUIModel) updateSpawning(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		name := strings.TrimSpace(m.textInput.Value())
		m.mode = modeNormal
		if name == "" {
			return m, AddToast("Empty name — spawn aborted.", ToastWarning)
		}
		if err := validateAgentName(name); err != nil {
			return m, AddToast("Invalid name: "+err.Error(), ToastWarning)
		}
		agentIdx := -1
		result := TUIResult{Action: ActionSpawn, AgentIdx: agentIdx}
		handler := m.actionHandler
		return m, func() tea.Msg {
			return actionResultMsg{result: handler(result, name)}
		}
	case "esc", "ctrl+c":
		m.mode = modeNormal
		return m, AddToast("Spawn cancelled.", ToastInfo)
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
		return m, func() tea.Msg {
			return actionResultMsg{result: cb(cursor)}
		}
	case "esc", "ctrl+c":
		m.mode = modeNormal
		cb := m.menuCallback
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
		m.mode = modeNormal
		if m.menuCursor >= 0 && m.menuCursor < len(defs) {
			if err := agent.SaveAgentChoice(defs[m.menuCursor].ID); err != nil {
				return m, AddToast(fmt.Sprintf("Failed to save config: %v", err), ToastError)
			}
			return m, AddToast(fmt.Sprintf("Configured to use %s", defs[m.menuCursor].DisplayName), ToastSuccess)
		}
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
	isIdle := sel != nil && sel.Activity == core.ActivityIdle
	isRebasing := sel != nil && sel.Rebasing

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
		ti.Validate = validateAgentName
		ti.PromptStyle = styles.PickerCursorStyle
		ti.TextStyle = styles.PickerSelectedStyle
		m.textInput = ti
		m.mode = modeSpawning
		return m, textinput.Blink
	case "m":
		if isWip && hasCommits && !isRebasing {
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
	case "b":
		if isWip && sel.HasConflicts && isIdle && m.rebasingAgentIdx < 0 {
			m.rebasingAgentIdx = agentIdx
			repoRoot := m.repoRoot
			return m, func() tea.Msg {
				return startRebase(repoRoot, agentIdx)
			}
		}
	case "enter":
		if sel != nil {
			return m.runInlineAction(ActionFocus, agentIdx)
		}
	}
	return m, nil
}

// startRebase runs the headless agent subprocess and waits for it to finish.
func startRebase(repoRoot string, agentIdx int) tea.Msg {
	cmd, err := ops.RebaseCmd(repoRoot, agentIdx)
	if err != nil {
		return rebaseDoneMsg{err: err}
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return rebaseDoneMsg{err: cmd.Run()}
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
	isIdle := a != nil && a.Activity == core.ActivityIdle
	hasConflicts := a != nil && a.HasConflicts
	isRebasing := a != nil && a.Rebasing

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
		{"enter:focus", hasAgent},
		{"spawn", true},
		{"merge", isWip && hasCommits && !isRebasing},
		{"re|base", isWip && hasConflicts && isIdle && !isRebasing},
		{"accept", isMerged},
		{"reject", isMerged || isResolving},
		{"cancel", isWip || isResolving},
	}

	accentStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.ColorAccent)
	enabledStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.ColorWhite)
	disabledStyle := lipgloss.NewStyle().Foreground(styles.ColorDimGray)
	sepStyle := lipgloss.NewStyle().Foreground(styles.ColorDimGray)

	var parts []string
	for i, act := range actions {
		if i > 0 {
			parts = append(parts, sepStyle.Render(" · "))
		}
		if act.enabled {
			if idx := strings.Index(act.name, ":"); idx >= 0 {
				key := act.name[:idx]
				label := act.name[idx+1:]
				parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Center,
					accentStyle.Render(key),
					enabledStyle.Render(" "+label),
				))
			} else if idx := strings.Index(act.name, "|"); idx >= 0 {
				prefix := act.name[:idx]
				rest := act.name[idx+1:]
				parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Center,
					enabledStyle.Render(prefix),
					accentStyle.Render(string(rest[0])),
					enabledStyle.Render(rest[1:]),
				))
			} else {
				parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Center,
					accentStyle.Render(string(act.name[0])),
					enabledStyle.Render(act.name[1:]),
				))
			}
		} else {
			name := act.name
			if idx := strings.Index(name, ":"); idx >= 0 {
				name = name[:idx] + " " + name[idx+1:]
			}
			name = strings.ReplaceAll(name, "|", "")
			parts = append(parts, disabledStyle.Render(name))
		}
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Center, parts...)
	return bar
}

func renderMenu(title string, options []string, cursor int, hint string) string {
	titleLine := styles.TitleStyle.Render(title)

	var items []string
	for i, opt := range options {
		cur := lipgloss.NewStyle().Width(2).Render("")
		style := styles.PickerNormalStyle
		if i == cursor {
			cur = styles.PickerCursorStyle.Render("▸ ")
			style = styles.PickerSelectedStyle
		}
		items = append(items, lipgloss.JoinHorizontal(lipgloss.Top, cur, style.Render(opt)))
	}
	optionsList := lipgloss.JoinVertical(lipgloss.Left, items...)

	hintLine := styles.PickerHintStyle.Render(hint)

	return lipgloss.JoinVertical(lipgloss.Left, titleLine, "", optionsList, "", hintLine, "")
}

func (m TUIModel) View() string {
	if m.quitting {
		return ""
	}

	srcBranch := core.SourceBranch(m.repoRoot)

	outerPad := lipgloss.NewStyle().PaddingLeft(2)

	title := styles.TitleStyle.Render("\uebc8 augmux session")

	header := lipgloss.JoinHorizontal(lipgloss.Center,
		styles.HeaderKeyStyle.Render("Repo:"),
		styles.HeaderValStyle.Render(" "+m.repoRoot+"   "),
		styles.HeaderKeyStyle.Render("Source:"),
		lipgloss.NewStyle().Render(" "),
		styles.RenderBranch(srcBranch),
	)
	sections := []string{title, header, ""}

	if m.mode == modeAgentSetup {
		menu := renderMenu(m.menuTitle, m.menuOptions, m.menuCursor,
			"j/k navigate · enter select · esc quit")
		sections = append(sections, menu)
		return outerPad.Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
	}

	if len(m.agents) == 0 {
		sections = append(sections, styles.LabelStyle.Render("No agents running."))
	} else {
		spinnerFrame := m.spinner.View()
		rebaseSpinnerFrame := m.rebaseSpinner.View()
		var cards []string
		for i, a := range m.agents {
			cards = append(cards, RunAgentCard(a, m.repoRoot, srcBranch, spinnerFrame, rebaseSpinnerFrame, i == m.cursor))
		}

		cols := m.cols()

		var rows []string
		for i := 0; i < len(cards); i += cols {
			end := min(i+cols, len(cards))
			row := lipgloss.JoinHorizontal(lipgloss.Top, cards[i:end]...)
			rows = append(rows, row)
		}
		sections = append(sections, lipgloss.JoinVertical(lipgloss.Left, rows...))
	}

	sections = append(sections, "")

	if toastView := m.toaster.View(); toastView != "" {
		sections = append(sections, toastView, "")
	}

	if m.mode == modeSpawning {
		spawnTitle := styles.TitleStyle.Render("Task name for new agent:")
		inputLine := lipgloss.NewStyle().Render(m.textInput.View())
		hint := styles.PickerHintStyle.Render("enter confirm · esc cancel")
		sections = append(sections, spawnTitle, "", inputLine, "", hint, "")
		return outerPad.Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
	}

	if m.mode == modeMenu {
		menu := renderMenu(m.menuTitle, m.menuOptions, m.menuCursor,
			"j/k navigate · enter select · esc cancel")
		sections = append(sections, menu)
		return outerPad.Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
	}

	var selected *core.AgentState
	if m.cursor >= 0 && m.cursor < len(m.agents) {
		selected = m.agents[m.cursor]
	}
	bar := renderActionBar(selected, m.repoRoot)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	contentHeight := lipgloss.Height(content)
	barHeight := 2 // action bar + 1 line bottom padding
	gap := m.height - contentHeight - barHeight
	if gap > 0 {
		content = content + strings.Repeat("\n", gap)
	}

	return outerPad.Render(content + "\n" + bar)
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
	rs := spinner.New(
		spinner.WithSpinner(spinner.Spinner{
			Frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
			FPS:    time.Second / 8,
		}),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(styles.ColorPurple)),
	)
	m := TUIModel{
		repoRoot:         repoRoot,
		width:            100,
		spinner:          s,
		rebaseSpinner:    rs,
		toaster:          newToaster(),
		actionHandler:    tuiActionHandler(repoRoot),
		rebasingAgentIdx: -1,
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
		var sink bytes.Buffer
		idx := result.AgentIdx

		agentLabel := fmt.Sprintf("agent %d", idx)
		if idx >= 0 {
			if ag, err := core.ReadAgent(repoRoot, idx); err == nil && ag.Description != "" {
				agentLabel = fmt.Sprintf("%q", ag.Description)
			}
		}

		switch result.Action {
		case ActionMerge:
			if idx >= 0 {
				err := ops.MergeOne(&sink, repoRoot, idx)
				if conflictErr, ok := err.(*ops.MergeConflictErr); ok {
					return MenuRequest{
						Title: fmt.Sprintf("Conflict merging %s — how to resolve?", agentLabel),
						Options: []string{
							"Continue — leave conflicts, resolve manually",
							"Abort — discard merge and reset",
						},
						Callback: func(choice int) ActionResult {
							var sink2 bytes.Buffer
							if choice == 1 || choice == -1 {
								ops.ResolveConflict(&sink2, conflictErr, -1)
								return ActionDone{
									Lines: []string{"Merge aborted"},
									Level: ToastWarning,
								}
							}
							ops.ResolveConflict(&sink2, conflictErr, 0)
							return ActionDone{
								Lines: []string{"Conflicts left for manual resolution"},
								Level: ToastWarning,
							}
						},
					}
				}
				if err != nil {
					return ActionDone{
						Lines: []string{fmt.Sprintf("Merge failed: %s", err)},
						Level: ToastError,
					}
				}
				return ActionDone{
					Lines: []string{fmt.Sprintf("Merged %s", agentLabel)},
					Level: ToastSuccess,
				}
			}

		case ActionSpawn:
			ops.SpawnByName(&sink, repoRoot, spawnName)
			return ActionDone{
				Lines: []string{fmt.Sprintf("Spawned %q", spawnName)},
				Level: ToastSuccess,
			}

		case ActionAccept:
			if idx >= 0 {
				if err := ops.AcceptOne(&sink, repoRoot, idx); err != nil {
					return ActionDone{
						Lines: []string{fmt.Sprintf("Accept failed: %s", err)},
						Level: ToastError,
					}
				}
				return ActionDone{
					Lines: []string{fmt.Sprintf("Accepted %s", agentLabel)},
					Level: ToastSuccess,
				}
			}

		case ActionReject:
			if idx >= 0 {
				if err := ops.RejectOne(&sink, repoRoot, idx); err != nil {
					return ActionDone{
						Lines: []string{fmt.Sprintf("Reject failed: %s", err)},
						Level: ToastError,
					}
				}
				return ActionDone{
					Lines: []string{fmt.Sprintf("Rejected %s", agentLabel)},
					Level: ToastSuccess,
				}
			}

		case ActionCancel:
			if idx >= 0 {
				ag, err := core.ReadAgent(repoRoot, idx)
				if err != nil {
					return ActionDone{
						Lines: []string{fmt.Sprintf("Cancel failed: %s", err)},
						Level: ToastError,
					}
				}

				srcBranch := core.SourceBranch(repoRoot)
				ahead := core.GitMust(repoRoot, "rev-list", "--count", srcBranch+".."+ag.Branch)
				aheadNum := 0
				fmt.Sscanf(ahead, "%d", &aheadNum)

				dirty := false
				if core.IsDir(ag.Worktree) {
					status, _ := core.Git(ag.Worktree, "status", "--porcelain")
					dirty = status != ""
				}

				if aheadNum > 0 || dirty {
					var warnings []string
					if aheadNum > 0 {
						commitWord := "commit"
						if aheadNum != 1 {
							commitWord = "commits"
						}
						warnings = append(warnings, fmt.Sprintf("%d %s", aheadNum, commitWord))
					}
					if dirty {
						warnings = append(warnings, "uncommitted changes")
					}
					detail := strings.Join(warnings, " and ")

					return MenuRequest{
						Title: fmt.Sprintf("Cancel %s? It has %s that will be lost.", agentLabel, detail),
						Options: []string{
							"Yes — discard and cancel",
							"No — keep agent",
						},
						Callback: func(choice int) ActionResult {
							if choice == 0 {
								var sink2 bytes.Buffer
								ops.CancelOne(&sink2, repoRoot, idx)
								return ActionDone{
									Lines: []string{fmt.Sprintf("Cancelled %s", agentLabel)},
									Level: ToastSuccess,
								}
							}
							return ActionDone{
								Lines: []string{"Cancel aborted"},
								Level: ToastInfo,
							}
						},
					}
				}

				ops.CancelOne(&sink, repoRoot, idx)
				return ActionDone{
					Lines: []string{fmt.Sprintf("Cancelled %s", agentLabel)},
					Level: ToastSuccess,
				}
			}

		case ActionFocus:
			if idx >= 0 {
				ag, err := core.ReadAgent(repoRoot, idx)
				if err != nil {
					return ActionDone{
						Lines: []string{fmt.Sprintf("Failed to focus: %s", err)},
						Level: ToastError,
					}
				}
				core.TmuxRun("select-window", "-t", ag.Window)
			}
		}

		return ActionDone{}
	}
}
