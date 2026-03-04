package ops

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/xemotrix/augmux/internal/core"
)

func teardownOne(w io.Writer, repoRoot string, idx int) {
	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return
	}

	// Kill tmux window
	core.TmuxRun("kill-window", "-t", fmt.Sprintf("=augmux-%d", idx))

	// Remove worktree
	if core.IsDir(ag.Worktree) {
		core.GitMust(repoRoot, "worktree", "remove", "--force", ag.Worktree)
	}
	core.GitMust(repoRoot, "worktree", "prune")

	// Delete branch
	core.GitMust(repoRoot, "branch", "-D", ag.Branch)

	// Remove state
	os.RemoveAll(core.TaskDir(repoRoot, idx))

	fmt.Fprintf(w, "  ✓ Agent %d torn down (branch: %s)\n", idx, ag.Branch)
	maybeCleanupSession(w, repoRoot)
}

// CancelOne removes a single agent, discarding all its changes.
func CancelOne(w io.Writer, repoRoot string, idx int) {
	repoRoot = core.MustAbs(repoRoot)
	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		core.Fatal("Agent %d not found.", idx)
	}

	fmt.Fprintln(w, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Fprintf(w, "Cancelling agent %d: %s\n", idx, ag.Description)

	if ag.MergeCommit != "" {
		currentHead := core.GitMust(repoRoot, "rev-parse", "HEAD")
		if currentHead == ag.MergeCommit {
			core.GitMust(repoRoot, "reset", "--hard", "HEAD~1")
			fmt.Fprintln(w, "  ✓ Merge commit undone.")
		} else {
			fmt.Fprintf(w, "  ⚠ Merge commit %s is not at HEAD — skipping revert.\n", ag.MergeCommit)
			fmt.Fprintf(w, "    You may need to manually revert: git revert %s\n", ag.MergeCommit)
		}
	}

	if ag.Resolving != "" {
		core.GitMust(repoRoot, "reset", "--hard", "HEAD")
		fmt.Fprintln(w, "  ✓ Conflict state cleared.")
	}

	teardownOne(w, repoRoot, idx)
	fmt.Fprintf(w, "  ✓ Agent %d cancelled and removed.\n", idx)
}

func TeardownAll(w io.Writer, repoRoot string) {
	repoRoot = core.MustAbs(repoRoot)
	sd := core.StateDir(repoRoot)
	if !core.IsDir(sd) {
		core.Fatal("No augmux session found.")
	}

	fmt.Fprintln(w, "Tearing down augmux session...")

	for _, idx := range core.ListAgents(repoRoot) {
		ag, err := core.ReadAgent(repoRoot, idx)
		if err != nil {
			continue
		}
		core.TmuxRun("kill-window", "-t", fmt.Sprintf("=augmux-%d", idx))
		if core.IsDir(ag.Worktree) {
			core.GitMust(repoRoot, "worktree", "remove", "--force", ag.Worktree)
			fmt.Fprintf(w, "    ✓ Removed worktree: %s\n", filepath.Base(ag.Worktree))
		}
		core.GitMust(repoRoot, "branch", "-D", ag.Branch)
		fmt.Fprintf(w, "    ✓ Deleted branch: %s\n", ag.Branch)
	}

	core.GitMust(repoRoot, "worktree", "prune")
	os.Remove(core.WorktreeBase(repoRoot))
	os.RemoveAll(sd)
	fmt.Fprintln(w, "  ✓ Full teardown complete.")
}

func maybeCleanupSession(w io.Writer, repoRoot string) {
	agents := core.ListAgents(repoRoot)
	if len(agents) == 0 {
		os.Remove(core.WorktreeBase(repoRoot))
		os.RemoveAll(core.StateDir(repoRoot))
		fmt.Fprintln(w, "  No agents remaining — session cleaned up.")
	}
}
