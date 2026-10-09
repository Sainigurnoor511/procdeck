// Package actions ends, restarts and opens services.
package actions

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/atotto/clipboard"
	"github.com/shirou/gopsutil/v4/process"

	"procdeck/internal/group"
	"procdeck/internal/scan"
)

const exitTimeout = 5 * time.Second

var ErrRefused = errors.New("refused")

func guard(svc group.Service) error {
	switch {
	case svc.Protected:
		return fmt.Errorf("%w: %s is running procdeck itself", ErrRefused, svc.Name)
	case svc.Restricted():
		return fmt.Errorf("%w: no access to %s", ErrRefused, svc.Name)
	}
	for _, p := range svc.Procs {
		if p.PID == int32(os.Getpid()) {
			return fmt.Errorf("%w: %s is running procdeck itself", ErrRefused, svc.Name)
		}
	}
	return nil
}

func alive(p scan.Proc) *process.Process {
	proc, err := process.NewProcess(p.PID)
	if err != nil {
		return nil
	}
	// a different create time means the pid now belongs to another process
	if created, err := proc.CreateTime(); err != nil || created != p.Created.UnixMilli() {
		return nil
	}
	return proc
}

// End kills the whole tree, root first so supervisors cannot respawn children.
func End(svc group.Service) error {
	if err := guard(svc); err != nil {
		return err
	}
	var failed []int32
	for _, p := range svc.Procs {
		proc := alive(p)
		if proc == nil {
			continue
		}
		if err := proc.Kill(); err != nil && alive(p) != nil {
			failed = append(failed, p.PID)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("could not end pid %v", failed)
	}
	return nil
}

func EndDuplicates(services []group.Service) (int, error) {
	ended := 0
	var errs []error
	for _, svc := range services {
		if !svc.Duplicate {
			continue
		}
		if err := End(svc); err != nil {
			errs = append(errs, err)
			continue
		}
		ended++
	}
	return ended, errors.Join(errs...)
}

func waitGone(svc group.Service) error {
	deadline := time.Now().Add(exitTimeout)
	for {
		remaining := 0
		for _, p := range svc.Procs {
			if alive(p) != nil {
				remaining++
			}
		}
		if remaining == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%d processes still running after %s", remaining, exitTimeout)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Restart reports usedOwnEnv when the original environment was unreadable.
func Restart(svc group.Service) (usedOwnEnv bool, err error) {
	if err := guard(svc); err != nil {
		return false, err
	}
	root := svc.Root
	if root.Cmdline == "" || root.Cwd == "" || root.ExePath == "" {
		return false, fmt.Errorf("%w: command or folder of %s is unreadable", ErrRefused, svc.Name)
	}
	var env []string
	if proc := alive(root); proc != nil {
		env, _ = proc.Environ()
	}
	if len(env) == 0 {
		env = os.Environ()
		usedOwnEnv = true
	}

	if err := End(svc); err != nil {
		return usedOwnEnv, err
	}
	if err := waitGone(svc); err != nil {
		return usedOwnEnv, err
	}
	title := fmt.Sprintf("%s · %s", svc.Name, svc.Project)
	if err := startInNewConsole(root.ExePath, root.Cmdline, root.Cwd, env, title); err != nil {
		_ = clipboard.WriteAll(root.Cmdline)
		return usedOwnEnv, fmt.Errorf("ended but could not start again (command copied): %w", err)
	}
	return usedOwnEnv, nil
}

func Open(link string) error {
	if link == "" {
		return errors.New("no link for this service")
	}
	return openURL(link)
}

func Copy(text string) error {
	return clipboard.WriteAll(text)
}
