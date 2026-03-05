package ops

import (
	"fmt"
	"io"
	"strings"

	"github.com/xemotrix/augmux/internal/components"
	"github.com/xemotrix/augmux/internal/core"
)

type conflictResult int

const (
	conflictContinue conflictResult = iota
	conflictAbort
)

func handleConflict(w io.Writer, repoRoot, task, mergeMsg string, agentIdx int) conflictResult {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  ✗ CONFLICT detected while merging '%s'\n", task)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "  Conflicting files:")
	files := core.GitMust(repoRoot, "diff", "--name-only", "--diff-filter=U")
	for f := range strings.SplitSeq(files, "\n") {
		if f != "" {
			fmt.Fprintf(w, "    %s\n", f)
		}
	}
	fmt.Fprintln(w)

	choice := components.RunMenu("How do you want to resolve?", []string{
		"Continue — leave conflicts in working tree, resolve manually",
		"Abort — discard merge and reset",
	})

	switch choice {
	case 0:
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  Conflicts left in working tree. To resolve:")
		fmt.Fprintf(w, "    1. Edit the conflicting files in: %s\n", repoRoot)
		fmt.Fprintln(w, "    2. git add <resolved files>")
		fmt.Fprintln(w, "    3. git commit")
		fmt.Fprintf(w, "    4. augmux merge %d\n", agentIdx)
		return conflictContinue

	default:
		core.GitMust(repoRoot, "reset", "--hard", "HEAD")
		fmt.Fprintln(w, "  ⊘ Merge aborted.")
		return conflictAbort
	}
}

// ResolveConflict handles a conflict resolution choice from the TUI.
// choice: 0 = continue (manual), anything else = abort.
func ResolveConflict(w io.Writer, info *MergeConflictErr, choice int) {
	td := core.TaskDir(info.RepoRoot, info.AgentIdx)

	switch choice {
	case 0:
		// Continue — leave conflicts for manual resolution
		fmt.Fprintln(w, "  Conflicts left in working tree. To resolve:")
		fmt.Fprintf(w, "    1. Edit the conflicting files in: %s\n", info.RepoRoot)
		fmt.Fprintln(w, "    2. git add <resolved files>")
		fmt.Fprintln(w, "    3. git commit")
		fmt.Fprintf(w, "    4. augmux merge %d\n", info.AgentIdx)
		core.WriteFileContent(td+"/resolving", info.MergeMsg)

	default:
		// Abort (including -1 = cancelled)
		core.GitMust(info.RepoRoot, "reset", "--hard", "HEAD")
		fmt.Fprintf(w, "  ⊘ Merge aborted. Agent %d preserved for retry.\n", info.AgentIdx)
	}
}
