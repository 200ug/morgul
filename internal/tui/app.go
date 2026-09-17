package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"codeberg.org/2ug/morgul/internal/build"
	"codeberg.org/2ug/morgul/internal/config"
	"codeberg.org/2ug/morgul/internal/podman"
)

const (
	notifTTL  = 15 * time.Second
	jkTimeout = 400 * time.Millisecond
)

type mode int

const (
	insertMode mode = iota
	normalMode
)

type page int

const (
	pageMain page = iota
	pageCreate
	pageEdit
	pageConfirm
)

type notification struct {
	text  string
	isErr bool
	setAt time.Time
}

type createState struct {
	stage   int
	profile string // preset id, or "custom"
	modules []string
	path    string
	shell   string
	mount   bool
	presets []config.Preset
	modlist []config.Module
}

type editState struct {
	spec     *build.Spec
	portsStr string
	volsStr  string
	envStr   string
	mount    bool
}

type confirmState struct {
	kind string // "remove" or "purge"
	name string
	ok   bool
}

type model struct {
	client   podman.Client
	store    *config.Store
	resolver *config.Resolver
	userHome string

	mode       mode
	search     textinput.Model
	containers []podman.Container
	filtered   []podman.Container
	selected   int
	pendingJ   bool
	notif      notification

	page      page
	form      *huh.Form
	createSt  *createState
	editSt    *editState
	confirmSt *confirmState

	width, height int
}

func New(client podman.Client, store *config.Store, userHome string) *model {
	ti := textinput.New()
	ti.Placeholder = "search containers..."
	ti.Prompt = "> "
	ti.CharLimit = 64
	ti.Focus()

	return &model{
		client:   client,
		store:    store,
		resolver: &config.Resolver{Store: store},
		userHome: userHome,
		mode:     insertMode,
		search:   ti,
		selected: 0,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.search.Focus(), listContainersCmd(m.client), refreshLoop(), notifTick())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case flushJMsg:
		if m.pendingJ {
			m.pendingJ = false
			m.search, _ = m.search.Update(keyRuneMsg('j'))
		}
		return m, nil

	case refreshMsg:
		if !m.notif.setAt.IsZero() && time.Since(m.notif.setAt) > notifTTL {
			m.notif = notification{}
		}
		return m, tea.Batch(listContainersCmd(m.client), refreshLoop())

	case notifTickMsg:
		if !m.notif.setAt.IsZero() && time.Since(m.notif.setAt) > notifTTL {
			m.notif = notification{}
		}
		return m, notifTick()

	case containersMsg:
		if msg.err != nil {
			m.notif = notification{text: "Error: " + msg.err.Error(), isErr: true, setAt: time.Now()}
			return m, nil
		}
		m.containers = msg.containers
		m.applyFilter()
		return m, nil

	case opResultMsg:
		if msg.err != nil {
			m.notif = notification{text: "Error: " + msg.err.Error(), isErr: true, setAt: time.Now()}
		} else {
			text := msg.notif
			if msg.warn != "" {
				text += " (" + msg.warn + ")"
			}
			m.notif = notification{text: text, setAt: time.Now()}
		}
		return m, listContainersCmd(m.client)

	case attachDoneMsg:
		if msg.err != nil {
			m.notif = notification{text: "Error: " + msg.err.Error(), isErr: true, setAt: time.Now()}
		} else {
			m.notif = notification{text: "Detached from container", setAt: time.Now()}
		}
		return m, listContainersCmd(m.client)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	if m.page != pageMain {
		return m.updateForm(msg)
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.page != pageMain {
		return m.updateForm(msg)
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyUp:
		m.moveSelection(-1)
		return m, nil
	case tea.KeyDown:
		m.moveSelection(1)
		return m, nil
	case tea.KeyEnter:
		return m, m.attachSelected()
	}

	if m.mode == insertMode {
		return m.handleInsertKey(msg)
	}
	return m.handleNormalKey(msg)
}

func (m model) handleInsertKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = normalMode
		m.search.Blur()
		return m, nil
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'j':
				if m.pendingJ {
					m.pendingJ = false
					m.search, _ = m.search.Update(keyRuneMsg('j'))
				}
				m.pendingJ = true
				return m, tea.Tick(jkTimeout, func(time.Time) tea.Msg { return flushJMsg{} })
			case 'k':
				if m.pendingJ {
					m.pendingJ = false
					m.mode = normalMode
					m.search.Blur()
					return m, nil
				}
			}
		}
	}

	if m.pendingJ {
		m.pendingJ = false
		m.search, _ = m.search.Update(keyRuneMsg('j'))
	}
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	m.applyFilter()
	return m, cmd
}

func (m model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		switch msg.Runes[0] {
		case 'i':
			m.mode = insertMode
			return m, m.search.Focus()
		case 'j':
			m.moveSelection(1)
			return m, nil
		case 'k':
			m.moveSelection(-1)
			return m, nil
		case 'n':
			return m.startCreate()
		case 'e':
			return m.startEdit()
		case 's':
			return m, m.actOnSelected(false)
		case 'x':
			return m, m.actOnSelected(true)
		case 'd':
			return m.startConfirm("remove")
		case 'p':
			return m.startConfirm("purge")
		case 'q':
			return m, tea.Quit
		}
	}
	return m, nil
}

