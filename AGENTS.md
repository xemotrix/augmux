# AGENTS.md

## What is augmux?

augmux is a Go CLI tool that orchestrates multiple AI coding agents in parallel using **tmux windows** and **git worktrees**. Each agent gets an isolated branch and worktree so they can work on different tasks simultaneously. A TUI dashboard lets you spawn, monitor, merge, and tear down agents.

## Project structure

```
main.go                          Entry point — CLI dispatch (tui / nuke / help)
internal/
  tui/
    tui.go                       Main bubbletea Model (TUIModel): Init, Update, View
    agent_card.go                Agent card rendering (border, status badge, activity)
    confirm.go                   Yes/no confirmation dialog (standalone bubbletea program)
  components/
    select_menu.go               Reusable select menu (standalone bubbletea program)
  styles/
    styles.go                    Centralized lipgloss styles and Kanagawa color palette
  core/
    state.go                     Agent state, filesystem-based session management
    exec.go                      Shell helpers (git, tmux, file I/O)
  agent/
    agent.go                     Agent CLI registry, config persistence
    rules.go                     Rules template injected into agent sessions
  ops/
    spawn.go                     Spawn: create branch, worktree, tmux window, start agent
    merge.go                     Squash-merge agent branch into source branch
    accept.go                    Accept/reject merge results
    conflict.go                  Conflict resolution handler
    teardown.go                  Cleanup: kill window, remove worktree/branch/state
```

## TUI architecture — bubbletea, bubbles, and lipgloss

The entire UI is built on the [Charm](https://charm.sh) stack. This is a hard architectural constraint — **all rendering must go through lipgloss, and all interactive behavior must go through bubbletea's Elm-architecture (Model/Update/View)**.

### Core principles

1. **Never do manual terminal rendering.** No raw ANSI escapes, no `fmt.Sprintf`-based layout hacks, no cursor manipulation. All layout, alignment, padding, borders, and coloring must use **lipgloss** primitives (`JoinHorizontal`, `JoinVertical`, `Place`, `NewStyle().Width()`, `NewStyle().Padding()`, etc.).

2. **Use bubbles components for interactive inputs.** The project already uses `bubbles/spinner`, `bubbles/textinput`. When adding new interactive elements (lists, viewports, progress bars, tables, etc.), always reach for the matching [bubbles](https://github.com/charmbracelet/bubbles) component first. Do not reimplement what bubbles already provides.

3. **Keep the Elm architecture clean.** `Update` handles all state transitions and returns `tea.Cmd`s for side effects. `View` is a pure function of the model — it must not trigger side effects or mutate state. Long-running work (git operations, spawning) is dispatched as `tea.Cmd` functions that return `tea.Msg` results.

4. **Centralize styles in `internal/styles/`.** All colors, text styles, and reusable style definitions live in `styles/styles.go`. Components import from there rather than defining ad-hoc lipgloss styles. The palette follows the Kanagawa theme.

5. **Use `lipgloss.JoinHorizontal` / `JoinVertical` for all layout composition.** Cards, rows, grids, headers, footers — everything is composed by joining styled blocks. When you need a grid, build rows of horizontally-joined cards, then vertically-join the rows. Do not calculate character positions manually.

### How the main TUI works

- `TUIModel` is the single top-level bubbletea model, run with `tea.NewProgram(m, tea.WithAltScreen())`.
- It has modes (`modeNormal`, `modeSpawning`, `modeMenu`, `modeAgentSetup`) that control which key handling and view rendering path is active.
- Agent state is refreshed every 500ms via a `tea.Tick` command. Activity detection (working/idle) is based on hashing tmux pane content.
- Actions (merge, spawn, accept, reject, cancel, focus) run as `tea.Cmd` goroutines and return `actionResultMsg` back into the Update loop. Some actions return a `MenuRequest` to show an inline menu.
- The agent card grid is responsive — columns are computed from terminal width divided by card width.

### Standalone bubbletea programs

Some UI elements run as their own `tea.NewProgram`:
- `components.RunSelectMenu` — single-select picker
- `tui.RunConfirm` — yes/no confirmation

These are used outside the main TUI (e.g., `augmux nuke` confirmation, initial agent setup prompt when not in TUI mode).

## Key dependencies

| Package | Purpose |
|---|---|
| `charmbracelet/bubbletea` | Elm-architecture TUI framework — all interactive behavior |
| `charmbracelet/bubbles` | Pre-built components (spinner, text input) |
| `charmbracelet/lipgloss` | Styling and layout — all rendering |

## Things to keep in mind

- **No tests exist yet.** The project is small and fast-moving.
- **State is filesystem-based.** Each agent's state lives in `.augmux-state/task-N/` as plain text files. There is no database.
- **git and tmux are called via shell exec** (`os/exec`), not through Go libraries.
- The TUI auto-refreshes by polling (500ms tick), not by watching file events.
- Agent activity detection works by hashing tmux pane output and checking if it changed recently.
