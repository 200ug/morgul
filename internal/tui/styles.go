package tui

import "github.com/charmbracelet/lipgloss"

var (
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	notifInfo     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	notifError    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	modeStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	labelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
)

func padWidth(w int) lipgloss.Style {
	return lipgloss.NewStyle().Width(w)
}
