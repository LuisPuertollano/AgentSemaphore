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
