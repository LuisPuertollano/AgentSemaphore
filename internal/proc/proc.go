// Package proc answers one question about a process id: is it running, and
// what is its executable name? Used to ignore session files left behind by
// agents that have exited, and files whose pid was reused by another program.
package proc

import "strings"

// Name returns the executable base name of a running process (lower case,
// without ".exe") and true, or "" and false if no such process is running.
func Name(pid int) (string, bool) {
	if pid <= 0 {
		return "", false
	}
	n, ok := name(pid)
	if !ok {
		return "", false
	}
	n = strings.ToLower(n)
	n = strings.TrimSuffix(n, ".exe")
	return n, true
}

// StartTicks returns when a process started, in the unit Claude Code writes as
// "procStart" in its session files on Linux: clock ticks since boot, field 22
// of /proc/<pid>/stat (checked 2026-11-17, Claude Code 2.1.326). Elsewhere it
// returns false: the format there is not known.
func StartTicks(pid int) (uint64, bool) {
	if pid <= 0 {
		return 0, false
	}
	return startTicks(pid)
}

// Domain identifies the PID space this process sees, in the format of
// Claude Code's "pidDomain" on Linux: "linux:<machine-id>:pid:[<namespace>]".
// A pid from another machine or PID namespace cannot be checked here. Empty
// where unknown.
func Domain() string { return domain() }
