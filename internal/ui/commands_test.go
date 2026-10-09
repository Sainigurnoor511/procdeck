package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCommandParse(t *testing.T) {
	for _, c := range []struct {
		line, name, arg string
	}{
		{"/theme", "theme", ""},
		{"theme", "theme", ""},
		{"/THEME", "theme", ""},
		{"/filter cel ery", "filter", "cel ery"},
		{"/filter   cel", "filter", "cel"},
		{"/", "", ""},
		{"//help", "/help", ""}, // exactly one slash comes off
		{"   ", "", ""},
		{"", "", ""},
	} {
		name, arg := parseCommand(c.line)
		if name != c.name || arg != c.arg {
			t.Errorf("parseCommand(%q) = (%q, %q), want (%q, %q)", c.line, name, arg, c.name, c.arg)
		}
	}
}

func TestCommandUnknownNameErrors(t *testing.T) {
	for _, line := range []string{"/nope", "//help", "/"} {
		m, cmd := runCommand(loaded(t), line)
		if m.mode != modeList {
			t.Errorf("%q left mode %d", line, m.mode)
		}
		if m.statusKind != statusError {
			t.Errorf("%q set status kind %d, want an error", line, m.statusKind)
		}
		if !strings.Contains(m.status, "/help") {
			t.Errorf("%q said %q without pointing at /help", line, m.status)
		}
		if name, _ := parseCommand(line); name != "" && !strings.Contains(m.status, name) {
			t.Errorf("%q said %q without naming the input", line, m.status)
		}
		assertCommandIsNotAScan(t, cmd)
	}
}

func TestCommandFilterMatchesTheFKey(t *testing.T) {
	viaKey := press(loaded(t), "f", "c", "e", "l", "enter")
	viaCmd, cmd := runCommand(loaded(t), "/filter cel")
	if len(viaCmd.visible) != len(viaKey.visible) || viaCmd.filter != viaKey.filter {
		t.Errorf("/filter cel gave filter %q and %d visible, f c e l gave %q and %d",
			viaCmd.filter, len(viaCmd.visible), viaKey.filter, len(viaKey.visible))
	}
	if viaCmd.mode != modeList {
		t.Errorf("/filter left mode %d", viaCmd.mode)
	}
	assertCommandIsNotAScan(t, cmd)

	empty, _ := runCommand(loaded(t), "/filter")
	if empty.statusKind != statusError || !strings.Contains(empty.status, "/clear") {
		t.Errorf("a bare /filter said %q", empty.status)
	}
	if empty.filter != "" {
		t.Errorf("a bare /filter set the filter to %q", empty.filter)
	}
}

func TestCommandClearClearsFilterAndStatus(t *testing.T) {
	m, _ := runCommand(loaded(t), "/filter cel")
	m.setStatus("something happened", true)
	m, cmd := runCommand(m, "/clear")
	if m.filter != "" || m.status != "" || m.statusKind != statusInfo {
		t.Errorf("/clear left filter %q status %q kind %d", m.filter, m.status, m.statusKind)
	}
	if len(m.visible) != 3 {
		t.Errorf("/clear left %d visible, want 3", len(m.visible))
	}
	assertCommandIsNotAScan(t, cmd)
}

func TestCommandAllMatchesTheAKey(t *testing.T) {
	viaKey := press(loaded(t), "a")
	viaCmd, cmd := runCommand(loaded(t), "/all")
	if viaCmd.showAll != viaKey.showAll || len(viaCmd.visible) != len(viaKey.visible) {
		t.Errorf("/all gave showAll=%v %d visible, a gave %v %d",
			viaCmd.showAll, len(viaCmd.visible), viaKey.showAll, len(viaKey.visible))
	}
	assertCommandIsNotAScan(t, cmd)
}

func TestCommandTheme(t *testing.T) {
	configHome(t)

	named, cmd := runCommand(loaded(t), "/theme nord")
	if named.theme.Name != "nord" || !strings.Contains(named.status, "theme: nord") {
		t.Errorf("/theme nord gave theme %q status %q", named.theme.Name, named.status)
	}
	assertCommandIsNotAScan(t, cmd)

	mixed, _ := runCommand(loaded(t), "/theme NORD")
	if mixed.theme.Name != "nord" {
		t.Errorf("/theme NORD gave theme %q", mixed.theme.Name)
	}

	// no argument cycles, exactly like the t key
	viaKey := press(loaded(t), "t")
	cycled, cmd := runCommand(loaded(t), "/theme")
	if cycled.theme.Name != viaKey.theme.Name {
		t.Errorf("/theme cycled to %q, t cycled to %q", cycled.theme.Name, viaKey.theme.Name)
	}
	assertCommandIsNotAScan(t, cmd)

	bogus, cmd := runCommand(loaded(t), "/theme bogus")
	if bogus.statusKind != statusError || !strings.Contains(bogus.status, "bogus") {
		t.Errorf("/theme bogus said %q", bogus.status)
	}
	for _, name := range ThemeNames() {
		if !strings.Contains(bogus.status, name) {
			t.Errorf("/theme bogus did not name %q: %q", name, bogus.status)
		}
	}
	if bogus.theme.Name != loaded(t).theme.Name {
		t.Errorf("/theme bogus changed the theme to %q", bogus.theme.Name)
	}
	assertCommandIsNotAScan(t, cmd)
}

