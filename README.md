# AgentSemaphore

A tiny traffic light in your system tray that shows what your AI agents are doing —
so you can stop watching their windows and turn off the notification noise.

| | Meaning |
|---|---|
| 🔴 | An agent is waiting for you (question, permission, approval) |
| 🟠 | An agent is working and does not need you |
| 🟢 | All quiet |

**Status: early prototype (Windows only).** Cross-platform (Linux, macOS) and more agents
(Gemini CLI, Codex, Copilot CLI…) are planned — see [docs/ROADMAP.md](docs/ROADMAP.md).

## What it watches today

- **Claude Code** — CLI and the Claude Desktop "Code" tab (red / orange / green).
- **Claude chat, Cowork and scheduled tasks** in Claude Desktop — red when they need you.

## Try the Windows prototype

Install with [install/windows](install/windows/README.md) (per user, opt-in silencing of
Claude's pop-ups, clean uninstall). Details of the prototype: [prototype/windows/README.md](prototype/windows/README.md).

## License

MIT — free forever, no telemetry, no accounts.
