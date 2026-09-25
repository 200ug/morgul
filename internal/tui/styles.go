package tui

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"codeberg.org/2ug/morgul/internal/config"
)

// Palette used by the whole TUI; every color is loaded from the config
// directory (falling back to config.DefaultColors) and reused by both the main
// view and the form themes.
var (
	colorAccent   lipgloss.Color
	colorDim      lipgloss.Color
	colorSuccess  lipgloss.Color
	colorError    lipgloss.Color
	colorWarning  lipgloss.Color
	colorText     lipgloss.Color
	colorOnAccent lipgloss.Color

	selectedStyle lipgloss.Style
	dimStyle      lipgloss.Style
	notifInfo     lipgloss.Style
	notifError    lipgloss.Style
	modeStyle     lipgloss.Style
	labelStyle    lipgloss.Style
)

func init() {
	applyColors(config.DefaultColors())
}

// Applies a palette to the shared color and style variables. Called once at
// startup with the user's colors.
func applyColors(c config.Colors) {
	colorAccent = lipgloss.Color(c.Accent)
	colorDim = lipgloss.Color(c.Dim)
	colorSuccess = lipgloss.Color(c.Success)
	colorError = lipgloss.Color(c.Error)
	colorWarning = lipgloss.Color(c.Warning)
	colorText = lipgloss.Color(c.Text)
	colorOnAccent = lipgloss.Color(c.OnAccent)

	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	dimStyle = lipgloss.NewStyle().Foreground(colorDim)
	notifInfo = lipgloss.NewStyle().Foreground(colorSuccess)
	notifError = lipgloss.NewStyle().Foreground(colorError)
	modeStyle = lipgloss.NewStyle().Foreground(colorWarning)
	labelStyle = lipgloss.NewStyle().Foreground(colorAccent)
}

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

// boxStyle with an explicit border color.
func boxStyleBorder(contentW, contentH int, border lipgloss.Color) lipgloss.Style {
	return boxStyle(contentW, contentH).BorderForeground(border)
}

// Builds a huh theme from the shared palette so the create/edit/confirm forms
// match the main view.
func formTheme() *huh.Theme {
	t := huh.ThemeBase()

	t.Focused.Base = t.Focused.Base.BorderForeground(colorDim)
	t.Focused.Card = t.Focused.Base
	t.Focused.Title = t.Focused.Title.Foreground(colorAccent).Bold(true)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(colorAccent).Bold(true)
	t.Focused.Directory = t.Focused.Directory.Foreground(colorAccent)
	t.Focused.Description = t.Focused.Description.Foreground(colorDim)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(colorError)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(colorError)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(colorAccent)
	t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(colorAccent)
	t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(colorAccent)
	t.Focused.Option = t.Focused.Option.Foreground(colorText)
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(colorAccent)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(colorAccent).Bold(true)
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(colorAccent)
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(colorText)
	t.Focused.UnselectedPrefix = t.Focused.UnselectedPrefix.Foreground(colorDim)

	button := lipgloss.NewStyle().Padding(0, 2).MarginRight(1)
	t.Focused.FocusedButton = button.Foreground(colorOnAccent).Background(colorAccent).Bold(true)
	t.Focused.BlurredButton = button.Foreground(colorDim)

	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(colorAccent)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(colorDim)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(colorAccent)

	// NOTE: the shortcut keys stay unstyled so the confirm field's hardcoded
	// lowercase "y"/"n" keys can be capitalized after rendering; only the
	// action descriptions are dimmed to distinguish them from the keys.
	t.Help.ShortKey = lipgloss.NewStyle()
	t.Help.ShortDesc = lipgloss.NewStyle().Foreground(colorDim)
	t.Help.ShortSeparator = lipgloss.NewStyle()

	t.Blurred = t.Focused
	t.Blurred.Base = t.Blurred.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.Card = t.Blurred.Base
	t.Blurred.NextIndicator = lipgloss.NewStyle()
	t.Blurred.PrevIndicator = lipgloss.NewStyle()

	t.Group.Title = t.Focused.Title
	t.Group.Description = t.Focused.Description
	return t
}

// NOTE: this custom keymap exists only to capitalize huh's lowercase help text
// (shortcuts and actions); the actual keybindings are unchanged. It is built
// once at package init and shared read-only, so there is no per-render cost.
var formKeyMap = func() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()

	km.Input.AcceptSuggestion.SetHelp("Ctrl+E", "Complete")
	km.Input.Prev.SetHelp("Shift+Tab", "Back")
	km.Input.Next.SetHelp("Enter", "Next")
	km.Input.Submit.SetHelp("Enter", "Submit")

	km.Select.Prev.SetHelp("Shift+Tab", "Back")
	km.Select.Next.SetHelp("Enter", "Select")
	km.Select.Submit.SetHelp("Enter", "Submit")
	km.Select.Up.SetHelp("↑", "Up")
	km.Select.Down.SetHelp("↓", "Down")
	km.Select.Left.SetHelp("←", "Left")
	km.Select.Right.SetHelp("→", "Right")
	km.Select.Filter.SetHelp("/", "Filter")
	km.Select.SetFilter.SetHelp("Esc", "Set filter")
	km.Select.ClearFilter.SetHelp("Esc", "Clear filter")
	km.Select.HalfPageUp.SetHelp("Ctrl+U", "½ page up")
	km.Select.HalfPageDown.SetHelp("Ctrl+D", "½ page down")
	km.Select.GotoTop.SetHelp("G/Home", "Go to start")
	km.Select.GotoBottom.SetHelp("G/End", "Go to end")

	km.MultiSelect.Prev.SetHelp("Shift+Tab", "Back")
	km.MultiSelect.Next.SetHelp("Enter", "Confirm")
	km.MultiSelect.Submit.SetHelp("Enter", "Submit")
	km.MultiSelect.Toggle.SetHelp("X", "Toggle")
	km.MultiSelect.Up.SetHelp("↑", "Up")
	km.MultiSelect.Down.SetHelp("↓", "Down")
	km.MultiSelect.Filter.SetHelp("/", "Filter")
	km.MultiSelect.SetFilter.SetHelp("Enter", "Set filter")
	km.MultiSelect.ClearFilter.SetHelp("Esc", "Clear filter")
	km.MultiSelect.HalfPageUp.SetHelp("Ctrl+U", "½ page up")
	km.MultiSelect.HalfPageDown.SetHelp("Ctrl+D", "½ page down")
	km.MultiSelect.GotoTop.SetHelp("G/Home", "Go to start")
	km.MultiSelect.GotoBottom.SetHelp("G/End", "Go to end")
	km.MultiSelect.SelectAll.SetHelp("Ctrl+A", "Select all")
	km.MultiSelect.SelectNone.SetHelp("Ctrl+A", "Select none")

	km.Confirm.Next.SetHelp("Enter", "Next")
	km.Confirm.Prev.SetHelp("Shift+Tab", "Back")
	km.Confirm.Toggle.SetHelp("←/→", "Toggle")
	km.Confirm.Submit.SetHelp("Enter", "Submit")
	// NOTE: huh's confirm field overrides the accept/reject help keys with
	// lowercase "y"/"n" during rendering, so those two are fixed separately
	// in formView.
	km.Confirm.Accept.SetHelp("Y", "Yes")
	km.Confirm.Reject.SetHelp("N", "No")

	return km
}()
