package ui

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"procdeck/internal/scan"
)

var t0 = time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)

func proc(pid int32, args, cwd string, ports ...int) scan.Proc {
	return scan.Proc{
		PID: pid, PPID: 1, Exe: "python", ExePath: cwd + `\.venv\Scripts\python.exe`,
		Cmdline: "python " + args, Cwd: cwd, Created: t0, Ports: ports,
	}
}

func snapshot(procs ...scan.Proc) snapMsg {
	return snapMsg{snap: scan.Snapshot{Self: 9999, Procs: procs, At: t0.Add(time.Hour)}}
}

func loaded(t *testing.T) Model {
	t.Helper()
	m, _ := New(nil).Update(snapshot(
		proc(10, "-m uvicorn main:app", `E:\dialer`, 8000),
		proc(20, "-m celery -A app worker", `E:\dialer`),
		proc(30, "-m celery -A app beat", `E:\dialer`),
	))
	return m.(Model)
}

func press(m Model, keys ...string) Model {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "backspace":
			msg = tea.KeyMsg{Type: tea.KeyBackspace}
		case "space":
			msg = tea.KeyMsg{Type: tea.KeySpace}
		case "pgup":
			msg = tea.KeyMsg{Type: tea.KeyPgUp}
		case "pgdown":
			msg = tea.KeyMsg{Type: tea.KeyPgDown}
		case "home":
			msg = tea.KeyMsg{Type: tea.KeyHome}
		case "end":
			msg = tea.KeyMsg{Type: tea.KeyEnd}
		case "ctrl+c":
			msg = tea.KeyMsg{Type: tea.KeyCtrlC}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

func TestFilterNarrowsAndEscClears(t *testing.T) {
	m := press(loaded(t), "f", "c", "e", "l", "enter")
	if len(m.visible) != 2 || m.mode != modeList {
		t.Fatalf("visible = %d, mode = %d", len(m.visible), m.mode)
	}
	m = press(m, "esc")
	if len(m.visible) != 3 {
		t.Errorf("esc did not clear the filter: %d visible", len(m.visible))
	}
	if m = press(loaded(t), "f", "8", "0", "0", "0"); len(m.visible) != 1 {
		t.Errorf("port filter matched %d", len(m.visible))
	}
}

func TestConfirmFlow(t *testing.T) {
	m := press(loaded(t), "x")
	if m.mode != modeConfirm || m.pending == nil {
		t.Fatal("x should ask for confirmation")
	}
	m = press(m, "n")
	if m.mode != modeList || m.pending != nil || m.status != "cancelled" {
		t.Errorf("n should cancel: mode=%d status=%q", m.mode, m.status)
	}

	m = press(loaded(t), "r")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd == nil || next.(Model).mode != modeList {
		t.Error("y should run the pending action")
	}
}

func TestSelectionFollowsPid(t *testing.T) {
	m := press(loaded(t), "down", "down")
	pid := m.selected
	next, _ := m.Update(snapshot(
		proc(5, "-m celery -A app worker", `E:\another`),
		proc(10, "-m uvicorn main:app", `E:\dialer`, 8000),
		proc(20, "-m celery -A app worker", `E:\dialer`),
		proc(30, "-m celery -A app beat", `E:\dialer`),
	))
	m = next.(Model)
	if svc, _ := m.current(); svc.Root.PID != pid {
		t.Errorf("selection moved to pid %d, want %d", svc.Root.PID, pid)
	}

	next, _ = m.Update(snapshot(proc(10, "-m uvicorn main:app", `E:\dialer`, 8000)))
	m = next.(Model)
	if m.cursor != 0 || m.selected != 10 {
		t.Errorf("cursor=%d selected=%d after list shrank", m.cursor, m.selected)
	}
}

func TestRefreshIntervalClamps(t *testing.T) {
	for _, c := range []struct {
		seconds int
		want    time.Duration
	}{
		{0, defaultRefreshSeconds * time.Second},
		{-1, defaultRefreshSeconds * time.Second},
		{maxRefreshSeconds + 1, defaultRefreshSeconds * time.Second},
		{minRefreshSeconds, time.Second},
		{30, 30 * time.Second},
		{maxRefreshSeconds, maxRefreshSeconds * time.Second},
	} {
		m := loaded(t).WithRefreshSeconds(c.seconds)
		if got := m.refreshInterval(); got != c.want {
			t.Errorf("refreshSeconds %d: interval %v, want %v", c.seconds, got, c.want)
		}
	}
}

// assertScheduledTick is the single-scan-ticker guard for the refresh chain. A
// tea.Batch command hands back its BatchMsg immediately and a scan closure
// returns as soon as the scan finishes, so anything that resolves promptly has
// forked the chain; a timer is the only thing that stays pending. The message
// channel is returned because bubbletea's Tick arms its timer when the command
// is built, so the closure resolves exactly once and must never be called
// twice.
func assertScheduledTick(t *testing.T, cmd tea.Cmd) <-chan tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("a chained scan scheduled no refresh")
	}
	got := make(chan tea.Msg, 1)
	go func() { got <- cmd() }()
	select {
	case msg := <-got:
		t.Fatalf("a chained scan produced an immediate %T — the refresh chain forked", msg)
	case <-time.After(250 * time.Millisecond):
	}
	return got
}

