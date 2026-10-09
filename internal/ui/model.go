// Package ui is the Bubble Tea front end.
package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"procdeck/internal/actions"
	"procdeck/internal/group"
	"procdeck/internal/scan"
)

const (
	refreshEvery = 2 * time.Second
	spinEvery    = 120 * time.Millisecond
	ctrlCWindow  = 1500 * time.Millisecond
)

// asterisk family, all width-1 in runewidth
var spinFrames = []string{"·", "✢", "✳", "∗", "✻", "✽"}

type mode int

const (
	modeList mode = iota
	modeFilter
	modeConfirm
)

type statusKind int

const (
	statusInfo statusKind = iota
	statusError
	statusHint
)

type snapMsg struct {
	snap scan.Snapshot
	err  error
	// chained scans schedule the next tick; one-off refreshes do not
	chained bool
}

type tickMsg struct{}

// spinMsg drives the spinner frames only. It must never trigger a scan, or the
// single refresh chain forks.
type spinMsg struct{}

type hintExpiredMsg struct{}

type doneMsg struct {
	text string
	err  error
}

type Model struct {
	scanner *scan.Scanner
	snap    scan.Snapshot
	loaded  bool

	services []group.Service
	visible  []group.Service
	cursor   int
	offset   int
	selected int32

	showAll bool
	filter  string
	mode    mode
	prompt  string
	pending tea.Cmd

	busy     string
	busyAt   time.Time
	busyFor  time.Duration
	frame    int
	spinning bool

	lastCtrlC time.Time

	status     string
	statusKind statusKind
	width      int
	height     int
}

func New(scanner *scan.Scanner) Model {
	// the first scan is already in flight from Init, so seed the spinner here:
	// a value-receiver Init cannot mutate the model
	return Model{
		scanner: scanner, width: 100, height: 30,
		busy: "scanning", busyAt: time.Now(), spinning: true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.scan(true), spinCmd())
}

func (m Model) scan(chained bool) tea.Cmd {
	scanner := m.scanner
	return func() tea.Msg {
		snap, err := scanner.Snapshot()
		return snapMsg{snap: snap, err: err, chained: chained}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampScroll()
		return m, nil

	case snapMsg:
		m.stopBusy()
		if msg.err != nil {
			m.setStatus("scan failed: "+msg.err.Error(), true)
		} else {
			m.snap, m.loaded = msg.snap, true
			m.rebuild()
		}
		if msg.chained {
			return m, tea.Tick(refreshEvery, func(time.Time) tea.Msg { return tickMsg{} })
		}
		return m, nil

	case tickMsg:
		return m, m.scan(true)

	case spinMsg:
		if m.busy == "" {
			// nothing in flight: end the frame chain so an idle app goes quiet
			m.spinning = false
			return m, nil
		}
		m.frame++
		m.busyFor = time.Since(m.busyAt)
		return m, spinCmd()

	case hintExpiredMsg:
		if m.statusKind == statusHint {
			m.status, m.statusKind = "", statusInfo
		}
		return m, nil

	case doneMsg:
		m.stopBusy()
		if msg.err != nil {
			m.setStatus(msg.err.Error(), true)
		} else {
			m.setStatus(msg.text, false)
		}
		return m, m.scan(false)

	case tea.KeyMsg:
		switch m.mode {
		case modeFilter:
			return m.updateFilter(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "ctrl+c":
		return m.interrupt()
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "pgup":
		m.move(-m.tableRows())
	case "pgdown":
		m.move(m.tableRows())
	case "home", "g":
		m.move(-len(m.visible))
	case "end", "G":
		m.move(len(m.visible))
	case "/":
		m.mode = modeFilter
	case "esc":
		m.filter = ""
		m.rebuild()
	case "a":
		m.showAll = !m.showAll
		m.rebuild()
	case "x":
		if svc, ok := m.current(); ok {
			m.ask(fmt.Sprintf("End %s?", describe(svc)), endCmd(svc))
		}
	case "r":
		if svc, ok := m.current(); ok {
			m.ask(fmt.Sprintf("Restart %s?", describe(svc)), restartCmd(svc))
		}
	case "D":
		if n := countDuplicates(m.services); n > 0 {
			m.ask(fmt.Sprintf("End all %d duplicates?", n), endDuplicatesCmd(m.services))
		} else {
			m.setStatus("no duplicates", false)
		}
	case "o", "enter":
		if svc, ok := m.current(); ok {
			return m, openCmd(svc)
		}
	case "c":
		if svc, ok := m.current(); ok {
			return m, copyCmd(svc)
		}
	}
	return m, nil
}

func (m Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m.interrupt()
	case tea.KeyEnter:
		m.mode = modeList
	case tea.KeyEsc:
		m.mode = modeList
		m.filter = ""
	case tea.KeyBackspace:
		if r := []rune(m.filter); len(r) > 0 {
			m.filter = string(r[:len(r)-1])
		}
	case tea.KeySpace:
		m.filter += " "
	case tea.KeyRunes:
		m.filter += string(msg.Runes)
	}
	m.rebuild()
	return m, nil
}

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m.interrupt()
	}
	cmd := m.pending
	m.mode, m.pending, m.prompt = modeList, nil, ""
	if s := msg.String(); s == "y" || s == "Y" {
		spin := m.startBusy("working")
		return m, tea.Batch(cmd, spin)
	}
	m.setStatus("cancelled", false)
	return m, nil
}

