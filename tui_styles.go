package main

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	colorPurple  = lipgloss.Color("141")
	colorGreen   = lipgloss.Color("78")
	colorYellow  = lipgloss.Color("220")
	colorRed     = lipgloss.Color("203")
	colorCyan    = lipgloss.Color("80")
	colorGray    = lipgloss.Color("245")
	colorDimGray = lipgloss.Color("240")
	colorWhite   = lipgloss.Color("255")

	// Title
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPurple).
			MarginBottom(1)

	// Status badges
	badgeActive    = lipgloss.NewStyle().Bold(true).Foreground(colorGreen)
	badgeMerged    = lipgloss.NewStyle().Bold(true).Foreground(colorCyan)
	badgeResolving = lipgloss.NewStyle().Bold(true).Foreground(colorYellow)
	badgeNoWindow  = lipgloss.NewStyle().Bold(true).Foreground(colorRed)

	// Labels and values
	labelStyle = lipgloss.NewStyle().Foreground(colorDimGray)
	valueStyle = lipgloss.NewStyle().Foreground(colorWhite)
	branchStyle = lipgloss.NewStyle().Foreground(colorCyan)
	aheadStyle = lipgloss.NewStyle().Foreground(colorYellow)

	// Agent card
	agentCardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorDimGray).
			Padding(0, 1).
			MarginBottom(0)

	// Picker
	pickerCursorStyle   = lipgloss.NewStyle().Foreground(colorPurple).Bold(true)
	pickerSelectedStyle = lipgloss.NewStyle().Foreground(colorWhite).Bold(true)
	pickerNormalStyle   = lipgloss.NewStyle().Foreground(colorGray)
	pickerHintStyle     = lipgloss.NewStyle().Foreground(colorDimGray).Italic(true)

	// Separator
	separatorStyle = lipgloss.NewStyle().Foreground(colorDimGray)

	// Header info
	headerKeyStyle = lipgloss.NewStyle().Foreground(colorGray).Bold(true)
	headerValStyle = lipgloss.NewStyle().Foreground(colorWhite)
)


const branchIcon = "\uf126" // Nerd Font code-fork icon

// renderBranch renders a branch name with the branch icon prefix.
func renderBranch(name string) string {
	return branchStyle.Render(branchIcon + " " + name)
}