func TestCommandThemesLists(t *testing.T) {
	m, cmd := runCommand(loaded(t), "/themes")
	for _, name := range ThemeNames() {
		if !strings.Contains(m.status, name) {
			t.Errorf("/themes did not list %q: %q", name, m.status)
		}
	}
	if !strings.Contains(m.status, m.activeTheme().Name) {
		t.Errorf("/themes did not mark the active theme: %q", m.status)
	}
	assertCommandIsNotAScan(t, cmd)
}

func TestCommandRefresh(t *testing.T) {
	configHome(t)

	// no argument reports and mutates nothing
	before := loaded(t).WithRefreshSeconds(7)
	reported, cmd := runCommand(before, "/refresh")
	if reported.refreshSeconds != 7 {
		t.Errorf("a bare /refresh changed the interval to %d", reported.refreshSeconds)
	}
	if !strings.Contains(reported.status, "7s") || reported.statusKind == statusError {
		t.Errorf("a bare /refresh said %q", reported.status)
	}
	if cmd != nil {
		t.Error("a bare /refresh produced a command")
	}

	set, cmd := runCommand(loaded(t), "/refresh 5")
	if set.refreshSeconds != 5 || set.statusKind == statusError {
		t.Errorf("/refresh 5 gave %d and said %q", set.refreshSeconds, set.status)
	}
	if !strings.Contains(set.status, "next scan") {
		t.Errorf("/refresh 5 did not say when it takes effect: %q", set.status)
	}
	if cmd == nil {
		t.Fatal("/refresh 5 did not persist the interval")
	}
	if msg, ok := cmd().(configSavedMsg); !ok || msg.err != nil {
		t.Errorf("/refresh 5 produced %#v", msg)
	}

	for _, line := range []string{"/refresh abc", "/refresh 0", "/refresh -3", "/refresh 99999"} {
		bad, cmd := runCommand(loaded(t), line)
		if bad.statusKind != statusError {
			t.Errorf("%q was accepted: %q", line, bad.status)
		}
		if bad.refreshSeconds != 0 {
			t.Errorf("%q set the interval to %d", line, bad.refreshSeconds)
		}
		if !strings.Contains(bad.status, "1 second") {
			t.Errorf("%q did not state the floor: %q", line, bad.status)
		}
		assertCommandIsNotAScan(t, cmd)
	}
}

// TestCommandDestructiveCommandsRouteThroughConfirm is the hard one: a slash
// command must not be a shortcut past the confirm prompt, and its wording must
// be the same one the single key produces.
func TestCommandDestructiveCommandsRouteThroughConfirm(t *testing.T) {
	for _, c := range []struct {
		line, key string
	}{
		{"/end", "x"},
		{"/restart", "r"},
		{"/end-duplicates", "D"},
	} {
		viaKey := press(fixture(t), c.key)
		viaCmd, cmd := runCommand(fixture(t), c.line)
		if viaCmd.mode != modeConfirm {
			t.Fatalf("%s left mode %d, want modeConfirm", c.line, viaCmd.mode)
		}
		if viaCmd.pending == nil {
			t.Fatalf("%s left no pending command", c.line)
		}
		if viaCmd.prompt != viaKey.prompt {
			t.Errorf("%s prompted %q, %s prompted %q", c.line, viaCmd.prompt, c.key, viaKey.prompt)
		}
		if cmd != nil {
			t.Errorf("%s acted before the confirm", c.line)
		}

		cancelled := press(viaCmd, "n")
		if cancelled.mode != modeList || cancelled.pending != nil || cancelled.status != "cancelled" {
			t.Errorf("%s then n left mode %d pending %v status %q",
				c.line, cancelled.mode, cancelled.pending != nil, cancelled.status)
		}
	}
}

