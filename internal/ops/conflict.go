package ops

import (
	"fmt"
	"io"
	"strings"

	"github.com/xemotrix/augmux/internal/agent"
	"github.com/xemotrix/augmux/internal/core"
	"github.com/xemotrix/augmux/internal/tui"
)

type conflictResult int

const (
	conflictContinue conflictResult = iota
	conflictAbort
	conflictAutoFixed
)

func handleConflict(w io.Writer, repoRoot, task, mergeMsg string, agentIdx int) conflictResult {
	ag := agent.ActiveAgent()
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

	choice := tui.RunMenu("How do you want to resolve?", []string{
		"Continue — leave conflicts in working tree, resolve manually",
		fmt.Sprintf("Auto-fix with %s — AI resolves conflicts keeping both sides", ag.Label()),
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

	case 1:
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  🤖 Running %s to auto-fix conflicts...\n", ag.Label())
		fmt.Fprintln(w)

		promptText := fmt.Sprintf(`There are merge conflicts in the following files:

%s

Each file contains git conflict markers (<<<<<<< HEAD, =======, >>>>>>> ...).

Your task:
1. Open each conflicting file
2. Resolve the conflicts by keeping the functionality from BOTH sides — do not discard changes from either side unless they are truly redundant
3. Remove all conflict markers
4. Make sure the resulting code compiles/works correctly

Do NOT create any new files. Only edit the conflicting files listed above.`, files)

		err := ag.RunInline(repoRoot, promptText)

		if err == nil {
			// Stage all changes first — files remain "unmerged" in git's
			// index until staged, even if the agent already removed the
			// conflict markers from the file contents.
			core.GitMust(repoRoot, "add", "-A")

			// Check for leftover conflict markers in the staged files.
			markers, _ := core.Git(repoRoot, "diff", "--cached", "--name-only", "-S", "<<<<<<<")
			if markers != "" {
				fmt.Fprintln(w)
				fmt.Fprintf(w, "  ⚠ %s finished but some conflicts remain unresolved.\n", ag.Label())
				fmt.Fprintln(w, "  Falling back to manual resolution.")
				// Unstage so the user sees the unmerged state.
				core.GitMust(repoRoot, "reset")
				printManualInstructions(w, repoRoot, agentIdx)
				return conflictContinue
			}
			core.Git(repoRoot, "commit", "-m", mergeMsg)
			fmt.Fprintln(w)
			fmt.Fprintf(w, "  ✓ %s resolved all conflicts and committed.\n", ag.Label())
			return conflictAutoFixed
		}

		fmt.Fprintln(w)
		fmt.Fprintf(w, "  ⚠ %s failed. Falling back to manual resolution.\n", ag.Label())
		printManualInstructions(w, repoRoot, agentIdx)
		return conflictContinue

	default:
		core.GitMust(repoRoot, "reset", "--hard", "HEAD")
		fmt.Fprintln(w, "  ⊘ Merge aborted.")
		return conflictAbort
	}
}

func printManualInstructions(w io.Writer, repoRoot string, agentIdx int) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Conflicts left in working tree. To resolve:")
	fmt.Fprintf(w, "    1. Edit the conflicting files in: %s\n", repoRoot)
	fmt.Fprintln(w, "    2. git add <resolved files>")
	fmt.Fprintln(w, "    3. git commit")
	fmt.Fprintf(w, "    4. augmux merge %d\n", agentIdx)
}
