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
	teardownOne(repoRoot, idx)
	return nil
}

func RejectOne(w io.Writer, repoRoot string, idx int) error {
	repoRoot = core.MustAbs(repoRoot)
	td := core.TaskDir(repoRoot, idx)

	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		return err
	}

	// Case 1: conflicts are being resolved
	if ag.Resolving != "" {
		core.GitMust(repoRoot, "reset", "--hard", "HEAD")
		os.Remove(td + "/resolving")
		return nil
	}

	// Case 2: merge was completed
	if ag.MergeCommit == "" {
		return fmt.Errorf("agent %d has not been merged yet. Nothing to reject", idx)
	}

	currentHead := core.GitMust(repoRoot, "rev-parse", "HEAD")
	if currentHead != ag.MergeCommit {
		return fmt.Errorf("cannot safely reset")
	}

	core.GitMust(repoRoot, "reset", "--hard", "HEAD~1")
	os.Remove(td + "/merge_commit")
	return nil
}

func MergeAll(w io.Writer, repoRoot string) {
	repoRoot = core.MustAbs(repoRoot)
	if !core.IsDir(core.StateDir(repoRoot)) {
		core.Fatal("No augmux session found.")
	}

	for _, idx := range core.ListAgents(repoRoot) {
		MergeOne(w, repoRoot, idx, MergeInteractive)
		fmt.Fprintln(w)
	}

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
}

func AcceptAll(w io.Writer, repoRoot string) {
	repoRoot = core.MustAbs(repoRoot)
	if !core.IsDir(core.StateDir(repoRoot)) {
		core.Fatal("No augmux session found.")
	}
	for _, idx := range core.ListAgents(repoRoot) {
		ag, err := core.ReadAgent(repoRoot, idx)
		if err != nil || ag.MergeCommit == "" {
			continue
		}
		AcceptOne(w, repoRoot, idx)
	}
}