func TestChainedSnapSchedulesOneTickAtTheSetInterval(t *testing.T) {
	m := loaded(t).WithRefreshSeconds(1)
	start := time.Now()
	next, cmd := m.Update(snapMsg{snap: m.snap, chained: true})
	if next.(Model).refreshSeconds != 1 {
		t.Fatalf("refreshSeconds = %d, want 1", next.(Model).refreshSeconds)
	}
	got := assertScheduledTick(t, cmd)
	// and the one pending command really is a tick at the interval that was set
	select {
	case msg := <-got:
		if _, ok := msg.(tickMsg); !ok {
			t.Fatalf("the scheduled command produced %T, want tickMsg", msg)
		}
		if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
			t.Errorf("the tick fired after %v, want about 1s", elapsed)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the scheduled tick never fired")
	}
}

// TestChangingRefreshSecondsDoesNotForkTheChain is the mechanism from
// refreshInterval's comment: a new interval is read at schedule time, so each
// chained scan still schedules exactly one timer and no second chain starts.
func TestChangingRefreshSecondsDoesNotForkTheChain(t *testing.T) {
	m := loaded(t).WithRefreshSeconds(1)
	next, cmd := m.Update(snapMsg{snap: m.snap, chained: true})
	assertScheduledTick(t, cmd)

	m = next.(Model).WithRefreshSeconds(5)
	next, cmd = m.Update(snapMsg{snap: m.snap, chained: true})
	assertScheduledTick(t, cmd)
	if got := next.(Model).refreshInterval(); got != 5*time.Second {
		t.Errorf("interval = %v, want 5s", got)
	}

	// a one-off refresh still schedules nothing at all
	if _, cmd := next.(Model).Update(snapMsg{snap: m.snap}); cmd != nil {
		t.Error("an unchained scan scheduled a tick")
	}
}

func TestTickMsgYieldsOneSnap(t *testing.T) {
	m := loaded(t)
	m.scanner = scan.New()
	next, cmd := m.Update(tickMsg{})
	if cmd == nil {
		t.Fatal("a tick produced no scan")
	}
	msg, ok := cmd().(snapMsg)
	if !ok {
		t.Fatalf("a tick produced %T, want snapMsg", cmd())
	}
	if !msg.chained {
		t.Error("the tick's scan was not chained, so the refresh chain ends here")
	}
	if next.(Model).status != m.status {
		t.Errorf("a tick changed the status to %q", next.(Model).status)
	}
}

