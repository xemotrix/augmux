package ops

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/xemotrix/augmux/internal/core"
	"github.com/xemotrix/augmux/internal/tui"
)

func MergeOne(w io.Writer, repoRoot string, idx int, interactive bool) error {
	repoRoot = core.MustAbs(repoRoot)
	td := core.TaskDir(repoRoot, idx)

	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		return err
	}

	// Already merged?
	if ag.MergeCommit != "" {
		fmt.Fprintf(w, "Agent %d is already merged (commit %s).\n", idx, ag.MergeCommit)
		fmt.Fprintf(w, "Use 'augmux accept %d' to finalize, or 'augmux reject %d' to undo and retry.\n", idx, idx)
		return fmt.Errorf("already merged")
	}

	srcBranch := core.SourceBranch(repoRoot)

	fmt.Fprintln(w, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Fprintf(w, "Agent %d: %s\n", idx, ag.Description)
	fmt.Fprintf(w, "Branch:   %s\n", ag.Branch)

	// Check resolving state (user came back after fixing conflicts)
	if ag.Resolving != "" {
		unmerged := core.GitMust(repoRoot, "diff", "--name-only", "--diff-filter=U")
		porcelain := core.GitMust(repoRoot, "status", "--porcelain")
		if unmerged != "" || porcelain != "" {
			fmt.Fprintln(w)
			fmt.Fprintf(w, "  ⚠ Conflicts still unresolved or uncommitted changes in: %s\n", repoRoot)
			fmt.Fprintln(w, "  Resolve all conflicts, 'git add', and 'git commit', then run:")
			fmt.Fprintf(w, "    augmux merge %d\n", idx)
			return fmt.Errorf("still resolving")
		}
		sha := core.GitMust(repoRoot, "rev-parse", "HEAD")
		core.WriteFileContent(td+"/merge_commit", sha)
		os.Remove(td + "/resolving")
		fmt.Fprintln(w, "  ✓ Conflicts resolved and merged.")
		fmt.Fprintln(w)
		printAcceptRejectHint(w, idx)
		return nil
	}

	// Check uncommitted changes in worktree
	if core.IsDir(ag.Worktree) {
		porcelain := core.GitMust(ag.Worktree, "status", "--porcelain")
		if porcelain != "" {
			if interactive {
				fmt.Fprintln(w)
				fmt.Fprintln(w, "  ⚠ Uncommitted changes in worktree:")
				for _, line := range strings.Split(core.GitMust(ag.Worktree, "status", "--short"), "\n") {
					fmt.Fprintf(w, "    %s\n", line)
				}
				fmt.Fprintln(w)
				choice := tui.RunMenu("Uncommitted changes — cannot merge", []string{
					"Commit them now with a default message",
					"Abort merge for this agent",
				})
				if choice == 0 {
					core.GitMust(ag.Worktree, "add", "-A")
					core.GitMust(ag.Worktree, "commit", "-m", "augmux: auto-commit uncommitted changes")
					fmt.Fprintln(w, "  ✓ Changes committed.")
				} else {
					fmt.Fprintf(w, "  ⊘ Merge aborted for agent %d. Worktree preserved.\n", idx)
					return fmt.Errorf("aborted")
				}
				fmt.Fprintln(w)
			} else {
				// Non-interactive: auto-commit uncommitted changes
				core.GitMust(ag.Worktree, "add", "-A")
				core.GitMust(ag.Worktree, "commit", "-m", "augmux: auto-commit uncommitted changes")
				fmt.Fprintln(w, "  ✓ Uncommitted changes auto-committed.")
			}
		}
	}

	// Checkout source branch
	core.Git(repoRoot, "checkout", srcBranch)

	// Count ahead
	ahead := core.GitMust(repoRoot, "rev-list", "--count", srcBranch+".."+ag.Branch)
	if ahead == "0" || ahead == "" {
		fmt.Fprintln(w, "  ⊘ No changes to merge.")
		teardownOne(w, repoRoot, idx)
		return nil
	}

	fmt.Fprintf(w, "  %s commit(s) to merge.\n", ahead)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Changes summary:")
	for _, line := range strings.Split(core.GitMust(repoRoot, "diff", "--stat", srcBranch+".."+ag.Branch), "\n") {
		fmt.Fprintf(w, "    %s\n", line)
	}
	fmt.Fprintln(w)

	// Merge message
	lastMsg := core.GitMust(repoRoot, "log", "-1", "--format=%s", ag.Branch)
	mergeMsg := "augmux: " + lastMsg

	// Try squash merge + commit
	_, mergeErr := core.Git(repoRoot, "merge", "--squash", ag.Branch)
	if mergeErr == nil {
		if _, commitErr := core.Git(repoRoot, "commit", "-m", mergeMsg); commitErr == nil {
			sha := core.GitMust(repoRoot, "rev-parse", "HEAD")
			core.WriteFileContent(td+"/merge_commit", sha)
			fmt.Fprintln(w, "  ✓ Merged successfully (squashed).")
			fmt.Fprintln(w)
			printAcceptRejectHint(w, idx)
			return nil
		}
	}

	// Conflict path
	if !interactive {
		// Non-interactive: abort merge on conflict, let user handle via CLI
		core.GitMust(repoRoot, "reset", "--hard", "HEAD")
		fmt.Fprintln(w, "  ✗ Conflicts detected. Use 'augmux merge' from CLI to resolve.")
		return fmt.Errorf("conflicts detected")
	}

	result := handleConflict(w, repoRoot, ag.Description, mergeMsg, idx)
	switch result {
	case conflictContinue:
		core.WriteFileContent(td+"/resolving", mergeMsg)
		return fmt.Errorf("resolving conflicts")
	case conflictAutoFixed:
		sha := core.GitMust(repoRoot, "rev-parse", "HEAD")
		core.WriteFileContent(td+"/merge_commit", sha)
		fmt.Fprintln(w)
		printAcceptRejectHint(w, idx)
		return nil
	default:
		fmt.Fprintf(w, "  Agent %d preserved for retry.\n", idx)
		return fmt.Errorf("aborted")
	}
}

func printAcceptRejectHint(w io.Writer, idx int) {
	fmt.Fprintln(w, "  Review the changes, then:")
	fmt.Fprintf(w, "    augmux accept %d   — finalize and clean up agent\n", idx)
	fmt.Fprintf(w, "    augmux reject %d   — undo merge, fix in agent, re-merge\n", idx)
}
