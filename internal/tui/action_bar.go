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
		{"f#cus", "o", status == core.AgentStatusNone},
		{"#pawn", "s", true},
		{"#erge", "m", status == core.AgentStatusWip && hasCommits},
		{"re#ase", "b", status == core.AgentStatusConflict},
		{"d#tails", "e", hasCommits},
		{"#ccept", "a", status == core.AgentStatusMerged},
		{"#eject", "r", status == core.AgentStatusMerged},
		{"#ancel", "c", status == core.AgentStatusWip},
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
