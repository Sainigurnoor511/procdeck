//go:build !windows

package scan

import "github.com/shirou/gopsutil/v4/process"

func listBasics() (map[int32]basic, error) {
	pids, err := process.Pids()
	if err != nil {
		return nil, err
	}
	out := map[int32]basic{}
	for _, pid := range pids {
		p := &process.Process{Pid: pid}
		ppid, _ := p.Ppid()
		name, _ := p.Name()
		out[pid] = basic{ppid: ppid, name: name}
	}
	return out, nil
}
