package ops

import (
	"fmt"

	"github.com/xemotrix/augmux/internal/core"
)

// SendRebase types the /@augmux-rebase slash command into the agent's existing
// tmux pane and switches focus to that window. The user must press Enter to
// submit the command.
func SendRebase(repoRoot string, idx int) error {
	ag, err := core.ReadAgent(repoRoot, idx)
	if err != nil {
		return fmt.Errorf("agent %d not found: %w", idx, err)
	}
	if err := core.TmuxRun("send-keys", "-t", ag.Window, "/augmux-rebase"); err != nil {
		return err
	}
	return core.TmuxRun("select-window", "-t", ag.Window)
}
