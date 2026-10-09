package group

import (
	"testing"
	"time"

	"procdeck/internal/scan"
)

const (
	venvPy   = `E:\Jungleworks\ai-dialer-backend\.venv\Scripts\python.exe`
	globalPy = `C:\Users\x\AppData\Local\Programs\Python\Python311\python.exe`
	backend  = `E:\Jungleworks\ai-dialer-backend`
)

var t0 = time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)

func py(pid, ppid int32, exePath, args string, age int, ports ...int) scan.Proc {
	return scan.Proc{
		PID: pid, PPID: ppid, Exe: "python", ExePath: exePath,
		Cmdline: `"` + exePath + `" ` + args, Cwd: backend,
		Created: t0.Add(time.Duration(age) * time.Second), Ports: ports, Mem: 100,
	}
}

func find(services []Service, name string) []Service {
	var out []Service
	for _, s := range services {
		if s.Name == name {
			out = append(out, s)
		}
	}
	return out
}

func TestVenvLauncherPairsAreOneService(t *testing.T) {
	snap := scan.Snapshot{Self: 9999, Procs: []scan.Proc{
		py(12276, 1, venvPy, "-m celery -A app.celery_app worker --pool=threads", 0),
		py(33176, 12276, globalPy, "-m celery -A app.celery_app worker --pool=threads", 1),
		py(30848, 1, venvPy, "-m celery -A app.celery_app beat", 2),
		py(22768, 30848, globalPy, "-m celery -A app.celery_app beat", 3),
		py(32504, 1, venvPy, "livekit_agent_worker.py dev", 4),
		py(25820, 32504, globalPy, "livekit_agent_worker.py dev", 5, 8081),
		py(24300, 25820, globalPy, `-c "from multiprocessing.spawn import spawn_main; spawn_main(parent_pid=25820)"`, 6),
		py(34228, 1, venvPy, "-m uvicorn main:app --host 0.0.0.0 --port 8000", 7),
		py(21164, 34228, globalPy, "-m uvicorn main:app --host 0.0.0.0 --port 8000", 8, 8000),
	}}
	services := Build(snap, Options{})
	if len(services) != 4 {
		t.Fatalf("got %d services, want 4: %+v", len(services), names(services))
	}
	for _, s := range services {
		if s.Duplicate {
			t.Errorf("%s wrongly marked duplicate", s.Name)
		}
		if s.Variant != "venv" {
			t.Errorf("%s variant = %q, want venv", s.Name, s.Variant)
		}
		if s.Project != "ai-dialer-backend" {
			t.Errorf("%s project = %q", s.Name, s.Project)
		}
	}
	lk := find(services, "livekit agent")[0]
	if len(lk.Procs) != 3 || lk.Procs[0].PID != 32504 {
		t.Errorf("livekit tree = %d procs, root %d", len(lk.Procs), lk.Procs[0].PID)
	}
	uv := find(services, "uvicorn")[0]
	if uv.Link != "http://localhost:8000" || uv.Mem != 200 {
		t.Errorf("uvicorn link=%q mem=%d", uv.Link, uv.Mem)
	}
}

func TestIndependentCopiesAreDuplicates(t *testing.T) {
	snap := scan.Snapshot{Self: 9999, Procs: []scan.Proc{
		py(10, 1, globalPy, "-m celery -A app.celery_app worker", 0),
		py(20, 2, venvPy, "-m celery -A app.celery_app worker", 50),
		py(21, 20, globalPy, "-m celery -A app.celery_app worker", 51),
		py(30, 3, venvPy, "-m celery -A app.celery_app beat", 60),
	}}
	services := Build(snap, Options{})
	workers := find(services, "celery worker")
	if len(workers) != 2 {
		t.Fatalf("got %d workers, want 2", len(workers))
	}
	for _, w := range workers {
		wantDup := w.Root.PID == 10
		if w.Duplicate != wantDup {
			t.Errorf("pid %d duplicate = %v, want %v", w.Root.PID, w.Duplicate, wantDup)
		}
	}
	if find(services, "celery beat")[0].Duplicate {
		t.Error("single beat must not be a duplicate")
	}
}

