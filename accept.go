package main

import (
	"fmt"
	"os"
)

func acceptOne(repoRoot string, idx int) error {
	repoRoot = mustAbs(repoRoot)
	agent, err := readAgent(repoRoot, idx)
	if err != nil {
		return err
	}
	if agent.MergeCommit == "" {
		return fmt.Errorf("agent %d has not been merged yet. Run 'augmux merge %d' first", idx, idx)
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Accepting agent %d: %s\n", idx, agent.Description)
	teardownOne(repoRoot, idx)
	fmt.Printf("  ✓ Agent %d accepted and cleaned up.\n", idx)
	return nil
}

func rejectOne(repoRoot string, idx int) error {
	repoRoot = mustAbs(repoRoot)
	td := taskDir(repoRoot, idx)

	agent, err := readAgent(repoRoot, idx)
	if err != nil {
		return err
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Rejecting agent %d: %s\n", idx, agent.Description)

	// Case 1: conflicts are being resolved
	if agent.Resolving != "" {
		gitMust(repoRoot, "reset", "--hard", "HEAD")
		os.Remove(td + "/resolving")
		fmt.Printf("  ✓ Conflict resolution aborted. Agent %d is still active.\n", idx)
		fmt.Println()
		fmt.Println("  Fix the issue in the agent's worktree, then run:")
		fmt.Printf("    augmux merge %d\n", idx)
		return nil
	}

	// Case 2: merge was completed
	if agent.MergeCommit == "" {
		return fmt.Errorf("agent %d has not been merged yet. Nothing to reject", idx)
	}

	currentHead := gitMust(repoRoot, "rev-parse", "HEAD")
	if currentHead != agent.MergeCommit {
		fmt.Println()
		fmt.Printf("  ⚠ WARNING: HEAD (%s) is not the merge commit (%s).\n", currentHead, agent.MergeCommit)
		fmt.Println("  Other commits have been made since this merge.")
		fmt.Println("  Cannot safely reset — aborting reject.")
		fmt.Println()
		fmt.Printf("  You may need to manually revert: git revert %s\n", agent.MergeCommit)
		return fmt.Errorf("cannot safely reset")
	}

	gitMust(repoRoot, "reset", "--hard", "HEAD~1")
	os.Remove(td + "/merge_commit")

	fmt.Printf("  ✓ Merge undone. Agent %d is still active.\n", idx)
	fmt.Println()
	fmt.Println("  Fix the issue in the agent's worktree, then run:")
	fmt.Printf("    augmux merge %d\n", idx)
	return nil
}

func mergeAll(repoRoot string) {
	repoRoot = mustAbs(repoRoot)
	if !isDir(stateDir(repoRoot)) {
		fatal("No augmux session found.")
	}

	srcBranch := sourceBranch(repoRoot)
	fmt.Printf("Merging all agent branches into '%s'...\n\n", srcBranch)

	for _, idx := range listAgents(repoRoot) {
		mergeOne(repoRoot, idx)
		fmt.Println()
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	merged, unmerged := 0, 0
	for _, idx := range listAgents(repoRoot) {
		agent, err := readAgent(repoRoot, idx)
		if err != nil {
			continue
		}
		if agent.MergeCommit != "" {
			merged++
		} else {
			unmerged++
		}
	}
	if merged > 0 {
		fmt.Printf("%d agent(s) merged — review, then 'augmux accept --all' or 'augmux reject <id>'.\n", merged)
	}
	if unmerged > 0 {
		fmt.Printf("%d agent(s) still unmerged.\n", unmerged)
		fmt.Println("Use 'augmux merge <id>' to retry individually, or 'augmux clean' to discard.")
	}
	if merged == 0 && unmerged == 0 {
		fmt.Println("✓ No agents remaining.")
	}
}

func acceptAll(repoRoot string) {
	repoRoot = mustAbs(repoRoot)
	if !isDir(stateDir(repoRoot)) {
		fatal("No augmux session found.")
	}
	fmt.Println("Accepting all merged agents...")
	fmt.Println()
	accepted := 0
	for _, idx := range listAgents(repoRoot) {
		agent, err := readAgent(repoRoot, idx)
		if err != nil || agent.MergeCommit == "" {
			continue
		}
		acceptOne(repoRoot, idx)
		accepted++
		fmt.Println()
	}
	if accepted == 0 {
		fmt.Println("No merged agents to accept.")
	} else {
		fmt.Printf("✓ %d agent(s) accepted and cleaned up.\n", accepted)
	}
}

func finishAll(repoRoot string) {
	repoRoot = mustAbs(repoRoot)
	mergeAll(repoRoot)
	fmt.Println()
	acceptAll(repoRoot)
}
