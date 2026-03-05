package ops

import (
	"fmt"

	"github.com/xemotrix/augmux/internal/agent"
	"github.com/xemotrix/augmux/internal/core"
)

// SendRebase sends the rebase command to an agent's tmux pane via send-keys.
// The exact text depends on the configured agent CLI.
func SendRebase(repoRoot string, idx int) error {
	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		return fmt.Errorf("agent %d not found: %w", idx, err)
	}

	srcBranch := core.SourceBranch(repoRoot)
	agentDef := agent.ConfiguredAgent()
	if agentDef == nil {
		return fmt.Errorf("no agent CLI configured")
	}

	var cmd string
	switch agentDef.ID {
	case "cursor":
		cmd = "/augmux-rebase"
	default:
		cmd = fmt.Sprintf("Rebase the current branch onto %s. "+
			"Commit any uncommitted work first, then run git rebase %s. "+
			"Resolve any conflicts by keeping both sides, stage resolved files, and continue the rebase.",
			srcBranch, srcBranch)
	}

	if err := core.TmuxRun("send-keys", "-t", ag.Window, "-l", cmd); err != nil {
		return err
	}
	return core.TmuxRun("send-keys", "-t", ag.Window, "Escape", "Enter")
}