func TestOldestKeptWhenNoVenv(t *testing.T) {
	snap := scan.Snapshot{Self: 9999, Procs: []scan.Proc{
		py(10, 1, globalPy, "-m uvicorn main:app", 30),
		py(11, 2, globalPy, "-m uvicorn main:app", 5),
	}}
	for _, s := range Build(snap, Options{}) {
		if s.Duplicate != (s.Root.PID == 10) {
			t.Errorf("pid %d duplicate = %v", s.Root.PID, s.Duplicate)
		}
	}
}

func TestWrapperTakesChildName(t *testing.T) {
	web := `E:\Projects\web`
	node := func(pid, ppid int32, args string, age int, ports ...int) scan.Proc {
		return scan.Proc{PID: pid, PPID: ppid, Exe: "node", ExePath: `C:\Program Files\nodejs\node.exe`,
			Cmdline: "node " + args, Cwd: web, Created: t0.Add(time.Duration(age) * time.Second), Ports: ports}
	}
	snap := scan.Snapshot{Self: 9999, Procs: []scan.Proc{
		node(100, 1, `C:\npm\bin\npm-cli.js run dev`, 0),
		{PID: 101, PPID: 100, Exe: "cmd", Cmdline: "cmd /c next dev", Cwd: web, Created: t0.Add(time.Second)},
		node(102, 101, `E:\Projects\web\node_modules\next\dist\bin\next dev`, 2),
		node(103, 102, `E:\Projects\web\node_modules\next\dist\server\lib\start-server.js`, 3, 3000),
	}}
	services := Build(snap, Options{})
	if len(services) != 1 {
		t.Fatalf("got %v, want one service", names(services))
	}
	s := services[0]
	if s.Name != "next dev" || s.Root.PID != 100 || len(s.Procs) != 4 || s.Link != "http://localhost:3000" {
		t.Errorf("got name=%q root=%d procs=%d link=%q", s.Name, s.Root.PID, len(s.Procs), s.Link)
	}
}

func TestToolingHiddenUnlessAll(t *testing.T) {
	snap := scan.Snapshot{Self: 9999, Procs: []scan.Proc{
		{PID: 1, Exe: "node", Cmdline: `node C:\Users\x\.vscode\extensions\pylance\server.js`, Cwd: `C:\x`, Created: t0},
		{PID: 2, Exe: "svchost", Restricted: true, Created: t0},
		{PID: 3, Exe: "myapi", ExePath: `E:\bin\myapi.exe`, Cmdline: "myapi", Cwd: `E:\bin`, Created: t0, Ports: []int{9000}},
		{PID: 4, Exe: "spooler", ExePath: `C:\Windows\System32\spooler.exe`, Cmdline: "spooler", Created: t0, Ports: []int{515}},
	}}
	def := Build(snap, Options{})
	if len(def) != 1 || def[0].Name != "myapi" {
		t.Errorf("default view = %v, want only the user listener", names(def))
	}
	if all := Build(snap, Options{All: true}); len(all) != 4 {
		t.Errorf("all view = %v, want 4", names(all))
	}
}

func TestSelfTreeIsProtected(t *testing.T) {
	snap := scan.Snapshot{Self: 51, Procs: []scan.Proc{
		py(50, 1, venvPy, "dev_shell.py", 0),
		{PID: 51, PPID: 50, Exe: "procdeck", Cmdline: "procdeck", Cwd: backend, Created: t0.Add(time.Second)},
		py(60, 1, venvPy, "other.py", 0),
	}}
	for _, s := range Build(snap, Options{}) {
		if s.Protected != (s.Root.PID == 50) {
			t.Errorf("pid %d protected = %v", s.Root.PID, s.Protected)
		}
	}
}

func TestRecycledParentPidIgnored(t *testing.T) {
	snap := scan.Snapshot{Self: 9999, Procs: []scan.Proc{
		py(10, 1, venvPy, "-m uvicorn main:app", 100),
		py(11, 10, venvPy, "-m uvicorn main:app", 0),
	}}
	if got := len(Build(snap, Options{})); got != 2 {
		t.Errorf("got %d services, want 2 (pid 10 is younger than its supposed child)", got)
	}
}

func names(services []Service) []string {
	var out []string
	for _, s := range services {
		out = append(out, s.Name)
	}
	return out
}
