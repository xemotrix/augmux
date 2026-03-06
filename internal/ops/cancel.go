package ops

import (
	"fmt"

	"github.com/xemotrix/augmux/internal/core"
)

// Cancel removes a single agent, discarding all its changes.
func Cancel(repoRoot string, idx int) error {
	var err error
	repoRoot, err = core.Abs(repoRoot)
	if err != nil {
		return err
	}
	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		return fmt.Errorf("agent %d not found: %w", idx, err)
	}

	if ag.MergeCommit != "" {
		currentHead, err := core.Git(repoRoot, "rev-parse", "HEAD")
		if err != nil {
			return fmt.Errorf("failed to get HEAD: %w", err)
		}
		if currentHead == ag.MergeCommit {
			if _, err := core.Git(repoRoot, "reset", "--hard", "HEAD~1"); err != nil {
				return fmt.Errorf("failed to reset merge commit: %w", err)
			}
		}
	}

	teardown(repoRoot, idx)
	return nil
}
