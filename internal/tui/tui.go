package tui

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/xemotrix/augmux/internal/agent"
	"github.com/xemotrix/augmux/internal/core"
	"github.com/xemotrix/augmux/internal/styles"
)

const (
	cardWidth = 42
	cardInner = cardWidth - 2 // minus border (2); lipgloss Width includes padding
	textWidth = cardInner - 2 // minus padding (2); actual chars per line
)

type tuiMode int

const (
	modeNormal       tuiMode = iota
	modeSpawning             // text input for spawn task name
	modeMenu                 // inline menu selection
	modeAgentSetup           // agent CLI picker shown on startup
	modeConflictTree         // full-screen conflict tree overlay
)

type TUIModel struct {
	repoRoot      string
	srcBranch     string // cached source branch (set in refreshAgents)
	width         int
	height        int
	cursor        int // index into the agents slice
	agents        []*core.Agent
	quitting      bool
	mode          tuiMode
	spinner       spinner.Model
	textInput     textinput.Model                      // for spawn
	toaster       toaster                              // toast notifications
	actionHandler func(TUIResult, string) ActionResult // returns action result
	menuTitle     string                               // inline menu title
	menuOptions   []string                             // inline menu options
	menuCursor    int                                  // inline menu cursor
	menuCallback  func(int) ActionResult               // inline menu callback
	actionBar     ActionBar

	conflictTree *conflictTreeState // non-nil when viewing conflict tree
}

type (
	tickMsg         time.Time
	conflictTreeMsg struct{ state *conflictTreeState }
)

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *TUIModel) refreshAgents() {
	indices := core.ListAgents(m.repoRoot)
	m.srcBranch = core.SourceBranch(m.repoRoot)
	m.agents = core.ReadAndEnrichAgents(m.repoRoot, indices, m.srcBranch)
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

	case addToastMsg, toastExpiredMsg:
		var cmd tea.Cmd
		m.toaster, cmd = m.toaster.Update(msg)
		return m, cmd

	case conflictTreeMsg:
		if msg.state != nil && len(msg.state.files) > 0 {
			m.conflictTree = msg.state
			m.mode = modeConflictTree
		} else {
			return m, AddToast("No changed files found", ToastWarning)
		}
		return m, nil

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
		case modeConflictTree:
			return m.updateConflictTree(msg)
		default:
			return m.updateNormal(msg)
		}
	}

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
		m.menuCursor = max(0, m.menuCursor-1)
	case "down", "j":
		m.menuCursor = min(len(m.menuOptions)-1, m.menuCursor+1)
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
	nAgents := len(m.agents)
	cols := m.cols()

	var sel *core.Agent
	if m.cursor >= 0 && m.cursor < nAgents {
		sel = m.agents[m.cursor]
	}

	selStatus := sel.Status()

	agentIdx := -1
	if sel != nil {
		agentIdx = sel.Index
	}

	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "h", "left":
		if nAgents == 0 {
			break
		}
		rowStart := (m.cursor / cols) * cols
		rowEnd := min(rowStart+cols-1, nAgents-1)
		if m.cursor == rowStart {
			m.cursor = rowEnd
		} else {
			m.cursor--
		}
	case "l", "right":
		if nAgents == 0 {
			break
		}
		rowStart := (m.cursor / cols) * cols
		rowEnd := min(rowStart+cols-1, nAgents-1)
		if m.cursor == rowEnd {
			m.cursor = rowStart
		} else {
			m.cursor++
		}
	case "k", "up":
		if nAgents == 0 {
			break
		}
		if m.cursor-cols >= 0 {
			m.cursor -= cols
		} else {
			col := m.cursor % cols
			target := ((nAgents-1)/cols)*cols + col
			if target >= nAgents {
				target -= cols
			}
			m.cursor = target
		}
	case "j", "down":
		if nAgents == 0 {
			break
		}
		if m.cursor+cols < nAgents {
			m.cursor += cols
		} else {
			m.cursor = m.cursor % cols
		}
	case "s":
		m.textInput = newSpawnTextInput()
		m.mode = modeSpawning
		return m, textinput.Blink
	case "m":
		if selStatus == core.AgentStatusWip && sel.HasCommits() {
			return m.runInlineAction(ActionMerge, agentIdx)
		}
	case "a":
		if selStatus == core.AgentStatusMerged {
			return m.runInlineAction(ActionAccept, agentIdx)
		}
	case "r":
		if selStatus == core.AgentStatusMerged {
			return m.runInlineAction(ActionReject, agentIdx)
		}
	case "c":
		if selStatus == core.AgentStatusWip {
			return m.runInlineAction(ActionCancel, agentIdx)
		}
	case "=":
		if sel != nil && sel.CommitsAhead > 0 {
			repoRoot := m.repoRoot
			ag := sel
			return m, func() tea.Msg {
				return conflictTreeMsg{state: computeConflictTree(repoRoot, ag)}
			}
		}
	case "b":
		if selStatus == core.AgentStatusConflict {
			return m.runInlineAction(ActionRebase, agentIdx)
		}
	case "enter":
		if sel != nil {
			return m.runInlineAction(ActionFocus, agentIdx)
		}
	}
	return m, nil
}

