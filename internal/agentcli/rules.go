package agentcli

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

// RebaseYoloCommandTemplate is the Cursor slash command prompt for /augmux-yolobase.
// This version automatically resolves conflicts without user confirmation.
// Placeholder: {{SOURCE_BRANCH}}
const RebaseYoloCommandTemplate = `Rebase your current branch onto the latest ` + "`{{SOURCE_BRANCH}}`" + ` (the source branch).

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
   - **Your goal is to preserve the functionality from both branches.** This does NOT mean blindly concatenating or keeping all changes from both sides. Think about each conflict individually:
     - Read and understand what each side intended to accomplish.
     - If both sides added different functionality to the same area, integrate both functionalities in a way that makes sense structurally and logically.
     - If both sides modified the same code differently, determine the correct version that satisfies both intents — this may mean rewriting the section to combine both purposes, picking one side when the other is redundant, or adapting one side's approach to coexist with the other.
     - If one side deleted or refactored code that the other side modified, decide whether the deletion/refactor still makes sense given the other side's changes, and adjust accordingly.
   - The key principle: **no functionality should be silently lost**, but the final code must be coherent, correct, and free of duplication.
   - Stage the resolved file:
` + "     ```" + `
     git add <file>
` + "     ```" + `
   - Continue the rebase:
` + "     ```" + `
     GIT_EDITOR=true git rebase --continue
` + "     ```" + `
   - If more conflicts appear, repeat this step.

5. After the rebase completes, verify the build still works and that functionality from both branches is intact.

Do NOT use ` + "`git rebase --abort`" + ` unless you truly cannot resolve a conflict. Do NOT push to any remote.
`

// BuildRebaseYoloCommand renders the yolo rebase command template for the given source branch.
func BuildRebaseYoloCommand(sourceBranch string) string {
	return strings.ReplaceAll(RebaseYoloCommandTemplate, "{{SOURCE_BRANCH}}", sourceBranch)
}

// RebaseCommandTemplate is the Cursor slash command prompt for /augmux-rebase.
// This version analyzes conflicts first, presents a summary, and waits for user
// confirmation before proceeding.
// Placeholder: {{SOURCE_BRANCH}}
const RebaseCommandTemplate = `Analyze and rebase your current branch onto ` + "`{{SOURCE_BRANCH}}`" + ` (the source branch) **with user confirmation**.

Follow these steps exactly:

## Phase 1 — Analysis (do NOT start the rebase yet)

1. **Commit uncommitted work.** If there are any uncommitted changes, stage and commit them:
` + "   ```" + `
   git add -A && git commit -m "augmux: auto-commit before rebase"
` + "   ```" + `

2. **Preview what the rebase will do.** Run:
` + "   ```" + `
   git diff {{SOURCE_BRANCH}}...HEAD --stat
` + "   ```" + `
   This shows the files you changed on your branch.

3. **Check for potential conflicts.** Run:
` + "   ```" + `
   git merge-tree --write-tree {{SOURCE_BRANCH}} HEAD
` + "   ```" + `
   If this command **succeeds** (exit code 0), there will be no conflicts. Tell the user and skip to Phase 2 immediately.

   If this command **fails**, there will be conflicts. Now analyze them:
   - Run ` + "`git merge-tree --write-tree --name-only {{SOURCE_BRANCH}} HEAD 2>&1`" + ` to identify which files conflict.
   - For each conflicting file, examine both versions:
     - **Source branch version:** ` + "`git show {{SOURCE_BRANCH}}:<file>`" + `
     - **Your branch version:** ` + "`git show HEAD:<file>`" + `
   - Understand what each side changed and why.

4. **Present a conflict summary to the user.** Format it clearly:
   - List each conflicting file.
   - For each file, briefly explain what **your branch** changed vs. what **` + "`{{SOURCE_BRANCH}}`" + `** changed.
   - For conflicts that have an **obvious resolution** (e.g., both sides added different imports, or changes are in non-overlapping sections), state what you would do.
   - For conflicts that are **ambiguous** (e.g., both sides modified the same function differently, or a design choice is involved), **ask the user a specific question** about how they want to resolve it.

5. **Wait for the user to confirm.** Ask:
   > "Ready to proceed with the rebase using the strategy above? (yes/no)"

   Do NOT proceed until the user confirms. If the user says no or asks for changes, adjust your plan and ask again.

## Phase 2 — Execute the rebase

6. **Start the rebase:**
` + "   ```" + `
   git rebase {{SOURCE_BRANCH}}
` + "   ```" + `

7. **If there are conflicts**, for each conflicted file:
   - Open the file and look for conflict markers (` + "`<<<<<<<`" + `, ` + "`=======`" + `, ` + "`>>>>>>>`" + `).
   - Resolve according to the plan confirmed by the user in Phase 1. Integrate **both** sides where appropriate.
   - Stage the resolved file:
` + "     ```" + `
     git add <file>
` + "     ```" + `
   - Continue the rebase:
` + "     ```" + `
     GIT_EDITOR=true git rebase --continue
` + "     ```" + `
   - If more conflicts appear, repeat this step.

8. After the rebase completes, verify the build still works and your changes are intact.

Do NOT use ` + "`git rebase --abort`" + ` unless the user explicitly asks you to. Do NOT push to any remote.
`

// BuildRebaseCommand renders the interactive rebase command template for the given source branch.
func BuildRebaseCommand(sourceBranch string) string {
	return strings.ReplaceAll(RebaseCommandTemplate, "{{SOURCE_BRANCH}}", sourceBranch)
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
	return strings.ReplaceAll(
		SquashCommandTemplate,
		"{{SOURCE_BRANCH}}",
		sourceBranch,
	)
}

// SpawnCmdWithRules returns the shell command to start the agent with a rules file.
// For agents that don't support rules, it falls back to the plain command.
func (a *AgentCliDef) SpawnCmdWithRules(rulesFile string) string {
	if a.ID == "auggie" && rulesFile != "" {
		return fmt.Sprintf("%s --rules '%s'", a.Command, rulesFile)
	}
	if a.ID == "cursor" {
		return fmt.Sprintf("%s --yolo", a.Command)
	}
	return a.Command
}
