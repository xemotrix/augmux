package ops

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/xemotrix/augmux/internal/agent"
	"github.com/xemotrix/augmux/internal/core"
)

// RebaseCmd returns an *exec.Cmd configured to run a headless agent instance
// that performs the rebase operation. The caller is responsible for starting
// the command and reading its stdout/stderr.
func RebaseCmd(repoRoot string, idx int) (*exec.Cmd, error) {
	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		return nil, fmt.Errorf("agent %d not found: %w", idx, err)
	}

	srcBranch := core.SourceBranch(repoRoot)
	agentDef := agent.ConfiguredAgent()
	if agentDef == nil {
		return nil, fmt.Errorf("no agent CLI configured")
	}

	prompt := fmt.Sprintf(
		"Rebase the current branch onto %s. "+
			"Commit any uncommitted work first, then run git rebase %s. "+
			"Resolve any conflicts by keeping both sides, stage resolved files, "+
			"and continue the rebase.",
		srcBranch, srcBranch,
	)

	rulesFile := filepath.Join(core.TaskDir(repoRoot, idx), "rules.md")

	return agentDef.RebaseExecCmd(prompt, rulesFile, ag.Worktree), nil
}
