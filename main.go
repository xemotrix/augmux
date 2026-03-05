package main

// conflict 1

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
		return
	}
	switch args[0] {
	case "nuke":
		cmdNuke()
	case "help", "-h", "--help":
		cmdHelp()
	default:
		core.Fatal("Unknown command: %s. Run 'augmux' to open the TUI or 'augmux help' for usage.", args[0])
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
	fmt.Fprintf(os.Stderr, `augmux v%s — parallel AI agents with tmux + git worktrees

Usage:
  augmux            Open the interactive TUI dashboard
  augmux nuke       Force cleanup (no merge, discard all agents)
  augmux help       Show this help
`, version)
}
