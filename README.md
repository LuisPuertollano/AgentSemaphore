# AgentSemaphore

> ⚠️ **Pre-alpha — testing stage.** This is an early, moving project: things will break, formats will change,
> and it has only been tried on a handful of machines. Please do not rely on it yet.
> **If you try it, you are warmly invited to help:** report what you see on your system, share findings about
> agents' state files or notifications, or send a pull request. See [CONTRIBUTING.md](CONTRIBUTING.md).

A tiny traffic light in your system tray that shows what your AI agents are doing —
so you can stop watching their windows and turn off the notification noise.

| | Meaning |
|---|---|
| 🔴 | An agent is waiting for you (question, permission, approval) |
| 🟠 | An agent is working and does not need you |
| 🟢 | All quiet |

**Status: pre-alpha. Windows prototype works; the cross-platform core is in progress.** Cross-platform (Linux, macOS) and more agents
(Gemini CLI, Codex, Copilot CLI…) are planned — see [docs/ROADMAP.md](docs/ROADMAP.md).

## What it watches today

- **Claude Code** — CLI and the Claude Desktop "Code" tab (red / orange / green).
- **Claude chat, Cowork and scheduled tasks** in Claude Desktop — red when they need you.

## Try the Windows prototype

Install with [install/windows](install/windows/README.md) (per user, opt-in silencing of
Claude's pop-ups, clean uninstall). Details of the prototype: [prototype/windows/README.md](prototype/windows/README.md).

## Cross-platform core (in progress)

A single Go binary for Windows (x64, ARM64), Linux and macOS, no runtime needed. It already reads
Claude Code sessions; Claude Desktop notifications follow.

```sh
go run ./cmd/agentsemaphore          # tray app
go run ./cmd/agentsemaphore -once    # print the current state as JSON and exit
```

CI builds the binaries on every push (artifacts on the workflow run).

## License

MIT — free forever, no telemetry, no accounts.
