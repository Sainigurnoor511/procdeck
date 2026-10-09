package ui

import (
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// fabricatedReport keeps the geometry assertions environment-independent: the
// real report's wording depends on the terminal and the config directory, and
// a test that renders it would pass or fail on the machine, not on the code.
// The values are deliberately longer than a 60-column panel can hold.
func fabricatedReport() []reportRow {
	return []reportRow{
		{"color profile", "truecolor, 24-bit (detected by lipgloss)"},
		{"truecolor", "in use"},
		{"background", "dark (detected)"},
		{"terminal size", "120 x 40, as reported to procdeck"},
		{"hyperlinks", "unknown: a guess from WT_SESSION, TERM_PROGRAM and VTE_VERSION, none of which are set — nothing is probed and procdeck emits the escape regardless"},
		{"config file", `C:\Users\someone\AppData\Roaming\procdeck\config.json`},
		{"config status", "readable · directory writable"},
		{"scan interval", "every 2s · the built-in default — nothing is remembered yet"},
		{"last scan", "5 processes, 4 services, 4 visible"},
		{"no access to", "1 process — run procdeck as administrator to manage them"},
		{"version", "procdeck 0.1.0-dev · go1.27.1 · windows/amd64"},
	}
}

func openHelp(t *testing.T, m Model) Model {
	t.Helper()
	next, cmd := runCommand(m, "/help")
	if next.mode != modeHelp {
		t.Fatalf("/help left mode %d, want modeHelp", next.mode)
	}
	if cmd != nil {
		t.Errorf("/help produced a command: %T", cmd())
	}
	return next
}

func withReport(m Model, rows []reportRow) Model {
	m.mode, m.report, m.panelOffset = modeDoctor, rows, 0
	return m
}

// TestHelpIsGeneratedFromTheTables is the whole reason /help exists as
// generated content: it iterates the two source tables, so a binding or a
// command that is added without a description fails here rather than quietly
// going undocumented.
func TestHelpIsGeneratedFromTheTables(t *testing.T) {
	m := resize(fixture(t), 200, 50)
	text := strings.Join(m.helpLines(m.viewWidth()-2), "\n")

	if !strings.Contains(text, "KEYS") || !strings.Contains(text, "COMMANDS") {
		t.Error("/help is missing a section header")
	}
	for _, b := range keymap {
		if b.desc == "" {
			t.Errorf("keymap entry %q has no description for /help", b.keys)
			continue
		}
		if !strings.Contains(text, b.keys) {
			t.Errorf("/help does not list the key %q", b.keys)
		}
		if !strings.Contains(text, b.desc) {
			t.Errorf("/help does not describe %q", b.keys)
		}
	}
	for _, c := range commandOrder {
		if c.desc == "" {
			t.Errorf("command /%s has no description for /help", c.name)
			continue
		}
		if !strings.Contains(text, commandUsage(c)) {
			t.Errorf("/help does not list %q", commandUsage(c))
		}
		if !strings.Contains(text, c.desc) {
			t.Errorf("/help does not describe /%s", c.name)
		}
	}
}

func TestHelpPanelScrollsAndClamps(t *testing.T) {
	m := openHelp(t, resize(fixture(t), 80, 24))
	total, rows := len(m.panelLines()), m.panelRows()
	if total <= rows {
		t.Fatalf("help is %d lines in a %d-line panel, so there is nothing to scroll", total, rows)
	}
	bottom := total - rows

	if got := press(m, "up").panelOffset; got != 0 {
		t.Errorf("up at the top gave offset %d, want a clamp to 0", got)
	}
	if got := press(m, "k").panelOffset; got != 0 {
		t.Errorf("k at the top gave offset %d", got)
	}
	if got := press(m, "down").panelOffset; got != 1 {
		t.Errorf("down gave offset %d, want 1", got)
	}
	if got := press(m, "j").panelOffset; got != 1 {
		t.Errorf("j gave offset %d, want 1", got)
	}
	if got := press(m, "end").panelOffset; got != bottom {
		t.Errorf("end gave offset %d, want %d", got, bottom)
	}
	if got := press(m, "end", "down", "down", "down").panelOffset; got != bottom {
		t.Errorf("scrolling past the bottom gave offset %d, want a clamp to %d", got, bottom)
	}
	if got := press(m, "end", "home").panelOffset; got != 0 {
		t.Errorf("home gave offset %d", got)
	}
	if got := press(m, "end", "g").panelOffset; got != 0 {
		t.Errorf("g gave offset %d", got)
	}

	// a page moves more than a line and still clamps
	page := press(m, "pgdown").panelOffset
	if page <= 1 || page > bottom {
		t.Errorf("pgdown gave offset %d, want between 2 and %d", page, bottom)
	}
	if got := press(m, "end", "pgup").panelOffset; got >= bottom || got < 0 {
		t.Errorf("pgup from the bottom gave offset %d", got)
	}

	// the footer has to say where in the panel the view is
	if view := press(m, "down").View(); !strings.Contains(view, "of "+strconv.Itoa(total)) {
		t.Errorf("the panel footer does not report the position out of %d", total)
	}
}

func TestHelpPanelEscAndQReturnToTheList(t *testing.T) {
	for _, key := range []tea.KeyMsg{{Type: tea.KeyEsc}, {Type: tea.KeyRunes, Runes: []rune("q")}} {
		m := press(openHelp(t, resize(fixture(t), 80, 24)), "down", "down")
		next, cmd := m.Update(key)
		after := next.(Model)
		if after.mode != modeList {
			t.Errorf("%s left mode %d, want modeList", key, after.mode)
		}
		if after.panelOffset != 0 {
			t.Errorf("%s left the scroll offset at %d", key, after.panelOffset)
		}
		if cmd != nil {
			t.Errorf("%s produced a command: %T — q must close the panel, not quit", key, cmd())
		}
	}
}

func TestDoctorReportsTheFacts(t *testing.T) {
	configHome(t)
	path := writeConfig(t, `{"theme":"claude","refreshIntervalSeconds":5}`)

	m := resize(fixture(t), 120, 40).WithRefreshSeconds(5)
	m, cmd := runCommand(m, "/doctor")
	if m.mode != modeDoctor {
		t.Fatalf("/doctor left mode %d, want modeDoctor", m.mode)
	}
	if cmd != nil {
		t.Errorf("/doctor produced a command: %T", cmd())
	}
	if len(m.report) == 0 {
		t.Fatal("/doctor collected nothing")
	}

	byLabel := map[string]string{}
	var all strings.Builder
	for _, r := range m.report {
		if r.label == "" || r.value == "" {
			t.Errorf("incomplete report row %+v", r)
		}
		byLabel[r.label] = r.value
		all.WriteString(r.label + "\t" + r.value + "\n")
	}
	text := all.String()

	for _, want := range []string{
		path,                                 // the config path
		"every 5s",                           // the effective scan interval
		plural(len(m.snap.Procs), "process"), // the process count
		plural(len(m.services), "service"),   // and what they grouped into
		"120 x 40",                           // the size as procdeck sees it
		"administrator",                      // the permission note
		appVersion,                           // and who is reporting
	} {
		if !strings.Contains(text, want) {
			t.Errorf("/doctor never mentions %q:\n%s", want, text)
		}
	}
	for _, label := range []string{"color profile", "background", "config status", "scan interval"} {
		if byLabel[label] == "" {
			t.Errorf("/doctor has no %q row:\n%s", label, text)
		}
	}

	// the OSC 8 row is inferred from environment variables, not probed, and it
	// has to read that way: an overclaim here is a wrong diagnosis
	osc := byLabel["hyperlinks"]
	if !strings.Contains(osc, "guess") {
		t.Errorf("the OSC 8 row claims more than it knows: %q", osc)
	}
	if !strings.Contains(osc, "regardless") {
		t.Errorf("the OSC 8 row does not say procdeck emits the escape anyway: %q", osc)
	}
}

func TestDoctorSnapshotsOnceAndDoesNotRescan(t *testing.T) {
	configHome(t)
	writeConfig(t, `{"theme":"claude"}`)

	m, cmd := runCommand(resize(fixture(t), 120, 40), "/doctor")
	assertCommandIsNotAScan(t, cmd)

	// rendering must not touch the report: it is a snapshot, not a live probe
	before := append([]reportRow(nil), m.report...)
	m.View()
	m.View()
	if len(m.report) != len(before) {
		t.Fatalf("rendering changed the report length to %d", len(m.report))
	}
	for i := range before {
		if m.report[i] != before[i] {
			t.Errorf("rendering changed row %d from %+v to %+v", i, before[i], m.report[i])
		}
	}
}

// TestPanelsHoldTheExactHeight is the hard geometry contract: a panel replaces
// the table and detail boxes rather than floating over them, so the frame must
// still be exactly height lines with nothing wider than width.
func TestPanelsHoldTheExactHeight(t *testing.T) {
	report := fabricatedReport()
	for _, size := range [][2]int{{60, 8}, {60, 10}, {60, 12}, {80, 24}, {120, 40}, {200, 50}} {
		base := resize(fixture(t), size[0], size[1])
		help := openHelp(t, base)
		states := map[string]Model{
			"help": help,
			// scrolled to the end is where a padding bug in the last window shows
			"help-end":   press(help, "end"),
			"doctor":     withReport(base, report),
			"doctor-end": press(withReport(base, report), "end"),
			// an empty report is reachable before /doctor has ever run
			"doctor-empty": withReport(base, nil),
		}
		for name, m := range states {
			lines := strings.Split(m.View(), "\n")
			if len(lines) != size[1] {
				t.Errorf("%s at %dx%d: %d lines, want %d", name, size[0], size[1], len(lines), size[1])
			}
			for i, l := range lines {
				if w := lipgloss.Width(l); w > size[0] {
					t.Errorf("%s at %dx%d: line %d is %d wide", name, size[0], size[1], i, w)
				}
			}
			// the table area is replaced, not overlaid, so its head row is gone
			if strings.Contains(m.View(), "NAME") {
				t.Errorf("%s at %dx%d: the table is still showing behind the panel", name, size[0], size[1])
			}
		}
		// and the panel is exactly as tall as the two boxes it stands in for
		listLines := len(strings.Split(base.View(), "\n"))
		panelLines := len(strings.Split(help.View(), "\n"))
		if listLines != panelLines {
			t.Errorf("at %dx%d the list is %d lines and the panel is %d",
				size[0], size[1], listLines, panelLines)
		}
	}
}
