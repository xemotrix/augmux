package styles

import "github.com/charmbracelet/lipgloss"

var (
	// Kanagawa color palette
	ColorAccent  = lipgloss.Color("#FF9E3B") // roninYellow — accent/brand
	ColorGreen   = lipgloss.Color("#98BB6C") // springGreen
	ColorYellow  = lipgloss.Color("#DCA561") // autumnYellow
	ColorRed     = lipgloss.Color("#E46876") // waveRed
	ColorCyan    = lipgloss.Color("#7FB4CA") // springBlue
	ColorGray    = lipgloss.Color("#727169") // fujiGray
	ColorDimGray = lipgloss.Color("#54546D") // sumiInk4
	ColorWhite   = lipgloss.Color("#DCD7BA") // fujiWhite

	// Title
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent).
			MarginBottom(1)

	// Status badges
	BadgeWip       = lipgloss.NewStyle().Bold(true).Foreground(ColorGreen)
	BadgeMerged    = lipgloss.NewStyle().Bold(true).Foreground(ColorCyan)
	BadgeResolving = lipgloss.NewStyle().Bold(true).Foreground(ColorYellow)
	BadgeConflicts = lipgloss.NewStyle().Bold(true).Foreground(ColorRed)
	// Labels and values
	LabelStyle  = lipgloss.NewStyle().Foreground(ColorDimGray)
	ValueStyle  = lipgloss.NewStyle().Foreground(ColorWhite)
	BranchStyle = lipgloss.NewStyle().Foreground(ColorCyan)
	AheadStyle  = lipgloss.NewStyle().Foreground(ColorYellow)
	DirtyStyle  = lipgloss.NewStyle().Foreground(ColorRed)

	// Picker
	PickerCursorStyle   = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	PickerSelectedStyle = lipgloss.NewStyle().Foreground(ColorWhite).Bold(true)
	PickerNormalStyle   = lipgloss.NewStyle().Foreground(ColorGray)
	PickerHintStyle     = lipgloss.NewStyle().Foreground(ColorDimGray).Italic(true)

	// Toast notifications
	ToastInfo    = lipgloss.NewStyle().Foreground(ColorCyan).Bold(true)
	ToastSuccess = lipgloss.NewStyle().Foreground(ColorGreen).Bold(true)
	ToastWarning = lipgloss.NewStyle().Foreground(ColorYellow).Bold(true)
	ToastError   = lipgloss.NewStyle().Foreground(ColorRed).Bold(true)

	// Separator
	SeparatorStyle = lipgloss.NewStyle().Foreground(ColorDimGray)

	// Header info
	HeaderKeyStyle = lipgloss.NewStyle().Foreground(ColorGray).Bold(true)
	HeaderValStyle = lipgloss.NewStyle().Foreground(ColorWhite)
)

const branchIcon = "\uf126" // Nerd Font code-fork icon

// RenderBranch renders a branch name with the branch icon prefix.
func RenderBranch(name string) string {
	return BranchStyle.Render(branchIcon + " " + name)
}
