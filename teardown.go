package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func teardownOne(repoRoot string, idx int) {
	agent, err := readAgent(repoRoot, idx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return
	}

	// Kill tmux window
	tmuxRun("kill-window", "-t", fmt.Sprintf("=augmux-%d", idx))

	// Remove worktree
	if isDir(agent.Worktree) {
		gitMust(repoRoot, "worktree", "remove", "--force", agent.Worktree)
	}
	gitMust(repoRoot, "worktree", "prune")

	// Delete branch
	gitMust(repoRoot, "branch", "-D", agent.Branch)

	// Remove state
	os.RemoveAll(taskDir(repoRoot, idx))

	fmt.Printf("  ✓ Agent %d torn down (branch: %s)\n", idx, agent.Branch)
	maybeCleanupSession(repoRoot)
}

// cancelOne removes a single agent, discarding all its changes.
// If the agent was already merged, the merge commit is also undone.
func cancelOne(repoRoot string, idx int) {
	repoRoot = mustAbs(repoRoot)
	agent, err := readAgent(repoRoot, idx)
	if err != nil {
		fatal("Agent %d not found.", idx)
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Cancelling agent %d: %s\n", idx, agent.Description)

	// If merged, undo the merge commit first
	if agent.MergeCommit != "" {
		currentHead := gitMust(repoRoot, "rev-parse", "HEAD")
		if currentHead == agent.MergeCommit {
			gitMust(repoRoot, "reset", "--hard", "HEAD~1")
			fmt.Println("  ✓ Merge commit undone.")
		} else {
			fmt.Printf("  ⚠ Merge commit %s is not at HEAD — skipping revert.\n", agent.MergeCommit)
			fmt.Printf("    You may need to manually revert: git revert %s\n", agent.MergeCommit)
		}
	}

	// If resolving conflicts, reset working tree
	if agent.Resolving != "" {
		gitMust(repoRoot, "reset", "--hard", "HEAD")
		fmt.Println("  ✓ Conflict state cleared.")
	}

	teardownOne(repoRoot, idx)
	fmt.Printf("  ✓ Agent %d cancelled and removed.\n", idx)
}

func teardownAll(repoRoot string) {
	repoRoot = mustAbs(repoRoot)
	sd := stateDir(repoRoot)
	if !isDir(sd) {
		fatal("No augmux session found.")
	}

	fmt.Println("Tearing down augmux session...")

	for _, idx := range listAgents(repoRoot) {
		agent, err := readAgent(repoRoot, idx)
		if err != nil {
			continue
		}
		tmuxRun("kill-window", "-t", fmt.Sprintf("=augmux-%d", idx))
		if isDir(agent.Worktree) {
			gitMust(repoRoot, "worktree", "remove", "--force", agent.Worktree)
			fmt.Printf("    ✓ Removed worktree: %s\n", filepath.Base(agent.Worktree))
		}
		gitMust(repoRoot, "branch", "-D", agent.Branch)
		fmt.Printf("    ✓ Deleted branch: %s\n", agent.Branch)
	}

	gitMust(repoRoot, "worktree", "prune")
	os.Remove(worktreeBase(repoRoot))
	os.RemoveAll(sd)
	fmt.Println("  ✓ Full teardown complete.")
}

func maybeCleanupSession(repoRoot string) {
	agents := listAgents(repoRoot)
	if len(agents) == 0 {
		os.Remove(worktreeBase(repoRoot))
		os.RemoveAll(stateDir(repoRoot))
		fmt.Println("  No agents remaining — session cleaned up.")
	}
}
