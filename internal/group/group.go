// Package group folds process trees into services and flags duplicates.
package group

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"procdeck/internal/classify"
	"procdeck/internal/scan"
)

type Service struct {
	Name      string
	Project   string
	Root      scan.Proc
	Procs     []scan.Proc // root first, then descendants by depth
	Ports     []int
	Link      string
	CPU       float64
	Mem       uint64
	Variant   string
	Duplicate bool
	// Protected services contain procdeck itself or one of its ancestors.
	Protected bool

	classified bool
}

func (s Service) Restricted() bool { return s.Root.Restricted }

type Options struct {
	// All also lists editor tooling and unclassified processes.
	All bool
}

func Build(snap scan.Snapshot, opts Options) []Service {
	byPID := make(map[int32]*scan.Proc, len(snap.Procs))
	for i := range snap.Procs {
		byPID[snap.Procs[i].PID] = &snap.Procs[i]
	}
	parentOf := func(p *scan.Proc) *scan.Proc {
		parent := byPID[p.PPID]
		// a parent younger than its child is a recycled pid, not a real parent
		if parent == nil || parent.PID == p.PID || parent.Created.After(p.Created) {
			return nil
		}
		return parent
	}

	matches := map[int32]classify.Match{}
	for i := range snap.Procs {
		p := &snap.Procs[i]
		if !opts.All && classify.IsTooling(p.ExePath, p.Cmdline) {
			continue
		}
		if m, ok := classify.Classify(p.Exe, p.Cmdline); ok {
			matches[p.PID] = m
		}
	}

	roots := map[int32]bool{}
	for i := range snap.Procs {
		p := &snap.Procs[i]
		m, ok := matches[p.PID]
		if !ok {
			continue
		}
		folded := false
		for a, hops := parentOf(p), 0; a != nil && hops < 64; a, hops = parentOf(a), hops+1 {
			am, classified := matches[a.PID]
			if !classified {
				continue
			}
			folded = am.Name == m.Name || am.Wrapper || m.Generic
			break
		}
		if !folded {
			roots[p.PID] = true
		}
	}

	type member struct {
		proc  scan.Proc
		depth int
	}
	members := map[int32][]member{}
	owned := map[int32]bool{}
	for i := range snap.Procs {
		p := &snap.Procs[i]
		depth := 0
		for a := p; a != nil && depth < 64; a = parentOf(a) {
			if roots[a.PID] {
				members[a.PID] = append(members[a.PID], member{*p, depth})
				owned[p.PID] = true
				break
			}
			depth++
		}
	}

	protected := map[int32]bool{}
	for p, hops := byPID[snap.Self], 0; p != nil && hops < 64; p, hops = parentOf(p), hops+1 {
		protected[p.PID] = true
	}

	var services []Service
	for rootPID, ms := range members {
		sort.SliceStable(ms, func(i, j int) bool {
			if ms[i].depth != ms[j].depth {
				return ms[i].depth < ms[j].depth
			}
			return ms[i].proc.PID < ms[j].proc.PID
		})
		root := *byPID[rootPID]
		svc := Service{Name: matches[rootPID].Name, Root: root, classified: true}
		renamed := !matches[rootPID].Wrapper
		for _, m := range ms {
			svc.Procs = append(svc.Procs, m.proc)
			// a wrapper such as "npm: dev" takes the name of the real tool it started
			if !renamed {
				if cm, ok := matches[m.proc.PID]; ok && !cm.Wrapper && !cm.Generic {
					svc.Name = cm.Name
					renamed = true
				}
			}
		}
		finish(&svc, protected)
		services = append(services, svc)
	}

	for i := range snap.Procs {
		p := snap.Procs[i]
		if owned[p.PID] {
			continue
		}
		if !opts.All && !isUserListener(p) {
			continue
		}
		svc := Service{Name: p.Exe, Root: p, Procs: []scan.Proc{p}}
		finish(&svc, protected)
		services = append(services, svc)
	}

	markDuplicates(services)
	sort.SliceStable(services, func(i, j int) bool {
		a, b := services[i], services[j]
		if a.Project != b.Project {
			return strings.ToLower(a.Project) < strings.ToLower(b.Project)
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if !a.Root.Created.Equal(b.Root.Created) {
			return a.Root.Created.Before(b.Root.Created)
		}
		return a.Root.PID < b.Root.PID
	})
	return services
}

var installDirs = []string{`\windows\`, `\program files`, `\programdata\`, `\appdata\`}

func isUserListener(p scan.Proc) bool {
	if p.Restricted || len(p.Ports) == 0 {
		return false
	}
	path := strings.ToLower(p.ExePath)
	for _, dir := range installDirs {
		if strings.Contains(path, dir) {
			return false
		}
	}
	return !classify.IsTooling(p.ExePath, p.Cmdline)
}

func finish(svc *Service, protected map[int32]bool) {
	seen := map[int]bool{}
	for _, p := range svc.Procs {
		svc.CPU += p.CPU
		svc.Mem += p.Mem
		if protected[p.PID] {
			svc.Protected = true
		}
		for _, port := range p.Ports {
			if !seen[port] {
				seen[port] = true
				svc.Ports = append(svc.Ports, port)
			}
		}
	}
	sort.Ints(svc.Ports)
	if len(svc.Ports) > 0 {
		svc.Link = fmt.Sprintf("http://localhost:%d", svc.Ports[0])
	}
	svc.Project = "?"
	if svc.Root.Cwd != "" {
		svc.Project = filepath.Base(svc.Root.Cwd)
	}
	svc.Variant = variant(svc.Root)
}

func variant(p scan.Proc) string {
	if !strings.HasPrefix(p.Exe, "py") {
		return ""
	}
	path := strings.ToLower(strings.ReplaceAll(p.ExePath, "/", `\`))
	for _, dir := range []string{`\.venv\`, `\venv\`, `\env\`} {
		if strings.Contains(path, dir) {
			return "venv"
		}
	}
	return "global"
}

func markDuplicates(services []Service) {
	sets := map[string][]int{}
	for i, s := range services {
		if !s.classified || s.Project == "?" {
			continue
		}
		key := s.Name + "\x00" + strings.ToLower(s.Root.Cwd)
		sets[key] = append(sets[key], i)
	}
	for _, idxs := range sets {
		if len(idxs) < 2 {
			continue
		}
		keeper := idxs[0]
		for _, i := range idxs[1:] {
			if better(services[i], services[keeper]) {
				keeper = i
			}
		}
		for _, i := range idxs {
			services[i].Duplicate = i != keeper
		}
	}
}

func better(a, b Service) bool {
	if (a.Variant == "venv") != (b.Variant == "venv") {
		return a.Variant == "venv"
	}
	return a.Root.Created.Before(b.Root.Created)
}
