package ops

import (
	"fmt"

	"github.com/xemotrix/augmux/internal/core"
)

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

	if ag.MergeCommit != "" {
		return fmt.Errorf("already merged")
	}

	srcBranch := core.SourceBranch(repoRoot)

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

	lastMsg := core.GitMust(repoRoot, "log", "-1", "--format=%s", ag.Branch)
	mergeMsg := "augmux: " + lastMsg

	// Try squash merge + commit; reject if conflicts arise.
	_, mergeErr := core.Git(repoRoot, "merge", "--squash", ag.Branch)
	if mergeErr != nil {
		core.GitMust(repoRoot, "reset", "--hard", "HEAD")
		return fmt.Errorf("merge has conflicts — resolve conflicts before merging")
	}

	if _, commitErr := core.Git(repoRoot, "commit", "-m", mergeMsg); commitErr != nil {
		core.GitMust(repoRoot, "reset", "--hard", "HEAD")
		return fmt.Errorf("failed to commit merge: %w", commitErr)
	}

	sha, err := core.Git(repoRoot, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("merge succeeded but failed to record commit sha: %w", err)
	}
	core.WriteFileContent(td+"/merge_commit", sha)
	return nil
}
