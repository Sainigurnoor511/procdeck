//go:build !windows

package actions

import (
	"os/exec"
	"runtime"

	"procdeck/internal/classify"
)

func startInNewConsole(exePath, cmdline, cwd string, env []string, title string) error {
	args := classify.Split(cmdline)
	cmd := exec.Command(exePath, args[1:]...)
	cmd.Dir = cwd
	cmd.Env = env
	return cmd.Start()
}

func openURL(link string) error {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	return exec.Command(opener, link).Start()
}
