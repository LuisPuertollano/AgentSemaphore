//go:build windows

package toasts

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

type windowsPlatform struct {
	dir     string
	tmp     string
	walMark time.Time
}

// NewPlatform returns the real Windows implementation.
func NewPlatform() Platform {
	return &windowsPlatform{
		dir: filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Windows", "Notifications"),
		tmp: filepath.Join(os.TempDir(), "agentsemaphore-wpn"),
	}
}

func (p *windowsPlatform) Available() bool {
	_, err := os.Stat(filepath.Join(p.dir, "wpndatabase.db"))
	return err == nil
}

func (p *windowsPlatform) Changed() bool {
	fi, err := os.Stat(filepath.Join(p.dir, "wpndatabase.db-wal"))
	if err != nil {
		return true // no WAL: cannot tell, read again
	}
	if fi.ModTime().Equal(p.walMark) {
		return false
	}
	p.walMark = fi.ModTime()
	return true
}

func (p *windowsPlatform) Read(since time.Time) ([]Stored, error) {
	return ReadDB(p.dir, p.tmp, since)
}

// ClaudeInForeground: the foreground window belongs to the Claude Desktop
// Store app (its executable lives under \WindowsApps\Claude_...).
func (p *windowsPlatform) ClaudeInForeground() bool {
	hwnd := windows.GetForegroundWindow()
	if hwnd == 0 {
		return false
	}
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &pid); err != nil || pid == 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(windows.UTF16ToString(buf[:size])), `\windowsapps\claude_`)
}
