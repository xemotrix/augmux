package ops

import (
	"github.com/xemotrix/augmux/internal/core"
)

// ResolveConflict handles a conflict resolution choice from the TUI.
// choice: 0 = continue (manual), anything else = abort.
func ResolveConflict(info *MergeConflictErr, choice int) {
	td := core.TaskDir(info.RepoRoot, info.AgentIdx)

	switch choice {
	case 0:
		core.WriteFileContent(td+"/resolving", info.MergeMsg)

	default:
		core.GitMust(info.RepoRoot, "reset", "--hard", "HEAD")
	}
}
