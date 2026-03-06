package ops

import (
	"fmt"

	"github.com/xemotrix/augmux/internal/core"
)

// ResolveConflict handles a conflict resolution choice from the TUI.
// choice: 0 = continue (manual), anything else = abort.
func ResolveConflict(info *MergeConflictErr, choice int) error {
	td := core.TaskDir(info.RepoRoot, info.AgentIdx)

	switch choice {
	case 0:
		if err := core.WriteFileContent(td+"/resolving", info.MergeMsg); err != nil {
			return fmt.Errorf("failed to write resolving state: %w", err)
		}
	default:
		if _, err := core.Git(info.RepoRoot, "reset", "--hard", "HEAD"); err != nil {
			return fmt.Errorf("failed to reset: %w", err)
		}
	}
	return nil
}