func TestCommandResolvesServiceByName(t *testing.T) {
	m, cmd := runCommand(loaded(t), "/end uvicorn")
	if m.mode != modeConfirm || m.pending == nil {
		t.Fatalf("/end uvicorn left mode %d", m.mode)
	}
	if !strings.Contains(m.prompt, "uvicorn") {
		t.Errorf("/end uvicorn prompted %q", m.prompt)
	}
	if cmd != nil {
		t.Error("/end uvicorn acted before the confirm")
	}

	// the user is typing, not pasting, so case must not matter
	mixed, _ := runCommand(loaded(t), "/end UVICORN")
	if mixed.mode != modeConfirm {
		t.Errorf("/end UVICORN is not case-insensitive: mode %d status %q", mixed.mode, mixed.status)
	}

	missing, cmd := runCommand(loaded(t), "/end zzz")
	if missing.mode != modeList || missing.pending != nil {
		t.Errorf("/end zzz left mode %d pending %v", missing.mode, missing.pending != nil)
	}
	if missing.statusKind != statusError || !strings.Contains(missing.status, "zzz") {
		t.Errorf("/end zzz said %q", missing.status)
	}
	assertCommandIsNotAScan(t, cmd)

	empty := loaded(t)
	empty.visible = nil
	empty.services = nil
	none, _ := runCommand(empty, "/end")
	if none.mode != modeList || none.statusKind != statusError {
		t.Errorf("/end on an empty list left mode %d status %q", none.mode, none.status)
	}
}

func TestCommandAmbiguousNameActsOnNothing(t *testing.T) {
	base := fixture(t)
	want := 0
	for _, svc := range base.visible {
		if strings.Contains(strings.ToLower(svc.Name), "cel") ||
			strings.Contains(strings.ToLower(svc.Project), "cel") {
			want++
		}
	}
	if want < 2 {
		t.Fatalf("the fixture has %d services matching cel, want at least 2", want)
	}

	m, cmd := runCommand(base, "/end cel")
	if m.mode != modeList || m.pending != nil {
		t.Fatalf("an ambiguous /end entered mode %d with pending %v", m.mode, m.pending != nil)
	}
	if m.statusKind != statusError {
		t.Errorf("an ambiguous /end was not an error: %q", m.status)
	}
	if !strings.Contains(m.status, fmt.Sprintf("%d services match", want)) {
		t.Errorf("an ambiguous /end said %q, want the count %d", m.status, want)
	}
	for _, svc := range base.visible {
		if !strings.Contains(strings.ToLower(svc.Name), "cel") {
			continue
		}
		if !strings.Contains(m.status, svc.Name) {
			t.Errorf("an ambiguous /end did not name %q: %q", svc.Name, m.status)
		}
	}
	assertCommandIsNotAScan(t, cmd)
}

func TestCommandConfigAndVersionReport(t *testing.T) {
	m, cmd := runCommand(loaded(t).WithRefreshSeconds(9), "/config")
	for _, want := range []string{"config:", "claude", "9s"} {
		if !strings.Contains(m.status, want) {
			t.Errorf("/config said %q, want it to mention %q", m.status, want)
		}
	}
	assertCommandIsNotAScan(t, cmd)

	m, cmd = runCommand(loaded(t), "/version")
	for _, want := range []string{"procdeck", appVersion, "go1"} {
		if !strings.Contains(m.status, want) {
			t.Errorf("/version said %q, want it to mention %q", m.status, want)
		}
	}
	assertCommandIsNotAScan(t, cmd)
}

func TestCommandQuit(t *testing.T) {
	_, cmd := runCommand(loaded(t), "/quit")
	if cmd == nil {
		t.Fatal("/quit produced no command")
	}
	if got, want := fmt.Sprintf("%T", cmd()), fmt.Sprintf("%T", tea.Quit()); got != want {
		t.Errorf("/quit produced %s, want %s", got, want)
	}
}

func TestCommandModeEntryAndExit(t *testing.T) {
	m := press(loaded(t), "/")
	if m.mode != modeCommand || m.cmdInput != "" || m.cmdSel != 0 {
		t.Fatalf("/ gave mode %d input %q sel %d", m.mode, m.cmdInput, m.cmdSel)
	}
	if !m.suggestOpen() {
		t.Error("/ did not open the suggestion list")
	}

	if got := press(m, "esc"); got.mode != modeList || got.cmdInput != "" {
		t.Errorf("esc gave mode %d input %q", got.mode, got.cmdInput)
	}

	// backspacing past the / returns to the list, one rune at a time
	typed := press(m, "t", "h")
	if typed.cmdInput != "th" {
		t.Fatalf("typing gave %q", typed.cmdInput)
	}
	if got := press(typed, "backspace"); got.cmdInput != "t" || got.mode != modeCommand {
		t.Errorf("one backspace gave mode %d input %q", got.mode, got.cmdInput)
	}
	if got := press(typed, "backspace", "backspace", "backspace"); got.mode != modeList {
		t.Errorf("backspacing past the / left mode %d", got.mode)
	}

	// ctrl+c cancels the command input the way it cancels everything else
	if got := press(typed, "ctrl+c"); got.mode != modeList || got.cmdInput != "" {
		t.Errorf("ctrl+c gave mode %d input %q", got.mode, got.cmdInput)
	}
}

