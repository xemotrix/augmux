package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	stateDirName     = ".augmux-state"
	worktreesDirName = ".augmux-worktrees"
)

// AgentState holds the state for a single agent.
type AgentState struct {
	Index       int
	Description string
	Branch      string
	Worktree    string
	MergeCommit string
	Resolving   string
}

// findRepoRoot finds the git repo root from the current directory.
func findRepoRoot() (string, error) {
	out, err := runCmd("git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}
	return out, nil
}

// findRepoFromState walks up from CWD looking for .augmux-state.
func findRepoFromState() (string, error) {
	dir, _ := os.Getwd()
	for dir != "/" {
		if isDir(filepath.Join(dir, stateDirName)) {
			return dir, nil
		}
		dir = filepath.Dir(dir)
	}
	gitRoot, err := findRepoRoot()
	if err == nil && isDir(filepath.Join(gitRoot, stateDirName)) {
		return gitRoot, nil
	}
	return "", fmt.Errorf("no augmux session found. Make sure you're inside the project repo where you ran 'augmux spawn'")
}

func stateDir(repoRoot string) string {
	return filepath.Join(repoRoot, stateDirName)
}

func worktreeBase(repoRoot string) string {
	return filepath.Join(repoRoot, worktreesDirName)
}

func sourceBranch(repoRoot string) string {
	return readFileContent(filepath.Join(stateDir(repoRoot), "source_branch"))
}

func taskDir(repoRoot string, idx int) string {
	return filepath.Join(stateDir(repoRoot), fmt.Sprintf("task-%d", idx))
}

// readAgent reads the state for a single agent.
func readAgent(repoRoot string, idx int) (*AgentState, error) {
	td := taskDir(repoRoot, idx)
	if !isDir(td) {
		return nil, fmt.Errorf("agent %d not found", idx)
	}
	return &AgentState{
		Index:       idx,
		Description: readFileContent(filepath.Join(td, "description")),
		Branch:      readFileContent(filepath.Join(td, "branch")),
		Worktree:    readFileContent(filepath.Join(td, "worktree")),
		MergeCommit: readFileContent(filepath.Join(td, "merge_commit")),
		Resolving:   readFileContent(filepath.Join(td, "resolving")),
	}, nil
}

// listAgents returns all agent indices, sorted.
func listAgents(repoRoot string) []int {
	sd := stateDir(repoRoot)
	entries, err := os.ReadDir(sd)
	if err != nil {
		return nil
	}
	var indices []int
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "task-") {
			n, err := strconv.Atoi(strings.TrimPrefix(e.Name(), "task-"))
			if err == nil {
				indices = append(indices, n)
			}
		}
	}
	sort.Ints(indices)
	return indices
}

// nextAgentIdx returns the next available agent index.
func nextAgentIdx(repoRoot string) int {
	max := 0
	for _, idx := range listAgents(repoRoot) {
		if idx > max {
			max = idx
		}
	}
	return max + 1
}
