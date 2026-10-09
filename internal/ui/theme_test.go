package ui

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	colorful "github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/termenv"

	"procdeck/internal/scan"
)

var updateBaseline = flag.Bool("update", false, "rewrite the golden render baseline")

// TestMain pins the color profile so the goldens carry exact 24-bit sequences.
// CLICOLOR_FORCE alone only reaches ANSI-16, which quantizes distinct theme
// hexes into the same slot and makes per-theme assertions meaningless.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	lipgloss.SetHasDarkBackground(true)
	os.Exit(m.Run())
}

// fixture is loaded() plus a real duplicate (a second celery worker in the same
// folder) and a restricted process, so the ▲ and ○ glyph paths both render.
//
// The younger worker is explicitly newer: group.markDuplicates breaks a tie on
// Created by map order, so equal timestamps would make the render unstable.
func fixture(t *testing.T) Model {
	t.Helper()
	younger := proc(21, "-m celery -A app worker", `E:\dialer`)
	younger.Created = t0.Add(time.Minute)
	m, _ := New(nil).Update(snapshot(
		proc(10, "-m uvicorn main:app", `E:\dialer`, 8000),
		proc(20, "-m celery -A app worker", `E:\dialer`),
		younger,
		proc(30, "-m celery -A app beat", `E:\dialer`),
		scan.Proc{PID: 40, PPID: 1, Exe: "guarded", Created: t0, Restricted: true},
	))
	return m.(Model)
}

func resize(m Model, w, h int) Model {
	next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return next.(Model)
}

func withBusy(m Model) Model {
	m.busy, m.busyAt, m.busyFor, m.frame = "working", t0, 7*time.Second, 3
	return m
}

func withStatus(m Model, text string, isErr bool) Model {
	m.setStatus(text, isErr)
	return m
}

func withHint(m Model, text string) Model {
	m.setHint(text)
	return m
}

// frames renders one string covering every state and size the themes have to
// survive. It is the byte-identity oracle for the claude theme.
func frames(m Model) string {
	var b strings.Builder
	for _, size := range [][2]int{{60, 10}, {80, 24}, {120, 40}} {
		base := resize(m, size[0], size[1])
		states := []struct {
			name string
			m    Model
		}{
			{"idle", base},
			{"filtering", press(base, "/", "c", "e", "l")},
			{"filtered", press(base, "/", "c", "e", "l", "enter")},
			{"confirm", press(base, "x")},
			{"all", press(base, "a")},
			{"moved", press(base, "down", "down")},
			{"busy", withBusy(base)},
			{"error", withStatus(base, "scan failed: access is denied", true)},
			{"ok", withStatus(base, "ended celery worker · dialer", false)},
			{"hint", withHint(base, "press ctrl+c again to exit")},
		}
		for _, st := range states {
			fmt.Fprintf(&b, "=== %s %dx%d\n", st.name, size[0], size[1])
			b.WriteString(st.m.View())
			b.WriteString("\n")
		}
	}
	return b.String()
}

const baselinePath = "testdata/claude-baseline.txt"

func TestClaudeRenderMatchesBaseline(t *testing.T) {
	got := frames(fixture(t))
	if *updateBaseline {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(baselinePath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log("wrote " + baselinePath)
		return
	}
	want, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("read baseline: %v (run with -update to create it)", err)
	}
	if got == string(want) {
		return
	}
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(want), "\n")
	for i := range gotLines {
		if i >= len(wantLines) {
			t.Fatalf("render grew to %d lines, baseline has %d", len(gotLines), len(wantLines))
		}
		if gotLines[i] != wantLines[i] {
			t.Fatalf("the default theme no longer renders byte-identically\nline %d\n got %q\nwant %q",
				i+1, gotLines[i], wantLines[i])
		}
	}
	t.Fatalf("render shrank to %d lines, baseline has %d", len(gotLines), len(wantLines))
}

// TestClaudeThemeIsUnchanged localises what the golden file can only detect:
// a single edited hex in the default palette.
func TestClaudeThemeIsUnchanged(t *testing.T) {
	want := Theme{
		Name:   "claude",
		Accent: lipgloss.AdaptiveColor{Dark: "#D97757", Light: "#B2552F"},
		Text:   lipgloss.AdaptiveColor{Dark: "#E8E4DF", Light: "#1C1A17"},
		Dim:    lipgloss.AdaptiveColor{Dark: "#8A827A", Light: "#6B635B"},
		Border: lipgloss.AdaptiveColor{Dark: "#45403A", Light: "#D6CFC7"},
		Green:  lipgloss.AdaptiveColor{Dark: "#7FA86A", Light: "#3F6B2E"},
		Amber:  lipgloss.AdaptiveColor{Dark: "#D3A05A", Light: "#8A5E14"},
		Red:    lipgloss.AdaptiveColor{Dark: "#C96A62", Light: "#A3362E"},
		SelBg:  lipgloss.AdaptiveColor{Dark: "#2E2A26", Light: "#EFE8E0"},
	}
	if themeOrder[0] != want {
		t.Errorf("the default palette changed\n got %+v\nwant %+v", themeOrder[0], want)
	}
	if defaultThemeName != want.Name {
		t.Errorf("defaultThemeName = %q, want %q", defaultThemeName, want.Name)
	}
}

