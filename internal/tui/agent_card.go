package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/xemotrix/augmux/internal/core"
	"github.com/xemotrix/augmux/internal/styles"
)

func RunAgentCard(
	a *core.AgentState,
	repoRoot, srcBranch, spinnerFrame string,
	selected ...bool,
) string {
	sel := len(selected) > 0 && selected[0]

	// Compute commits ahead early — needed for border styles.Color and branch line
	ahead := core.GitMust(repoRoot, "rev-list", "--count", srcBranch+".."+a.Branch)
	if ahead == "" {
		ahead = "?"
	}
	aheadNum := 0
	fmt.Sscanf(ahead, "%d", &aheadNum)

	borderColor := agentBorderColor(a, aheadNum)
	bdr := lipgloss.NewStyle().Foreground(borderColor)
	if sel {
		bdr = bdr.Bold(true)
	}

	// Border characters: thicker (double-line) when selected
	var cTL, cTR, cBL, cBR, cH, cV string
	if sel {
		cTL, cTR, cBL, cBR, cH, cV = "╔", "╗", "╚", "╝", "═", "║"
	} else {
		cTL, cTR, cBL, cBR, cH, cV = "╭", "╮", "╰", "╯", "─", "│"
	}

	statusRaw := AgentStatusRaw(a)
	statusStyled := AgentStatusStyled(a, statusRaw)

	// Top border: ╭─❮3❯──────────────────────── ● wip ─╮
	idLabel := fmt.Sprintf("❮%d❯", a.Index)
	idStyled := lipgloss.NewStyle().Bold(true).Foreground(styles.ColorWhite).Render(idLabel)

	// cardWidth - "╭─"(2) - idLabel - fill - " status "(1+statusRaw+1) - "─╮"(2)
	used := 2 + lipgloss.Width(idLabel) + 1 + lipgloss.Width(statusRaw) + 1 + 2
	fill := max(cardWidth-used, 1)
	topLine := bdr.Render(cTL+cH) + idStyled +
		bdr.Render(strings.Repeat(cH, fill)) +
		" " + statusStyled + " " +
		bdr.Render(cH+cTR)

	// Description line with activity indicator on the right
	activityStr := activityIndicator(a, spinnerFrame)
	activityRaw := activityRawStr(a)
	maxName := max(textWidth-lipgloss.Width(activityRaw)-1, 10)
	name := truncate(a.Description, maxName)
	namePad := max(textWidth-lipgloss.Width(name)-lipgloss.Width(activityRaw), 1)
	nameLine := bdr.Render(cV) + " " + styles.ValueStyle.Render(name) +
		strings.Repeat(" ", namePad) + activityStr + " " + bdr.Render(cV)

	// Branch line
	aheadStr := ahead + " ↑"

	// Count uncommitted changes in the worktree
	dirty := countUncommitted(a.Worktree)
	dirtyStr := fmt.Sprintf("%d ✎", dirty)

	rightInfo := aheadStr + "  " + dirtyStr
	iconWidth := 2 // branchIcon + space
	maxBranch := textWidth - lipgloss.Width(rightInfo) - 1 - iconWidth
	branch := truncate(a.Branch, maxBranch)
	branchGap := max(textWidth-lipgloss.Width(branch)-iconWidth-lipgloss.Width(rightInfo), 1)
	branchLine := bdr.Render(cV) + " " + styles.RenderBranch(branch) +
		strings.Repeat(" ", branchGap) +
		styles.AheadStyle.Render(aheadStr) + "  " + styles.DirtyStyle.Render(dirtyStr) +
		" " + bdr.Render(cV)

	// Bottom border
	bottomLine := bdr.Render(cBL + strings.Repeat(cH, cardWidth-2) + cBR)

	return topLine + "\n" + nameLine + "\n" + branchLine + "\n" + bottomLine
}

func activityIndicator(a *core.AgentState, spinnerFrame string) string {
	if a.Activity == core.ActivityWorking {
		return lipgloss.NewStyle().Foreground(styles.ColorYellow).Render(spinnerFrame + " working")
	}
	return lipgloss.NewStyle().Foreground(styles.ColorDimGray).Render("○ idle")
}

func activityRawStr(a *core.AgentState) string {
	if a.Activity == core.ActivityWorking {
		return "⠋ working"
	}
	return "○ idle"
}

func agentBorderColor(a *core.AgentState, commitsAhead int) lipgloss.TerminalColor {
	if a.MergeCommit != "" {
		return styles.ColorCyan // merged
	}
	if a.Resolving != "" {
		return styles.ColorRed // resolving conflicts
	}
	if a.HasConflicts {
		return styles.ColorRed // would conflict if merged
	}
	if a.Activity == core.ActivityWorking {
		return styles.ColorYellow // working
	}
	// idle
	if commitsAhead > 0 {
		return styles.ColorGreen // idle, some commits
	}
	return styles.ColorDimGray // idle, no commits
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

func countUncommitted(worktree string) int {
	out := core.GitMust(worktree, "status", "--porcelain")
	if out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
}