func TestConfigSavedMsgDoesNotCrashTheModel(t *testing.T) {
	next, cmd := loaded(t).Update(configSavedMsg{what: "scan interval"})
	if cmd != nil {
		t.Error("a successful config write produced a follow-up cmd")
	}
	if s := next.(Model).status; s != "" {
		t.Errorf("a successful config write set the status to %q", s)
	}

	next, cmd = loaded(t).Update(configSavedMsg{what: "scan interval", err: os.ErrPermission})
	after := next.(Model)
	if cmd != nil {
		t.Error("a failed config write produced a follow-up cmd")
	}
	if after.statusKind != statusError || !strings.Contains(after.status, "could not remember the scan interval") {
		t.Errorf("status = %q kind = %d", after.status, after.statusKind)
	}
}

func TestViewLinesFitWidth(t *testing.T) {
	for _, size := range [][2]int{{60, 8}, {60, 10}, {60, 12}, {80, 24}, {120, 40}, {200, 50}} {
		next, _ := loaded(t).Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		base := next.(Model)
		// the suggestion rows are reserved in the layout budget, so command
		// mode has to hold the exact-height contract on its own
		states := map[string]Model{
			"list":         base,
			"cmd-empty":    press(base, "/"),
			"cmd-matching": press(base, "/", "e", "n"),
			"cmd-args":     press(base, "/", "e", "n", "d", "space", "c", "e", "l"),
		}
		for name, m := range states {
			lines := strings.Split(m.View(), "\n")
			if len(lines) != size[1] {
				t.Errorf("%s at %dx%d: %d lines", name, size[0], size[1], len(lines))
			}
			for i, l := range lines {
				if w := lipgloss.Width(l); w > size[0] {
					t.Errorf("%s at %dx%d: line %d is %d wide", name, size[0], size[1], i, w)
				}
			}
			if rows := m.tableRows(); rows < 1 {
				t.Errorf("%s at %dx%d: tableRows = %d", name, size[0], size[1], rows)
			}
		}
	}
}

func TestInputBoxGeometry(t *testing.T) {
	states := map[string]Model{
		"idle":      loaded(t),
		"filtered":  press(loaded(t), "f", "c", "e", "l", "enter"),
		"filtering": press(loaded(t), "f", "c", "e", "l"),
		"confirm":   press(loaded(t), "x"),
		"command":   press(loaded(t), "/"),
		"cmd-typed": press(loaded(t), "/", "t", "h"),
		"cmd-args":  press(loaded(t), "/", "f", "i", "l", "t", "e", "r", "space"),
	}
	for name, m := range states {
		for _, w := range []int{60, 80, 120} {
			box := m.inputBox(w)
			if got := lipgloss.Width(box); got != w {
				t.Errorf("%s at w=%d: width = %d", name, w, got)
			}
			if got := lipgloss.Height(box); got != 3 {
				t.Errorf("%s at w=%d: height = %d", name, w, got)
			}
		}
	}
}

func TestConfirmPromptInInputBox(t *testing.T) {
	want := ""
	if svc, ok := loaded(t).current(); ok {
		want = describe(svc)
	}
	if want == "" {
		t.Fatal("no selected service")
	}
	for _, size := range [][2]int{{60, 9}, {80, 24}} {
		next, _ := loaded(t).Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		view := press(next.(Model), "x").View()
		if !strings.Contains(view, "y / n") {
			t.Errorf("%dx%d: no confirm prompt in view", size[0], size[1])
		}
		if !strings.Contains(view, want) {
			t.Errorf("%dx%d: view does not describe %q", size[0], size[1], want)
		}
	}

	// a real-world-length prompt must still show the answer keys
	m := loaded(t)
	m.mode, m.prompt = modeConfirm, "End python: app.scheduler_service · closeloop-backend (2 processes)?"
	for _, w := range []int{60, 80, 120} {
		box := m.inputBox(w)
		if !strings.Contains(box, "y / n") {
			t.Errorf("w=%d: a long prompt hid the answer keys: %q", w, box)
		}
		if got := lipgloss.Width(box); got != w {
			t.Errorf("w=%d: width = %d", w, got)
		}
	}
	// widths too narrow for the prompt at all must degrade, not panic
	for _, w := range []int{11, 14} {
		box := m.inputBox(w)
		if got := lipgloss.Width(box); got != w {
			t.Errorf("w=%d: width = %d", w, got)
		}
		if got := lipgloss.Height(box); got != 3 {
			t.Errorf("w=%d: height = %d", w, got)
		}
	}
}

