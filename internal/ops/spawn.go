package ops

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/xemotrix/augmux/internal/agent"
	"github.com/xemotrix/augmux/internal/core"
	"github.com/xemotrix/augmux/internal/tui"
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
	os.MkdirAll(sd, 0755)
	os.MkdirAll(core.WorktreeBase(repoRoot), 0755)
	core.WriteFileContent(filepath.Join(sd, "source_branch"), branch)
	core.WriteFileContent(filepath.Join(sd, "repo_root"), repoRoot)
	fmt.Printf("augmux session initialized (source branch: %s)\n", branch)
	return nil
}

func spawnOne(repoRoot, name string, ag *agent.AgentDef) {
	sd := core.StateDir(repoRoot)
	srcBranch := core.SourceBranch(repoRoot)
	idx := core.NextAgentIdx(repoRoot)

	safe := core.SafeName(name)
	branchName := fmt.Sprintf("augmux/%s-%d", safe, idx)
	wtPath := filepath.Join(core.WorktreeBase(repoRoot), fmt.Sprintf("%s-%d", safe, idx))

	td := filepath.Join(sd, fmt.Sprintf("task-%d", idx))
	os.MkdirAll(td, 0755)
	core.WriteFileContent(filepath.Join(td, "description"), name)
	core.WriteFileContent(filepath.Join(td, "branch"), branchName)
	core.WriteFileContent(filepath.Join(td, "worktree"), wtPath)

	core.GitMust(repoRoot, "worktree", "prune")
	core.GitMust(repoRoot, "branch", branchName, srcBranch)
	if _, err := core.Git(repoRoot, "worktree", "add", wtPath, branchName); err != nil {
		core.Fatal("failed to create worktree: %v", err)
	}

	winName := fmt.Sprintf("augmux-%d", idx)
	core.TmuxRun("new-window", "-n", winName, "-c", wtPath)
	core.TmuxRun("set-hook", "-w", "-t", winName, "after-split-window",
		fmt.Sprintf("send-keys 'cd \"%s\" && clear' Enter", wtPath))

	core.TmuxRun("send-keys", "-t", winName, ag.SpawnCmd(), "Enter")

	fmt.Printf("  ✓ Agent %d: '%s' → branch %s\n", idx, name, branchName)
	fmt.Printf("    Window: %s\n", winName)
	fmt.Printf("    Worktree: %s\n", wtPath)
}

func Spawn(repoRoot string, args []string) {
	repoRoot = core.MustAbs(repoRoot)
	if err := ensureSession(repoRoot); err != nil {
		core.Fatal(err.Error())
	}
	ag := agent.ActiveAgent()
	if len(args) == 0 {
		name := tui.RunTextInput("Task name for new agent:")
		if name == "" {
			fmt.Println("Empty name — aborting.")
			return
		}
		spawnOne(repoRoot, name, ag)
	} else {
		for _, name := range args {
			spawnOne(repoRoot, name, ag)
		}
	}
	fmt.Println()
	fmt.Println("Check status: augmux status")
}
