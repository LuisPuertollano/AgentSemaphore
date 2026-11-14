// Package autostart starts AgentSemaphore at sign-in, per user, without admin
// rights:
//
//	Windows  HKCU\Software\Microsoft\Windows\CurrentVersion\Run, value "AgentSemaphore"
//	Linux    $XDG_CONFIG_HOME/autostart/agentsemaphore.desktop (~/.config/autostart)
//	macOS    ~/Library/LaunchAgents/io.github.luispuertollano.agentsemaphore.plist
//
// Disable removes only that entry.
package autostart

import "os"

// Name of the entry in every OS.
const Name = "AgentSemaphore"

// Enabled reports whether the autostart entry exists.
func Enabled() bool { return enabled() }

// Enable creates the entry for the given executable (default: this one).
func Enable(exe string) error {
	if exe == "" {
		var err error
		if exe, err = os.Executable(); err != nil {
			return err
		}
	}
	return enable(exe)
}

// Disable removes the entry; it is not an error if there is none.
func Disable() error { return disable() }
