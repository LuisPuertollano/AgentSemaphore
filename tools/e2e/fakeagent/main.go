// Command fakeagent stands in for an agent process in end-to-end tests. Built
// as "claude", "copilot", "codex"…, it waits, so a source sees a live process
// with that name. With arguments it first runs them as a child, with its own
// stdin, as an agent runs a hook command, then waits.
//
// (A renamed copy of `sleep` does not work everywhere: newer Linux images ship
// coreutils as one multi-call binary that dispatches on its own name.)
package main

import (
	"os"
	"os/exec"
	"time"
)

func main() {
	if len(os.Args) > 1 {
		c := exec.Command(os.Args[1], os.Args[2:]...)
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
		_ = c.Run()
	}
	time.Sleep(10 * time.Minute)
}