func keyRuneMsg(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func (m *model) applyFilter() {
	query := m.search.Value()
	if query == "" {
		m.filtered = m.containers
	} else {
		names := make([]string, len(m.containers))
		for i, c := range m.containers {
			names[i] = c.Name
		}
		matches := fuzzyFind(query, names)
		m.filtered = make([]podman.Container, 0, len(matches))
		for _, mt := range matches {
			m.filtered = append(m.filtered, m.containers[mt])
		}
	}
	m.clampSelection()
}

func (m *model) clampSelection() {
	if len(m.filtered) == 0 {
		m.selected = -1
		return
	}
	if m.selected < 0 || m.selected >= len(m.filtered) {
		m.selected = 0
	}
}

func (m *model) moveSelection(delta int) {
	if len(m.filtered) == 0 {
		return
	}
	m.selected += delta
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.filtered) {
		m.selected = len(m.filtered) - 1
	}
}

func (m model) selectedContainer() *podman.Container {
	if m.selected < 0 || m.selected >= len(m.filtered) {
		return nil
	}
	return &m.filtered[m.selected]
}

func (m model) attachSelected() tea.Cmd {
	c := m.selectedContainer()
	if c == nil {
		return nil
	}
	return attachCmd(m.client, *c)
}

// Stops or kills the selected container. Action needs only the container name,
// so it works even when the spec (pod_spec.json) is missing.
func (m model) actOnSelected(force bool) tea.Cmd {
	c := m.selectedContainer()
	if c == nil {
		return nil
	}
	return stopCmd(m.client, c.Name, force)
}

// The TUI has no visible separators; panes are rendered with explicit widths
// and joined flush.
func (m model) View() string {
	if m.page != pageMain && m.form != nil {
		return m.form.View()
	}
	return m.mainView()
}

func (m model) mainView() string {
	notifH := 0
	if m.notif.text != "" {
		notifH = 1
	}
	paneH := m.height - notifH
	if paneH < 1 {
		paneH = 1
	}
	leftW := m.width / 2
	rightW := m.width - leftW
	if leftW < 1 {
		leftW = 1
	}
	if rightW < 1 {
		rightW = 1
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, m.leftPane(leftW, paneH), m.detailsPane(rightW, paneH))

	var out strings.Builder
	if notifH > 0 {
		out.WriteString(m.notifView())
		out.WriteString("\n")
	}
	out.WriteString(body)
	return out.String()
}

func (m model) notifView() string {
	if m.notif.isErr {
		return notifError.Render(m.notif.text)
	}
	return notifInfo.Render(m.notif.text)
}

func (m model) leftPane(width, height int) string {
	listH := height - 2
	if listH < 0 {
		listH = 0
	}
	search := padWidth(width).Render(m.search.View())
	list := m.listView(width, listH)
	status := m.statusLine(width)
	return lipgloss.JoinVertical(lipgloss.Left, search, list, status)
}

func (m model) listView(width, height int) string {
	if len(m.filtered) == 0 {
		return padWidth(width).Render(dimStyle.Render("  (no containers)"))
	}
	start := 0
	if m.selected >= height {
		start = m.selected - height + 1
	}
	lines := make([]string, 0, height)
	for i := start; i < len(m.filtered) && len(lines) < height; i++ {
		c := m.filtered[i]
		line := "  " + c.Name
		if i == m.selected {
			line = "▸ " + c.Name
			line = selectedStyle.Render(line)
		} else if !c.Running {
			line = dimStyle.Render(line)
		}
		lines = append(lines, line)
	}
	return padWidth(width).Render(strings.Join(lines, "\n"))
}

func (m model) statusLine(width int) string {
	mode := "INSERT"
	if m.mode == normalMode {
		mode = "NORMAL"
	}
	left := fmt.Sprintf("%d container(s)", len(m.containers))
	line := left + "  " + modeStyle.Render("["+mode+"]")
	return padWidth(width).Render(dimStyle.Render(line))
}

func (m model) detailsPane(width, height int) string {
	style := padWidth(width)
	c := m.selectedContainer()
	if c == nil {
		return style.Render(dimStyle.Render("No container selected"))
	}

	preset := c.Preset
	if preset == "" {
		preset = "custom"
	}
	modules := strings.Join(c.Modules, ", ")
	if modules == "" {
		modules = "-"
	}
	ports := "-"
	if len(c.Ports) > 0 {
		parts := make([]string, 0, len(c.Ports))
		for _, p := range c.Ports {
			parts = append(parts, fmt.Sprintf("%d:%d/%s", p.Host, p.Container, p.Protocol))
		}
		ports = strings.Join(parts, ", ")
	}
	mounts := "-"
	if len(c.Mounts) > 0 {
		mounts = strings.Join(c.Mounts, ", ")
	}
	status := "down"
	if c.Running {
		status = "running"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", labelStyle.Render("Name:"), c.Name)
	fmt.Fprintf(&b, "%s %s\n", labelStyle.Render("Status:"), status)
	fmt.Fprintf(&b, "%s %s\n", labelStyle.Render("Preset:"), preset)
	fmt.Fprintf(&b, "%s %s\n", labelStyle.Render("Modules:"), modules)
	fmt.Fprintf(&b, "%s %s\n", labelStyle.Render("Image:"), c.ImageTag)
	fmt.Fprintf(&b, "%s %s\n", labelStyle.Render("Project:"), c.ProjectPath)
	fmt.Fprintf(&b, "%s %s\n", labelStyle.Render("Ports:"), ports)
	fmt.Fprintf(&b, "%s %s\n", labelStyle.Render("Mounts:"), mounts)
	return style.Render(b.String())
}
