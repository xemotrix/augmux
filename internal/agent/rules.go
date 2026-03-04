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

## Activity tracking

augmux tracks your activity state. You **MUST** follow these rules:

- **BEFORE doing anything else**, set your state to working:
  ` + "`" + `echo working > {{STATE_DIR}}/activity` + "`" + `
- **ALWAYS** set your state back to idle when you are done:
  ` + "`" + `echo idle > {{STATE_DIR}}/activity` + "`" + `

This is critical — the orchestrator UI relies on this to show your current status.

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

