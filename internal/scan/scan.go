// Package scan takes snapshots of running processes and their listening TCP ports.
package scan

import (
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

type Proc struct {
	PID        int32
	PPID       int32
	Exe        string // lower-case base name without .exe
	ExePath    string
	Cmdline    string
	Cwd        string
	Created    time.Time
	CPU        float64 // percent of total machine capacity
	Mem        uint64
	Ports      []int
	Restricted bool
}

type Snapshot struct {
	Procs []Proc
	Self  int32
	At    time.Time
}

type basic struct {
	ppid int32
	name string
}

type cached struct {
	created int64
	proc    Proc
}

type Scanner struct {
	mu      sync.Mutex
	cache   map[int32]*cached
	prevCPU map[int32]float64
	prevAt  time.Time
	ncpu    float64
}

func New() *Scanner {
	return &Scanner{
		cache:   map[int32]*cached{},
		prevCPU: map[int32]float64{},
		ncpu:    float64(runtime.NumCPU()),
	}
}

func (s *Scanner) Snapshot() (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	basics, err := listBasics()
	if err != nil {
		return Snapshot{}, err
	}
	ports := listeningPorts()
	now := time.Now()
	elapsed := now.Sub(s.prevAt).Seconds()

	snap := Snapshot{Self: int32(os.Getpid()), At: now}
	cpuNow := make(map[int32]float64, len(basics))
	alive := make(map[int32]bool, len(basics))

	for pid, b := range basics {
		if pid == 0 || pid == 4 {
			continue
		}
		p := &process.Process{Pid: pid}
		created, _ := p.CreateTime()

		c := s.cache[pid]
		if c == nil || c.created != created {
			c = &cached{created: created, proc: readStatic(p, created, b)}
			s.cache[pid] = c
		}
		alive[pid] = true

		proc := c.proc
		proc.Ports = ports[pid]
		if mem, err := p.MemoryInfo(); err == nil && mem != nil {
			proc.Mem = mem.RSS
		}
		if t, err := p.Times(); err == nil && t != nil {
			total := t.User + t.System
			cpuNow[pid] = total
			if prev, ok := s.prevCPU[pid]; ok && elapsed > 0 && total >= prev {
				proc.CPU = (total - prev) / elapsed * 100 / s.ncpu
			}
		}
		snap.Procs = append(snap.Procs, proc)
	}

	for pid := range s.cache {
		if !alive[pid] {
			delete(s.cache, pid)
		}
	}
	s.prevCPU = cpuNow
	s.prevAt = now
	sort.Slice(snap.Procs, func(i, j int) bool { return snap.Procs[i].PID < snap.Procs[j].PID })
	return snap, nil
}

func readStatic(p *process.Process, created int64, b basic) Proc {
	proc := Proc{
		PID:     p.Pid,
		PPID:    b.ppid,
		Exe:     strings.TrimSuffix(strings.ToLower(b.name), ".exe"),
		Created: time.UnixMilli(created),
	}
	if exe, err := p.Exe(); err == nil {
		proc.ExePath = exe
	}
	cmdline, err := p.Cmdline()
	if err != nil || (cmdline == "" && proc.ExePath == "") {
		proc.Restricted = true
		return proc
	}
	proc.Cmdline = cmdline
	if cwd, err := p.Cwd(); err == nil {
		proc.Cwd = strings.TrimRight(cwd, `\/`)
	}
	return proc
}

func listeningPorts() map[int32][]int {
	out := map[int32][]int{}
	conns, err := gnet.Connections("tcp")
	if err != nil {
		return out
	}
	seen := map[int32]map[int]bool{}
	for _, c := range conns {
		if c.Status != "LISTEN" || c.Pid == 0 || c.Laddr.Port == 0 {
			continue
		}
		port := int(c.Laddr.Port)
		if seen[c.Pid] == nil {
			seen[c.Pid] = map[int]bool{}
		}
		if seen[c.Pid][port] {
			continue
		}
		seen[c.Pid][port] = true
		out[c.Pid] = append(out[c.Pid], port)
	}
	for pid := range out {
		sort.Ints(out[pid])
	}
	return out
}