func TestCommandSuggestionNavigation(t *testing.T) {
	m := press(loaded(t), "/")
	down := press(m, "down")
	if down.cmdSel != 1 {
		t.Errorf("down gave sel %d", down.cmdSel)
	}
	if got := press(m, "up"); got.cmdSel != 0 {
		t.Errorf("up at the top gave sel %d, want a clamp to 0", got.cmdSel)
	}

	// the selection clamps to the match count, never past it
	far := m
	for i := 0; i < len(commandOrder)+5; i++ {
		far = press(far, "down")
	}
	if far.cmdSel != len(commandOrder)-1 {
		t.Errorf("sel ran to %d, want a clamp at %d", far.cmdSel, len(commandOrder)-1)
	}

	// typing resets the selection, so enter cannot run a stale highlight
	if got := press(down, "t"); got.cmdSel != 0 {
		t.Errorf("typing left sel at %d", got.cmdSel)
	}
}

func TestCommandTabCompletesWithoutExecuting(t *testing.T) {
	m := press(loaded(t), "/", "t", "h", "tab")
	if m.mode != modeCommand {
		t.Fatalf("tab left mode %d, want modeCommand", m.mode)
	}
	if m.cmdInput != "theme " {
		t.Errorf("tab gave %q, want \"theme \" — a command with arguments gets a trailing space", m.cmdInput)
	}
	if m.theme.Name != loaded(t).theme.Name {
		t.Error("tab executed the completion")
	}
	// the trailing space closes the list, because arguments come next
	if m.suggestOpen() {
		t.Error("the suggestion list is still open after a trailing space")
	}

	// a command that takes no arguments completes without the space
	noArgs := press(loaded(t), "/", "t", "h", "e", "m", "e", "s", "tab")
	if noArgs.cmdInput != "themes" {
		t.Errorf("tab gave %q, want \"themes\"", noArgs.cmdInput)
	}
}

func TestCommandEnterRunsSelectionThenLiteralText(t *testing.T) {
	configHome(t)

	// the popup is open, so enter runs the highlighted suggestion
	m := press(loaded(t), "/", "t", "h", "e", "m", "e", "s")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	after := next.(Model)
	if after.mode != modeList || after.cmdInput != "" {
		t.Errorf("enter left mode %d input %q", after.mode, after.cmdInput)
	}
	if !strings.Contains(after.status, "themes:") {
		t.Errorf("enter did not run the selection: %q", after.status)
	}

	// once a space is typed the list is closed, so enter takes the literal text
	typed := press(loaded(t), "/", "t", "h", "e", "m", "e", "space", "n", "o", "r", "d")
	if typed.suggestOpen() {
		t.Fatal("the suggestion list is open while arguments are being typed")
	}
	next, cmd := typed.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := next.(Model).theme.Name; got != "nord" {
		t.Errorf("enter on %q gave theme %q", typed.cmdInput, got)
	}
	assertCommandIsNotAScan(t, cmd)
}

func TestCommandSuggestionsNarrowAsYouType(t *testing.T) {
	all := press(loaded(t), "/")
	if got := len(matchCommands(all.cmdInput)); got != len(commandOrder) {
		t.Errorf("an empty input matched %d commands, want %d", got, len(commandOrder))
	}
	if got := len(matchCommands(press(all, "t", "h").cmdInput)); got != 2 {
		t.Errorf("th matched %d commands, want theme and themes", got)
	}
	// a non-match shows nothing rather than the whole registry
	if got := matchCommands(press(all, "z", "z", "z").cmdInput); got != nil {
		t.Errorf("zzz matched %d commands", len(got))
	}
	if press(all, "z", "z", "z").suggestionRows() != 0 {
		t.Error("a non-match still reserved suggestion rows")
	}
}

// assertCommandIsNotAScan is the single-scan-ticker guard for slash commands:
// no command may produce a snapMsg or a tickMsg, or the refresh chain forks.
// Commands with real side effects (/open, /copy) are never invoked here.
func assertCommandIsNotAScan(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case snapMsg, tickMsg:
		t.Fatalf("a slash command produced %T — the refresh chain forked", msg)
	case themeSavedMsg, configSavedMsg:
	default:
		t.Fatalf("a slash command produced an unexpected %T", msg)
	}
}
