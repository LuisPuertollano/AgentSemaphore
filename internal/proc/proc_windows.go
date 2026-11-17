//go:build windows

package proc

import (
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

const stillActive = 259

func name(pid int) (string, bool) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", false
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil || code != stillActive {
		return "", false
	}
	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return "", false
	}
	return filepath.Base(windows.UTF16ToString(buf[:size])), true
}

func startTicks(int) (uint64, bool) { return 0, false }

func domain() string { return "" }

func parent(pid int) (int, bool) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, false
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if int(e.ProcessID) == pid {
			return int(e.ParentProcessID), true
		}
	}
	return 0, false
}
