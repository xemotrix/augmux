package ops

import (
	"fmt"
	"io"
	"os"

	"github.com/xemotrix/augmux/internal/core"
)

func AcceptOne(w io.Writer, repoRoot string, idx int) error {
	repoRoot = core.MustAbs(repoRoot)
	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		return err
	}
	if ag.MergeCommit == "" {
		return fmt.Errorf("agent %d has not been merged yet. Run 'augmux merge %d' first", idx, idx)
	}
	fmt.Fprintln(w, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Fprintf(w, "Accepting agent %d: %s\n", idx, ag.Description)
	teardownOne(w, repoRoot, idx)
	fmt.Fprintf(w, "  ✓ Agent %d accepted and cleaned up.\n", idx)
	return nil
}

func RejectOne(w io.Writer, repoRoot string, idx int) error {
	repoRoot = core.MustAbs(repoRoot)
	td := core.TaskDir(repoRoot, idx)

	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		return err
	}

	fmt.Fprintln(w, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Fprintf(w, "Rejecting agent %d: %s\n", idx, ag.Description)

	// Case 1: conflicts are being resolved
	if ag.Resolving != "" {
		core.GitMust(repoRoot, "reset", "--hard", "HEAD")
		os.Remove(td + "/resolving")
		fmt.Fprintf(w, "  ✓ Conflict resolution aborted. Agent %d is still active.\n", idx)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  Fix the issue in the agent's worktree, then run:")
		fmt.Fprintf(w, "    augmux merge %d\n", idx)
		return nil
	}

	// Case 2: merge was completed
	if ag.MergeCommit == "" {
		return fmt.Errorf("agent %d has not been merged yet. Nothing to reject", idx)
	}

	currentHead := core.GitMust(repoRoot, "rev-parse", "HEAD")
	if currentHead != ag.MergeCommit {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  ⚠ WARNING: HEAD (%s) is not the merge commit (%s).\n", currentHead, ag.MergeCommit)
		fmt.Fprintln(w, "  Other commits have been made since this merge.")
		fmt.Fprintln(w, "  Cannot safely reset — aborting reject.")
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  You may need to manually revert: git revert %s\n", ag.MergeCommit)
		return fmt.Errorf("cannot safely reset")
	}

	core.GitMust(repoRoot, "reset", "--hard", "HEAD~1")
	os.Remove(td + "/merge_commit")

	fmt.Fprintf(w, "  ✓ Merge undone. Agent %d is still active.\n", idx)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Fix the issue in the agent's worktree, then run:")
	fmt.Fprintf(w, "    augmux merge %d\n", idx)
	return nil
}

func MergeAll(w io.Writer, repoRoot string) {
	repoRoot = core.MustAbs(repoRoot)
	if !core.IsDir(core.StateDir(repoRoot)) {
		core.Fatal("No augmux session found.")
	}

	srcBranch := core.SourceBranch(repoRoot)
	fmt.Fprintf(w, "Merging all agent branches into '%s'...\n\n", srcBranch)

	for _, idx := range core.ListAgents(repoRoot) {
		MergeOne(w, repoRoot, idx, true)
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	merged, unmerged := 0, 0
	for _, idx := range core.ListAgents(repoRoot) {
		ag, err := core.ReadAgent(repoRoot, idx)
		if err != nil {
			continue
		}
		if ag.MergeCommit != "" {
			merged++
		} else {
			unmerged++
		}
	}
	if merged > 0 {
		fmt.Fprintf(w, "%d agent(s) merged — review, then 'augmux accept --all' or 'augmux reject <id>'.\n", merged)
	}
	if unmerged > 0 {
		fmt.Fprintf(w, "%d agent(s) still unmerged.\n", unmerged)
		fmt.Fprintln(w, "Use 'augmux merge <id>' to retry individually, or 'augmux nuke' to discard.")
	}
	if merged == 0 && unmerged == 0 {
		fmt.Fprintln(w, "✓ No agents remaining.")
	}
}

func AcceptAll(w io.Writer, repoRoot string) {
	repoRoot = core.MustAbs(repoRoot)
	if !core.IsDir(core.StateDir(repoRoot)) {
		core.Fatal("No augmux session found.")
	}
	fmt.Fprintln(w, "Accepting all merged agents...")
	fmt.Fprintln(w)
	accepted := 0
	for _, idx := range core.ListAgents(repoRoot) {
		ag, err := core.ReadAgent(repoRoot, idx)
		if err != nil || ag.MergeCommit == "" {
			continue
		}
		AcceptOne(w, repoRoot, idx)
		accepted++
		fmt.Fprintln(w)
	}
	if accepted == 0 {
		fmt.Fprintln(w, "No merged agents to accept.")
	} else {
		fmt.Fprintf(w, "✓ %d agent(s) accepted and cleaned up.\n", accepted)
	}
}

func FinishAll(w io.Writer, repoRoot string) {
	repoRoot = core.MustAbs(repoRoot)
	MergeAll(w, repoRoot)
	fmt.Fprintln(w)
	AcceptAll(w, repoRoot)
}
