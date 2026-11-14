//go:build !windows

package autostart

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestEnableDisableRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	if Enabled() {
		t.Fatal("enabled before enabling")
	}
	if err := Enable("/opt/agent semaphore/agentsemaphore"); err != nil {
		t.Fatal(err)
	}
	if !Enabled() {
		t.Fatal("not enabled after Enable")
	}
	p, _ := path()
	if !strings.HasPrefix(p, home) {
		t.Fatalf("file outside HOME: %s", p)
	}
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), "/opt/agent semaphore/agentsemaphore") {
		t.Fatalf("executable missing from %s:\n%s", p, data)
	}
	if err := Disable(); err != nil || Enabled() {
		t.Fatalf("disable: %v, still enabled: %v", err, Enabled())
	}
	if err := Disable(); err != nil {
		t.Fatalf("second disable must be a no-op: %v", err)
	}
}

func TestContentPerOS(t *testing.T) {
	if c := content("linux", `/x/a"b`); !strings.Contains(c, `Exec="/x/a\"b"`) || !strings.HasPrefix(c, "[Desktop Entry]") {
		t.Errorf("linux content:\n%s", c)
	}
	if c := content("darwin", "/Apps/A&B"); !strings.Contains(c, "<string>/Apps/A&amp;B</string>") || !strings.Contains(c, "<key>RunAtLoad</key><true/>") {
		t.Errorf("darwin content:\n%s", c)
	}
	_ = runtime.GOOS
}
