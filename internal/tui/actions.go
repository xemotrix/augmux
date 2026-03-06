package tui

import (
	"fmt"
	"strings"

	"github.com/xemotrix/augmux/internal/core"
	"github.com/xemotrix/augmux/internal/ops"
)

type ActionHandler struct {
	repoRoot string
}

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
	Action     TUIAction
	AgentIdx   int  // selected agent index, or -1 if none
	CommitRule bool // when true, agent rules instruct committing after changes
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

func validAction(action TUIAction, status core.AgentStatus) bool {
	switch action {
	case ActionSpawn:
		return true
	case ActionFocus:
		return status != core.AgentStatusNone
	case ActionMerge:
		return status == core.AgentStatusWip
	case ActionAccept, ActionReject:
		return status == core.AgentStatusMerged
	case ActionCancel:
		return status == core.AgentStatusWip ||
			status == core.AgentStatusIdle ||
			status == core.AgentStatusWorking
	case ActionRebase:
		return status == core.AgentStatusConflict
	default:
		return false
	}
}

func (ah *ActionHandler) handleMerge(idx int, agentLabel string) ActionResult {
	if idx == -1 {
		return ActionDone{}
	}
	if err := ops.Merge(ah.repoRoot, idx); err != nil {
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

func (ah *ActionHandler) handleSpawn(spawnName string, commitRule bool) ActionResult {
	if err := ops.SpawnByName(ah.repoRoot, spawnName, commitRule); err != nil {
		return ActionDone{
			Lines: []string{fmt.Sprintf("Spawn failed: %s", err)},
			Level: ToastError,
		}
	}
	return ActionDone{
		Lines: []string{fmt.Sprintf("Spawned %q", spawnName)},
		Level: ToastSuccess,
	}
}

func (ah *ActionHandler) handleAccept(idx int, agentLabel string) ActionResult {
	if idx == -1 {
		return ActionDone{}
	}
	if err := ops.Accept(ah.repoRoot, idx); err != nil {
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

func (ah *ActionHandler) handleReject(idx int, agentLabel string) ActionResult {
	if idx == -1 {
		return ActionDone{}
	}
	if err := ops.Reject(ah.repoRoot, idx); err != nil {
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

func (ah *ActionHandler) handleFocus(idx int) ActionResult {
	if idx == -1 {
		return ActionDone{}
	}
	ag, err := core.ReadAgent(ah.repoRoot, idx)
	if err != nil {
		return ActionDone{
			Lines: []string{fmt.Sprintf("Failed to focus: %s", err)},
			Level: ToastError,
		}
	}
	core.TmuxRun("select-window", "-t", ag.Window)
	return ActionDone{}
}

func (ah *ActionHandler) handleRebase(idx int, agentLabel string) ActionResult {
	if idx == -1 {
		return ActionDone{}
	}
	if err := ops.SendRebase(ah.repoRoot, idx); err != nil {
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

func (ah *ActionHandler) confirmCancelRequest(idx int, agentLabel, detail string) MenuRequest {
	return MenuRequest{
		Title: fmt.Sprintf("Cancel %s? It has %s that will be lost.", agentLabel, detail),
		Options: []string{
			"Yes — discard and cancel",
			"No — keep agent",
		},
		Callback: func(choice int) ActionResult {
			if choice != 0 {
				return ActionDone{
					Lines: []string{"Cancel aborted"},
					Level: ToastInfo,
				}
			}
			if err := ops.Cancel(ah.repoRoot, idx); err != nil {
				return ActionDone{
					Lines: []string{fmt.Sprintf("Cancel failed: %s", err)},
					Level: ToastError,
				}
			}
			return ActionDone{
				Lines: []string{fmt.Sprintf("Cancelled %s", agentLabel)},
				Level: ToastSuccess,
			}
		},
	}
}

func (ah *ActionHandler) handleCancel(idx int, agentLabel string) ActionResult {
	if idx == -1 {
		return ActionDone{}
	}
	ag, err := core.ReadAgent(ah.repoRoot, idx)
	if err != nil {
		return ActionDone{
			Lines: []string{fmt.Sprintf("Cancel failed: %s", err)},
			Level: ToastError,
		}
	}
	if ag == nil {
		return ActionDone{}
	}

	if ag.UncommittedCount > 0 || ag.CommitsAhead > 0 {
		var warnings []string
		if ag.CommitsAhead > 0 {
			commitWord := "commits"
			if ag.CommitsAhead == 1 {
				commitWord = "commit"
			}
			warnings = append(warnings, fmt.Sprintf("%d %s", ag.CommitsAhead, commitWord))
		}
		if ag.UncommittedCount > 0 {
			uncommitWord := "uncommitted changes"
			if ag.UncommittedCount == 1 {
				uncommitWord = "uncommitted change"
			}
			warnings = append(warnings, fmt.Sprintf("%d %s", ag.UncommittedCount, uncommitWord))
		}
		detail := strings.Join(warnings, " and ")
		return ah.confirmCancelRequest(idx, agentLabel, detail)
	}

	if err := ops.Cancel(ah.repoRoot, idx); err != nil {
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

func (ah *ActionHandler) Handle(result TUIResult, spawnName string) ActionResult {
	idx := result.AgentIdx

	agentLabel := fmt.Sprintf("agent %d", idx)
	var ag *core.Agent
	if idx >= 0 {
		agents := core.ReadAndEnrichAgents(ah.repoRoot, []int{idx})
		if len(agents) != 1 {
			return ActionDone{
				Lines: []string{fmt.Sprintf("Agent %d no longer exists", idx)},
				Level: ToastWarning,
			}
		}
		ag = agents[0]

		if ag.Description != "" {
			agentLabel = fmt.Sprintf("%q", ag.Description)
		}
	}

	if !validAction(result.Action, ag.Status()) {
		return ActionDone{}
	}

	switch result.Action {
	case ActionMerge:
		return ah.handleMerge(idx, agentLabel)
	case ActionSpawn:
		return ah.handleSpawn(spawnName, result.CommitRule)
	case ActionAccept:
		return ah.handleAccept(idx, agentLabel)
	case ActionReject:
		return ah.handleReject(idx, agentLabel)
	case ActionRebase:
		return ah.handleRebase(idx, agentLabel)
	case ActionFocus:
		return ah.handleFocus(idx)
	case ActionCancel:
		return ah.handleCancel(idx, agentLabel)
	default:
		return ActionDone{}
	}
}
