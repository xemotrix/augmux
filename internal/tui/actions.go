package tui

import (
	"fmt"
	"strings"

	"github.com/xemotrix/augmux/internal/core"
	"github.com/xemotrix/augmux/internal/ops"
)

// TUIAction represents an action the user triggered from the interactive TUI.
type TUIAction int

const (
	ActionSpawn TUIAction = iota
	ActionMerge
	ActionAccept
	ActionReject
	ActionCancel
	ActionFocus
	ActionRebase
)

// TUIResult holds the result of an interactive TUI session.
type TUIResult struct {
	Action   TUIAction
	AgentIdx int // selected agent index, or -1 if none
}

// ActionResult is returned by TUI action handlers. Implementations are
// ActionDone (simple output) and MenuRequest (inline menu needed).
type ActionResult interface {
	isActionResult()
}

// ActionDone signals that the action completed. Lines are shown as status.
type ActionDone struct {
	Lines []string
	Level ToastLevel
}

func (ActionDone) isActionResult() {}

// MenuRequest signals that the action needs user input via an inline menu.
type MenuRequest struct {
	Title    string
	Options  []string
	Lines    []string                      // status lines shown above the menu
	Callback func(choice int) ActionResult // called with selected index (-1 = cancelled)
}

func (MenuRequest) isActionResult() {}

// actionResultMsg wraps an ActionResult returned by the action handler.
type actionResultMsg struct {
	result ActionResult
}

func tuiActionHandler(repoRoot string) func(TUIResult, string) ActionResult {
	return func(result TUIResult, spawnName string) ActionResult {
		idx := result.AgentIdx

		agentLabel := fmt.Sprintf("agent %d", idx)
		if idx >= 0 {
			if ag, err := core.ReadAgent(repoRoot, idx); err == nil && ag.Description != "" {
				agentLabel = fmt.Sprintf("%q", ag.Description)
			}
		}

		switch result.Action {
		case ActionMerge:
			if idx >= 0 {
				err := ops.MergeOne(repoRoot, idx)
				if conflictErr, ok := err.(*ops.MergeConflictErr); ok {
					return MenuRequest{
						Title: fmt.Sprintf("Conflict merging %s — how to resolve?", agentLabel),
						Options: []string{
							"Continue — leave conflicts, resolve manually",
							"Abort — discard merge and reset",
						},
						Callback: func(choice int) ActionResult {
							if choice == 1 || choice == -1 {
								ops.ResolveConflict(conflictErr, -1)
								return ActionDone{
									Lines: []string{"Merge aborted"},
									Level: ToastWarning,
								}
							}
							ops.ResolveConflict(conflictErr, 0)
							return ActionDone{
								Lines: []string{"Conflicts left for manual resolution"},
								Level: ToastWarning,
							}
						},
					}
				}
				if err != nil {
					return ActionDone{
						Lines: []string{fmt.Sprintf("Merge failed: %s", err)},
						Level: ToastError,
					}
				}
				return ActionDone{
					Lines: []string{fmt.Sprintf("Merged %s", agentLabel)},
					Level: ToastSuccess,
				}
			}

		case ActionSpawn:
			if err := ops.SpawnByName(repoRoot, spawnName); err != nil {
				return ActionDone{
					Lines: []string{fmt.Sprintf("Spawn failed: %s", err)},
					Level: ToastError,
				}
			}
			return ActionDone{
				Lines: []string{fmt.Sprintf("Spawned %q", spawnName)},
				Level: ToastSuccess,
			}

		case ActionAccept:
			if idx >= 0 {
				if err := ops.AcceptOne(repoRoot, idx); err != nil {
					return ActionDone{
						Lines: []string{fmt.Sprintf("Accept failed: %s", err)},
						Level: ToastError,
					}
				}
				return ActionDone{
					Lines: []string{fmt.Sprintf("Accepted %s", agentLabel)},
					Level: ToastSuccess,
				}
			}

		case ActionReject:
			if idx >= 0 {
				if err := ops.RejectOne(repoRoot, idx); err != nil {
					return ActionDone{
						Lines: []string{fmt.Sprintf("Reject failed: %s", err)},
						Level: ToastError,
					}
				}
				return ActionDone{
					Lines: []string{fmt.Sprintf("Rejected %s", agentLabel)},
					Level: ToastSuccess,
				}
			}

		case ActionCancel:
			if idx >= 0 {
				ag, err := core.ReadAgent(repoRoot, idx)
				if err != nil {
					return ActionDone{
						Lines: []string{fmt.Sprintf("Cancel failed: %s", err)},
						Level: ToastError,
					}
				}

				srcBranch := core.SourceBranch(repoRoot)
				ahead := core.GitMust(repoRoot, "rev-list", "--count", srcBranch+".."+ag.Branch)
				aheadNum := 0
				fmt.Sscanf(ahead, "%d", &aheadNum)

				dirty := false
				if core.IsDir(ag.Worktree) {
					status, _ := core.Git(ag.Worktree, "status", "--porcelain")
					dirty = status != ""
				}

				if aheadNum > 0 || dirty {
					var warnings []string
					if aheadNum > 0 {
						commitWord := "commit"
						if aheadNum != 1 {
							commitWord = "commits"
						}
						warnings = append(warnings, fmt.Sprintf("%d %s", aheadNum, commitWord))
					}
					if dirty {
						warnings = append(warnings, "uncommitted changes")
					}
					detail := strings.Join(warnings, " and ")

					return MenuRequest{
						Title: fmt.Sprintf("Cancel %s? It has %s that will be lost.", agentLabel, detail),
						Options: []string{
							"Yes — discard and cancel",
							"No — keep agent",
						},
						Callback: func(choice int) ActionResult {
							if choice == 0 {
								if err := ops.CancelOne(repoRoot, idx); err != nil {
									return ActionDone{
										Lines: []string{fmt.Sprintf("Cancel failed: %s", err)},
										Level: ToastError,
									}
								}
								return ActionDone{
									Lines: []string{fmt.Sprintf("Cancelled %s", agentLabel)},
									Level: ToastSuccess,
								}
							}
							return ActionDone{
								Lines: []string{"Cancel aborted"},
								Level: ToastInfo,
							}
						},
					}
				}

				if err := ops.CancelOne(repoRoot, idx); err != nil {
					return ActionDone{
						Lines: []string{fmt.Sprintf("Cancel failed: %s", err)},
						Level: ToastError,
					}
				}
				return ActionDone{
					Lines: []string{fmt.Sprintf("Cancelled %s", agentLabel)},
					Level: ToastSuccess,
				}
			}

		case ActionRebase:
			if idx >= 0 {
				if err := ops.SendRebase(repoRoot, idx); err != nil {
					return ActionDone{
						Lines: []string{fmt.Sprintf("Rebase failed: %s", err)},
						Level: ToastError,
					}
				}
				return ActionDone{
					Lines: []string{fmt.Sprintf("Sent rebase command to %s", agentLabel)},
					Level: ToastSuccess,
				}
			}

		case ActionFocus:
			if idx >= 0 {
				ag, err := core.ReadAgent(repoRoot, idx)
				if err != nil {
					return ActionDone{
						Lines: []string{fmt.Sprintf("Failed to focus: %s", err)},
						Level: ToastError,
					}
				}
				core.TmuxRun("select-window", "-t", ag.Window)
			}
		}

		return ActionDone{}
	}
}
