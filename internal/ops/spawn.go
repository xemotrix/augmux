package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xemotrix/augmux/internal/agent"
	"github.com/xemotrix/augmux/internal/core"
)

func ensureSession(repoRoot string) error {
	sd := core.StateDir(repoRoot)
	if core.IsDir(sd) {
		return nil
	}
	if os.Getenv("TMUX") == "" {
		return fmt.Errorf("not inside a tmux session. Run this from within tmux")
	}
	branch, err := core.Git(repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to determine current branch: %w", err)
	}
	os.MkdirAll(sd, 0o755)
	os.MkdirAll(core.WorktreeBase(repoRoot), 0o755)
	core.WriteFileContent(filepath.Join(sd, "source_branch"), branch)
	core.WriteFileContent(filepath.Join(sd, "repo_root"), repoRoot)
	return nil
}

func spawn(repoRoot, name string, ag *agent.AgentDef) error {
	sd := core.StateDir(repoRoot)
	srcBranch := core.SourceBranch(repoRoot)
	idx := core.NextAgentIdx(repoRoot)

	safe := core.SafeName(name)
	branchName := fmt.Sprintf("augmux/%s-%d", safe, idx)
	wtPath := filepath.Join(core.WorktreeBase(repoRoot), fmt.Sprintf("%s-%d", safe, idx))

	td := filepath.Join(sd, fmt.Sprintf("task-%d", idx))
	os.MkdirAll(td, 0o755)
	core.WriteFileContent(filepath.Join(td, "description"), name)
	core.WriteFileContent(filepath.Join(td, "branch"), branchName)
	core.WriteFileContent(filepath.Join(td, "worktree"), wtPath)

	core.GitMust(repoRoot, "worktree", "prune")
	if _, err := core.Git(repoRoot, "branch", branchName, srcBranch); err != nil {
		return fmt.Errorf("failed to create branch %s: %w", branchName, err)
	}
	if _, err := core.Git(repoRoot, "worktree", "add", wtPath, branchName); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	rulesFile := filepath.Join(td, "rules.md")
	rulesContent := agent.BuildRules(name, branchName, wtPath, srcBranch)
	core.WriteFileContent(rulesFile, rulesContent)

	if ag.ID == "cursor" {
		srcCursor := filepath.Join(repoRoot, ".cursor")
		if core.IsDir(srcCursor) {
			core.CopyDir(srcCursor, filepath.Join(wtPath, ".cursor"))
		}

		cursorRulesDir := filepath.Join(wtPath, ".cursor", "rules")
		os.MkdirAll(cursorRulesDir, 0o755)
		mdcContent := agent.BuildCursorMDC(rulesContent)
		core.WriteFileContent(filepath.Join(cursorRulesDir, "augmux.mdc"), mdcContent)

		cursorCmdsDir := filepath.Join(wtPath, ".cursor", "commands")
		os.MkdirAll(cursorCmdsDir, 0o755)
		rebaseCmd := agent.BuildRebaseCommand(srcBranch)
		core.WriteFileContent(filepath.Join(cursorCmdsDir, "augmux-rebase.md"), rebaseCmd)
		rebaseYoloCmd := agent.BuildRebaseYoloCommand(srcBranch)
		core.WriteFileContent(filepath.Join(cursorCmdsDir, "augmux-yolobase.md"), rebaseYoloCmd)

		squashCmd := agent.BuildSquashCommand(srcBranch)
		core.WriteFileContent(filepath.Join(cursorCmdsDir, "augmux-squash.md"), squashCmd)

		gitignorePath := filepath.Join(wtPath, ".gitignore")
		appendToGitignore(gitignorePath, ".cursor/")
	}

	winName := fmt.Sprintf("ax-%d-%s", idx, safe)
	core.WriteFileContent(filepath.Join(td, "window"), winName)
	core.TmuxRun("new-window", "-n", winName, "-c", wtPath)
	core.TmuxRun("set-option", "-w", "-t", winName, "@augmux_worktree", wtPath)
	core.TmuxRun("set-hook", "-w", "-t", winName, "after-split-window",
		`run-shell 'tmux send-keys -t "#{pane_id}" "cd \"#{@augmux_worktree}\" && clear" Enter'`)

	core.TmuxRun("send-keys", "-t", winName, ag.SpawnCmdWithRules(rulesFile), "Enter")
	return nil
}

// appendToGitignore adds an entry to a .gitignore file if it's not already present.
func appendToGitignore(path, entry string) {
	data, _ := os.ReadFile(path)
	content := string(data)
	for line := range strings.SplitSeq(content, "\n") {
		if strings.TrimSpace(line) == entry {
			return
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		f.WriteString("\n")
	}
	f.WriteString(entry + "\n")
}

// SpawnByName spawns a single agent with the given name.
func SpawnByName(repoRoot string, name string) error {
	var err error
	repoRoot, err = core.Abs(repoRoot)
	if err != nil {
		return err
	}
	if err := ensureSession(repoRoot); err != nil {
		return err
	}
	ag, err := agent.ActiveAgent()
	if err != nil {
		return err
	}
	return spawn(repoRoot, name, ag)
}
