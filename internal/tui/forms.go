package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"codeberg.org/2ug/morgul/internal/build"
	"codeberg.org/2ug/morgul/internal/config"
	"codeberg.org/2ug/morgul/internal/podman"
)

func (m model) updateForm(msg tea.Msg) (model, tea.Cmd) {
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

func (m model) finishForm() (model, tea.Cmd) {
	switch m.page {
	case pageCreate:
		return m.finishCreateStage()
	case pageEdit:
		return m.finishEdit()
	case pageConfirm:
		return m.finishConfirm()
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

// --- create ---

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
	cwd, _ := os.Getwd()
	m.createSt = &createState{
		stage:   0,
		path:    cwd,
		shell:   config.DefaultShell,
		presets: presets,
		modlist: mods,
	}
	m.page = pageCreate
	m.form = m.buildCreateForm()
	return m, m.form.Init()
}

func (m model) buildCreateForm() *huh.Form {
	s := m.createSt
	switch s.stage {
	case 0:
		opts := make([]huh.Option[string], 0, len(s.presets)+1)
		for _, p := range s.presets {
			opts = append(opts, huh.NewOption(p.ID, p.ID))
		}
		opts = append(opts, huh.NewOption("custom...", "custom"))
		sel := huh.NewSelect[string]().Title("Profile").Options(opts...).Value(&s.profile).Filtering(true)
		return huh.NewForm(huh.NewGroup(sel)).WithTheme(formTheme()).WithKeyMap(formKeyMap)
	case 1:
		opts := make([]huh.Option[string], 0, len(s.modlist))
		for _, mod := range s.modlist {
			opts = append(opts, huh.NewOption(mod.ID, mod.ID))
		}
		ms := huh.NewMultiSelect[string]().Title("Modules").Options(opts...).Value(&s.modules).Filtering(true)
		return huh.NewForm(huh.NewGroup(ms)).WithTheme(formTheme()).WithKeyMap(formKeyMap)
	default:
		pathInput := huh.NewInput().Title("Project path").Value(&s.path).SuggestionsFunc(m.pathSuggestions, nil)
		shellInput := huh.NewInput().Title("Shell").Value(&s.shell)
		mount := huh.NewConfirm().Title("Project mount").Value(&s.mount)
		return huh.NewForm(huh.NewGroup(pathInput, shellInput, mount)).WithTheme(formTheme()).WithKeyMap(formKeyMap)
	}
}

func (m model) finishCreateStage() (model, tea.Cmd) {
	s := m.createSt
	switch s.stage {
	case 0:
		if s.profile == "custom" {
			s.stage = 1
		} else {
			if p, err := m.store.LoadPreset(s.profile); err == nil {
				if p.Shell != "" {
					s.shell = p.Shell
				}
				if p.ProjectMount != nil {
					s.mount = *p.ProjectMount
				}
			}
			s.stage = 2
		}
		m.form = m.buildCreateForm()
		return m, m.form.Init()
	case 1:
		s.stage = 2
		m.form = m.buildCreateForm()
		return m, m.form.Init()
	default:
		return m.finalizeCreate()
	}
}

func (m model) finalizeCreate() (model, tea.Cmd) {
	s := m.createSt
	var bp *config.Blueprint
	var err error
	if s.profile == "custom" {
		bp, err = m.resolver.ResolveCustom(s.modules, s.shell, &s.mount)
	} else {
		bp, err = m.resolver.ResolvePreset(s.profile)
		if err == nil {
			bp.Shell = s.shell
			mt := s.mount
			bp.ProjectMount = &mt
		}
	}
	if err != nil {
		return m.notifyError("Error: " + err.Error())
	}

	projectPath := s.path
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

func (m model) pathSuggestions() []string {
	if m.createSt == nil || m.createSt.path == "" {
		return nil
	}
	dir := m.createSt.path
	if !strings.HasSuffix(dir, "/") {
		dir = filepath.Dir(dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, filepath.Join(dir, e.Name())+"/")
		}
	}
	return out
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

// --- confirm ---

func (m model) startConfirm(kind string) (model, tea.Cmd) {
	if kind == "purge" {
		m.confirmSt = &confirmState{kind: "purge"}
	} else {
		c := m.selectedContainer()
		if c == nil {
			return m.notifyError("Error: no container selected")
		}
		m.confirmSt = &confirmState{kind: "remove", name: c.Name}
	}
	m.page = pageConfirm
	m.form = m.buildConfirmForm()
	return m, m.form.Init()
}

func (m model) buildConfirmForm() *huh.Form {
	s := m.confirmSt
	var confirm *huh.Confirm
	if s.kind == "purge" {
		confirm = huh.NewConfirm().
			Title(fmt.Sprintf("Purge all %d container(s)?", len(m.containers))).
			Description("Stop, remove, and delete all containers, images, volumes, and .morgul dirs. This cannot be undone.").
			Affirmative("Purge").Negative("Cancel").
			Value(&s.ok)
	} else {
		confirm = huh.NewConfirm().
			Title("Remove container " + s.name + "?").
			Description("This will also delete its image and .morgul data.").
			Affirmative("Remove").Negative("Cancel").
			Value(&s.ok)
	}
	return huh.NewForm(huh.NewGroup(confirm)).WithTheme(formTheme()).WithKeyMap(formKeyMap)
}

func (m model) finishConfirm() (model, tea.Cmd) {
	s := m.confirmSt
	m2, cmd := m.resetToMain()
	if !s.ok {
		return m2, cmd
	}
	if s.kind == "purge" {
		m2.notif = newNotification("Purging...", false, true)
		return m2, tea.Batch(cmd, purgeCmd(m.client, m.containers))
	}
	var c *podman.Container
	for i := range m.containers {
		if m.containers[i].Name == s.name {
			c = &m.containers[i]
			break
		}
	}
	if c == nil {
		m2.notif = newNotification("Error: container not found", true, false)
		return m2, cmd
	}
	m2.notif = newNotification("Removing container...", false, true)
	return m2, tea.Batch(cmd, removeCmd(m.client, *c))
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
