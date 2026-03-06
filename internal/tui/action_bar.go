package tui

import (
	"strings"

	"github.com/xemotrix/augmux/internal/core"
	"github.com/xemotrix/augmux/internal/styles"
)

type ActionBar struct{}

func (ab *ActionBar) View(selectedAgent *core.Agent, width int) string {
	type action struct {
		name      string
		highlight string
		enabled   bool
	}
	status := selectedAgent.Status()
	hasCommits := selectedAgent != nil && selectedAgent.CommitsAhead > 0

	actions := []action{
		{"f#cus", "o", validAction(ActionFocus, status)},
		{"#pawn", "s", validAction(ActionSpawn, status)},
		{"#erge", "m", validAction(ActionMerge, status) && hasCommits},
		{"re#ase", "b", validAction(ActionRebase, status)},
		{"#ccept", "a", validAction(ActionAccept, status)},
		{"#eject", "r", validAction(ActionReject, status)},
		{"#ancel", "c", validAction(ActionCancel, status)},
	}

	var actionBlocks []string
	for _, act := range actions {
		var rendered string
		if act.enabled {
			enabledSgmts := make([]string, 0)
			for sgmt := range strings.SplitSeq(act.name, "#") {
				enabledSgmts = append(enabledSgmts, styles.EnabledStyle.Render(sgmt))
			}
			accentSgmnt := styles.AccentStyle.Render(act.highlight)
			rendered = strings.Join(enabledSgmts, accentSgmnt)
		} else {
			name := strings.ReplaceAll(act.name, "#", act.highlight)
			rendered = styles.DisabledStyle.Render(name)
		}
		actionBlocks = append(actionBlocks, rendered)
	}

	blocks := strings.Join(actionBlocks, " ")

	return blocks
}
