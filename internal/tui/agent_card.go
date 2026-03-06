package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/xemotrix/augmux/internal/core"
	"github.com/xemotrix/augmux/internal/styles"
)

type AgentCard struct{}

func (ac *AgentCard) View(a *core.Agent, spinnerFrame string, selected bool) string {
	if a == nil {
		return ""
	}

	stateColor := ac.borderColor(a)
	bdrStyle := lipgloss.NewStyle().Foreground(stateColor)

	var border lipgloss.Border
	if selected {
		border = lipgloss.ThickBorder()
		bdrStyle = bdrStyle.Bold(true)
	} else {
		border = lipgloss.RoundedBorder()
	}

	// Top border — manually constructed to embed status badge on the left.
	statusBadge := ac.statusBadge(a, stateColor, selected)

	topLeft := border.TopLeft + border.Top
	topRight := border.Top + border.TopRight
	used := lipgloss.Width(topLeft) + 1 + lipgloss.Width(statusBadge) + 1 + lipgloss.Width(topRight)
	fill := max(cardWidth-used, 1)
	topLine := lipgloss.JoinHorizontal(lipgloss.Top,
		bdrStyle.Render(topLeft),
		lipgloss.NewStyle().Render(" "),
		statusBadge,
		lipgloss.NewStyle().Render(" "),
		bdrStyle.Render(strings.Repeat(border.Top, fill)),
		bdrStyle.Render(topRight),
	)

	// Card body — lipgloss handles side borders, bottom border, and padding.
	bodyStyle := lipgloss.NewStyle().
		Border(border, false, true, true, true).
		BorderForeground(stateColor).
		Width(cardInner).
		Padding(0, 1)

	// Row 1: description (left-aligned) + activity indicator (right-aligned)
	activityStr := activityIndicator(a, spinnerFrame)
	activityWidth := lipgloss.Width(activityStr)
	maxName := max(textWidth-activityWidth-1, 10)
	nameLeft := lipgloss.NewStyle().
		Width(textWidth - activityWidth).
		Render(styles.ValueStyle.Render(truncate(a.Description, maxName)))
	nameLine := lipgloss.JoinHorizontal(lipgloss.Top, nameLeft, activityStr)

	// Row 2: branch (left-aligned) + commits ahead / dirty count (right-aligned)
	aheadStr := fmt.Sprintf("%d ↑", a.CommitsAhead)
	dirtyStr := fmt.Sprintf("%d ✎", a.UncommittedCount)
	rightInfo := lipgloss.JoinHorizontal(lipgloss.Top,
		styles.AheadStyle.MarginRight(2).Render(aheadStr),
		styles.DirtyStyle.Render(dirtyStr),
	)
	rightInfoWidth := lipgloss.Width(aheadStr) + 2 + lipgloss.Width(dirtyStr)
	iconWidth := 2
	maxBranch := textWidth - rightInfoWidth - 1 - iconWidth
	branchLeft := lipgloss.NewStyle().
		Width(textWidth - rightInfoWidth).
		Render(styles.RenderBranch(truncate(a.Branch, maxBranch)))
	branchLine := lipgloss.JoinHorizontal(lipgloss.Top, branchLeft, rightInfo)

	body := lipgloss.JoinVertical(lipgloss.Left, nameLine, branchLine)
	return lipgloss.JoinVertical(lipgloss.Left, topLine, bodyStyle.Render(body))
}

func activityIndicator(a *core.Agent, spinnerFrame string) string {
	if a.Activity == core.ActivityWorking {
		return lipgloss.NewStyle().Foreground(styles.ColorYellow).Render(spinnerFrame + " working")
	}
	return lipgloss.NewStyle().Foreground(styles.ColorDimGray).Render("○ idle")
}

func (ac *AgentCard) borderColor(a *core.Agent) lipgloss.TerminalColor {
	switch a.Status() {
	case core.AgentStatusIdle:
		if a.CommitsAhead > 0 {
			return styles.ColorGreen
		}
		return styles.ColorDimGray
	case core.AgentStatusWorking:
		return styles.ColorYellow
	case core.AgentStatusConflict:
		return styles.ColorRed
	case core.AgentStatusMerged:
		return styles.ColorCyan
	default:
		return styles.ColorDimGray
	}
}

func (ac *AgentCard) statusBadge(a *core.Agent, color lipgloss.TerminalColor, selected bool) string {
	var status string
	switch a.Status() {
	case core.AgentStatusWorking:
		status = "● working"
	case core.AgentStatusWip:
		status = "● wip"
	case core.AgentStatusIdle:
		status = "● idle"
	case core.AgentStatusConflict:
		status = "● conflict"
	case core.AgentStatusMerged:
		status = "● merged"
	default:
		status = "error"
	}
	return lipgloss.NewStyle().
		Bold(selected).
		Foreground(color).
		Render(status)
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 3 {
		return string(runes[:n])
	}
	return string(runes[:n-3]) + "..."
}
