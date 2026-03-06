package ops

import (
	"fmt"
	"os"

	"github.com/xemotrix/augmux/internal/core"
)

// MergeConflictErr is returned by MergeOne when conflicts are detected.
// The working tree is left in the conflicted state so the caller can
// resolve or abort.
type MergeConflictErr struct {
	RepoRoot  string
	AgentIdx  int
	AgentDesc string
	MergeMsg  string
	Files     string // newline-separated list of conflicting files
}

func (e *MergeConflictErr) Error() string {
	return "merge conflicts detected"
}

func MergeOne(repoRoot string, idx int) error {
	var err error
	repoRoot, err = core.Abs(repoRoot)
	if err != nil {
		return err
	}
	td := core.TaskDir(repoRoot, idx)

	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		return err
	}

	// Already merged?
	if ag.MergeCommit != "" {
		return fmt.Errorf("already merged")
	}

	srcBranch := core.SourceBranch(repoRoot)

	// Check resolving state (user came back after fixing conflicts)
	if ag.Resolving != "" {
		unmerged, _ := core.Git(repoRoot, "diff", "--name-only", "--diff-filter=U")
		porcelain, _ := core.Git(repoRoot, "status", "--porcelain")
		if unmerged != "" || porcelain != "" {
			return fmt.Errorf("still resolving")
		}
		sha, err := core.Git(repoRoot, "rev-parse", "HEAD")
		if err != nil {
			return fmt.Errorf("failed to get HEAD sha: %w", err)
		}
		core.WriteFileContent(td+"/merge_commit", sha)
		os.Remove(td + "/resolving")
		return nil
	}

	// Ensure the main repo working tree is clean before we touch it.
	mainStatus, _ := core.Git(repoRoot, "status", "--porcelain")
	if mainStatus != "" {
		return fmt.Errorf("main repo has uncommitted changes; commit or stash before merging")
	}

	// Auto-commit uncommitted changes in worktree
	if core.IsDir(ag.Worktree) {
		porcelain, _ := core.Git(ag.Worktree, "status", "--porcelain")
		if porcelain != "" {
			if _, err := core.Git(ag.Worktree, "add", "-A"); err != nil {
				return fmt.Errorf("failed to stage changes in worktree: %w", err)
			}
			if _, err := core.Git(ag.Worktree, "commit", "-m", "augmux: auto-commit uncommitted changes"); err != nil {
				return fmt.Errorf("failed to auto-commit in worktree: %w", err)
			}
		}
	}

	// Checkout source branch
	if _, err := core.Git(repoRoot, "checkout", srcBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", srcBranch, err)
	}

	// Count ahead
	ahead, err := core.Git(repoRoot, "rev-list", "--count", srcBranch+".."+ag.Branch)
	if err != nil || ahead == "0" || ahead == "" {
		teardownOne(repoRoot, idx)
		return nil
	}

	// Merge message
	lastMsg := core.GitMust(repoRoot, "log", "-1", "--format=%s", ag.Branch)
	mergeMsg := "augmux: " + lastMsg

	// Try squash merge + commit
	_, mergeErr := core.Git(repoRoot, "merge", "--squash", ag.Branch)
	if mergeErr == nil {
		if _, commitErr := core.Git(repoRoot, "commit", "-m", mergeMsg); commitErr == nil {
			sha, err := core.Git(repoRoot, "rev-parse", "HEAD")
			if err != nil {
				return fmt.Errorf("merge succeeded but failed to record commit sha: %w", err)
			}
			core.WriteFileContent(td+"/merge_commit", sha)
			return nil
		}
	}

	// Return conflict info without resetting — caller shows inline menu.
	files := core.GitMust(repoRoot, "diff", "--name-only", "--diff-filter=U")
	return &MergeConflictErr{
		RepoRoot:  repoRoot,
		AgentIdx:  idx,
		AgentDesc: ag.Description,
		MergeMsg:  mergeMsg,
		Files:     files,
	}
}
