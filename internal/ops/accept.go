package ops

import (
	"fmt"

	"github.com/xemotrix/augmux/internal/core"
)

func Accept(repoRoot string, idx int) error {
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
	teardown(repoRoot, idx)
	return nil
}
