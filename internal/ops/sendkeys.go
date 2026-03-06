package ops

import (
	"fmt"
	"path/filepath"

	"github.com/xemotrix/augmux/internal/agent"
	"github.com/xemotrix/augmux/internal/core"
)

// SendRebase spawns a new tmux pane in the agent's window and runs a fresh
// agent instance with the rebase prompt. This avoids sending keys to the
// existing agent pane, which doesn't reliably submit in all agent CLIs.
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

	prompt := fmt.Sprintf(
		"Rebase the current branch onto %s. "+
			"Commit any uncommitted work first, then run git rebase %s. "+
			"Resolve any conflicts by keeping both sides, stage resolved files, "+
			"and continue the rebase.",
		srcBranch, srcBranch,
	)

	rulesFile := filepath.Join(core.TaskDir(repoRoot, idx), "rules.md")
	cmd := agentDef.RebasePaneCmd(prompt, rulesFile)

	return core.TmuxRun("split-window", "-h", "-t", ag.Window, "-c", ag.Worktree, cmd)
}

// SendSquash spawns a new tmux pane in the agent's window and runs a fresh
// agent instance with the squash prompt, instructing it to squash all commits
// on the branch into a single commit.
func SendSquash(repoRoot string, idx int) error {
	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		return fmt.Errorf("agent %d not found: %w", idx, err)
	}

	srcBranch := core.SourceBranch(repoRoot)
	agentDef := agent.ConfiguredAgent()
	if agentDef == nil {
		return fmt.Errorf("no agent CLI configured")
	}

	prompt := fmt.Sprintf(
		"Squash all commits on this branch into a single commit. "+
			"Commit any uncommitted work first, then run git reset --soft %s "+
			"to collapse all commits while keeping changes staged, "+
			"then create a single commit with a clear summary message.",
		srcBranch,
	)

	rulesFile := filepath.Join(core.TaskDir(repoRoot, idx), "rules.md")
	cmd := agentDef.SquashPaneCmd(prompt, rulesFile)

	return core.TmuxRun("split-window", "-h", "-t", ag.Window, "-c", ag.Worktree, cmd)
}
