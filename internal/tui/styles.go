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

// Wraps content in a bordered box with sharp corners. contentW/contentH describe
// the inner content; horizontal padding and the border are added around it. The
// box is fixed to that size, padding short content and truncating overflow.
func boxStyle(contentW, contentH int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Padding(0, boxHPad).
		Width(contentW + 2*boxHPad).
		Height(contentH).
		MaxWidth(contentW + 2*boxHPad + 2).
		MaxHeight(contentH + 2)
}
