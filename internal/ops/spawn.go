package ops

import (
	"fmt"
	"io"
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
	branch := core.GitMust(repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
	os.MkdirAll(sd, 0o755)
	os.MkdirAll(core.WorktreeBase(repoRoot), 0o755)
	core.WriteFileContent(filepath.Join(sd, "source_branch"), branch)
	core.WriteFileContent(filepath.Join(sd, "repo_root"), repoRoot)
	return nil
}

func spawnOne(w io.Writer, repoRoot, name string, ag *agent.AgentDef) {
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
	core.GitMust(repoRoot, "branch", branchName, srcBranch)
	if _, err := core.Git(repoRoot, "worktree", "add", wtPath, branchName); err != nil {
		core.Fatal("failed to create worktree: %v", err)
	}

	// Write rules file for agents that support it.
	rulesFile := filepath.Join(td, "rules.md")
	rulesContent := agent.BuildRules(name, branchName, wtPath, srcBranch)
	core.WriteFileContent(rulesFile, rulesContent)

	// For Cursor, inject rules and commands via .cursor/ in the worktree.
	if ag.ID == "cursor" {
		cursorRulesDir := filepath.Join(wtPath, ".cursor", "rules")
		os.MkdirAll(cursorRulesDir, 0o755)
		mdcContent := agent.BuildCursorMDC(rulesContent)
		core.WriteFileContent(filepath.Join(cursorRulesDir, "augmux.mdc"), mdcContent)

		cursorCmdsDir := filepath.Join(wtPath, ".cursor", "commands")
		os.MkdirAll(cursorCmdsDir, 0o755)
		rebaseCmd := agent.BuildRebaseCommand(srcBranch)
		core.WriteFileContent(filepath.Join(cursorCmdsDir, "augmux-rebase.md"), rebaseCmd)

		gitignorePath := filepath.Join(wtPath, ".gitignore")
		appendToGitignore(gitignorePath, ".cursor/")
	}

	winName := fmt.Sprintf("ax-%d-%s", idx, safe)
	core.WriteFileContent(filepath.Join(td, "window"), winName)
	core.TmuxRun("new-window", "-n", winName, "-c", wtPath)
	core.TmuxRun("set-hook", "-w", "-t", winName, "after-split-window",
		fmt.Sprintf("send-keys 'cd \"%s\" && clear' Enter", wtPath))

	core.TmuxRun("send-keys", "-t", winName, ag.SpawnCmdWithRules(rulesFile), "Enter")
}

// appendToGitignore adds an entry to a .gitignore file if it's not already present.
func appendToGitignore(path, entry string) {
	data, _ := os.ReadFile(path)
	content := string(data)
	for _, line := range strings.Split(content, "\n") {
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
func SpawnByName(w io.Writer, repoRoot string, name string) {
	repoRoot = core.MustAbs(repoRoot)
	if err := ensureSession(repoRoot); err != nil {
		core.Fatal(err.Error())
	}
	ag := agent.ActiveAgent()
	spawnOne(w, repoRoot, name, ag)
}
