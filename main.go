package main

import (
	"fmt"
	"os"
	"strconv"
)

const version = "0.2.0"

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		cmdHelp()
		return
	}
	switch args[0] {
	case "spawn":
		cmdSpawn(args[1:])
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
	case "clean":
		cmdClean()
	case "help", "-h", "--help":
		cmdHelp()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		cmdHelp()
		os.Exit(1)
	}
}

func cmdSpawn(args []string) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		fatal(err.Error())
	}
	spawn(repoRoot, args)
}

func cmdMerge(args []string) {
	repoRoot, err := findRepoFromState()
	if err != nil {
		fatal(err.Error())
	}
	if len(args) > 0 && args[0] == "--all" {
		mergeAll(repoRoot)
		return
	}
	if len(args) > 0 {
		idx, err := strconv.Atoi(args[0])
		if err != nil {
			fatal("Invalid agent ID: %s", args[0])
		}
		mergeOne(repoRoot, idx)
		return
	}
	// No args — interactive picker (show unmerged agents)
	indices := runPicker("Select agents to merge", repoRoot, func(a *AgentState) bool {
		return a.MergeCommit == "" && a.Resolving == ""
	})
	for _, idx := range indices {
		mergeOne(repoRoot, idx)
	}
}

func cmdAccept(args []string) {
	repoRoot, err := findRepoFromState()
	if err != nil {
		fatal(err.Error())
	}
	if len(args) > 0 && args[0] == "--all" {
		acceptAll(repoRoot)
		return
	}
	if len(args) > 0 {
		idx, err := strconv.Atoi(args[0])
		if err != nil {
			fatal("Invalid agent ID: %s", args[0])
		}
		acceptOne(repoRoot, idx)
		return
	}
	// No args — interactive picker (show merged agents)
	indices := runPicker("Select agents to accept", repoRoot, func(a *AgentState) bool {
		return a.MergeCommit != ""
	})
	for _, idx := range indices {
		acceptOne(repoRoot, idx)
	}
}

func cmdReject(args []string) {
	repoRoot, err := findRepoFromState()
	if err != nil {
		fatal(err.Error())
	}
	if len(args) > 0 {
		idx, err := strconv.Atoi(args[0])
		if err != nil {
			fatal("Invalid agent ID: %s", args[0])
		}
		rejectOne(repoRoot, idx)
		return
	}
	// No args — interactive picker (show merged or resolving agents)
	indices := runPicker("Select agents to reject", repoRoot, func(a *AgentState) bool {
		return a.MergeCommit != "" || a.Resolving != ""
	})
	for _, idx := range indices {
		rejectOne(repoRoot, idx)
	}
}

func cmdCancel(args []string) {
	repoRoot, err := findRepoFromState()
	if err != nil {
		fatal(err.Error())
	}
	if len(args) > 0 {
		idx, err := strconv.Atoi(args[0])
		if err != nil {
			fatal("Invalid agent ID: %s", args[0])
		}
		cancelOne(repoRoot, idx)
		return
	}
	// No args — interactive picker (show all agents)
	indices := runPicker("Select agents to cancel", repoRoot, nil)
	for _, idx := range indices {
		cancelOne(repoRoot, idx)
	}
}

func cmdFinish() {
	repoRoot, err := findRepoFromState()
	if err != nil {
		fatal(err.Error())
	}
	fmt.Println("Finishing augmux session...")
	fmt.Println()
	finishAll(repoRoot)
}

func cmdClean() {
	repoRoot, err := findRepoFromState()
	if err != nil {
		fatal(err.Error())
	}
	agents := listAgents(repoRoot)
	msg := fmt.Sprintf("This will destroy ALL %d agent(s), their worktrees, branches, and all uncommitted work.\nThis action cannot be undone.", len(agents))
	if !runConfirm(msg) {
		fmt.Println("Aborted.")
		return
	}
	teardownAll(repoRoot)
}

func cmdHelp() {
	fmt.Printf(`augmux v%s — parallel AI agents with tmux + git worktrees

Usage:
  augmux spawn                        Open editor to write prompt, then spawn agent
  augmux spawn "name" ["name2" ...]   Create named agent(s) (agent CLI with no initial prompt)
  augmux status                       Show session status (grid view)
  augmux status --watch               Live dashboard (auto-refresh)
  augmux merge <id>                   Merge agent into source branch (keeps agent alive)
  augmux merge --all                  Merge all unmerged agents
  augmux accept <id>                  Accept a merged agent (teardown worktree + window)
  augmux accept --all                 Accept all merged agents
  augmux reject <id>                  Undo merge, keep agent alive to fix and re-merge
  augmux cancel <id>                  Remove agent and discard all its changes
  augmux finish                       Merge all + accept all (one-shot cleanup)
  augmux clean                        Force cleanup (no merge, discard all)
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
