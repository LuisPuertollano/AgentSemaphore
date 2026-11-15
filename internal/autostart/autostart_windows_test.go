//go:build windows

package autostart

import (
	"testing"

	"golang.org/x/sys/windows/registry"
)

// Runs on the CI's Windows runners: writes and removes a real HKCU Run value.
func TestRegistryRoundTrip(t *testing.T) {
	_ = Disable()
	t.Cleanup(func() { _ = Disable() })
	if Enabled() {
		t.Fatal("enabled before enabling")
	}
	exe := `C:\Program Files\Agent Semaphore\agentsemaphore.exe`
	if err := Enable(exe); err != nil {
		t.Fatal(err)
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	v, _, err := k.GetStringValue(Name)
	k.Close()
	if err != nil || v != `"`+exe+`"` {
		t.Fatalf("Run value = %q, %v; want the quoted path", v, err)
	}
	if !Enabled() {
		t.Fatal("not enabled after Enable")
	}
	if err := Disable(); err != nil || Enabled() {
		t.Fatalf("disable: %v, still enabled: %v", err, Enabled())
	}
	if err := Disable(); err != nil {
		t.Fatalf("second disable must be a no-op: %v", err)
	}
}
