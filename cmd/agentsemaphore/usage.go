package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/LuisPuertollano/AgentSemaphore/internal/usage"
)

// runStatusline is what Claude Code runs as its status line once
// "usage install claude" was done. It keeps only the rate-limit numbers from
// the input, then shows the user's own status line (run with the same input)
// or, if there was none, a short "5h 23% · 7d 41%". It always exits 0, and
// on any problem it still prints something rather than break the line.
func runStatusline() (code int) {
	defer func() {
		if r := recover(); r != nil {
			hookLog("statusline panic: %v", r)
		}
		code = 0
	}()
	data, _ := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	now := time.Now()
	dir, err := usage.Dir()
	if err != nil {
		hookLog("statusline: %v", err)
		return 0
	}
	c, ok := usage.Parse(data, now)
	if ok {
		if err := usage.Save(dir, c); err != nil {
			hookLog("statusline: %v", err)
		}
	}
	if prev := usage.PreviousCommand(dir); prev != "" {
		if out, err := runShell(prev, data); err == nil {
			os.Stdout.Write(out)
			return 0
		} else {
			hookLog("statusline: your own status line command failed: %v", err)
		}
	}
	if ok {
		fmt.Println(usage.StatusText(c, now))
	}
	return 0
}

// runShell runs the user's previous status line command with the same input,
// the way a shell would (sh on Linux/macOS; bash if present, else cmd, on Windows).
func runShell(cmdline string, stdin []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var c *exec.Cmd
	switch {
	case runtime.GOOS != "windows":
		c = exec.CommandContext(ctx, "sh", "-c", cmdline)
	default:
		if bash, err := exec.LookPath("bash"); err == nil {
			c = exec.CommandContext(ctx, bash, "-c", cmdline)
		} else {
			c = exec.CommandContext(ctx, "cmd", "/C", cmdline)
		}
	}
	c.Stdin = strings.NewReader(string(stdin))
	c.Stderr = io.Discard
	return c.Output()
}

// runUsage: agentsemaphore usage install|uninstall|status claude
func runUsage(args []string) int {
	if len(args) != 2 || args[1] != "claude" {
		fmt.Fprintln(os.Stderr, "usage: agentsemaphore usage install|uninstall|status claude")
		return 2
	}
	home, err := os.UserHomeDir()
	if err == nil {
		_, err = usage.Dir()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dir, _ := usage.Dir()
	switch args[0] {
	case "install":
		exe, err := os.Executable()
		if err == nil {
			exe, err = filepath.EvalSymlinks(exe)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		path, err := usage.InstallClaude(dir, hookExe(exe), home)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("Claude Code's status line now goes through AgentSemaphore:", path)
		fmt.Println("Your own status line, if you had one, is still shown. Undo: agentsemaphore usage uninstall claude")
		fmt.Println("Plan usage appears in the menu after the next Claude Code reply (Pro, Max and Ultra plans).")
	case "uninstall":
		path, what, err := usage.UninstallClaude(dir, home)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("%s: %s\n", path, what)
	case "status":
		if usage.ClaudeInstalled(home) {
			fmt.Println("installed")
		} else {
			fmt.Println("not installed")
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: agentsemaphore usage install|uninstall|status claude")
		return 2
	}
	return 0
}
