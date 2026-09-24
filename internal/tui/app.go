package tui

import (
	"fmt"
	"regexp"
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

	// frame padding separating the app from the terminal edges
	padTop    = 1
	padBottom = 1
	padLeft   = 2
	padRight  = 2

	// horizontal padding applied inside each bordered box
	boxHPad = 1

	// blank rows between the list entries and the count/mode line
	listMargin = 2

	// minimum content widths so boxes stay usable on narrow terminals
	listMinW = 20
	podMinW  = 20
)

// confirmKeyRe matches the confirm field's standalone accept/reject shortcut
// keys, which huh renders as lowercase "y"/"n".
var confirmKeyRe = regexp.MustCompile(`\b[yn]\b`)

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
	// Zero for persistent notifications, which survive the TTL wipe
	// until they are replaced by a later notification.
	setAt time.Time
}

// Builds a notification. Persistent notifications stay until they are replaced;
// others are cleared automatically after notifTTL.
func newNotification(text string, isErr, persistent bool) notification {
	n := notification{text: text, isErr: isErr}
	if !persistent {
		n.setAt = time.Now()
	}
	return n
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

func New(client podman.Client, store *config.Store, userHome string, colors config.Colors) *model {
	applyColors(colors)

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
			m.notif = newNotification("Error: "+msg.err.Error(), true, false)
			return m, nil
		}
		m.containers = msg.containers
		m.applyFilter()
		return m, nil

	case opResultMsg:
		if msg.err != nil {
			m.notif = newNotification("Error: "+msg.err.Error(), true, false)
		} else {
			text := msg.notif
			if msg.warn != "" {
				text += " (" + msg.warn + ")"
			}
			m.notif = newNotification(text, false, false)
		}
		return m, listContainersCmd(m.client)

	case attachDoneMsg:
		if msg.err != nil {
			m.notif = newNotification("Error: "+msg.err.Error(), true, false)
		} else {
			m.notif = newNotification("Detached from container", false, false)
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

func (m model) View() string {
	if m.page != pageMain && m.form != nil {
		return m.formView()
	}
	return m.mainView()
}

// Renders the active form wrapped in the same border and frame padding as the
// main view.
func (m model) formView() string {
	m.form.WithWidth(m.formWidth())
	v := strings.TrimRight(m.form.View(), "\n")
	v = capitalizeConfirmKeys(v)
	if v == "" {
		return ""
	}
	boxed := boxStyle(lipgloss.Width(v), lipgloss.Height(v)).Render(v)
	return lipgloss.NewStyle().Padding(padTop, padRight, padBottom, padLeft).Render(boxed)
}

// NOTE: huh's confirm field hardcodes the accept/reject shortcut keys as
// lowercase "y"/"n" during rendering, ignoring the keymap. Capitalize just
// those two on the (unstyled) help footer line.
func capitalizeConfirmKeys(v string) string {
	lines := strings.Split(v, "\n")
	last := lines[len(lines)-1]
	if !strings.Contains(last, " • ") {
		return v
	}
	lines[len(lines)-1] = confirmKeyRe.ReplaceAllStringFunc(last, strings.ToUpper)
	return strings.Join(lines, "\n")
}

// Width the form should render at, so its box and frame padding stay within the
// terminal. Uses huh's default 80 columns when there is room.
func (m model) formWidth() int {
	// border (2) + box padding (2*boxHPad) + frame padding (padLeft+padRight)
	avail := m.width - 2 - 2*boxHPad - padLeft - padRight
	if avail > 80 {
		avail = 80
	}
	if avail < 1 {
		avail = 1
	}
	return avail
}

func (m model) mainView() string {
	maxW := m.width - padLeft - padRight
	maxH := m.height - padTop - padBottom
	if maxW < 1 {
		maxW = 1
	}
	if maxH < 1 {
		maxH = 1
	}

	podLines := m.podLines()

	// content widths; the border (2) and padding (2*boxHPad) add 4 per box
	listW := m.listContentWidth()
	if listW < listMinW {
		listW = listMinW
	}
	podW := 0
	for _, l := range podLines {
		if w := lipgloss.Width(l); w > podW {
			podW = w
		}
	}
	if podW < podMinW {
		podW = podMinW
	}
	// keep the middle section within the terminal, truncating the pod first
	if avail := maxW - 8 - listW; podW > avail {
		podW = avail
	}
	if podW < podMinW {
		podW = podMinW
		listW = maxW - 8 - podW
		if listW < 1 {
			listW = 1
		}
	}

	// middle height: list rows + margin + count/mode + blank, or the pod info
	// plus one blank row; capped to the terminal
	listRows := len(m.filtered)
	if listRows < 1 {
		listRows = 1 // "(no containers)" placeholder row
	}
	listH := listRows + listMargin + 2
	podH := len(podLines) + 1
	midH := listH
	if podH > midH {
		midH = podH
	}
	if cap := maxH - 8; midH > cap {
		midH = cap
	}
	if midH < 1 {
		midH = 1
	}

	// search and notification span the full middle width
	fullW := listW + podW + 4
	if fullW < 1 {
		fullW = 1
	}

	search := boxStyle(fullW, 1).Render(m.search.View())
	list := boxStyle(listW, midH).Render(m.listBox(listW, midH))
	pod := boxStyle(podW, midH).Render(m.podBox(podW, podLines))
	notif := boxStyle(fullW, 1).Render(m.notifView())

	middle := lipgloss.JoinHorizontal(lipgloss.Top, list, pod)
	body := lipgloss.JoinVertical(lipgloss.Top, search, middle, notif)
	return lipgloss.NewStyle().Padding(padTop, padRight, padBottom, padLeft).Render(body)
}

func (m model) notifView() string {
	if m.notif.isErr {
		return notifError.Render(m.notif.text)
	}
	return notifInfo.Render(m.notif.text)
}

// Width of the widest list entry or the status line, whichever is larger.
func (m model) listContentWidth() int {
	w := m.listNaturalWidth()
	if st := lipgloss.Width(m.statusText()); st > w {
		w = st
	}
	return w
}

// Width of the widest container list entry (selection marker, status tag, and
// name).
func (m model) listNaturalWidth() int {
	max := 0
	for _, c := range m.filtered {
		if len(c.Name) > max {
			max = len(c.Name)
		}
	}
	return max + 6
}

// Content of the container list box: entries, a margin, the count/mode line,
// and a trailing blank row.
func (m model) listBox(width, height int) string {
	rows := height - listMargin - 2
	if rows < 0 {
		rows = 0
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		m.listView(width, rows),
		padWidth(width).Render(""),
		padWidth(width).Render(""),
		m.statusLine(width),
		padWidth(width).Render(""),
	)
}

func (m model) podBox(width int, lines []string) string {
	rendered := make([]string, 0, len(lines))
	for _, l := range lines {
		rendered = append(rendered, padWidth(width).Render(l))
	}
	return strings.Join(rendered, "\n")
}

func (m model) listView(width, height int) string {
	if height <= 0 {
		return ""
	}
	lines := make([]string, 0, height)
	if len(m.filtered) == 0 {
		lines = append(lines, dimStyle.Render("  (no containers)"))
	} else {
		start := 0
		if m.selected >= height {
			start = m.selected - height + 1
		}
		for i := start; i < len(m.filtered) && len(lines) < height; i++ {
			c := m.filtered[i]
			status := "[d]"
			if c.Running {
				status = "[u]"
			}
			marker := "  "
			if i == m.selected {
				marker = "▸ "
			}
			line := marker + status + " " + c.Name
			if i == m.selected {
				line = selectedStyle.Render(line)
			}
			lines = append(lines, line)
		}
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return padWidth(width).Render(strings.Join(lines, "\n"))
}

func (m model) statusText() string {
	mode := "INSERT"
	if m.mode == normalMode {
		mode = "NORMAL"
	}
	return fmt.Sprintf("%d container(s)  [%s]", len(m.containers), mode)
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

// Lines shown in the pod info box.
func (m model) podLines() []string {
	c := m.selectedContainer()
	if c == nil {
		return []string{dimStyle.Render("No container selected")}
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

	return []string{
		labelStyle.Render("Name:") + " " + c.Name,
		labelStyle.Render("Status:") + " " + status,
		labelStyle.Render("Preset:") + " " + preset,
		labelStyle.Render("Modules:") + " " + modules,
		labelStyle.Render("Image:") + " " + c.ImageTag,
		labelStyle.Render("Project:") + " " + c.ProjectPath,
		labelStyle.Render("Ports:") + " " + ports,
		labelStyle.Render("Mounts:") + " " + mounts,
	}
}
