package ops

import (
	"fmt"
	"io"
	"os"

	"github.com/xemotrix/augmux/internal/components"
	"github.com/xemotrix/augmux/internal/core"
)

// MergeMode controls how MergeOne handles interactive decision points.
type MergeMode int

const (
	// MergeInteractive shows tui.RunMenu prompts for conflicts and uncommitted
	// changes. Used by the CLI commands (augmux merge).
	MergeInteractive MergeMode = iota

	// MergeTUI auto-commits uncommitted changes and returns *MergeConflictErr
	// (without resetting the working tree) when conflicts are found, so the
	// caller can show an inline menu inside the TUI.
	MergeTUI
)

// MergeConflictErr is returned by MergeOne in MergeTUI mode when conflicts
// are detected. The working tree is left in the conflicted state so the
// caller can resolve or abort.
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

func MergeOne(w io.Writer, repoRoot string, idx int, mode MergeMode) error {
	repoRoot = core.MustAbs(repoRoot)
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
		unmerged := core.GitMust(repoRoot, "diff", "--name-only", "--diff-filter=U")
		porcelain := core.GitMust(repoRoot, "status", "--porcelain")
		if unmerged != "" || porcelain != "" {
			return fmt.Errorf("still resolving")
		}
		sha := core.GitMust(repoRoot, "rev-parse", "HEAD")
		core.WriteFileContent(td+"/merge_commit", sha)
		os.Remove(td + "/resolving")
		return nil
	}

	// Check uncommitted changes in worktree
	if core.IsDir(ag.Worktree) {
		porcelain := core.GitMust(ag.Worktree, "status", "--porcelain")
		if porcelain != "" {
			if mode == MergeInteractive {
				choice := components.RunSelectMenu("Uncommitted changes — cannot merge", []string{
					"Commit them now with a default message",
					"Abort merge for this agent",
				})
				if choice == 0 {
					core.GitMust(ag.Worktree, "add", "-A")
					core.GitMust(ag.Worktree, "commit", "-m", "augmux: auto-commit uncommitted changes")
				} else {
					return fmt.Errorf("aborted")
				}
			} else {
				// MergeTUI: auto-commit uncommitted changes
				core.GitMust(ag.Worktree, "add", "-A")
				core.GitMust(ag.Worktree, "commit", "-m", "augmux: auto-commit uncommitted changes")
			}
		}
	}

	// Checkout source branch
	core.Git(repoRoot, "checkout", srcBranch)

	// Count ahead
	ahead := core.GitMust(repoRoot, "rev-list", "--count", srcBranch+".."+ag.Branch)
	if ahead == "0" || ahead == "" {
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
			sha := core.GitMust(repoRoot, "rev-parse", "HEAD")
			core.WriteFileContent(td+"/merge_commit", sha)
			return nil
		}
	}

	// Conflict path
	switch mode {
	case MergeTUI:
		// Return conflict info without resetting — caller shows inline menu.
		files := core.GitMust(repoRoot, "diff", "--name-only", "--diff-filter=U")
		return &MergeConflictErr{
			RepoRoot:  repoRoot,
			AgentIdx:  idx,
			AgentDesc: ag.Description,
			MergeMsg:  mergeMsg,
			Files:     files,
		}

	default: // MergeInteractive
		result := handleConflict(repoRoot)
		switch result {
		case conflictContinue:
			core.WriteFileContent(td+"/resolving", mergeMsg)
			return fmt.Errorf("resolving conflicts")
		default:
			return fmt.Errorf("aborted")
		}
	}
}
