package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"codeberg.org/2ug/morgul/internal/config"
)

// Enters the single-view container creation form.
func (m model) startCreate() (model, tea.Cmd) {
	presets, err := m.store.LoadPresets()
	if err != nil {
		return m.notifyError("Error: " + err.Error())
	}
	mods, err := m.store.LoadModules()
	if err != nil {
		return m.notifyError("Error: " + err.Error())
	}
	if len(presets) == 0 && len(mods) == 0 {
		return m.notifyError("Error: no presets or modules found in " + m.store.Dir)
	}

	search := textinput.New()
	search.Placeholder = "filter presets..."
	search.Prompt = "> "
	search.CharLimit = 64
	blink := search.Focus()

	path := textinput.New()
	path.Prompt = ""
	path.Placeholder = "project path"
	path.CharLimit = 256
	if cwd, err := os.Getwd(); err == nil {
		path.SetValue(cwd)
	}

	m.createSt = &createState{
		search:         search,
		path:           path,
		presets:        presets,
		modlist:        mods,
		moduleSelected: map[string]bool{},
		focus:          focusProfile,
	}
	m.createSt.selected = m.selectedProfile()
	m.syncCreateMount()
	m.page = pageCreate
	return m, blink
}

// Routes a key to the focused pane of the creation view.
func (m model) handleCreateKey(msg tea.KeyMsg) (model, tea.Cmd) {
	s := m.createSt

	if msg.Type == tea.KeyEsc {
		return m.resetToMain()
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'q' {
		if s.focus != focusProfile && s.focus != focusPath {
			return m.resetToMain()
		}
	}

	switch msg.Type {
	case tea.KeyTab:
		m.advanceCreateFocus(1)
		return m, nil
	case tea.KeyShiftTab:
		m.advanceCreateFocus(-1)
		return m, nil
	}

	switch s.focus {
	case focusProfile:
		return m.createProfileKey(msg)
	case focusModules:
		return m.createModulesKey(msg)
	case focusPath:
		return m.createPathKey(msg)
	case focusMount:
		return m.createMountKey(msg)
	case focusCreate:
		return m.createSubmitKey(msg)
	}
	return m, nil
}

func (m *model) advanceCreateFocus(delta int) {
	list := m.createFocusList()
	idx := -1
	for i, f := range list {
		if f == m.createSt.focus {
			idx = i
			break
		}
	}
	if idx < 0 {
		idx = 0
	} else {
		idx += delta
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(list) {
		idx = len(list) - 1
	}
	m.setCreateFocus(list[idx])
}

func (m *model) setCreateFocus(f createFocus) {
	s := m.createSt
	s.focus = f
	if f == focusProfile {
		s.search.Focus()
	} else {
		s.search.Blur()
	}
	if f == focusPath {
		s.path.Focus()
	} else {
		s.path.Blur()
	}
}

// Returns the focusable panes in left-to-right order, omitting
// the modules pane unless "custom" is selected.
func (m model) createFocusList() []createFocus {
	if m.createSt.selected == "custom" {
		return []createFocus{focusProfile, focusModules, focusPath, focusMount, focusCreate}
	}
	return []createFocus{focusProfile, focusPath, focusMount, focusCreate}
}

func (m model) createProfileKey(msg tea.KeyMsg) (model, tea.Cmd) {
	var cmd tea.Cmd
	m.createSt.search, cmd = m.createSt.search.Update(msg)
	m.createSt.selected = m.selectedProfile()
	m.syncCreateMount()
	return m, cmd
}

func (m model) createPathKey(msg tea.KeyMsg) (model, tea.Cmd) {
	var cmd tea.Cmd
	m.createSt.path, cmd = m.createSt.path.Update(msg)
	return m, cmd
}

func (m model) createModulesKey(msg tea.KeyMsg) (model, tea.Cmd) {
	s := m.createSt
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		switch msg.Runes[0] {
		case 'j':
			if s.modSel < len(s.modlist)-1 {
				s.modSel++
			}
		case 'k':
			if s.modSel > 0 {
				s.modSel--
			}
		}
		return m, nil
	}
	if msg.Type == tea.KeyEnter || msg.Type == tea.KeySpace {
		if s.modSel >= 0 && s.modSel < len(s.modlist) {
			id := s.modlist[s.modSel].ID
			if s.moduleSelected[id] {
				delete(s.moduleSelected, id)
			} else {
				s.moduleSelected[id] = true
			}
		}
		return m, nil
	}
	return m, nil
}

func (m model) createMountKey(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter, tea.KeySpace:
		m.createSt.mount = !m.createSt.mount
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'y', 'Y':
				m.createSt.mount = true
			case 'n', 'N':
				m.createSt.mount = false
			}
		}
	}
	return m, nil
}

