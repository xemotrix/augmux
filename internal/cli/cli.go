package cli

import (
	"bytes"
	"fmt"
	"os"
	"strconv"

	"github.com/xemotrix/augmux/internal/core"
	"github.com/xemotrix/augmux/internal/ops"
	"github.com/xemotrix/augmux/internal/tui"
)

const version = "0.2.0"

func Run() {
	args := os.Args[1:]
	if len(args) == 0 {
		cmdHelp()
		return
	}
	switch args[0] {
	case "spawn":
		cmdSpawn(args[1:])
	case "tui":
		cmdTUI()
	case "status":
		cmdStatus()
	case "merge":
		cmdMerge(args[1:])
	case "accept":
		cmdAccept(args[1:])
	case "reject":
		cmdReject(args[1:])
	case "cancel":
		cmdCancel(args[1:])
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

func cmdSpawn(args []string) {
	repoRoot, err := core.FindRepoRoot()
	if err != nil {
		core.Fatal(err.Error())
	}
	ops.Spawn(os.Stdout, repoRoot, args)
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
	tui.RunInteractiveTUI(repoRoot, tuiActionHandler(repoRoot))
}

func cmdStatus() {
	repoRoot, err := core.FindRepoFromState()
	if err != nil {
		core.Fatal(err.Error())
	}
	tui.RunInteractiveTUI(repoRoot, tuiActionHandler(repoRoot))
}

func cmdMerge(args []string) {
	repoRoot, err := core.FindRepoFromState()
	if err != nil {
		core.Fatal(err.Error())
	}
	if len(args) > 0 && args[0] == "--all" {
		ops.MergeAll(os.Stdout, repoRoot)
		return
	}
	if len(args) > 0 {
		idx, err := strconv.Atoi(args[0])
		if err != nil {
			core.Fatal("Invalid agent ID: %s", args[0])
		}
		if err := ops.MergeOne(os.Stdout, repoRoot, idx, ops.MergeInteractive); err != nil {
			core.Fatal(err.Error())
		}
		return
	}
	indices := tui.RunPicker("Select agents to merge", repoRoot, func(a *core.AgentState) bool {
		return a.MergeCommit == "" && a.Resolving == ""
	})
	for _, idx := range indices {
		ops.MergeOne(os.Stdout, repoRoot, idx, ops.MergeInteractive)
	}
}

func cmdAccept(args []string) {
	repoRoot, err := core.FindRepoFromState()
	if err != nil {
		core.Fatal(err.Error())
	}
	if len(args) > 0 && args[0] == "--all" {
		ops.AcceptAll(os.Stdout, repoRoot)
		return
	}
	if len(args) > 0 {
		idx, err := strconv.Atoi(args[0])
		if err != nil {
			core.Fatal("Invalid agent ID: %s", args[0])
		}
		if err := ops.AcceptOne(os.Stdout, repoRoot, idx); err != nil {
			core.Fatal(err.Error())
		}
		return
	}
	indices := tui.RunPicker("Select agents to accept", repoRoot, func(a *core.AgentState) bool {
		return a.MergeCommit != ""
	})
	for _, idx := range indices {
		ops.AcceptOne(os.Stdout, repoRoot, idx)
	}
}

func cmdReject(args []string) {
	repoRoot, err := core.FindRepoFromState()
	if err != nil {
		core.Fatal(err.Error())
	}
	if len(args) > 0 {
		idx, err := strconv.Atoi(args[0])
		if err != nil {
			core.Fatal("Invalid agent ID: %s", args[0])
		}
		if err := ops.RejectOne(os.Stdout, repoRoot, idx); err != nil {
			core.Fatal(err.Error())
		}
		return
	}
	indices := tui.RunPicker("Select agents to reject", repoRoot, func(a *core.AgentState) bool {
		return a.MergeCommit != "" || a.Resolving != ""
	})
	for _, idx := range indices {
		ops.RejectOne(os.Stdout, repoRoot, idx)
	}
}

func cmdCancel(args []string) {
	repoRoot, err := core.FindRepoFromState()
	if err != nil {
		core.Fatal(err.Error())
	}
	if len(args) > 0 {
		idx, err := strconv.Atoi(args[0])
		if err != nil {
			core.Fatal("Invalid agent ID: %s", args[0])
		}
		ops.CancelOne(os.Stdout, repoRoot, idx)
		return
	}
	indices := tui.RunPicker("Select agents to cancel", repoRoot, nil)
	for _, idx := range indices {
		ops.CancelOne(os.Stdout, repoRoot, idx)
	}
}

func tuiActionHandler(repoRoot string) func(tui.TUIResult, string) tui.ActionResult {
	return func(result tui.TUIResult, spawnName string) tui.ActionResult {
		// All ops output is discarded — the TUI refreshes agent state
		// automatically, so no feedback text is needed.
		var sink bytes.Buffer
		idx := result.AgentIdx

		switch result.Action {
		case tui.ActionMerge:
			if idx >= 0 {
				err := ops.MergeOne(&sink, repoRoot, idx, ops.MergeTUI)
				if conflictErr, ok := err.(*ops.MergeConflictErr); ok {
					return tui.MenuRequest{
						Title: fmt.Sprintf("Conflict merging agent %d — how to resolve?", idx),
						Options: []string{
							"Continue — leave conflicts, resolve manually",
							"Abort — discard merge and reset",
						},
						Callback: func(choice int) tui.ActionResult {
							var sink2 bytes.Buffer
							// Remap choice: 0=continue, 1=abort (maps to default in ResolveConflict)
							if choice == 1 {
								choice = -1
							}
							ops.ResolveConflict(&sink2, conflictErr, choice)
							return tui.ActionDone{}
						},
					}
				}
			}

		case tui.ActionSpawn:
			ops.SpawnByName(&sink, repoRoot, spawnName)

		case tui.ActionAccept:
			if idx >= 0 {
				ops.AcceptOne(&sink, repoRoot, idx)
			}

		case tui.ActionReject:
			if idx >= 0 {
				ops.RejectOne(&sink, repoRoot, idx)
			}

		case tui.ActionCancel:
			if idx >= 0 {
				ops.CancelOne(&sink, repoRoot, idx)
			}

		case tui.ActionFocus:
			if idx >= 0 {
				ag, err := core.ReadAgent(repoRoot, idx)
				if err == nil {
					core.TmuxRun("select-window", "-t", ag.Window)
				}
			}
		}

		return tui.ActionDone{}
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