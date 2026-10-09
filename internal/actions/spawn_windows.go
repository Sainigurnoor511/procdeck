//go:build windows

package actions

import (
	"os/exec"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

func startInNewConsole(exePath, cmdline, cwd string, env []string, title string) error {
	app, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return err
	}
	cmd, err := windows.UTF16PtrFromString(cmdline)
	if err != nil {
		return err
	}
	dir, err := windows.UTF16PtrFromString(cwd)
	if err != nil {
		return err
	}
	titlePtr, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return err
	}

	var block []uint16
	for _, kv := range env {
		block = append(block, utf16.Encode([]rune(kv))...)
		block = append(block, 0)
	}
	block = append(block, 0)

	si := windows.StartupInfo{Title: titlePtr}
	si.Cb = uint32(unsafe.Sizeof(si))
	var pi windows.ProcessInformation
	flags := uint32(windows.CREATE_NEW_CONSOLE | windows.CREATE_UNICODE_ENVIRONMENT)
	if err := windows.CreateProcess(app, cmd, nil, nil, false, flags, &block[0], dir, &si, &pi); err != nil {
		return err
	}
	windows.CloseHandle(pi.Thread)
	windows.CloseHandle(pi.Process)
	return nil
}

func openURL(link string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", link).Start()
}
