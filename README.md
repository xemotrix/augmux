# augmux

Run multiple AI coding agents in parallel using tmux windows and git worktrees.

Each agent gets its own isolated branch and worktree, so they can work on different tasks simultaneously without stepping on each other. When they're done, you merge their work back with a two-phase review flow.

## Prerequisites

- **Go 1.26+** — to compile
- **tmux** — must be running inside a tmux session
- **git** — the project must be a git repository
- **Agent CLI** — one of the supported AI agents:
  - **auggie** (Augment Code) — `auggie` command
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

## How It Works

augmux manages a session tied to your current git branch (the "source branch"). When you spawn agents:

1. A new git branch is created from the source branch (e.g. `augmux/fix-auth-1`)
2. A git worktree is created in `.augmux-worktrees/` pointing to that branch
3. A new tmux window opens in the worktree directory with the configured agent CLI running

Each agent works in complete isolation. When you're ready, you squash-merge their branch back into the source branch, review the result, and accept or reject it.

### State

Session state is stored in `.augmux-state/` at the repo root. Each agent has a `task-N/` directory containing its description, branch name, worktree path, and merge status. Both `.augmux-state/` and `.augmux-worktrees/` are gitignored.

## Commands

### Spawning agents

```bash
# Shows a text input to write a prompt, then spawn an agent with it
augmux spawn

# Spawn one or more agents by name (opens agent CLI with no initial prompt)
augmux spawn "fix auth bug"
augmux spawn "add tests" "update docs" "refactor api"
```

### Checking status

```bash
augmux status            # one-shot grid view
augmux status --watch    # live dashboard (alias for 'augmux tui', also -w)
augmux tui               # interactive dashboard (navigate, act on agents)
```

### Merging

Merge uses a two-phase flow: **merge → review → accept/reject**.

```bash
# Merge a specific agent (squash merge into source branch)
augmux merge 1

# If there are multiple agents, shows an interactive picker
augmux merge

# Merge all unmerged agents
augmux merge --all
```

After merging, the agent's tmux window and worktree are kept alive. You can review the changes, run tests, etc.

### Accepting & Rejecting

```bash
# Happy with the merge — clean up the agent (kills window, removes worktree + branch)
augmux accept 1
augmux accept --all

# If there are multiple agents, shows an interactive picker
augmux accept
augmux reject

# Not happy — undo the merge commit, agent stays alive for fixes
augmux reject 1
```

After rejecting, switch to the agent's window, fix the issue, commit, and run `augmux merge` again.

### Conflict resolution

When a merge has conflicts, you get two options:

1. **Continue** — conflict markers are left in the working tree for manual resolution. After fixing, `git add`, `git commit`, then `augmux merge <id>` again.
2. **Abort** — resets the working tree, agent is preserved for retry.

### Cancelling

```bash
# Remove an agent and discard all its changes (no merge)
augmux cancel 1

# If there are multiple agents, shows an interactive picker
augmux cancel
```

### Nuking

```bash
# Force cleanup — discard everything without merging
augmux nuke
```

A conflict 1!!!

A conflict 2!!! wtf

## Typical Workflow

```bash
# Start a tmux session and cd into your project
tmux
cd ~/projects/myapp

# Spawn some agents
augmux spawn "add user authentication"
augmux spawn "write API tests"
augmux spawn "refactor database layer"

# Switch between windows with Ctrl-b n / Ctrl-b p
# Split panes with Ctrl-b % — new panes open in the agent's worktree

# Check progress
augmux status

# Agent 1 is done — merge it
augmux merge 1
# Review the squashed diff...
augmux accept 1    # looks good
# or
augmux reject 1    # nope, fix and re-merge

```
