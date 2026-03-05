package ops

import (
	"fmt"
	"io"
	"os"

	"github.com/xemotrix/augmux/internal/core"
)

func teardownOne(repoRoot string, idx int) {
	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		return
	}

	// Kill tmux window (use stored window name)
	core.TmuxRun("kill-window", "-t", fmt.Sprintf("=%s", ag.Window))

	// Remove worktree
	if core.IsDir(ag.Worktree) {
		core.GitMust(repoRoot, "worktree", "remove", "--force", ag.Worktree)
	}
	core.GitMust(repoRoot, "worktree", "prune")

	// Delete branch
	core.GitMust(repoRoot, "branch", "-D", ag.Branch)

	// Remove state
	os.RemoveAll(core.TaskDir(repoRoot, idx))

	maybeCleanupSession(repoRoot)
}

// CancelOne removes a single agent, discarding all its changes.
func CancelOne(w io.Writer, repoRoot string, idx int) {
	repoRoot = core.MustAbs(repoRoot)
	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		core.Fatal("Agent %d not found.", idx)
	}

	if ag.MergeCommit != "" {
		currentHead := core.GitMust(repoRoot, "rev-parse", "HEAD")
		if currentHead == ag.MergeCommit {
			core.GitMust(repoRoot, "reset", "--hard", "HEAD~1")
		}
	}

	if ag.Resolving != "" {
		core.GitMust(repoRoot, "reset", "--hard", "HEAD")
	}

	teardownOne(repoRoot, idx)
}

func TeardownAll(w io.Writer, repoRoot string) {
	repoRoot = core.MustAbs(repoRoot)
	sd := core.StateDir(repoRoot)
	if !core.IsDir(sd) {
		core.Fatal("No augmux session found.")
	}

	for _, idx := range core.ListAgents(repoRoot) {
		ag, err := core.ReadAgent(repoRoot, idx)
		if err != nil {
			continue
		}
		core.TmuxRun("kill-window", "-t", fmt.Sprintf("=%s", ag.Window))
		if core.IsDir(ag.Worktree) {
			core.GitMust(repoRoot, "worktree", "remove", "--force", ag.Worktree)
		}
		core.GitMust(repoRoot, "branch", "-D", ag.Branch)
	}

	core.GitMust(repoRoot, "worktree", "prune")
	os.Remove(core.WorktreeBase(repoRoot))
	os.RemoveAll(sd)
}

func maybeCleanupSession(repoRoot string) {
	agents := core.ListAgents(repoRoot)
	if len(agents) == 0 {
		os.Remove(core.WorktreeBase(repoRoot))
		os.RemoveAll(core.StateDir(repoRoot))
	}
}
