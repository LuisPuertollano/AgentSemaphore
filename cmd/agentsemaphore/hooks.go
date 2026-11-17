package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/LuisPuertollano/AgentSemaphore/internal/source/hooks"
)

// runHook is what an agent's hook runs: "agentsemaphore hook <agent> <event>",
// with the event's JSON on stdin. It must never get in the agent's way: it
// prints nothing (Gemini CLI wants "{}", its documented no-op), always exits 0,
// and only logs problems to hooks.log next to the settings.
func runHook(args []string) (code int) {
	defer func() {
		if r := recover(); r != nil {
			hookLog("panic: %v", r)
		}
		code = 0
	}()
	if len(args) >= 1 && args[0] == "gemini" {
		defer fmt.Println("{}")
	}
	if len(args) != 2 {
		hookLog("usage: agentsemaphore hook <agent> <event>, got %q", args)
		return 0
	}
	dir, err := hooks.Dir()
	if err == nil {
		err = hooks.Handle(dir, args[0], args[1], os.Stdin, os.Getppid(), time.Now())
	}
	if err != nil {
		hookLog("%s %s: %v", args[0], args[1], err)
	}
	return 0
}

func hookLog(format string, a ...any) {
	dir, err := hooks.Dir()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(filepath.Dir(dir), "hooks.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	log.New(f, "", log.LstdFlags).Printf(format, a...)
}

// runHooks manages our entries in other agents' hook systems.
func runHooks(args []string) int {
	usage := func() int {
		fmt.Fprintln(os.Stderr, "usage: agentsemaphore hooks install|uninstall|status|show copilot|codex|gemini")
		return 2
	}
	if len(args) != 2 {
		return usage()
	}
	cmd, agent := args[0], args[1]
	if _, ok := hooks.Agents[agent]; !ok {
		return usage()
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	exe = hookExe(exe)
	switch cmd {
	case "show":
		s, err := hooks.Snippet(agent, exe)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(s)
	case "install":
		path, err := hooks.Install(agent, exe, home)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("Hooks installed:", path)
		if agent == "codex" {
			fmt.Println("Codex runs a new hook only after you trust it: open Codex, type /hooks and trust the AgentSemaphore entries.")
		}
		fmt.Println("They call", exe, "- if you move or reinstall AgentSemaphore elsewhere, run this again.")
	case "uninstall":
		path, removed, err := hooks.Uninstall(agent, home)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if removed {
			fmt.Println("Hooks removed from", path)
		} else {
			fmt.Println("No AgentSemaphore hooks found in", path)
		}
	case "status":
		if hooks.Installed(agent, home) {
			fmt.Println("installed")
		} else {
			fmt.Println("not installed")
		}
	default:
		return usage()
	}
	return 0
}

// hookExe picks the program the agents' hooks should run. On Windows the
// installed agentsemaphore.exe is a GUI program, which PowerShell does not wait
// for and whose hook input got lost in CI (see docs/FINDINGS.md), so hooks use
// its console twin agentsemaphore-cli.exe when it sits next to it.
func hookExe(exe string) string {
	if runtime.GOOS != "windows" {
		return exe
	}
	cli := filepath.Join(filepath.Dir(exe), "agentsemaphore-cli.exe")
	if _, err := os.Stat(cli); err == nil {
		return cli
	}
	return exe
}