func (m TUIModel) updateConflictTree(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	halfPage := max(m.height/2, 1)

	switch msg.String() {
	case "j", "down":
		m.conflictTree.scroll++
	case "k", "up":
		if m.conflictTree.scroll > 0 {
			m.conflictTree.scroll--
		}
	case "ctrl+d":
		m.conflictTree.scroll += halfPage
	case "ctrl+u":
		m.conflictTree.scroll -= halfPage
		if m.conflictTree.scroll < 0 {
			m.conflictTree.scroll = 0
		}
	default:
		m.mode = modeNormal
		m.conflictTree = nil
	}
	return m, nil
}

func (m TUIModel) runInlineAction(action TUIAction, agentIdx int) (tea.Model, tea.Cmd) {
	result := TUIResult{Action: action, AgentIdx: agentIdx}
	handler := m.actionHandler
	return m, func() tea.Msg {
		return actionResultMsg{result: handler(result, "")}
	}
}

func newSpawnTextInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "e.g. fix auth bug"
	ti.Focus()
	ti.CharLimit = 80
	ti.Width = 50
	ti.Validate = validateAgentName
	ti.PromptStyle = styles.AccentStyle
	ti.TextStyle = styles.EnabledStyle
	return ti
}

func renderMenu(title string, options []string, cursor int, hint string) string {
	titleLine := styles.TitleStyle.Render(title)

	var items []string
	for i, opt := range options {
		cur := lipgloss.NewStyle().Width(2).Render("")
		style := styles.DefaultStyle
		if i == cursor {
			cur = styles.AccentStyle.Render("▸ ")
			style = styles.EnabledStyle
		}
		items = append(items, lipgloss.JoinHorizontal(lipgloss.Top, cur, style.Render(opt)))
	}
	optionsList := lipgloss.JoinVertical(lipgloss.Left, items...)

	hintLine := styles.HintStyle.Render(hint)

	return lipgloss.JoinVertical(lipgloss.Left, titleLine, "", optionsList, "", hintLine, "")
}

func (m TUIModel) View() string {
	if m.quitting {
		return ""
	}

	if m.mode == modeConflictTree && m.conflictTree != nil {
		return viewConflictTree(m.conflictTree, m.width, m.height)
	}

	srcBranch := m.srcBranch

	outerPad := lipgloss.NewStyle().PaddingLeft(2)

	title := styles.TitleStyle.Render("\uebc8 augmux session")

	repoLine := lipgloss.JoinHorizontal(lipgloss.Center,
		styles.HeaderKeyStyle.Render("Repo:"),
		styles.HeaderValStyle.Render(" "+m.repoRoot),
	)
	branchLine := lipgloss.JoinHorizontal(lipgloss.Center,
		styles.HeaderKeyStyle.Render("Source:"),
		lipgloss.NewStyle().Render(" "),
		styles.RenderBranch(srcBranch),
	)
	horizontalHeader := repoLine + "   " + branchLine
	var header string
	if lipgloss.Width(horizontalHeader) <= m.width-2 {
		header = horizontalHeader
	} else {
		header = lipgloss.JoinVertical(lipgloss.Left, repoLine, branchLine)
	}
	sections := []string{title, header, ""}

	if m.mode == modeAgentSetup {
		menu := renderMenu(m.menuTitle, m.menuOptions, m.menuCursor,
			"j/k navigate · enter select · esc quit")
		sections = append(sections, menu)
		return outerPad.Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
	}

	if len(m.agents) == 0 {
		sections = append(sections, styles.HintStyle.Render("No agents running."))
	} else {
		spinnerFrame := m.spinner.View()
		var cards []string
		for i, a := range m.agents {
			cards = append(cards, RunAgentCard(a, spinnerFrame, i == m.cursor))
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
		hint := styles.HintStyle.Render("enter confirm · esc cancel")
		sections = append(sections, spawnTitle, "", inputLine, "", hint, "")
		return outerPad.Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
	}

	if m.mode == modeMenu {
		menu := renderMenu(m.menuTitle, m.menuOptions, m.menuCursor,
			"j/k navigate · enter select · esc cancel")
		sections = append(sections, menu)
		return outerPad.Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
	}

	var selected *core.Agent
	if m.cursor >= 0 && m.cursor < len(m.agents) {
		selected = m.agents[m.cursor]
	}
	bar := m.actionBar.View(selected.Status(), selected.HasCommits(), m.width-2)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	contentHeight := lipgloss.Height(content)
	barHeight := lipgloss.Height(bar) + 1
	gap := m.height - contentHeight - barHeight
	if gap > 0 {
		content = content + strings.Repeat("\n", gap)
	}

	return outerPad.Render(content + "\n" + bar)
}

// RunInteractiveTUI runs the interactive TUI dashboard.
func RunInteractiveTUI(repoRoot string) {
	s := spinner.New(
		spinner.WithSpinner(spinner.Spinner{
			Frames: []string{"◐", "◓", "◑", "◒"},
			FPS:    time.Second / 30,
		}),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(styles.ColorYellow)),
	)
	m := TUIModel{
		repoRoot:      repoRoot,
		width:         100,
		spinner:       s,
		toaster:       newToaster(),
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
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}
