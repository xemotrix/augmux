package ops

import (
	"fmt"

	"github.com/xemotrix/augmux/internal/core"
)

// SendRebase types the /augmux-rebase slash command into the agent's
// existing tmux pane, presses Enter to submit it, and switches focus to that
// window. Uses the interactive variant so the user can review conflicts.
func SendRebase(repoRoot string, idx int) error {
	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		return fmt.Errorf("agent %d not found: %w", idx, err)
	}
	if err := core.TmuxRun("send-keys", "-t", ag.Window, "/augmux-rebase", "Enter"); err != nil {
		return err
	}
	return core.TmuxRun("select-window", "-t", ag.Window)
}
