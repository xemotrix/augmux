// Package styles holds reusable lipgloss styles and pallete colors
package styles

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Kanagawa color palette
	ColorAccent  = lipgloss.Color("#FF9E3B") // roninYellow — accent/brand
	ColorGreen   = lipgloss.Color("#98BB6C") // springGreen
	ColorYellow  = lipgloss.Color("#DCA561") // autumnYellow
	ColorRed     = lipgloss.Color("#E46876") // waveRed
	ColorCyan    = lipgloss.Color("#7FB4CA") // springBlue
	ColorGray    = lipgloss.Color("#727169") // fujiGray
	ColorDimGray = lipgloss.Color("#938AA9") // springViolet1
	ColorWhite   = lipgloss.Color("#DCD7BA") // fujiWhite
	ColorPurple  = lipgloss.Color("#957FB8") // oniViolet

	// Title
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent).
			MarginBottom(1)

	// Status badges
	BadgeWip       = lipgloss.NewStyle().Bold(true).Foreground(ColorGreen)
	BadgeMerged    = lipgloss.NewStyle().Bold(true).Foreground(ColorCyan)
	BadgeConflicts = lipgloss.NewStyle().Bold(true).Foreground(ColorRed)
	BadgeWorking   = lipgloss.NewStyle().Bold(true).Foreground(ColorYellow)

	// Labels and values
	ValueStyle  = lipgloss.NewStyle().Foreground(ColorWhite)
	BranchStyle = lipgloss.NewStyle().Foreground(ColorCyan)
	AheadStyle  = lipgloss.NewStyle().Foreground(ColorYellow)
	DirtyStyle  = lipgloss.NewStyle().Foreground(ColorRed)

	// Text Styles
	AccentStyle   = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	EnabledStyle  = lipgloss.NewStyle().Foreground(ColorWhite).Bold(true)
	DisabledStyle = lipgloss.NewStyle().Foreground(ColorDimGray)
	HintStyle     = lipgloss.NewStyle().Foreground(ColorDimGray).Italic(true)
	DefaultStyle  = lipgloss.NewStyle().Foreground(ColorGray)

	// Confirm Styles
	ConfirmActiveStyle   = lipgloss.NewStyle().Bold(true).Foreground(ColorWhite).Background(ColorAccent).Padding(0, 2)
	ConfirmInactiveStyle = lipgloss.NewStyle().Foreground(ColorGray).Padding(0, 2)
	ConfirmWarningStyle  = lipgloss.NewStyle().Foreground(ColorRed).Bold(true)

	// Toast notifications
	ToastInfo    = lipgloss.NewStyle().Foreground(ColorCyan).Bold(true)
	ToastSuccess = lipgloss.NewStyle().Foreground(ColorGreen).Bold(true)
	ToastWarning = lipgloss.NewStyle().Foreground(ColorYellow).Bold(true)
	ToastError   = lipgloss.NewStyle().Foreground(ColorRed).Bold(true)

	// Header info
	HeaderKeyStyle = lipgloss.NewStyle().Foreground(ColorGray).Bold(true)
	HeaderValStyle = lipgloss.NewStyle().Foreground(ColorWhite)
)

const branchIcon = "\uf126" // Nerd Font code-fork icon

// RenderBranch renders a branch name with the branch icon prefix.
func RenderBranch(name string) string {
	return BranchStyle.Render(branchIcon + " " + name)
}
