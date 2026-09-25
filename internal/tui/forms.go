package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"codeberg.org/2ug/morgul/internal/build"
	"codeberg.org/2ug/morgul/internal/config"
)

func (m model) updateForm(msg tea.Msg) (model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && m.abortFormKey(key) {
		return m.resetToMain()
	}
	f, cmd := m.form.Update(msg)
	m.form = f.(*huh.Form)
	switch m.form.State {
	case huh.StateCompleted:
		return m.finishForm()
	case huh.StateAborted:
		return m.resetToMain()
	}
	return m, cmd
}

// Reports whether a key should exit the current form: esc always, and q unless
// the focused field is a text input (where q is typed).
func (m model) abortFormKey(key tea.KeyMsg) bool {
	if key.Type == tea.KeyEsc {
		return true
	}
	if key.Type != tea.KeyRunes || len(key.Runes) != 1 || key.Runes[0] != 'q' {
		return false
	}
	_, isInput := m.form.GetFocusedField().(*huh.Input)
	return !isInput
}

func (m model) finishForm() (model, tea.Cmd) {
	switch m.page {
	case pageEdit:
		return m.finishEdit()
	}
	return m.resetToMain()
}

func (m model) resetToMain() (model, tea.Cmd) {
	m.page = pageMain
	m.form = nil
	m.createSt = nil
	m.editSt = nil
	m.confirmSt = nil
	m.mode = insertMode
	return m, m.search.Focus()
}

func (m model) notifyError(text string) (model, tea.Cmd) {
	m2, cmd := m.resetToMain()
	m2.notif = newNotification(text, true, false)
	return m2, cmd
}

// --- edit ---

func (m model) startEdit() (model, tea.Cmd) {
	c := m.selectedContainer()
	if c == nil {
		return m.notifyError("Error: no container selected")
	}
	spec, err := build.LoadSpec(c.ProjectPath, c.Name)
	if err != nil {
		return m.notifyError("Error: could not load spec: " + err.Error())
	}
	m.editSt = &editState{
		spec:     spec,
		portsStr: formatPorts(spec.Ports),
		volsStr:  formatVolumes(spec.VirtualVolumes),
		envStr:   formatEnvFiles(spec.EnvFiles),
		mount:    spec.ProjectMount,
	}
	m.page = pageEdit
	m.form = m.buildEditForm()
	return m, m.form.Init()
}

func (m model) buildEditForm() *huh.Form {
	s := m.editSt
	ports := huh.NewInput().Title("Ports (host:container)").Value(&s.portsStr)
	vols := huh.NewInput().Title("Volumes (name:path)").Value(&s.volsStr)
	envs := huh.NewInput().Title("Env files (path)").Value(&s.envStr)
	mount := huh.NewConfirm().Title("Project mount").Value(&s.mount)
	return huh.NewForm(huh.NewGroup(ports, vols, envs, mount)).WithTheme(formTheme()).WithKeyMap(formKeyMap)
}

func (m model) finishEdit() (model, tea.Cmd) {
	s := m.editSt
	ports, err := parsePorts(s.portsStr)
	if err != nil {
		return m.notifyError("invalid ports: " + err.Error())
	}
	vols, err := parseVolumes(s.volsStr)
	if err != nil {
		return m.notifyError("invalid volumes: " + err.Error())
	}
	envs, err := parseEnvFiles(s.envStr)
	if err != nil {
		return m.notifyError("invalid env files: " + err.Error())
	}
	for _, ef := range envs {
		exp := os.ExpandEnv(strings.Replace(ef, "~", m.userHome, 1))
		if _, err := os.Stat(exp); err != nil {
			return m.notifyError("env file does not exist: " + ef)
		}
	}
	spec := s.spec
	spec.ProjectMount = s.mount
	spec.Ports = ports
	spec.VirtualVolumes = vols
	spec.EnvFiles = envs

	m2, cmd := m.resetToMain()
	m2.notif = newNotification("Recreating container...", false, true)
	return m2, tea.Batch(cmd, recreateCmd(m.client, spec))
}

// --- formatting/parsing ---

func formatPorts(ports []config.PortMap) string {
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		parts = append(parts, fmt.Sprintf("%d:%d", p.Host, p.Container))
	}
	return strings.Join(parts, ", ")
}

func parsePorts(raw string) ([]config.PortMap, error) {
	src := stripWhitespace(raw)
	if src == "" {
		return nil, nil
	}
	ports := []config.PortMap{}
	for _, seg := range strings.Split(src, ",") {
		if seg == "" {
			continue
		}
		parts := strings.Split(seg, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid mapping %q, expected host:container", seg)
		}
		host, err := strconv.Atoi(parts[0])
		if err != nil || host < 1 || host > 65535 {
			return nil, fmt.Errorf("invalid host port %q", parts[0])
		}
		container, err := strconv.Atoi(parts[1])
		if err != nil || container < 1 || container > 65535 {
			return nil, fmt.Errorf("invalid container port %q", parts[1])
		}
		ports = append(ports, config.PortMap{Host: host, Container: container})
	}
	return ports, nil
}

func formatVolumes(vols []config.VirtualVolume) string {
	parts := make([]string, 0, len(vols))
	for _, v := range vols {
		parts = append(parts, fmt.Sprintf("%s:%s", v.Name, v.Path))
	}
	return strings.Join(parts, ", ")
}

func parseVolumes(raw string) ([]config.VirtualVolume, error) {
	src := stripWhitespace(raw)
	if src == "" {
		return nil, nil
	}
	vols := []config.VirtualVolume{}
	for _, seg := range strings.Split(src, ",") {
		if seg == "" {
			continue
		}
		parts := strings.SplitN(seg, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid volume %q, expected name:path", seg)
		}
		vols = append(vols, config.VirtualVolume{Name: parts[0], Path: parts[1]})
	}
	return vols, nil
}

func formatEnvFiles(envs []string) string {
	return strings.Join(envs, ", ")
}

func parseEnvFiles(raw string) ([]string, error) {
	src := stripWhitespace(raw)
	if src == "" {
		return nil, nil
	}
	var envs []string
	for _, seg := range strings.Split(src, ",") {
		if seg != "" {
			envs = append(envs, seg)
		}
	}
	return envs, nil
}

func stripWhitespace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}
