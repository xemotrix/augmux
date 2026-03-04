package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	StateDirName     = ".augmux-state"
	WorktreesDirName = ".augmux-worktrees"
)

// Agent activity states.
const (
	ActivityIdle    = "idle"
	ActivityWorking = "working"
)

// IdleTimeout is how long a tmux pane must be inactive before the agent is
// considered idle.
const IdleTimeout = 10 * time.Second

// AgentState holds the state for a single agent.
type AgentState struct {
	Index       int
	Description string
	Branch      string
	Worktree    string
	MergeCommit string
	Resolving   string
	Activity    string // "idle" or "working"
	Window      string // tmux window name (e.g. "augmux-1")
}

// FindRepoRoot finds the git repo root from the current directory.
func FindRepoRoot() (string, error) {
	out, err := RunCmd("git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}
	return out, nil
}

// FindRepoFromState walks up from CWD looking for .augmux-state.
func FindRepoFromState() (string, error) {
	dir, _ := os.Getwd()
	for dir != "/" {
		if IsDir(filepath.Join(dir, StateDirName)) {
			return dir, nil
		}
		dir = filepath.Dir(dir)
	}
	gitRoot, err := FindRepoRoot()
	if err == nil && IsDir(filepath.Join(gitRoot, StateDirName)) {
		return gitRoot, nil
	}
	return "", fmt.Errorf("no augmux session found. Make sure you're inside the project repo where you ran 'augmux spawn'")
}

func StateDir(repoRoot string) string {
	return filepath.Join(repoRoot, StateDirName)
}

func WorktreeBase(repoRoot string) string {
	return filepath.Join(repoRoot, WorktreesDirName)
}

func SourceBranch(repoRoot string) string {
	return ReadFileContent(filepath.Join(StateDir(repoRoot), "source_branch"))
}

func TaskDir(repoRoot string, idx int) string {
	return filepath.Join(StateDir(repoRoot), fmt.Sprintf("task-%d", idx))
}

// ReadAgent reads the state for a single agent.
func ReadAgent(repoRoot string, idx int) (*AgentState, error) {
	td := TaskDir(repoRoot, idx)
	if !IsDir(td) {
		return nil, fmt.Errorf("agent %d not found", idx)
	}
	window := ReadFileContent(filepath.Join(td, "window"))
	if window == "" {
		window = fmt.Sprintf("augmux-%d", idx)
	}
	activity := detectActivity(window)
	return &AgentState{
		Index:       idx,
		Description: ReadFileContent(filepath.Join(td, "description")),
		Branch:      ReadFileContent(filepath.Join(td, "branch")),
		Worktree:    ReadFileContent(filepath.Join(td, "worktree")),
		MergeCommit: ReadFileContent(filepath.Join(td, "merge_commit")),
		Resolving:   ReadFileContent(filepath.Join(td, "resolving")),
		Activity:    activity,
		Window:      window,
	}, nil
}

// detectActivity checks the tmux pane's last activity timestamp to determine
// whether the agent is working or idle. If the pane had output within
// IdleTimeout, the agent is considered working.
func detectActivity(window string) string {
	epochStr, err := TmuxQuery("display-message", "-t", window, "-p", "#{pane_last_activity}")
	if err != nil {
		return ActivityIdle
	}
	epoch, err := strconv.ParseInt(epochStr, 10, 64)
	if err != nil {
		return ActivityIdle
	}
	lastActivity := time.Unix(epoch, 0)
	if time.Since(lastActivity) <= IdleTimeout {
		return ActivityWorking
	}
	return ActivityIdle
}

// ListAgents returns all agent indices, sorted.
func ListAgents(repoRoot string) []int {
	sd := StateDir(repoRoot)
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

// NextAgentIdx returns the lowest available agent index (starting from 1).
func NextAgentIdx(repoRoot string) int {
	taken := make(map[int]bool)
	for _, idx := range ListAgents(repoRoot) {
		taken[idx] = true
	}
	for i := 1; ; i++ {
		if !taken[i] {
			return i
		}
	}
}

// SafeName sanitizes a name for use in branch/path names.
func SafeName(name string) string {
	r := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-")
	s := r.Replace(strings.ToLower(strings.TrimSpace(name)))
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}
