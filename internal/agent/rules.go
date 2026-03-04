package agent

import (
	"fmt"
	"strings"
)

// RulesTemplate is the system prompt injected into auggie sessions spawned by augmux.
// Placeholders: {{TASK}}, {{BRANCH}}, {{WORKTREE}}, {{SOURCE_BRANCH}}
const RulesTemplate = `# augmux agent rules

You are running inside **augmux**, a tool that orchestrates multiple AI coding agents
in parallel using tmux windows and git worktrees.

## Your environment

- You are working in an **isolated git worktree** — your changes will NOT affect other agents.
- **Task:** {{TASK}}
- **Branch:** ` + "`{{BRANCH}}`" + `
- **Worktree path:** ` + "`{{WORKTREE}}`" + `
- **Source branch:** ` + "`{{SOURCE_BRANCH}}`" + ` (the branch your work will be merged back into)

## Activity tracking (CRITICAL — read carefully)

augmux tracks your activity state. You **MUST** follow these rules **on EVERY single message you receive**, not just the first one:

1. **IMMEDIATELY at the start of EVERY response**, before doing anything else, set your state to working:
  ` + "`" + `echo working > {{STATE_DIR}}/activity` + "`" + `
2. **At the END of EVERY response**, when you have finished all work for that message, set your state back to idle:
  ` + "`" + `echo idle > {{STATE_DIR}}/activity` + "`" + `

**This applies to EVERY prompt you receive — not just the first one.**
If you receive a follow-up message, you MUST set working again at the start and idle again at the end.
The orchestrator UI relies on this to show your current status. If you forget, the UI will show stale state and the user will not know what you are doing.

## Important instructions

- When you finish your task, **commit your changes** with a clear commit message unless the user tells you otherwise.
- Do **NOT** push to any remote.
- Do **NOT** switch branches or leave the worktree directory.
- Focus exclusively on the task you have been given.
`

// BuildRules renders the rules template with the given values.
func BuildRules(task, branch, worktree, sourceBranch, stateDir string) string {
	r := strings.NewReplacer(
		"{{TASK}}", task,
		"{{BRANCH}}", branch,
		"{{WORKTREE}}", worktree,
		"{{SOURCE_BRANCH}}", sourceBranch,
		"{{STATE_DIR}}", stateDir,
	)
	return r.Replace(RulesTemplate)
}

// SpawnCmdWithRules returns the shell command to start the agent with a rules file.
// For agents that don't support rules, it falls back to the plain command.
func (a *AgentDef) SpawnCmdWithRules(rulesFile string) string {
	if a.ID == "auggie" && rulesFile != "" {
		return fmt.Sprintf("%s --rules '%s'", a.Command, rulesFile)
	}
	return a.Command
}

