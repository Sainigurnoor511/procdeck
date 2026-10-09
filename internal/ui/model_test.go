package ui

import (
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
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

func TestFilterNarrowsAndEscClears(t *testing.T) {
	m := press(loaded(t), "/", "c", "e", "l", "enter")
	if len(m.visible) != 2 || m.mode != modeList {
		t.Fatalf("visible = %d, mode = %d", len(m.visible), m.mode)
	}
	m = press(m, "esc")
	if len(m.visible) != 3 {
		t.Errorf("esc did not clear the filter: %d visible", len(m.visible))
	}
	if m = press(loaded(t), "/", "8", "0", "0", "0"); len(m.visible) != 1 {
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

func TestViewLinesFitWidth(t *testing.T) {
	for _, size := range [][2]int{{60, 12}, {80, 24}, {120, 40}, {200, 50}} {
		next, _ := loaded(t).Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		lines := strings.Split(next.(Model).View(), "\n")
		if len(lines) > size[1] {
			t.Errorf("%dx%d: %d lines", size[0], size[1], len(lines))
		}
		for i, l := range lines {
			if w := lipgloss.Width(l); w > size[0] {
				t.Errorf("%dx%d: line %d is %d wide", size[0], size[1], i, w)
			}
		}
	}
}
