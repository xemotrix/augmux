package ops

import (
	"fmt"
	"os"

	"github.com/xemotrix/augmux/internal/core"
)

func teardown(repoRoot string, idx int) {
	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		return
	}

	core.TmuxRun("kill-window", "-t", fmt.Sprintf("=%s", ag.Window))

	if core.IsDir(ag.Worktree) {
		core.GitMust(repoRoot, "worktree", "remove", "--force", ag.Worktree)
	}
	core.GitMust(repoRoot, "worktree", "prune")

	core.ClearConflictCache(ag.Branch)
	core.GitMust(repoRoot, "branch", "-D", ag.Branch)

	os.RemoveAll(core.TaskDir(repoRoot, idx))

	maybeCleanupSession(repoRoot)
}

// TeardownAll removes all agents, worktrees, branches, and session state.
func TeardownAll(repoRoot string) error {
	var err error
	repoRoot, err = core.Abs(repoRoot)
	if err != nil {
		return err
	}
	sd := core.StateDir(repoRoot)
	if !core.IsDir(sd) {
		return fmt.Errorf("no augmux session found")
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
	return nil
}

func maybeCleanupSession(repoRoot string) {
	agents := core.ListAgents(repoRoot)
	if len(agents) == 0 {
		os.Remove(core.WorktreeBase(repoRoot))
		os.RemoveAll(core.StateDir(repoRoot))
	}
}
