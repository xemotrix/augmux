package main

import (
	"fmt"
	"strings"
)

type conflictResult int

const (
	conflictContinue conflictResult = iota
	conflictAbort
	conflictAutoFixed
)

func handleConflict(repoRoot, task, mergeMsg string, agentIdx int) conflictResult {
	agent := activeAgent()
	fmt.Println()
	fmt.Printf("  ✗ CONFLICT detected while merging '%s'\n", task)
	fmt.Println()

	fmt.Println("  Conflicting files:")
	files := gitMust(repoRoot, "diff", "--name-only", "--diff-filter=U")
	for _, f := range strings.Split(files, "\n") {
		if f != "" {
			fmt.Printf("    %s\n", f)
		}
	}
	fmt.Println()

	choice := runMenu("How do you want to resolve?", []string{
		"Continue — leave conflicts in working tree, resolve manually",
		fmt.Sprintf("Auto-fix with %s — AI resolves conflicts keeping both sides", agent.Label()),
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
		fmt.Printf("  🤖 Running %s to auto-fix conflicts...\n", agent.Label())
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

		err := agent.RunInline(repoRoot, promptText)

		if err == nil {
			remaining := gitMust(repoRoot, "diff", "--name-only", "--diff-filter=U")
			if remaining != "" {
				fmt.Println()
				fmt.Printf("  ⚠ %s finished but some conflicts remain unresolved.\n", agent.Label())
				fmt.Println("  Falling back to manual resolution.")
				printManualInstructions(repoRoot, agentIdx)
				return conflictContinue
			}
			gitMust(repoRoot, "add", "-A")
			git(repoRoot, "commit", "-m", mergeMsg)
			fmt.Println()
			fmt.Printf("  ✓ %s resolved all conflicts and committed.\n", agent.Label())
			return conflictAutoFixed
		}

		fmt.Println()
		fmt.Printf("  ⚠ %s failed. Falling back to manual resolution.\n", agent.Label())
		printManualInstructions(repoRoot, agentIdx)
		return conflictContinue

	default:
		gitMust(repoRoot, "reset", "--hard", "HEAD")
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
