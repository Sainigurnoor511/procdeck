//go:build windows

package actions

import (
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"procdeck/internal/group"
	"procdeck/internal/scan"
)

func spawnTree(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("cmd", "/c", "ping -n 60 127.0.0.1 >nul")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return cmd
}

func serviceFor(t *testing.T, rootPID int32) group.Service {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		snap, err := scan.New().Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		var svc group.Service
		for _, p := range snap.Procs {
			if p.PID == rootPID {
				svc.Root = p
				svc.Procs = append([]scan.Proc{p}, svc.Procs...)
			} else if p.PPID == rootPID {
				svc.Procs = append(svc.Procs, p)
			}
		}
		if len(svc.Procs) >= 2 && svc.Root.PID == rootPID {
			svc.Name = "test"
			return svc
		}
		if time.Now().After(deadline) {
			t.Fatalf("tree for pid %d not found", rootPID)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestEndKillsWholeTree(t *testing.T) {
	cmd := spawnTree(t)
	svc := serviceFor(t, int32(cmd.Process.Pid))
	if err := End(svc); err != nil {
		t.Fatal(err)
	}
	if err := waitGone(svc); err != nil {
		t.Fatal(err)
	}
}

func TestGuardRefusesProtectedAndSelf(t *testing.T) {
	self := scan.Proc{PID: int32(os.Getpid())}
	cases := []group.Service{
		{Name: "a", Protected: true},
		{Name: "b", Root: scan.Proc{Restricted: true}},
		{Name: "c", Procs: []scan.Proc{self}},
	}
	for _, svc := range cases {
		if err := End(svc); !errors.Is(err, ErrRefused) {
			t.Errorf("%s: err = %v, want refusal", svc.Name, err)
		}
	}
}

func TestRestartRefusesBeforeKilling(t *testing.T) {
	cmd := spawnTree(t)
	svc := serviceFor(t, int32(cmd.Process.Pid))
	svc.Root.Cwd = ""
	if _, err := Restart(svc); !errors.Is(err, ErrRefused) {
		t.Fatalf("err = %v, want refusal", err)
	}
	if alive(svc.Procs[0]) == nil {
		t.Error("process was killed despite refusal")
	}
}

func TestStalePidIsNotKilled(t *testing.T) {
	cmd := spawnTree(t)
	svc := serviceFor(t, int32(cmd.Process.Pid))
	svc.Procs[0].Created = svc.Procs[0].Created.Add(-time.Hour)
	svc.Procs = svc.Procs[:1]
	if err := End(svc); err != nil {
		t.Fatal(err)
	}
	real := scan.Proc{PID: svc.Root.PID, Created: svc.Root.Created}
	if alive(real) == nil {
		t.Error("process with mismatched create time was killed")
	}
}
