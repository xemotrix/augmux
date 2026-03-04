package ops

import (
	"fmt"
	"os"
	"strings"

	"github.com/xemotrix/augmux/internal/core"
	"github.com/xemotrix/augmux/internal/tui"
)

func MergeOne(repoRoot string, idx int) error {
	repoRoot = core.MustAbs(repoRoot)
	td := core.TaskDir(repoRoot, idx)

	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		return err
	}

	// Already merged?
	if ag.MergeCommit != "" {
		fmt.Printf("Agent %d is already merged (commit %s).\n", idx, ag.MergeCommit)
		fmt.Printf("Use 'augmux accept %d' to finalize, or 'augmux reject %d' to undo and retry.\n", idx, idx)
		return fmt.Errorf("already merged")
	}

	srcBranch := core.SourceBranch(repoRoot)

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Agent %d: %s\n", idx, ag.Description)
	fmt.Printf("Branch:   %s\n", ag.Branch)

	// Check resolving state (user came back after fixing conflicts)
	if ag.Resolving != "" {
		unmerged := core.GitMust(repoRoot, "diff", "--name-only", "--diff-filter=U")
		porcelain := core.GitMust(repoRoot, "status", "--porcelain")
		if unmerged != "" || porcelain != "" {
			fmt.Println()
			fmt.Printf("  ⚠ Conflicts still unresolved or uncommitted changes in: %s\n", repoRoot)
			fmt.Println("  Resolve all conflicts, 'git add', and 'git commit', then run:")
			fmt.Printf("    augmux merge %d\n", idx)
			return fmt.Errorf("still resolving")
		}
		sha := core.GitMust(repoRoot, "rev-parse", "HEAD")
		core.WriteFileContent(td+"/merge_commit", sha)
		os.Remove(td + "/resolving")
		fmt.Println("  ✓ Conflicts resolved and merged.")
		fmt.Println()
		printAcceptRejectHint(idx)
		return nil
	}

	// Check uncommitted changes in worktree
	if core.IsDir(ag.Worktree) {
		porcelain := core.GitMust(ag.Worktree, "status", "--porcelain")
		if porcelain != "" {
			fmt.Println()
			fmt.Println("  ⚠ Uncommitted changes in worktree:")
			for _, line := range strings.Split(core.GitMust(ag.Worktree, "status", "--short"), "\n") {
				fmt.Printf("    %s\n", line)
			}
			fmt.Println()
			choice := tui.RunMenu("Uncommitted changes — cannot merge", []string{
				"Commit them now with a default message",
				"Abort merge for this agent",
			})
			if choice == 0 {
				core.GitMust(ag.Worktree, "add", "-A")
				core.GitMust(ag.Worktree, "commit", "-m", "augmux: auto-commit uncommitted changes")
				fmt.Println("  ✓ Changes committed.")
			} else {
				fmt.Printf("  ⊘ Merge aborted for agent %d. Worktree preserved.\n", idx)
				return fmt.Errorf("aborted")
			}
			fmt.Println()
		}
	}

	// Checkout source branch
	core.Git(repoRoot, "checkout", srcBranch)

	// Count ahead
	ahead := core.GitMust(repoRoot, "rev-list", "--count", srcBranch+".."+ag.Branch)
	if ahead == "0" || ahead == "" {
		fmt.Println("  ⊘ No changes to merge.")
		teardownOne(repoRoot, idx)
		return nil
	}

	fmt.Printf("  %s commit(s) to merge.\n", ahead)
	fmt.Println()
	fmt.Println("  Changes summary:")
	for _, line := range strings.Split(core.GitMust(repoRoot, "diff", "--stat", srcBranch+".."+ag.Branch), "\n") {
		fmt.Printf("    %s\n", line)
	}
	fmt.Println()

	// Merge message
	lastMsg := core.GitMust(repoRoot, "log", "-1", "--format=%s", ag.Branch)
	mergeMsg := "augmux: " + lastMsg

	// Try squash merge + commit
	_, mergeErr := core.Git(repoRoot, "merge", "--squash", ag.Branch)
	if mergeErr == nil {
		if _, commitErr := core.Git(repoRoot, "commit", "-m", mergeMsg); commitErr == nil {
			sha := core.GitMust(repoRoot, "rev-parse", "HEAD")
			core.WriteFileContent(td+"/merge_commit", sha)
			fmt.Println("  ✓ Merged successfully (squashed).")
			fmt.Println()
			printAcceptRejectHint(idx)
			return nil
		}
	}

	// Conflict path
	result := handleConflict(repoRoot, ag.Description, mergeMsg, idx)
	switch result {
	case conflictContinue:
		core.WriteFileContent(td+"/resolving", mergeMsg)
		return fmt.Errorf("resolving conflicts")
	case conflictAutoFixed:
		sha := core.GitMust(repoRoot, "rev-parse", "HEAD")
		core.WriteFileContent(td+"/merge_commit", sha)
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
