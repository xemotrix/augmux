package ops

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/xemotrix/augmux/internal/core"
)

func teardownOne(repoRoot string, idx int) {
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

	fmt.Printf("  ✓ Agent %d torn down (branch: %s)\n", idx, ag.Branch)
	maybeCleanupSession(repoRoot)
}

// CancelOne removes a single agent, discarding all its changes.
func CancelOne(repoRoot string, idx int) {
	repoRoot = core.MustAbs(repoRoot)
	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		core.Fatal("Agent %d not found.", idx)
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Cancelling agent %d: %s\n", idx, ag.Description)

	if ag.MergeCommit != "" {
		currentHead := core.GitMust(repoRoot, "rev-parse", "HEAD")
		if currentHead == ag.MergeCommit {
			core.GitMust(repoRoot, "reset", "--hard", "HEAD~1")
			fmt.Println("  ✓ Merge commit undone.")
		} else {
			fmt.Printf("  ⚠ Merge commit %s is not at HEAD — skipping revert.\n", ag.MergeCommit)
			fmt.Printf("    You may need to manually revert: git revert %s\n", ag.MergeCommit)
		}
	}

	if ag.Resolving != "" {
		core.GitMust(repoRoot, "reset", "--hard", "HEAD")
		fmt.Println("  ✓ Conflict state cleared.")
	}

	teardownOne(repoRoot, idx)
	fmt.Printf("  ✓ Agent %d cancelled and removed.\n", idx)
}

func TeardownAll(repoRoot string) {
	repoRoot = core.MustAbs(repoRoot)
	sd := core.StateDir(repoRoot)
	if !core.IsDir(sd) {
		core.Fatal("No augmux session found.")
	}

	fmt.Println("Tearing down augmux session...")

	for _, idx := range core.ListAgents(repoRoot) {
		ag, err := core.ReadAgent(repoRoot, idx)
		if err != nil {
			continue
		}
		core.TmuxRun("kill-window", "-t", fmt.Sprintf("=augmux-%d", idx))
		if core.IsDir(ag.Worktree) {
			core.GitMust(repoRoot, "worktree", "remove", "--force", ag.Worktree)
			fmt.Printf("    ✓ Removed worktree: %s\n", filepath.Base(ag.Worktree))
		}
		core.GitMust(repoRoot, "branch", "-D", ag.Branch)
		fmt.Printf("    ✓ Deleted branch: %s\n", ag.Branch)
	}

	core.GitMust(repoRoot, "worktree", "prune")
	os.Remove(core.WorktreeBase(repoRoot))
	os.RemoveAll(sd)
	fmt.Println("  ✓ Full teardown complete.")
}

func maybeCleanupSession(repoRoot string) {
	agents := core.ListAgents(repoRoot)
	if len(agents) == 0 {
		os.Remove(core.WorktreeBase(repoRoot))
		os.RemoveAll(core.StateDir(repoRoot))
		fmt.Println("  No agents remaining — session cleaned up.")
	}
}