func TestEveryThemeIsComplete(t *testing.T) {
	if len(themeOrder) != 6 {
		t.Errorf("registry holds %d themes, want 6", len(themeOrder))
	}
	seen := map[string]bool{}
	for _, th := range themeOrder {
		if th.Name == "" {
			t.Fatal("a theme has no name")
		}
		if seen[th.Name] {
			t.Errorf("duplicate theme name %q", th.Name)
		}
		seen[th.Name] = true

		for field, c := range palette(th) {
			for half, hex := range map[string]string{"Dark": c.Dark, "Light": c.Light} {
				if hex == "" {
					t.Errorf("%s: %s.%s is empty", th.Name, field, half)
					continue
				}
				if _, err := colorful.Hex(hex); err != nil {
					t.Errorf("%s: %s.%s = %q does not parse: %v", th.Name, field, half, hex, err)
				}
			}
		}
	}
}

// palette names every color field so the completeness and distinctness tests
// cannot silently skip one that is added later.
func palette(th Theme) map[string]lipgloss.AdaptiveColor {
	return map[string]lipgloss.AdaptiveColor{
		"Accent": th.Accent, "Text": th.Text, "Dim": th.Dim, "Border": th.Border,
		"Green": th.Green, "Amber": th.Amber, "Red": th.Red, "SelBg": th.SelBg,
	}
}

// TestSemanticColorsAreDistinct guards against an accidental near-duplicate in
// the running / duplicate / danger triple and against any of the three
// collapsing onto Accent, which is what mono's Red used to do. It is a
// separation floor, not a WCAG contrast claim.
func TestSemanticColorsAreDistinct(t *testing.T) {
	const floor = 0.08
	// mono gives up brightness between Text and Amber to free Red from
	// Accent, so the duplicate row must keep the weight channel instead
	if th, ok := themeByName("mono"); !ok || !th.DupBold {
		t.Errorf("mono DupBold = %v (found = %v), want true", th.DupBold, ok)
	}
	for _, th := range themeOrder {
		for _, half := range []string{"Dark", "Light"} {
			pick := func(c lipgloss.AdaptiveColor) colorful.Color {
				hex := c.Dark
				if half == "Light" {
					hex = c.Light
				}
				parsed, err := colorful.Hex(hex)
				if err != nil {
					t.Fatalf("%s: %q does not parse: %v", th.Name, hex, err)
				}
				return parsed
			}
			green, amber, red, accent := pick(th.Green), pick(th.Amber), pick(th.Red), pick(th.Accent)
			for _, pair := range []struct {
				name string
				a, b colorful.Color
			}{
				{"green/amber", green, amber},
				{"amber/red", amber, red},
				{"green/red", green, red},
				{"green/accent", green, accent},
				{"amber/accent", amber, accent},
				{"red/accent", red, accent},
			} {
				if d := pair.a.DistanceLab(pair.b); d < floor {
					t.Errorf("%s %s: %s distance %.3f < %.2f\n  green/amber %.3f  amber/red %.3f  green/red %.3f\n  green/accent %.3f  amber/accent %.3f  red/accent %.3f",
						th.Name, half, pair.name, d, floor,
						green.DistanceLab(amber), amber.DistanceLab(red), green.DistanceLab(red),
						green.DistanceLab(accent), amber.DistanceLab(accent), red.DistanceLab(accent))
				}
			}
		}
	}
}

// TestThemeDoesNotChangeGeometry is the hard constraint: a theme may only
// change colors, never a width or a line count.
func TestThemeDoesNotChangeGeometry(t *testing.T) {
	for _, size := range [][2]int{{60, 10}, {80, 24}, {120, 40}, {200, 50}} {
		var want []int
		for _, th := range themeOrder {
			m := resize(fixture(t).WithTheme(th.Name), size[0], size[1])
			lines := strings.Split(m.View(), "\n")
			if len(lines) != size[1] {
				t.Errorf("%s at %dx%d: %d lines", th.Name, size[0], size[1], len(lines))
				continue
			}
			widths := make([]int, len(lines))
			for i, l := range lines {
				widths[i] = lipgloss.Width(l)
				if widths[i] > size[0] {
					t.Errorf("%s at %dx%d: line %d is %d wide", th.Name, size[0], size[1], i, widths[i])
				}
			}
			if want == nil {
				want = widths
				continue
			}
			for i := range widths {
				if widths[i] != want[i] {
					t.Errorf("%s at %dx%d: line %d is %d wide, claude renders %d",
						th.Name, size[0], size[1], i, widths[i], want[i])
				}
			}
		}
	}
}

