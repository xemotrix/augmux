package ops

import (
	"fmt"
	"os"

	"github.com/xemotrix/augmux/internal/core"
)

func AcceptOne(repoRoot string, idx int) error {
	var err error
	repoRoot, err = core.Abs(repoRoot)
	if err != nil {
		return err
	}
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

func RejectOne(repoRoot string, idx int) error {
	var err error
	repoRoot, err = core.Abs(repoRoot)
	if err != nil {
		return err
	}
	td := core.TaskDir(repoRoot, idx)

	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		return err
	}

	if ag.MergeCommit == "" {
		return fmt.Errorf("agent %d has not been merged yet. Nothing to reject", idx)
	}

	currentHead, err := core.Git(repoRoot, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}
	if currentHead != ag.MergeCommit {
		return fmt.Errorf("cannot safely reset: HEAD (%s) does not match merge commit (%s)", currentHead, ag.MergeCommit)
	}

	if _, err := core.Git(repoRoot, "reset", "--hard", "HEAD~1"); err != nil {
		return fmt.Errorf("failed to reset merge commit: %w", err)
	}
	os.Remove(td + "/merge_commit")
	return nil
}

