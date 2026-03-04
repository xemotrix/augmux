package ops

import (
	"fmt"
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

func handleConflict(repoRoot, task, mergeMsg string, agentIdx int) conflictResult {
	ag := agent.ActiveAgent()
	fmt.Println()
	fmt.Printf("  ✗ CONFLICT detected while merging '%s'\n", task)
	fmt.Println()

	fmt.Println("  Conflicting files:")
	files := core.GitMust(repoRoot, "diff", "--name-only", "--diff-filter=U")
	for _, f := range strings.Split(files, "\n") {
		if f != "" {
			fmt.Printf("    %s\n", f)
		}
	}
	fmt.Println()

	choice := tui.RunMenu("How do you want to resolve?", []string{
		"Continue — leave conflicts in working tree, resolve manually",
		fmt.Sprintf("Auto-fix with %s — AI resolves conflicts keeping both sides", ag.Label()),
		"Abort — discard merge and reset",
	})

	switch choice {
	case 0:
		fmt.Println()
		fmt.Println("  Conflicts left in working tree. To resolve:")
		fmt.Printf("    1. Edit the conflicting files in: %s\n", repoRoot)
		fmt.Println("    2. git add <resolved files>")
		fmt.Println("    3. git commit")
		fmt.Printf("    4. augmux merge %d\n", agentIdx)
		return conflictContinue

	case 1:
		fmt.Println()
		fmt.Printf("  🤖 Running %s to auto-fix conflicts...\n", ag.Label())
		fmt.Println()

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
				fmt.Println()
				fmt.Printf("  ⚠ %s finished but some conflicts remain unresolved.\n", ag.Label())
				fmt.Println("  Falling back to manual resolution.")
				// Unstage so the user sees the unmerged state.
				core.GitMust(repoRoot, "reset")
				printManualInstructions(repoRoot, agentIdx)
				return conflictContinue
			}
			core.Git(repoRoot, "commit", "-m", mergeMsg)
			fmt.Println()
			fmt.Printf("  ✓ %s resolved all conflicts and committed.\n", ag.Label())
			return conflictAutoFixed
		}

		fmt.Println()
		fmt.Printf("  ⚠ %s failed. Falling back to manual resolution.\n", ag.Label())
		printManualInstructions(repoRoot, agentIdx)
		return conflictContinue

	default:
		core.GitMust(repoRoot, "reset", "--hard", "HEAD")
		fmt.Println("  ⊘ Merge aborted.")
		return conflictAbort
	}
}

func printManualInstructions(repoRoot string, agentIdx int) {
	fmt.Println()
	fmt.Println("  Conflicts left in working tree. To resolve:")
	fmt.Printf("    1. Edit the conflicting files in: %s\n", repoRoot)
	fmt.Println("    2. git add <resolved files>")
	fmt.Println("    3. git commit")
	fmt.Printf("    4. augmux merge %d\n", agentIdx)
}