// TestHyperlinkIntactUnderEveryTheme repeats the OSC 8 assertions on the
// selected row for every palette, so no accent or selection background can
// corrupt the escape sequence.
func TestHyperlinkIntactUnderEveryTheme(t *testing.T) {
	const open = "\x1b]8;;http://localhost:8000\x1b\\"
	const term = "\x1b]8;;\x1b\\"
	for _, th := range themeOrder {
		m := resize(fixture(t).WithTheme(th.Name), 120, 24)
		// land on the uvicorn row so the linked row is also the selected one
		m = press(m, "down", "down", "down")
		svc, _ := m.current()
		if svc.Link != "http://localhost:8000" {
			t.Fatalf("%s: selected service link = %q", th.Name, svc.Link)
		}
		view := m.View()
		i := strings.Index(view, open)
		if i < 0 {
			t.Errorf("%s: opening OSC 8 sequence is missing", th.Name)
			continue
		}
		rest := view[i+len(open):]
		j := strings.Index(rest, term)
		if j < 0 {
			t.Errorf("%s: OSC 8 terminator is missing", th.Name)
			continue
		}
		if text := rest[:j]; strings.Contains(text, "\x1b]") || strings.Contains(text, "\n") {
			t.Errorf("%s: link text is broken up: %q", th.Name, text)
		}
	}
}

func TestThemeKeysCycle(t *testing.T) {
	names := ThemeNames()
	m := fixture(t)
	for i := 1; i <= len(names)+1; i++ {
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
		m = next.(Model)
		want := names[i%len(names)]
		if m.theme.Name != want {
			t.Fatalf("press %d of t: theme = %q, want %q", i, m.theme.Name, want)
		}
		if !strings.Contains(m.status, "theme: "+want) {
			t.Errorf("press %d of t: status = %q", i, m.status)
		}
		assertNotAScan(t, cmd)
	}

	m = fixture(t)
	for i := 1; i <= len(names)+1; i++ {
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("T")})
		m = next.(Model)
		want := names[((len(names)-i)%len(names)+len(names))%len(names)]
		if m.theme.Name != want {
			t.Fatalf("press %d of T: theme = %q, want %q", i, m.theme.Name, want)
		}
		assertNotAScan(t, cmd)
	}
}

// assertNotAScan is the single-scan-ticker guard: a theme switch must never
// produce a snapMsg or a tickMsg, only the theme write.
func assertNotAScan(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case themeSavedMsg:
	case snapMsg, tickMsg:
		t.Fatalf("a theme switch produced %T — the refresh chain forked", msg)
	default:
		t.Fatalf("a theme switch produced an unexpected %T", msg)
	}
}

func TestThemeKeysDoNotShadowExistingBindings(t *testing.T) {
	m := fixture(t)
	if len(m.visible) < 2 {
		t.Fatalf("fixture has %d visible services", len(m.visible))
	}
	// T must cycle backwards, not jump to the end of the list the way G does
	if got := press(m, "T"); got.cursor != m.cursor {
		t.Errorf("T moved the cursor from %d to %d", m.cursor, got.cursor)
	}
	if got := press(m, "t"); got.cursor != m.cursor {
		t.Errorf("t moved the cursor from %d to %d", m.cursor, got.cursor)
	}
	// and G must still be end-of-list, not a theme switch
	got := press(m, "G")
	if got.cursor != len(m.visible)-1 {
		t.Errorf("G put the cursor at %d, want %d", got.cursor, len(m.visible)-1)
	}
	if got.theme.Name != m.theme.Name {
		t.Errorf("G changed the theme to %q", got.theme.Name)
	}
}

func TestZeroModelRenders(t *testing.T) {
	m := Model{width: 80, height: 24}
	if m.activeTheme().Name != defaultThemeName {
		t.Errorf("a zero Model reports theme %q", m.activeTheme().Name)
	}
	view := m.View()
	if view == "" {
		t.Fatal("a zero Model rendered nothing")
	}
	if got := len(strings.Split(view, "\n")); got != 24 {
		t.Errorf("a zero Model rendered %d lines, want 24", got)
	}
}