func TestHyperlinkSurvivesSelection(t *testing.T) {
	// inner is 118 at this width, so layout() allocates the link column
	next, _ := loaded(t).Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	// move onto the uvicorn row so the linked row is also the selected one
	m := press(next.(Model), "down", "down")
	if svc, _ := m.current(); svc.Link != "http://localhost:8000" {
		t.Fatalf("selected service link = %q", svc.Link)
	}
	view := m.View()

	const open = "\x1b]8;;http://localhost:8000\x1b\\"
	const term = "\x1b]8;;\x1b\\"
	i := strings.Index(view, open)
	if i < 0 {
		t.Fatal("opening OSC 8 sequence is missing")
	}
	rest := view[i+len(open):]
	j := strings.Index(rest, term)
	if j < 0 {
		t.Fatal("OSC 8 terminator is missing")
	}
	if text := rest[:j]; strings.Contains(text, "\x1b]") || strings.Contains(text, "\n") {
		t.Errorf("link text is broken up: %q", text)
	}
}

func TestSpinnerStopsWhenIdle(t *testing.T) {
	// loaded() fed a snapshot, so stopBusy already cleared busy
	next, cmd := loaded(t).Update(spinMsg{})
	m := next.(Model)
	if cmd != nil {
		t.Error("spinner chain kept going with nothing in flight")
	}
	if m.spinning {
		t.Error("spinning should be false once the chain ends")
	}

	m.busy, m.busyAt = "working", time.Now()
	before := m.frame
	next, cmd = m.Update(spinMsg{})
	if cmd == nil {
		t.Fatal("spinner chain stopped while busy")
	}
	if got := next.(Model).frame; got != before+1 {
		t.Errorf("frame = %d, want %d", got, before+1)
	}
}

func TestSpinnerDoesNotRescan(t *testing.T) {
	m := loaded(t)
	m.busy, m.busyAt = "working", time.Now()
	_, cmd := m.Update(spinMsg{})
	if cmd == nil {
		t.Fatal("no spinner cmd")
	}
	if got, want := fmt.Sprintf("%T", cmd()), fmt.Sprintf("%T", spinMsg{}); got != want {
		t.Errorf("spinMsg produced %s, want %s", got, want)
	}
}

func TestCtrlCClearsThenExits(t *testing.T) {
	m := press(loaded(t), "f", "c", "e", "l", "ctrl+c")
	if m.mode != modeList || m.filter != "" || len(m.visible) != 3 {
		t.Fatalf("mode=%d filter=%q visible=%d", m.mode, m.filter, len(m.visible))
	}
	if m.statusKind != statusHint {
		t.Errorf("statusKind = %d, want statusHint", m.statusKind)
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("second ctrl+c returned no cmd")
	}
	if got, want := fmt.Sprintf("%T", cmd()), fmt.Sprintf("%T", tea.Quit()); got != want {
		t.Errorf("second ctrl+c produced %s, want %s", got, want)
	}
}

func TestCtrlCWindowExpires(t *testing.T) {
	m := loaded(t)
	m.lastCtrlC = time.Now().Add(-5 * time.Second)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("stale ctrl+c returned no cmd")
	}
	if got := fmt.Sprintf("%T", cmd()); got == fmt.Sprintf("%T", tea.Quit()) {
		t.Error("ctrl+c outside the window quit the app")
	}
	if next.(Model).statusKind != statusHint {
		t.Error("stale ctrl+c should re-arm the hint")
	}
}
