# augmux

Run multiple AI coding agents in parallel using tmux windows and git worktrees.

Each agent gets its own isolated branch and worktree, so they can work on different tasks simultaneously without stepping on each other. When they're done, you merge their work back with a two-phase review flow.

![augmux](docs/augmux.png)

## Prerequisites

- **Go 1.26+** — to compile
- **tmux** — must be running inside a tmux session
- **git** — the project must be a git repository
- **Agent CLI** — one of the supported AI agents:
  - **Auggie** (Augment Code) — `auggie` command
  - **Cursor** (Cursor AI) — `agent` command (install via `curl https://cursor.com/install -fsS | bash`)

## Install

```bash
go install github.com/xemotrix/augmux@latest
```

Or build from source:

```bash
git clone https://github.com/xemotrix/augmux.git
cd augmux
go build -o bin/augmux .
```

## Usage

augmux is driven entirely through an interactive TUI. Run it from within a tmux session inside any git repo:

```bash
augmux
```

On first launch, you'll be prompted to pick which agent CLI to use. The choice is saved to `~/.config/augmux/config.json`.

The only other commands are:

```bash
augmux nuke    # force cleanup — discard all agents without merging
augmux help    # show help
```

## TUI

The dashboard shows a responsive grid of agent cards. Each card displays:

- **Task description** and **activity indicator** (working/idle, detected by monitoring tmux pane output)
- **Branch name**, **commits ahead** of the source branch, and **uncommitted file count**
- **Status badge** — `wip`, `merged`, `resolving`, or `conflicts`
- **Color-coded borders** — green (idle with commits), yellow (working), cyan (merged), red (conflicts/resolving), gray (idle, no commits)

### Keybindings

| Key | Action |
|---|---|
| `h` `j` `k` `l` / arrows | Navigate between agent cards |
| `s` | **Spawn** — enter a task name and launch a new agent |
| `m` | **Merge** — squash-merge the selected agent into the source branch |
| `a` | **Accept** — confirm a merge and clean up the agent |
| `r` | **Reject** — undo the merge commit; agent stays alive for fixes |
| `c` | **Cancel** — discard an agent and all its changes |
| `enter` | **Focus** — switch to the agent's tmux window |
| `q` / `ctrl+c` | Quit the TUI |

Actions are context-sensitive — they only activate when applicable to the selected agent's state (e.g. merge is only available for `wip` agents that have commits).

## How It Works

augmux manages a session tied to your current git branch (the "source branch"). When you spawn an agent:

1. A new git branch is created from the source branch (e.g. `augmux/fix-auth-1`)
2. A git worktree is created in `.augmux-worktrees/` pointing to that branch
3. A rules file with context about the task, branch, and worktree is injected into the agent's session
4. A new tmux window opens in the worktree directory with the agent CLI running

Each agent works in complete isolation. When you merge, augmux squash-merges the agent's branch into the source branch. You then review the result and accept or reject it.

### Agent rules injection

augmux injects a rules file into each agent session so the agent knows its task, branch, and constraints (e.g. "don't push to remote"). For Auggie, this is passed via the `--rules` flag. For Cursor, it's written to `.cursor/rules/augmux.mdc` in the agent's worktree.

### Merge flow

Merging uses a two-phase flow: **merge → review → accept/reject**.

- **Merge** squash-merges the agent's branch. If there are uncommitted changes in the worktree, they are auto-committed first.
- **Accept** confirms the merge and tears down the agent (kills the tmux window, removes the worktree and branch).
- **Reject** undoes the merge commit. The agent stays alive so you can switch to its window, fix the issue, and merge again.

### Conflict resolution

When a merge has conflicts, the TUI presents two options:

1. **Continue** — conflict markers are left in the working tree for manual resolution
2. **Abort** — resets the working tree; the agent is preserved for retry

### State

Session state is stored in `.augmux-state/` at the repo root. Each agent has a `task-N/` directory containing its description, branch name, worktree path, window name, and merge status. Both `.augmux-state/` and `.augmux-worktrees/` are gitignored.

## Typical Workflow

```bash
tmux
cd ~/projects/myapp

# Launch the TUI
augmux

# Press 's' to spawn agents — enter task names like:
#   "add user authentication"
#   "write API tests"
#   "refactor database layer"

# The TUI shows all agents with live activity status
# Navigate with h/j/k/l, press 'enter' to jump to an agent's window

# When an agent finishes, select it and press 'm' to merge
# Review the squashed diff, then:
#   'a' to accept (looks good — clean up)
#   'r' to reject (needs fixes — agent stays alive)

# Press 'q' to quit the TUI when done
```
