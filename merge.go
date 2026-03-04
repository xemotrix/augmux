package main

import (
	"fmt"
	"os"
	"strings"
)

func mergeOne(repoRoot string, idx int) error {
	repoRoot = mustAbs(repoRoot)
	td := taskDir(repoRoot, idx)

	agent, err := readAgent(repoRoot, idx)
	if err != nil {
		return err
	}

	// Already merged?
	if agent.MergeCommit != "" {
		fmt.Printf("Agent %d is already merged (commit %s).\n", idx, agent.MergeCommit)
		fmt.Printf("Use 'augmux accept %d' to finalize, or 'augmux reject %d' to undo and retry.\n", idx, idx)
		return fmt.Errorf("already merged")
	}

	srcBranch := sourceBranch(repoRoot)

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Agent %d: %s\n", idx, agent.Description)
	fmt.Printf("Branch:   %s\n", agent.Branch)

	// Check resolving state (user came back after fixing conflicts)
	if agent.Resolving != "" {
		unmerged := gitMust(repoRoot, "diff", "--name-only", "--diff-filter=U")
		porcelain := gitMust(repoRoot, "status", "--porcelain")
		if unmerged != "" || porcelain != "" {
			fmt.Println()
			fmt.Printf("  ⚠ Conflicts still unresolved or uncommitted changes in: %s\n", repoRoot)
			fmt.Println("  Resolve all conflicts, 'git add', and 'git commit', then run:")
			fmt.Printf("    augmux merge %d\n", idx)
			return fmt.Errorf("still resolving")
		}
		sha := gitMust(repoRoot, "rev-parse", "HEAD")
		writeFileContent(td+"/merge_commit", sha)
		os.Remove(td + "/resolving")
		fmt.Println("  ✓ Conflicts resolved and merged.")
		fmt.Println()
		printAcceptRejectHint(idx)
		return nil
	}

	// Check uncommitted changes in worktree
	if isDir(agent.Worktree) {
		porcelain := gitMust(agent.Worktree, "status", "--porcelain")
		if porcelain != "" {
			fmt.Println()
			fmt.Println("  ⚠ Uncommitted changes in worktree:")
			for _, line := range strings.Split(gitMust(agent.Worktree, "status", "--short"), "\n") {
				fmt.Printf("    %s\n", line)
			}
			fmt.Println()
			choice := runMenu("Uncommitted changes — cannot merge", []string{
				"Commit them now with a default message",
				"Abort merge for this agent",
			})
			if choice == 0 {
				gitMust(agent.Worktree, "add", "-A")
				gitMust(agent.Worktree, "commit", "-m", "augmux: auto-commit uncommitted changes")
				fmt.Println("  ✓ Changes committed.")
			} else {
				fmt.Printf("  ⊘ Merge aborted for agent %d. Worktree preserved.\n", idx)
				return fmt.Errorf("aborted")
			}
			fmt.Println()
		}
	}

	// Checkout source branch
	git(repoRoot, "checkout", srcBranch)

	// Count ahead
	ahead := gitMust(repoRoot, "rev-list", "--count", srcBranch+".."+agent.Branch)
	if ahead == "0" || ahead == "" {
		fmt.Println("  ⊘ No changes to merge.")
		teardownOne(repoRoot, idx)
		return nil
	}

	fmt.Printf("  %s commit(s) to merge.\n", ahead)
	fmt.Println()
	fmt.Println("  Changes summary:")
	for _, line := range strings.Split(gitMust(repoRoot, "diff", "--stat", srcBranch+".."+agent.Branch), "\n") {
		fmt.Printf("    %s\n", line)
	}
	fmt.Println()

	// Merge message
	lastMsg := gitMust(repoRoot, "log", "-1", "--format=%s", agent.Branch)
	mergeMsg := "augmux: " + lastMsg

	// Try squash merge + commit
	_, mergeErr := git(repoRoot, "merge", "--squash", agent.Branch)
	if mergeErr == nil {
		if _, commitErr := git(repoRoot, "commit", "-m", mergeMsg); commitErr == nil {
			sha := gitMust(repoRoot, "rev-parse", "HEAD")
			writeFileContent(td+"/merge_commit", sha)
			fmt.Println("  ✓ Merged successfully (squashed).")
			fmt.Println()
			printAcceptRejectHint(idx)
			return nil
		}
	}

	// Conflict path
	result := handleConflict(repoRoot, agent.Description, mergeMsg, idx)
	switch result {
	case conflictContinue:
		writeFileContent(td+"/resolving", mergeMsg)
		return fmt.Errorf("resolving conflicts")
	case conflictAutoFixed:
		sha := gitMust(repoRoot, "rev-parse", "HEAD")
		writeFileContent(td+"/merge_commit", sha)
		fmt.Println()
		printAcceptRejectHint(idx)
		return nil
	default:
		fmt.Printf("  Agent %d preserved for retry.\n", idx)
		return fmt.Errorf("aborted")
	}
}

func printAcceptRejectHint(idx int) {
	fmt.Println("  Review the changes, then:")
	fmt.Printf("    augmux accept %d   — finalize and clean up agent\n", idx)
	fmt.Printf("    augmux reject %d   — undo merge, fix in agent, re-merge\n", idx)
}
