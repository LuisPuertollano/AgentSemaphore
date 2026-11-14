//go:build darwin

package desktop

import "os/exec"

const (
	canOpen  = true
	canFlash = false
)

func openClaude() error { return exec.Command("open", "-a", "Claude").Start() }

func flashClaude() {}
