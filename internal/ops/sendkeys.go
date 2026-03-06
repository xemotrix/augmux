package ops

import (
	"fmt"

	"github.com/xemotrix/augmux/internal/core"
)

// SendRebase types the /augmux-yolobase slash command into the agent's
// existing tmux pane, presses Enter to submit it, and switches focus to that
// window. Uses the yolo variant so the rebase runs automatically without
// user confirmation.
func SendRebase(repoRoot string, idx int) error {
	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		return fmt.Errorf("agent %d not found: %w", idx, err)
	}
	if err := core.TmuxRun("send-keys", "-t", ag.Window, "/augmux-yolobase", "Enter"); err != nil {
		return err
	}
	return core.TmuxRun("select-window", "-t", ag.Window)
}
