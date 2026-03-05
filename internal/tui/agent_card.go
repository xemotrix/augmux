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

	ahead := core.GitMust(repoRoot, "rev-list", "--count", srcBranch+".."+a.Branch)
	if ahead == "" {
		ahead = "?"
	}
	aheadNum := 0
	fmt.Sscanf(ahead, "%d", &aheadNum)

	borderColor := agentBorderColor(a, aheadNum)

	var border lipgloss.Border
	if sel {
		border = lipgloss.DoubleBorder()
	} else {
		border = lipgloss.RoundedBorder()
	}

	bdr := lipgloss.NewStyle().Foreground(borderColor)
	if sel {
		bdr = bdr.Bold(true)
	}

	// Top border — manually constructed to embed agent ID and status badge.
	statusRaw := AgentStatusRaw(a)
	statusStyled := AgentStatusStyled(a, statusRaw)
	idLabel := fmt.Sprintf("❮%d❯", a.Index)
	idStyled := lipgloss.NewStyle().Bold(true).Foreground(styles.ColorWhite).Render(idLabel)

	topLeft := border.TopLeft + border.Top
	topRight := border.Top + border.TopRight
	used := lipgloss.Width(topLeft) + lipgloss.Width(idLabel) + 1 + lipgloss.Width(statusRaw) + 1 + lipgloss.Width(topRight)
	fill := max(cardWidth-used, 1)
	topLine := bdr.Render(topLeft) + idStyled +
		bdr.Render(strings.Repeat(border.Top, fill)) +
		" " + statusStyled + " " +
		bdr.Render(topRight)

	// Card body — lipgloss handles side borders, bottom border, and padding.
	bodyStyle := lipgloss.NewStyle().
		Border(border, false, true, true, true).
		BorderForeground(borderColor).
		Width(cardInner).
		Padding(0, 1)

	// Row 1: description (left) + activity indicator (right)
	activityStr := activityIndicator(a, spinnerFrame)
	activityWidth := lipgloss.Width(activityRawStr(a))
	maxName := max(textWidth-activityWidth-1, 10)
	nameLeft := lipgloss.NewStyle().
		Width(textWidth - activityWidth).
		Render(styles.ValueStyle.Render(truncate(a.Description, maxName)))
	nameLine := nameLeft + activityStr

	// Row 2: branch (left) + commits ahead / dirty count (right)
	aheadStr := ahead + " ↑"
	dirty := countUncommitted(a.Worktree)
	dirtyStr := fmt.Sprintf("%d ✎", dirty)
	rightInfo := styles.AheadStyle.Render(aheadStr) + "  " + styles.DirtyStyle.Render(dirtyStr)
	rightInfoWidth := lipgloss.Width(aheadStr + "  " + dirtyStr)
	iconWidth := 2
	maxBranch := textWidth - rightInfoWidth - 1 - iconWidth
	branchLeft := lipgloss.NewStyle().
		Width(textWidth - rightInfoWidth).
		Render(styles.RenderBranch(truncate(a.Branch, maxBranch)))
	branchLine := branchLeft + rightInfo

	body := lipgloss.JoinVertical(lipgloss.Left, nameLine, branchLine)
	return topLine + "\n" + bodyStyle.Render(body)
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
