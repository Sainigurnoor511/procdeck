package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"
)

// The two panels replace the table and detail boxes rather than floating over
// them: Bubble Tea v1 has no compositor, and the exact-height contract means
// every line View emits has to be accounted for. panelRows is the budget they
// get, and it is exactly what the two boxes they stand in for would have used,
// so the frame height never moves when a panel opens.
func (m Model) panelRows() int {
	// tableRows()+1 is the table box's content (data rows plus the head row);
	// detailLines is the detail box whole, borders included
	n := m.tableRows() + 1
	if m.showDetail() {
		n += detailLines
	}
	return n
}

func (m Model) panelMode() bool { return m.mode == modeHelp || m.mode == modeDoctor }

// panelContent is the scrollable body of whichever panel is open, plus the
// title for its border. It is rebuilt per frame for /help, which is pure
// formatting of two static tables; /doctor only formats m.report, which was
// snapshotted when the command ran.
func (m Model) panelContent(inner int) (title string, lines []string) {
	if m.mode == modeDoctor {
		return "doctor", m.reportLines(inner)
	}
	return "help", m.helpLines(inner)
}

// helpLines is the whole point of keeping one keymap table and one command
// registry: this is generated from both, so a binding or a command cannot be
// documented here and nowhere else, or added to the app and forgotten here.
// Nothing below is hand-maintained text.
func (m Model) helpLines(inner int) []string {
	st := m.styles()

	keyW := 0
	for _, b := range keymap {
		if n := runewidth.StringWidth(b.keys); n > keyW {
			keyW = n
		}
	}
	nameW := 0
	for _, c := range commandOrder {
		if n := runewidth.StringWidth(commandUsage(c)); n > nameW {
			nameW = n
		}
	}
	// one gutter for both halves, so the two description columns line up and
	// the panel reads as one document instead of two tables
	gutter := keyW
	if nameW > gutter {
		gutter = nameW
	}
	left := gutter + 3 // leading space plus two columns of separation
	if left > inner-2 {
		left = inner - 2
	}
	if left < 1 {
		left = 1
	}
	row := func(key, desc string) string {
		return st.text.Render(fit(" "+key, left)) + st.dim.Render(fit(desc, inner-left))
	}

	out := []string{st.head.Render(fit(" KEYS", inner))}
	for _, b := range keymap {
		out = append(out, row(b.keys, b.desc))
	}
	out = append(out, strings.Repeat(" ", inner), st.head.Render(fit(" COMMANDS", inner)))
	for _, c := range commandOrder {
		out = append(out, row(commandUsage(c), c.desc))
	}
	return out
}

// commandUsage is how a command is written in /help and nowhere else is it
// spelled by hand: the slash, the name and the argument placeholder.
func commandUsage(c command) string {
	if c.args == "" {
		return "/" + c.name
	}
	return "/" + c.name + " " + c.args
}

// reportRow is one /doctor line. The value carries its own hedging where the
// fact is inferred rather than detected, because a diagnostic that reads as
// certain about a guess is worse than no diagnostic.
type reportRow struct {
	label string
	value string
}

// buildReport snapshots everything /doctor says. It runs once per invocation,
// never per frame: it stats the config file and creates a temp file to test
// the directory, and View is called on every key and every spinner frame.
func (m Model) buildReport() []reportRow {
	rows := []reportRow{
		{"color profile", profileName(lipgloss.ColorProfile()) + " (detected by lipgloss)"},
		{"truecolor", yesNo(lipgloss.ColorProfile() == termenv.TrueColor, "in use", "not in use")},
		{"background", backgroundName() + " (detected)"},
		{"terminal size", fmt.Sprintf("%d x %d, as reported to procdeck", m.width, m.height)},
		{"hyperlinks", osc8Guess()},
	}
	rows = append(rows, configRows()...)

	interval := int(m.refreshInterval().Seconds())
	source := "the built-in default — nothing is remembered yet"
	if n := m.refreshSeconds; n >= minRefreshSeconds && n <= maxRefreshSeconds {
		cfg, _ := loadConfig()
		source = fmt.Sprintf("the config file reads %ds", cfg.RefreshIntervalSeconds)
	}
	rows = append(rows,
		reportRow{"scan interval", fmt.Sprintf("every %ds · %s", interval, source)},
		reportRow{"last scan", fmt.Sprintf("%s, %s, %d visible",
			plural(len(m.snap.Procs), "process"), plural(len(m.services), "service"),
			len(m.visible))},
	)

	restricted := 0
	for _, p := range m.snap.Procs {
		if p.Restricted {
			restricted++
		}
	}
	note := "none — every process answered"
	if restricted > 0 {
		note = fmt.Sprintf("%s — run procdeck as administrator to manage them",
			plural(restricted, "process"))
	}
	return append(rows,
		reportRow{"no access to", note},
		reportRow{"version", fmt.Sprintf("procdeck %s · %s · %s/%s",
			appVersion, runtime.Version(), runtime.GOOS, runtime.GOARCH)},
	)
}

