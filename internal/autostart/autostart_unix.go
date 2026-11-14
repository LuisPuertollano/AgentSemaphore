//go:build !windows

package autostart

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// path returns the autostart file for this OS.
func path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "LaunchAgents", "io.github.luispuertollano.agentsemaphore.plist"), nil
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "autostart", "agentsemaphore.desktop"), nil
}

// content returns the file body for exe on this OS.
func content(goos, exe string) string {
	if goos == "darwin" {
		return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>io.github.luispuertollano.agentsemaphore</string>
  <key>ProgramArguments</key><array><string>` + xmlEscape(exe) + `</string></array>
  <key>RunAtLoad</key><true/>
</dict>
</plist>
`
	}
	return "[Desktop Entry]\nType=Application\nName=" + Name + "\nComment=Traffic light for your AI agents\n" +
		"Exec=\"" + strings.ReplaceAll(exe, `"`, `\"`) + "\"\nX-GNOME-Autostart-enabled=true\n"
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

func enabled() bool {
	p, err := path()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

func enable(exe string) error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(content(runtime.GOOS, exe)), 0o644)
}

func disable() error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