func (m model) createSubmitKey(msg tea.KeyMsg) (model, tea.Cmd) {
	if msg.Type == tea.KeyEnter {
		return m.finalizeCreate()
	}
	return m, nil
}

// Returns the display names shown in the preset list, with the
// pseudo "custom" option last.
func (m model) profileOptions() []string {
	names := make([]string, 0, len(m.createSt.presets)+1)
	for _, p := range m.createSt.presets {
		names = append(names, p.ID)
	}
	names = append(names, "custom")
	return names
}

// Returns the preset-list order for the current search
// query: ranked fuzzy matches, or every option in order when the query is empty.
func (m model) rankedProfileIndices() []int {
	names := m.profileOptions()
	if m.createSt.search.Value() == "" {
		idx := make([]int, len(names))
		for i := range names {
			idx[i] = i
		}
		return idx
	}
	return fuzzyFind(m.createSt.search.Value(), names)
}

// Resolves the preset id (or "custom") currently picked by the
// search filter: the top fuzzy match, or the first option when the query is
// empty.
func (m model) selectedProfile() string {
	s := m.createSt
	if len(s.presets) == 0 {
		return "custom"
	}
	names := m.profileOptions()
	query := s.search.Value()
	if query == "" {
		return s.presets[0].ID
	}
	matches := fuzzyFind(query, names)
	if len(matches) == 0 {
		return ""
	}
	name := names[matches[0]]
	if name == "custom" {
		return "custom"
	}
	return name
}

// Applies the selected preset's project mount as the default
// whenever the selection changes.
func (m model) syncCreateMount() {
	s := m.createSt
	if s.selected == s.lastSelected {
		return
	}
	s.lastSelected = s.selected
	s.mount = false
	if s.selected == "" || s.selected == "custom" {
		return
	}
	if p, err := m.store.LoadPreset(s.selected); err == nil && p.ProjectMount != nil {
		s.mount = *p.ProjectMount
	}
}

func (m model) finalizeCreate() (model, tea.Cmd) {
	s := m.createSt
	var (
		bp  *config.Blueprint
		err error
	)
	if s.selected == "" {
		return m.notifyError("Error: no profile selected")
	}
	if s.selected == "custom" {
		mods := make([]string, 0, len(s.moduleSelected))
		for _, mod := range s.modlist {
			if s.moduleSelected[mod.ID] {
				mods = append(mods, mod.ID)
			}
		}
		bp, err = m.resolver.ResolveCustom(mods, &s.mount)
	} else {
		bp, err = m.resolver.ResolvePreset(s.selected)
		if err == nil {
			mt := s.mount
			bp.ProjectMount = &mt
		}
	}
	if err != nil {
		return m.notifyError("Error: " + err.Error())
	}

	projectPath := strings.TrimSpace(s.path.Value())
	if projectPath == "" {
		projectPath = "."
	}
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		return m.notifyError("Error: " + err.Error())
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return m.notifyError("Error: project path is not a directory: " + abs)
	}

	m2, cmd := m.resetToMain()
	m2.notif = newNotification("Creating container...", false, true)
	return m2, tea.Batch(cmd, createCmd(m.client, bp, m.store.BaseDockerfile(), m.userHome, abs))
}

// --- rendering ---

