package core

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
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

// IdleTimeout is how long the pane content must remain unchanged before the
// agent is considered idle.
const IdleTimeout = 1 * time.Second

// paneState tracks the last known content hash and when it last changed.
type paneState struct {
	hash       string
	lastChange time.Time
}

var (
	paneStates   = make(map[string]*paneState)
	paneStatesMu sync.Mutex
)

// Agent holds the state for a single agent.
type Agent struct {
	Index            int
	Description      string
	Branch           string
	Worktree         string
	MergeCommit      string
	Activity         string // "idle" or "working"
	Window           string // tmux window name (e.g. "ax-1-fix-auth")
	HasConflicts     bool   // true if merging this branch would produce conflicts
	CommitsAhead     int    // commits ahead of source branch
	UncommittedCount int    // number of uncommitted files in worktree
}

type AgentStatus byte

const (
	AgentStatusNone AgentStatus = iota // for where there are no agents
	AgentStatusWip
	AgentStatusMerged
	AgentStatusIdle
	AgentStatusWorking
	AgentStatusConflict
)

func (a *Agent) Status() AgentStatus {
	if a == nil {
		return AgentStatusNone
	}
	if a.HasConflicts {
		return AgentStatusConflict
	}
	if a.MergeCommit != "" {
		return AgentStatusMerged
	}
	if a.Activity == ActivityWorking {
		return AgentStatusWorking
	}
	if a.Activity == ActivityIdle {
		return AgentStatusIdle
	}
	return AgentStatusWip
}

// conflictCacheEntry stores a cached merge-tree result keyed on the resolved
// refs of both branches. The expensive merge-tree call is skipped when
// neither branch has moved since the last check.
type conflictCacheEntry struct {
	srcRef   string
	agentRef string
	result   bool
}

var (
	conflictCache   = make(map[string]*conflictCacheEntry) // key: agentBranch name
	conflictCacheMu sync.Mutex
)

// DetectConflicts checks whether merging agentBranch into srcBranch would
// produce conflicts. It uses "git merge-tree --write-tree" which performs an
// in-memory merge without touching the working tree or index. Results are
// cached by branch tip refs so the expensive merge-tree only runs when a
// branch actually moves.
func DetectConflicts(repoRoot, srcBranch, agentBranch string) bool {
	srcRef, err1 := Git(repoRoot, "rev-parse", srcBranch)
	agentRef, err2 := Git(repoRoot, "rev-parse", agentBranch)
	if err1 != nil || err2 != nil {
		_, err := Git(repoRoot, "merge-tree", "--write-tree", srcBranch, agentBranch)
		return err != nil
	}

	conflictCacheMu.Lock()
	defer conflictCacheMu.Unlock()

	if entry, ok := conflictCache[agentBranch]; ok {
		if entry.srcRef == srcRef && entry.agentRef == agentRef {
			return entry.result
		}
	}

	_, err := Git(repoRoot, "merge-tree", "--write-tree", srcBranch, agentBranch)
	hasConflicts := err != nil

	conflictCache[agentBranch] = &conflictCacheEntry{
		srcRef:   srcRef,
		agentRef: agentRef,
		result:   hasConflicts,
	}
	return hasConflicts
}

// ClearConflictCache removes the cached conflict result for a branch.
func ClearConflictCache(agentBranch string) {
	conflictCacheMu.Lock()
	defer conflictCacheMu.Unlock()
	delete(conflictCache, agentBranch)
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
func ReadAgent(repoRoot string, idx int) (*Agent, error) {
	td := TaskDir(repoRoot, idx)
	if !IsDir(td) {
		return nil, fmt.Errorf("agent %d not found", idx)
	}
	window := ReadFileContent(filepath.Join(td, "window"))
	if window == "" {
		window = fmt.Sprintf("augmux-%d", idx)
	}
	activity := detectActivity(window)
	return &Agent{
		Index:       idx,
		Description: ReadFileContent(filepath.Join(td, "description")),
		Branch:      ReadFileContent(filepath.Join(td, "branch")),
		Worktree:    ReadFileContent(filepath.Join(td, "worktree")),
		MergeCommit: ReadFileContent(filepath.Join(td, "merge_commit")),
		Activity:    activity,
		Window:      window,
	}, nil
}

// EnrichAgent populates derived fields (CommitsAhead, UncommittedCount,
// HasConflicts) that require git operations. Call once per refresh cycle
// rather than recomputing in every consumer.
func EnrichAgent(repoRoot, srcBranch string, a *Agent) {
	if a.Branch != "" && srcBranch != "" {
		ahead, err := Git(repoRoot, "rev-list", "--count", srcBranch+".."+a.Branch)
		if err == nil {
			fmt.Sscanf(ahead, "%d", &a.CommitsAhead)
		}
	}

	if IsDir(a.Worktree) {
		status, _ := Git(a.Worktree, "status", "--porcelain")
		if status != "" {
			a.UncommittedCount = len(strings.Split(status, "\n"))
		}
	}

	if a.MergeCommit == "" && a.Branch != "" {
		a.HasConflicts = DetectConflicts(repoRoot, srcBranch, a.Branch)
	}
}

// capturePaneHash captures the tmux pane content and returns a hex-encoded
// SHA-256 hash, or an empty string on error.
func capturePaneHash(window string) string {
	content, err := TmuxQuery("capture-pane", "-t", window, "-p")
	if err != nil {
		return ""
	}
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}

// detectActivity captures the tmux pane content, hashes it, and compares with
// the previous snapshot. If the content changed, the agent is "working". If it
// has stayed the same for longer than IdleTimeout, the agent is "idle".
func detectActivity(window string) string {
	hash := capturePaneHash(window)
	if hash == "" {
		return ActivityIdle
	}

	now := time.Now()

	paneStatesMu.Lock()
	defer paneStatesMu.Unlock()

	ps, ok := paneStates[window]
	if !ok || ps.hash != hash {
		// First check or content changed — mark as working.
		paneStates[window] = &paneState{hash: hash, lastChange: now}
		return ActivityWorking
	}

	// Content unchanged — check how long it's been stable.
	if now.Sub(ps.lastChange) > IdleTimeout {
		return ActivityIdle
	}
	return ActivityWorking
}

// ReadAndEnrichAgents reads and enriches all agents in parallel.
// Each agent's ReadAgent + EnrichAgent runs in its own goroutine.
func ReadAndEnrichAgents(repoRoot string, indices []int, srcBranch string) []*Agent {
	results := make([]*Agent, len(indices))
	var wg sync.WaitGroup
	for i, idx := range indices {
		wg.Add(1)
		go func(i, idx int) {
			defer wg.Done()
			a, err := ReadAgent(repoRoot, idx)
			if err != nil {
				return
			}
			EnrichAgent(repoRoot, srcBranch, a)
			results[i] = a
		}(i, idx)
	}
	wg.Wait()

	var out []*Agent
	for _, a := range results {
		if a != nil {
			out = append(out, a)
		}
	}
	return out
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