func profileName(p termenv.Profile) string {
	switch p {
	case termenv.TrueColor:
		return "truecolor, 24-bit"
	case termenv.ANSI256:
		return "256 colors"
	case termenv.ANSI:
		return "16 colors"
	case termenv.Ascii:
		return "no color"
	}
	return "unrecognised"
}

func backgroundName() string {
	if lipgloss.HasDarkBackground() {
		return "dark"
	}
	return "light"
}

func yesNo(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}

// osc8Guess is a guess and says so. There is no way to ask a terminal whether
// it understands OSC 8 without writing one and watching what happens, so this
// reads environment variables the common hyperlink-capable terminals set and
// reports what it found. procdeck emits the escape either way: a terminal that
// does not understand it ignores it, which is the whole point of the format.
func osc8Guess() string {
	var seen []string
	for _, key := range []string{"WT_SESSION", "TERM_PROGRAM", "VTE_VERSION"} {
		if os.Getenv(key) != "" {
			seen = append(seen, key)
		}
	}
	const tail = " — nothing is probed and procdeck emits the escape regardless"
	if len(seen) == 0 {
		return "unknown: a guess from WT_SESSION, TERM_PROGRAM and VTE_VERSION, none of which are set" + tail
	}
	return "probably: a guess from " + strings.Join(seen, ", ") + ", which is set" + tail
}

// configRows reports where the config lives, whether it can be read and
// whether the directory can be written. The write test is a real temp file
// because permissions, antivirus and a read-only profile directory all fail at
// create time and at no earlier point.
func configRows() []reportRow {
	path, err := configPath()
	if err != nil {
		return []reportRow{{"config file", "unavailable: " + err.Error()}}
	}
	read := "readable"
	switch _, err := os.ReadFile(path); {
	case os.IsNotExist(err):
		read = "absent — written on the first theme or interval change"
	case err != nil:
		read = "unreadable: " + err.Error()
	}
	write := "writable"
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, "doctor-*.tmp")
	if err != nil {
		write = "not writable: " + err.Error()
	} else {
		name := f.Name()
		f.Close()
		os.Remove(name)
	}
	return []reportRow{
		{"config file", path},
		{"config status", read + " · directory " + write},
	}
}

// reportLines formats the snapshotted report. An empty report means /doctor
// has not run in this session, which only the panel-geometry tests can reach.
func (m Model) reportLines(inner int) []string {
	st := m.styles()
	if len(m.report) == 0 {
		return []string{st.dim.Render(fit(" nothing collected — run /doctor again", inner))}
	}
	labelW := 0
	for _, r := range m.report {
		if n := runewidth.StringWidth(r.label); n > labelW {
			labelW = n
		}
	}
	left := labelW + 3
	if left > inner-2 {
		left = inner - 2
	}
	if left < 1 {
		left = 1
	}
	out := make([]string, 0, len(m.report))
	for _, r := range m.report {
		out = append(out, st.dim.Render(fit(" "+r.label, left))+st.text.Render(fit(r.value, inner-left)))
	}
	return out
}

// panelBox renders exactly panelRows() content lines no matter how long the
// panel is, padding a short panel and windowing a long one, and borrows box()'s
// footer slot for the position indicator the table box already uses.
func (m Model) panelBox(title string, lines []string, inner int) []string {
	rows := m.panelRows()
	off := clampOffset(m.panelOffset, len(lines), rows)
	end := off + rows
	if end > len(lines) {
		end = len(lines)
	}
	body := make([]string, 0, rows)
	body = append(body, lines[off:end]...)
	for len(body) < rows {
		body = append(body, strings.Repeat(" ", inner))
	}
	foot := ""
	if len(lines) > rows {
		foot = fmt.Sprintf("%d–%d of %d", off+1, end, len(lines))
	}
	return m.box(title, foot, body, inner)
}

func clampOffset(off, total, rows int) int {
	if max := total - rows; off > max {
		off = max
	}
	if off < 0 {
		off = 0
	}
	return off
}

// updatePanel is the key handling for both panels. They are read-only, so the
// only state is the scroll offset and the way out.
func (m Model) updatePanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m.interrupt()
	case "esc", "q":
		m.mode, m.panelOffset = modeList, 0
		return m, nil
	case "up", "k":
		m.panelOffset--
	case "down", "j":
		m.panelOffset++
	case "pgup":
		m.panelOffset -= m.panelRows()
	case "pgdown":
		m.panelOffset += m.panelRows()
	case "home", "g":
		m.panelOffset = 0
	case "end", "G":
		// clamped below, so there is no need to know the length twice
		m.panelOffset = len(m.panelLines())
	}
	m.panelOffset = clampOffset(m.panelOffset, len(m.panelLines()), m.panelRows())
	return m, nil
}

// panelLines is panelContent's body at the width the panel will render at, for
// the clamping that has to happen outside View.
func (m Model) panelLines() []string {
	_, lines := m.panelContent(m.viewWidth() - 2)
	return lines
}
