//go:build windows

package desktop

import (
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	canOpen  = true
	canFlash = true
	// appID of the Claude Desktop Store app (FINDINGS §3).
	claudeAppID = `Claude_pzs8sxrjxfjjc!Claude`
)

var (
	user32            = windows.NewLazySystemDLL("user32.dll")
	procFlashWindowEx = user32.NewProc("FlashWindowEx")
	procGetWindow     = user32.NewProc("GetWindow")
)

// explorer.exe resolves the app ID and activates the running instance.
func openClaude() error {
	return exec.Command("explorer.exe", `shell:AppsFolder\`+claudeAppID).Start()
}

type flashInfo struct {
	cbSize    uint32
	hwnd      windows.HWND
	dwFlags   uint32
	uCount    uint32
	dwTimeout uint32
}

const (
	flashwTray       = 0x2
	flashwTimerNoFG  = 0xC
	gwOwner          = 4
	processQueryInfo = windows.PROCESS_QUERY_LIMITED_INFORMATION
)

func flashClaude() {
	h := findClaudeWindow()
	if h == 0 || h == windows.GetForegroundWindow() {
		return
	}
	fi := flashInfo{hwnd: h, dwFlags: flashwTray | flashwTimerNoFG}
	fi.cbSize = uint32(unsafe.Sizeof(fi))
	procFlashWindowEx.Call(uintptr(unsafe.Pointer(&fi)))
}

// findClaudeWindow returns the first visible, unowned top-level window of the
// Claude Desktop Store app (its executable is under \WindowsApps\Claude_...).
func findClaudeWindow() windows.HWND {
	var found windows.HWND
	cb := syscall.NewCallback(func(h windows.HWND, _ uintptr) uintptr {
		if !windows.IsWindowVisible(h) {
			return 1
		}
		if owner, _, _ := procGetWindow.Call(uintptr(h), gwOwner); owner != 0 {
			return 1
		}
		if isClaudeDesktop(h) {
			found = h
			return 0 // stop
		}
		return 1
	})
	_ = windows.EnumWindows(cb, nil)
	return found
}

func isClaudeDesktop(h windows.HWND) bool {
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(h, &pid); err != nil || pid == 0 {
		return false
	}
	ph, err := windows.OpenProcess(processQueryInfo, false, pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(ph)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(ph, 0, &buf[0], &size); err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(windows.UTF16ToString(buf[:size])), `\windowsapps\claude_`)
}
