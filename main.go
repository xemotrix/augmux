package main

import (
	"fmt"
	"os"

	"github.com/xemotrix/augmux/internal/core"
	"github.com/xemotrix/augmux/internal/ops"
	"github.com/xemotrix/augmux/internal/tui"
)

const version = "0.2.0"

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		cmdTUI()
	}
	switch args[0] {
	case "nuke":
		cmdNuke()
	case "help", "-h", "--help":
		cmdHelp()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		cmdHelp()
		os.Exit(1)
	}
}

func cmdNuke() {
	repoRoot, err := core.FindRepoFromState()
	if err != nil {
		core.Fatal(err.Error())
	}
	agents := core.ListAgents(repoRoot)
	msg := fmt.Sprintf("This will destroy ALL %d agent(s), their worktrees, branches, and all uncommitted work.\nThis action cannot be undone.", len(agents))
	if !tui.RunConfirm(msg) {
		fmt.Println("Aborted.")
		return
	}
	ops.TeardownAll(os.Stdout, repoRoot)
}

func cmdTUI() {
	repoRoot, err := core.FindRepoFromState()
	if err != nil {
		// No session yet — fall back to git repo root and open TUI with no agents.
		repoRoot, err = core.FindRepoRoot()
		if err != nil {
			core.Fatal(err.Error())
		}
	}
	tui.RunInteractiveTUI(repoRoot)
}

func cmdHelp() {
	fmt.Printf(`augmux v%s — parallel AI agents with tmux + git worktrees

Usage:
  augmux spawn                        Open editor to write prompt, then spawn agent
  augmux spawn "name" ["name2" ...]   Create named agent(s) (agent CLI with no initial prompt)
  augmux status                       Show session status (grid view)
  augmux tui                          Interactive dashboard (navigate, act)
  augmux status --watch               Alias for 'augmux tui'
  augmux merge <id>                   Merge agent into source branch (keeps agent alive)
  augmux merge --all                  Merge all unmerged agents
  augmux accept <id>                  Accept a merged agent (teardown worktree + window)
  augmux accept --all                 Accept all merged agents
  augmux reject <id>                  Undo merge, keep agent alive to fix and re-merge
  augmux cancel <id>                  Remove agent and discard all its changes
  augmux nuke                         Force cleanup (no merge, discard all)
  augmux help                         Show this help

Workflow:
  1. augmux spawn                      # opens editor, then spawns with prompt
  2. augmux spawn "api endpoints"     # spawns agent (no initial prompt)
  3. Ctrl-b + n/p to switch windows   # interact with agents
  4. augmux merge 1                   # merge agent 1 (keeps agent alive)
  5. # review changes, run tests...
  6. augmux accept 1                  # happy → clean up agent
  7. augmux reject 1                  # unhappy → undo merge, fix, re-merge

`, version)
}
