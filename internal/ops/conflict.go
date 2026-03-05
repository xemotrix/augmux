package ops

import (
	"io"

	"github.com/xemotrix/augmux/internal/components"
	"github.com/xemotrix/augmux/internal/core"
)

type conflictResult int

const (
	conflictContinue conflictResult = iota
	conflictAbort
)

func handleConflict(repoRoot string) conflictResult {
	choice := components.RunSelectMenu("How do you want to resolve?", []string{
		"Continue — leave conflicts in working tree, resolve manually",
		"Abort — discard merge and reset",
	})

	switch choice {
	case 0:
		return conflictContinue

	default:
		core.GitMust(repoRoot, "reset", "--hard", "HEAD")
		return conflictAbort
	}
}

// ResolveConflict handles a conflict resolution choice from the TUI.
// choice: 0 = continue (manual), anything else = abort.
func ResolveConflict(w io.Writer, info *MergeConflictErr, choice int) {
	td := core.TaskDir(info.RepoRoot, info.AgentIdx)

	switch choice {
	case 0:
		core.WriteFileContent(td+"/resolving", info.MergeMsg)

	default:
		core.GitMust(info.RepoRoot, "reset", "--hard", "HEAD")
	}
}
