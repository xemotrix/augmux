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

## Important instructions

- When you finish your task, **commit your changes** with a clear commit message unless the user tells you otherwise.
- Do **NOT** push to any remote.
- Do **NOT** switch branches or leave the worktree directory.
- Focus exclusively on the task you have been given.
`

// BuildRules renders the rules template with the given values.
func BuildRules(task, branch, worktree, sourceBranch string) string {
	r := strings.NewReplacer(
		"{{TASK}}", task,
		"{{BRANCH}}", branch,
		"{{WORKTREE}}", worktree,
		"{{SOURCE_BRANCH}}", sourceBranch,
	)
	return r.Replace(RulesTemplate)
}

// BuildCursorMDC wraps rules content in .mdc frontmatter for Cursor's .cursor/rules/ directory.
func BuildCursorMDC(rulesContent string) string {
	return fmt.Sprintf("---\ndescription: augmux agent rules\nalwaysApply: true\n---\n%s", rulesContent)
}

// RebaseCommandTemplate is the Cursor slash command prompt for /augmux-rebase.
// Placeholder: {{SOURCE_BRANCH}}
const RebaseCommandTemplate = `Rebase your current branch onto the latest ` + "`{{SOURCE_BRANCH}}`" + ` (the source branch).

Follow these steps exactly:

1. **Commit uncommitted work.** If there are any uncommitted changes, stage and commit them:
` + "   ```" + `
   git add -A && git commit -m "augmux: auto-commit before rebase"
` + "   ```" + `

2. **Start the rebase:**
` + "   ```" + `
   git rebase {{SOURCE_BRANCH}}
` + "   ```" + `

3. **If the rebase succeeds with no conflicts**, you are done.

4. **If there are conflicts**, for each conflicted file:
   - Open the file and look for conflict markers (` + "`<<<<<<<`" + `, ` + "`=======`" + `, ` + "`>>>>>>>`" + `).
   - Resolve by integrating **both** sides: keep the new features from ` + "`{{SOURCE_BRANCH}}`" + ` AND your own changes. Do not discard either side.
   - Stage the resolved file:
` + "     ```" + `
     git add <file>
` + "     ```" + `
   - Continue the rebase:
` + "     ```" + `
     GIT_EDITOR=true git rebase --continue
` + "     ```" + `
   - If more conflicts appear, repeat this step.

5. After the rebase completes, verify the build still works and your changes are intact.

Do NOT use ` + "`git rebase --abort`" + ` unless you truly cannot resolve a conflict. Do NOT push to any remote.
`

// BuildRebaseCommand renders the rebase command template for the given source branch.
func BuildRebaseCommand(sourceBranch string) string {
	return strings.Replace(RebaseCommandTemplate, "{{SOURCE_BRANCH}}", sourceBranch, -1)
}

// SquashCommandTemplate is the Cursor slash command prompt for /augmux-squash.
// Placeholder: {{SOURCE_BRANCH}}
const SquashCommandTemplate = `Squash all commits on your current branch into a single commit.

Follow these steps exactly:

1. **Commit uncommitted work.** If there are any uncommitted changes, stage and commit them:
` + "   ```" + `
   git add -A && git commit -m "augmux: auto-commit before squash"
` + "   ```" + `

2. **Soft-reset back to the source branch** to collapse all commits while keeping your changes staged:
` + "   ```" + `
   git reset --soft {{SOURCE_BRANCH}}
` + "   ```" + `

3. **Create a single commit** with a clear, descriptive message summarizing all the work done on this branch:
` + "   ```" + `
   git commit -m "<summarize all changes in one concise message>"
` + "   ```" + `

4. Verify that your working tree is clean (` + "`git status`" + ` shows nothing to commit) and the branch has exactly one commit ahead of ` + "`{{SOURCE_BRANCH}}`" + `.

Do NOT push to any remote.
`

// BuildSquashCommand renders the squash command template for the given source branch.
func BuildSquashCommand(sourceBranch string) string {
	return strings.Replace(SquashCommandTemplate, "{{SOURCE_BRANCH}}", sourceBranch, -1)
}

// SpawnCmdWithRules returns the shell command to start the agent with a rules file.
// For agents that don't support rules, it falls back to the plain command.
func (a *AgentDef) SpawnCmdWithRules(rulesFile string) string {
	if a.ID == "auggie" && rulesFile != "" {
		return fmt.Sprintf("%s --rules '%s'", a.Command, rulesFile)
	}
	if a.ID == "cursor" {
		return fmt.Sprintf("%s --yolo", a.Command)
	}
	return a.Command
}

// RebasePaneCmd returns the shell command to run a non-interactive agent
// instance that performs only the rebase operation. The prompt is passed as a
// CLI argument so no send-keys interaction is needed.
func (a *AgentDef) RebasePaneCmd(prompt, rulesFile string) string {
	return a.oneOffPaneCmd(prompt, rulesFile)
}

// SquashPaneCmd returns the shell command to run a non-interactive agent
// instance that performs the squash operation.
func (a *AgentDef) SquashPaneCmd(prompt, rulesFile string) string {
	return a.oneOffPaneCmd(prompt, rulesFile)
}

// oneOffPaneCmd builds the shell command for a one-shot agent invocation
// in a split pane (used by both rebase and squash).
func (a *AgentDef) oneOffPaneCmd(prompt, rulesFile string) string {
	escaped := strings.ReplaceAll(prompt, "'", "'\\''")
	switch a.ID {
	case "auggie":
		if rulesFile != "" {
			return fmt.Sprintf("%s --rules '%s' '%s'", a.Command, rulesFile, escaped)
		}
		return fmt.Sprintf("%s '%s'", a.Command, escaped)
	case "cursor":
		return fmt.Sprintf("%s --yolo --print '%s'", a.Command, escaped)
	default:
		return fmt.Sprintf("%s '%s'", a.Command, escaped)
	}
}