// interrupt is Claude Code's ctrl+c: the first press cancels whatever is on
// screen, a second press inside ctrlCWindow exits.
func (m Model) interrupt() (tea.Model, tea.Cmd) {
	now := time.Now()
	if !m.lastCtrlC.IsZero() && now.Sub(m.lastCtrlC) <= ctrlCWindow {
		return m, tea.Quit
	}
	m.lastCtrlC = now
	m.mode, m.prompt, m.pending = modeList, "", nil
	if m.filter != "" {
		m.filter = ""
		m.rebuild()
	}
	m.setHint("press ctrl+c again to exit")
	return m, tea.Tick(ctrlCWindow, func(time.Time) tea.Msg { return hintExpiredMsg{} })
}

func spinCmd() tea.Cmd {
	return tea.Tick(spinEvery, func(time.Time) tea.Msg { return spinMsg{} })
}

// startBusy returns a frame ticker only when one is not already running, so a
// second start never forks a second chain.
func (m *Model) startBusy(label string) tea.Cmd {
	m.busy, m.busyAt, m.busyFor, m.frame = label, time.Now(), 0, 0
	m.status, m.statusKind = "", statusInfo
	if m.spinning {
		return nil
	}
	m.spinning = true
	return spinCmd()
}

func (m *Model) stopBusy() {
	m.busy, m.busyFor = "", 0
}

func (m *Model) ask(prompt string, cmd tea.Cmd) {
	m.mode, m.prompt, m.pending = modeConfirm, prompt, cmd
}

func (m *Model) setStatus(text string, isErr bool) {
	m.status, m.statusKind = text, statusInfo
	if isErr {
		m.statusKind = statusError
	}
}

func (m *Model) setHint(text string) {
	m.status, m.statusKind = text, statusHint
}

func (m Model) current() (group.Service, bool) {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return group.Service{}, false
	}
	return m.visible[m.cursor], true
}

func (m *Model) move(delta int) {
	m.cursor += delta
	m.clampScroll()
	if svc, ok := m.current(); ok {
		m.selected = svc.Root.PID
	}
}

func (m *Model) rebuild() {
	m.services = group.Build(m.snap, group.Options{All: m.showAll})
	m.visible = m.visible[:0:0]
	needle := strings.ToLower(strings.TrimSpace(m.filter))
	for _, svc := range m.services {
		if needle == "" || strings.Contains(searchText(svc), needle) {
			m.visible = append(m.visible, svc)
		}
	}
	for i, svc := range m.visible {
		if svc.Root.PID == m.selected {
			m.cursor = i
			break
		}
	}
	m.clampScroll()
	if svc, ok := m.current(); ok {
		m.selected = svc.Root.PID
	}
}

func (m *Model) clampScroll() {
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	rows := m.tableRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	if max := len(m.visible) - rows; m.offset > max {
		m.offset = max
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func searchText(svc group.Service) string {
	parts := []string{svc.Name, svc.Project, svc.Root.Cmdline, svc.Variant}
	for _, p := range svc.Ports {
		parts = append(parts, strconv.Itoa(p))
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func describe(svc group.Service) string {
	s := fmt.Sprintf("%s · %s", svc.Name, svc.Project)
	if n := len(svc.Procs); n > 1 {
		s += fmt.Sprintf(" (%d processes)", n)
	}
	return s
}

func countDuplicates(services []group.Service) int {
	n := 0
	for _, s := range services {
		if s.Duplicate {
			n++
		}
	}
	return n
}

func endCmd(svc group.Service) tea.Cmd {
	return func() tea.Msg {
		return doneMsg{text: "ended " + describe(svc), err: actions.End(svc)}
	}
}

func restartCmd(svc group.Service) tea.Cmd {
	return func() tea.Msg {
		ownEnv, err := actions.Restart(svc)
		text := "restarted " + describe(svc) + " in a new window"
		if ownEnv {
			text += " (original environment unreadable, used procdeck's)"
		}
		return doneMsg{text: text, err: err}
	}
}

func endDuplicatesCmd(services []group.Service) tea.Cmd {
	return func() tea.Msg {
		n, err := actions.EndDuplicates(services)
		return doneMsg{text: fmt.Sprintf("ended %d duplicates", n), err: err}
	}
}

func openCmd(svc group.Service) tea.Cmd {
	return func() tea.Msg {
		return doneMsg{text: "opened " + svc.Link, err: actions.Open(svc.Link)}
	}
}

func copyCmd(svc group.Service) tea.Cmd {
	return func() tea.Msg {
		return doneMsg{text: "copied command of " + svc.Name, err: actions.Copy(svc.Root.Cmdline)}
	}
}