func (m model) createView() string {
	s := m.createSt
	innerW := m.width - padLeft - padRight
	innerH := m.height - padTop - padBottom
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	presetW := m.createPresetWidth()
	custom := s.selected == "custom"
	modW := 0
	extra := 8
	if custom {
		modW = m.createModuleWidth()
		extra = 12
	}
	stageW := m.createStageWidth()
	if maxStage := innerW - extra - presetW - modW; stageW > maxStage {
		stageW = maxStage
	}
	if stageW < 16 {
		stageW = 16
	}

	// the search bar spans the full width of the columns below it
	searchW := presetW + modW + stageW + extra - 4
	if searchW < 1 {
		searchW = 1
	}
	if searchW > innerW-4 {
		searchW = innerW - 4
	}

	// content-based column height, capped to the terminal
	presetH := len(m.rankedProfileIndices())
	modH := 0
	if custom {
		modH = len(s.modlist)
	}
	colH := max(presetH, modH, 4)
	if maxH := innerH - 5; colH > maxH {
		colH = maxH
	}
	if colH < 1 {
		colH = 1
	}

	search := createBox(searchW, 1, s.focus == focusProfile).Render(s.search.View())
	preset := createBox(presetW, colH, s.focus == focusProfile).Render(m.createPresetList(presetW, colH))
	stage := createBox(stageW, colH, s.focus == focusPath || s.focus == focusMount || s.focus == focusCreate).Render(m.createStage(stageW, colH))

	var middle string
	if custom {
		mod := createBox(modW, colH, s.focus == focusModules).Render(m.createModuleList(modW, colH))
		middle = lipgloss.JoinHorizontal(lipgloss.Top, preset, mod, stage)
	} else {
		middle = lipgloss.JoinHorizontal(lipgloss.Top, preset, stage)
	}

	body := lipgloss.JoinVertical(lipgloss.Left, search, middle)
	return lipgloss.NewStyle().Padding(padTop, padRight, padBottom, padLeft).Render(body)
}

// boxStyle with a plain (bright) border for the active segment, dimmed otherwise.
func createBox(contentW, contentH int, active bool) lipgloss.Style {
	if active {
		return boxStyle(contentW, contentH)

	}
	return boxStyleBorder(contentW, contentH, colorDim)
}

// Returns the width of the widest line in the stage pane.
func (m model) createStageWidth() int {
	s := m.createSt
	w := 0
	if l := lipgloss.Width("Path: " + s.path.View()); l > w {
		w = l
	}
	if l := len("Mount: Yes"); l > w {
		w = l
	}
	if l := len("Create") + 2; l > w {
		w = l
	}
	return w
}

func (m model) createPresetWidth() int {
	w := 0
	for _, name := range m.profileOptions() {
		if len(name) > w {
			w = len(name)
		}
	}
	if w < 12 {
		w = 12
	}
	return w + 2
}

func (m model) createModuleWidth() int {
	w := 0
	for _, mod := range m.createSt.modlist {
		if len(mod.ID) > w {
			w = len(mod.ID)
		}
	}
	if w < 12 {
		w = 12
	}
	return w + 4
}

func (m model) createPresetList(width, height int) string {
	names := m.profileOptions()
	ranked := m.rankedProfileIndices()

	lines := make([]string, 0, height)
	for i := 0; i < len(ranked) && len(lines) < height; i++ {
		name := names[ranked[i]]
		line := "  " + name
		if i == 0 {
			line = "▸ " + name
			line = selectedStyle.Render(line)
		}
		lines = append(lines, line)
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return padWidth(width).Render(strings.Join(lines, "\n"))
}

func (m model) createModuleList(width, height int) string {
	s := m.createSt
	start := 0
	if s.modSel >= height {
		start = s.modSel - height + 1
	}
	lines := make([]string, 0, height)
	for i := start; i < len(s.modlist) && len(lines) < height; i++ {
		mod := s.modlist[i]
		checked := " "
		if s.moduleSelected[mod.ID] {
			checked = "x"
		}
		line := fmt.Sprintf("[%s] %s", checked, mod.ID)
		if i == s.modSel {
			line = selectedStyle.Render(line)
		}
		lines = append(lines, line)
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return padWidth(width).Render(strings.Join(lines, "\n"))
}

func (m model) createStage(width, height int) string {
	s := m.createSt

	pathLine := "Path: " + s.path.View()
	if s.focus == focusPath {
		pathLine = selectedStyle.Render(pathLine)
	}

	mountVal := "No"
	if s.mount {
		mountVal = "Yes"
	}
	mountLine := "Mount: " + mountVal
	if s.focus == focusMount {
		mountLine = selectedStyle.Render(mountLine)
	} else {
		mountLine = dimStyle.Render(mountLine)
	}

	createLine := "  Create"
	if s.focus == focusCreate {
		createLine = selectedStyle.Render("▸ Create")
	}

	lines := []string{pathLine, mountLine, "", createLine}
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return padWidth(width).Render(strings.Join(lines, "\n"))
}
