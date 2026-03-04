package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func ensureSession(repoRoot string) error {
	sd := stateDir(repoRoot)
	if isDir(sd) {
		return nil
	}
	if os.Getenv("TMUX") == "" {
		return fmt.Errorf("not inside a tmux session. Run this from within tmux")
	}
	branch := gitMust(repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
	os.MkdirAll(sd, 0755)
	os.MkdirAll(worktreeBase(repoRoot), 0755)
	writeFileContent(filepath.Join(sd, "source_branch"), branch)
	writeFileContent(filepath.Join(sd, "repo_root"), repoRoot)
	fmt.Printf("augmux session initialized (source branch: %s)\n", branch)
	return nil
}


func safeName(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "-")
	re := regexp.MustCompile(`[^a-z0-9-]`)
	s = re.ReplaceAllString(s, "")
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

func spawnOne(repoRoot, name, initialPrompt string, agent *AgentDef) {
	sd := stateDir(repoRoot)
	srcBranch := sourceBranch(repoRoot)
	idx := nextAgentIdx(repoRoot)

	safe := safeName(name)
	branchName := fmt.Sprintf("augmux/%s-%d", safe, idx)
	wtPath := filepath.Join(worktreeBase(repoRoot), fmt.Sprintf("%s-%d", safe, idx))

	td := filepath.Join(sd, fmt.Sprintf("task-%d", idx))
	os.MkdirAll(td, 0755)
	writeFileContent(filepath.Join(td, "description"), name)
	writeFileContent(filepath.Join(td, "branch"), branchName)
	writeFileContent(filepath.Join(td, "worktree"), wtPath)

	gitMust(repoRoot, "worktree", "prune")
	gitMust(repoRoot, "branch", branchName, srcBranch)
	if _, err := git(repoRoot, "worktree", "add", wtPath, branchName); err != nil {
		fatal("failed to create worktree: %v", err)
	}

	winName := fmt.Sprintf("augmux-%d", idx)
	tmuxRun("new-window", "-n", winName, "-c", wtPath)
	tmuxRun("set-hook", "-w", "-t", winName, "after-split-window",
		fmt.Sprintf("send-keys 'cd \"%s\" && clear' Enter", wtPath))

	if initialPrompt != "" {
		promptFile := filepath.Join(td, "prompt")
		writeFileContent(promptFile, initialPrompt)
		tmuxRun("send-keys", "-t", winName, agent.SpawnWithPromptCmd(promptFile), "Enter")
	} else {
		tmuxRun("send-keys", "-t", winName, agent.SpawnCmd(), "Enter")
	}

	fmt.Printf("  ✓ Agent %d: '%s' → branch %s\n", idx, name, branchName)
	fmt.Printf("    Window: %s\n", winName)
	fmt.Printf("    Worktree: %s\n", wtPath)
}

func spawn(repoRoot string, args []string) {
	repoRoot = mustAbs(repoRoot)
	if err := ensureSession(repoRoot); err != nil {
		fatal(err.Error())
	}
	agent := activeAgent()
	if len(args) == 0 {
		name := runTextInput("Task name for new agent:")
		if name == "" {
			fmt.Println("Empty name — aborting.")
			return
		}
		spawnOne(repoRoot, name, "", agent)
	} else {
		for _, name := range args {
			spawnOne(repoRoot, name, "", agent)
		}
	}
	fmt.Println()
	fmt.Println("Check status: augmux status")
}
