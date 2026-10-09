//go:build windows

package scan

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// One toolhelp snapshot lists every process without opening a handle per pid.
func listBasics() (map[int32]basic, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	out := map[int32]basic{}
	for err = windows.Process32First(snap, &entry); err == nil; err = windows.Process32Next(snap, &entry) {
		out[int32(entry.ProcessID)] = basic{
			ppid: int32(entry.ParentProcessID),
			name: windows.UTF16ToString(entry.ExeFile[:]),
		}
	}
	return out, nil
}
