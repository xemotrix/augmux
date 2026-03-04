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
		cmdStatus(args[1:])
	case "merge":
		cmdMerge(args[1:])
	case "accept":
		cmdAccept(args[1:])
	case "reject":
		cmdReject(args[1:])
	case "cancel":
		cmdCancel(args[1:])
	case "finish":
		cmdFinish()
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

func cmdStatus(args []string) {
	repoRoot, err := core.FindRepoFromState()
	if err != nil {
		core.Fatal(err.Error())
	}
	if len(args) > 0 && (args[0] == "--watch" || args[0] == "-w") {
		tui.RunInteractiveTUI(repoRoot, tuiActionHandler(repoRoot))
		return
	}
	fmt.Println(tui.RenderStatusView(repoRoot, 120))
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
		if err := ops.MergeOne(os.Stdout, repoRoot, idx, true); err != nil {
			core.Fatal(err.Error())
		}
		return
	}
	indices := tui.RunPicker("Select agents to merge", repoRoot, func(a *core.AgentState) bool {
		return a.MergeCommit == "" && a.Resolving == ""
	})
	for _, idx := range indices {
		ops.MergeOne(os.Stdout, repoRoot, idx, true)
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

func tuiActionHandler(repoRoot string) func(tui.TUIResult, string) []string {
	return func(result tui.TUIResult, spawnName string) []string {
		var buf bytes.Buffer
		switch result.Action {
		case tui.ActionSpawn:
			ops.SpawnByName(&buf, repoRoot, spawnName)
		case tui.ActionMerge:
			if result.AgentIdx >= 0 {
				ops.MergeOne(&buf, repoRoot, result.AgentIdx, false)
			}
		case tui.ActionAccept:
			if result.AgentIdx >= 0 {
				ops.AcceptOne(&buf, repoRoot, result.AgentIdx)
			}
		case tui.ActionReject:
			if result.AgentIdx >= 0 {
				ops.RejectOne(&buf, repoRoot, result.AgentIdx)
			}
		case tui.ActionCancel:
			if result.AgentIdx >= 0 {
				ops.CancelOne(&buf, repoRoot, result.AgentIdx)
			}
		case tui.ActionFinish:
			ops.FinishAll(&buf, repoRoot)
		case tui.ActionFocus:
			if result.AgentIdx >= 0 {
				ag, err := core.ReadAgent(repoRoot, result.AgentIdx)
				if err == nil {
					core.TmuxRun("select-window", "-t", ag.Window)
				}
			}
		}
		return nil
	}
}

func cmdFinish() {
	repoRoot, err := core.FindRepoFromState()
	if err != nil {
		core.Fatal(err.Error())
	}
	fmt.Println("Finishing augmux session...")
	fmt.Println()
	ops.FinishAll(os.Stdout, repoRoot)
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
  augmux finish                       Merge all + accept all (one-shot cleanup)
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
  8. augmux finish                    # merge + accept all remaining

`, version)
}