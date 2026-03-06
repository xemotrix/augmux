# augmux Git Workflow Guide

augmux orchestrates multiple AI coding agents in parallel. Each agent gets an **isolated git branch and worktree**, so agents can work on different tasks simultaneously without interfering with each other. A TUI dashboard lets you spawn, monitor, merge, and tear down agents.

## Quick Start

1. Navigate to your git repository.
2. Make sure you're inside a **tmux session** and on the branch you want agents to work from (the "source branch").
3. Run `augmux` to open the TUI.
4. Press `s` to spawn an agent — give it a task name like "fix auth bug".
5. The agent starts working in its own worktree. Monitor its progress from the dashboard.
6. When it's done, press `m` to merge, review the result, run tests, etc. Then `a` to accept or `r` to reject.

## How It Works

### Git Isolation Model

Every agent operates in its own **git worktree** on its own **branch**. This is the core of augmux's parallelism model:

- The **source branch** is whichever branch you were on when you first spawned an agent (e.g. `main` or `feature/xyz`). All agent work merges back here.
- Each agent gets a branch named `augmux/<task-name>-<index>` forked from the source branch.
- Each agent gets a worktree at `.augmux-worktrees/<task-name>-<index>/` — a full, independent working directory.

Agents cannot see or affect each other's files. They also cannot affect your source branch until you explicitly merge.

### Directory Layout

During a session, augmux creates two directories at the repo root:

```
.augmux-state/              # Session metadata (plain text files)
  source_branch             # Name of the branch agents merge back into
  repo_root                 # Absolute path to the main repo
  task-1/                   # State for agent 1
    description             # Task name
    branch                  # e.g. "augmux/fix-auth-bug-1"
    worktree                # Path to the agent's worktree
    window                  # tmux window name
    rules.md                # Instructions injected into the agent
    merge_commit            # (after merge) SHA of the squash commit
  task-2/
    ...

.augmux-worktrees/          # Agent working directories
  fix-auth-bug-1/           # Full working tree for agent 1
  add-tests-2/              # Full working tree for agent 2
```

Both directories are cleaned up automatically when all agents are removed.

## Agent Lifecycle

### 1. Spawn

**TUI key:** `s`

When you spawn an agent:

1. augmux records the current branch as the source branch (first spawn only).
2. Creates a new branch off the source: `git branch augmux/<name>-<idx> <source>`.
3. Creates a worktree: `git worktree add <path> <branch>`.
4. Opens a tmux window at the worktree and starts the agent CLI.
5. Injects rules telling the agent its branch, worktree path, and source branch.

The agent is instructed to commit its changes but never push to a remote.

### 2. Work

The agent makes changes and commits them on its own branch. You can monitor progress from the TUI dashboard, which refreshes every 500ms and shows:

- **Activity status** — "working" or "idle" (based on whether the tmux pane content is changing).
- **Commits ahead** — how many commits the agent has made beyond the source branch.
- **Uncommitted changes** — files modified but not yet committed.
- **Conflict indicator** — whether merging would produce conflicts (detected in-memory via `git merge-tree`, without touching any working tree).

### 3. Merge

**TUI key:** `m`

Merging squash-merges the agent's work onto the source branch:

1. If the agent has uncommitted changes, they are auto-committed first.
2. augmux checks out the source branch in the main repo.
3. Runs `git merge --squash <agent-branch>` — this collapses all of the agent's commits into a single staged changeset.
4. Commits with the message `augmux: <agent's last commit message>`.
5. Records the resulting commit SHA.

If there are conflicts, the merge is aborted cleanly (`git reset --hard HEAD`) and you'll see an error. Use the rebase action to have the agent resolve conflicts first.

**Important:** The main repo's working tree must be clean before merging. Commit or stash any local changes first.

### 4. Review — Accept or Reject

After merging, the agent enters a "merged" state. You can review the diff and decide:

#### Accept

**TUI key:** `a`

- Keeps the squash-merge commit on the source branch.
- Tears down the agent (removes worktree, branch, tmux window, state files).

#### Reject

**TUI key:** `r`

- Runs `git reset --hard HEAD~1` to remove the squash-merge commit from the source branch.
- The agent's branch and worktree remain intact — the agent can continue working.
- A safety check ensures HEAD still matches the recorded merge commit before resetting, preventing data loss if other work has landed since.

### 5. Cancel

**TUI key:** `c`

Discards an agent entirely without merging:

- If the agent was already merged and HEAD matches that merge commit, the merge commit is reset first.
- The worktree, branch, tmux window, and state files are all removed.
- If the agent has commits or uncommitted changes, you'll get a confirmation prompt.

### 6. Rebase

**TUI key:** `b` (shown when conflicts are detected)

When the source branch moves forward (e.g. you accepted another agent's merge), existing agents may fall behind and develop conflicts. The rebase action sends a command into the agent's tmux pane instructing it to:

1. Auto-commit any uncommitted work.
2. Run `git rebase <source_branch>`.
3. Resolve any conflicts (preserving intent from both sides).
4. Continue the rebase until complete.

This brings the agent's branch up to date with the source branch.

### 7. Focus

**TUI key:** `Enter` or `o`

Switches your tmux focus to the agent's window so you can interact with it directly.

## Nuke — Emergency Cleanup

```
augmux nuke
```

Destroys the entire session:

- Kills all agent tmux windows.
- Removes all worktrees and branches.
- Deletes `.augmux-state/` and `.augmux-worktrees/`.
- Does **not** undo any merge commits already on the source branch.

## TUI Keyboard Reference

| Key | Action | Available When |
|---|---|---|
| `s` | Spawn new agent | Always |
| `m` | Merge agent | Agent has commits, no conflicts |
| `b` | Rebase agent | Agent has conflicts |
| `a` | Accept merge | Agent is merged |
| `r` | Reject merge | Agent is merged |
| `c` | Cancel agent | Agent is not merged |
| `Enter` / `o` | Focus agent window | Agent selected |
| `e` | Show diff details | Agent has commits |
| `h`/`j`/`k`/`l` or arrows | Navigate agents | Always |
| `q` | Quit TUI | Always |

## Design Notes

- **Squash merge, not regular merge.** Each agent's work becomes a single commit on the source branch regardless of how many intermediate commits the agent made. This keeps history clean.
- **No remote interaction.** augmux never pushes or pulls. Agents are instructed not to push. Everything stays local.
- **In-memory conflict detection.** `git merge-tree --write-tree` checks for conflicts without touching any working tree or index, making it safe to run on every refresh cycle.
- **Filesystem-based state.** All session state lives in plain text files under `.augmux-state/`. There is no database.
- **Agents are sandboxed.** Each agent can only modify files in its own worktree. The source branch is only modified when you explicitly merge from the TUI.
