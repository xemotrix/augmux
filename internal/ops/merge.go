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
		if err := core.WriteFileContent(td+"/merge_commit", sha); err != nil {
			return fmt.Errorf("failed to record merge commit: %w", err)
		}
		os.Remove(td + "/resolving")
		return nil
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

	// Count ahead
	ahead, err := core.Git(repoRoot, "rev-list", "--count", srcBranch+".."+ag.Branch)
	if err != nil || ahead == "0" || ahead == "" {
		teardownOne(repoRoot, idx)
		return nil
	}

	lastMsg := core.GitMust(repoRoot, "log", "-1", "--format=%s", ag.Branch)
	mergeMsg := "augmux: " + lastMsg

	// Try plumbing-level merge (no checkout required).
	treeOID, _, mergeTreeErr := core.GitWithStderr(repoRoot, "merge-tree", "--write-tree", srcBranch, ag.Branch)
	if mergeTreeErr == nil {
		srcHead, err := core.Git(repoRoot, "rev-parse", srcBranch)
		if err != nil {
			return fmt.Errorf("failed to resolve %s: %w", srcBranch, err)
		}
		commitOID, err := core.Git(repoRoot, "commit-tree", treeOID, "-p", srcHead, "-m", mergeMsg)
		if err != nil {
			return fmt.Errorf("failed to create merge commit: %w", err)
		}
		// CAS update — fails if srcBranch moved since we read srcHead.
		if _, err := core.Git(repoRoot, "update-ref", "refs/heads/"+srcBranch, commitOID, srcHead); err != nil {
			return fmt.Errorf("branch %s moved during merge, try again: %w", srcBranch, err)
		}
		if err := core.WriteFileContent(td+"/merge_commit", commitOID); err != nil {
			return fmt.Errorf("failed to record merge commit: %w", err)
		}
		return nil
	}

	// Conflicts detected — fall back to checkout + merge --squash for manual resolution.
	mainStatus, _ := core.Git(repoRoot, "status", "--porcelain")
	if mainStatus != "" {
		return fmt.Errorf("main repo has uncommitted changes; commit or stash before merging")
	}
	if _, err := core.Git(repoRoot, "checkout", srcBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", srcBranch, err)
	}
	core.Git(repoRoot, "merge", "--squash", ag.Branch)

	files := core.GitMust(repoRoot, "diff", "--name-only", "--diff-filter=U")
	return &MergeConflictErr{
		RepoRoot:  repoRoot,
		AgentIdx:  idx,
		AgentDesc: ag.Description,
		MergeMsg:  mergeMsg,
		Files:     files,
	}
}
