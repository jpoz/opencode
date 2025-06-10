package style

import "github.com/charmbracelet/lipgloss"

var PrimaryText = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))  // bright white
var SecondaryText = lipgloss.NewStyle().Foreground(lipgloss.Color("7")) // white
var Muted = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))         // darker gray / muted white
var Accent = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))        // blue
var Warning = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))       // yellow
var Error = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))         // red
var Help = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))        // another shade of gray for help text

func BaseStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.NoColor{}).  // No background color (transparent/inherit)
		Foreground(lipgloss.Color("15")) // Default foreground bright white
}
