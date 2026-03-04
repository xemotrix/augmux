package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Kanagawa color palette
	colorAccent  = lipgloss.Color("#FF9E3B") // roninYellow — accent/brand
	colorGreen   = lipgloss.Color("#98BB6C") // springGreen
	colorYellow  = lipgloss.Color("#DCA561") // autumnYellow
	colorRed     = lipgloss.Color("#E46876") // waveRed
	colorCyan    = lipgloss.Color("#7FB4CA") // springBlue
	colorGray    = lipgloss.Color("#727169") // fujiGray
	colorDimGray = lipgloss.Color("#54546D") // sumiInk4
	colorWhite   = lipgloss.Color("#DCD7BA") // fujiWhite

	// Title
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			MarginBottom(1)

	// Status badges
	badgeWip       = lipgloss.NewStyle().Bold(true).Foreground(colorGreen)
	badgeMerged    = lipgloss.NewStyle().Bold(true).Foreground(colorCyan)
	badgeResolving = lipgloss.NewStyle().Bold(true).Foreground(colorYellow)
	// Labels and values
	labelStyle = lipgloss.NewStyle().Foreground(colorDimGray)
	valueStyle = lipgloss.NewStyle().Foreground(colorWhite)
	branchStyle = lipgloss.NewStyle().Foreground(colorCyan)
	aheadStyle = lipgloss.NewStyle().Foreground(colorYellow)
	dirtyStyle = lipgloss.NewStyle().Foreground(colorRed)

	// Picker
	pickerCursorStyle   = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
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

// RenderBranch renders a branch name with the branch icon prefix.
func RenderBranch(name string) string {
	return branchStyle.Render(branchIcon + " " + name)
}