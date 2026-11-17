# AgentSemaphore — instructions for Claude Code

A traffic-light indicator in the system tray / menu bar that tells you, at a glance,
what your AI agents are doing — so you can stop watching their windows and stop
getting spammed by notifications.

- 🔴 **Red** — an agent is blocked waiting for you (question, permission, approval)
- 🟠 **Orange** — an agent is working and does not need you
- 🟢 **Green** — everything idle / nothing pending

## Owner and working style

- Owner: Luis (GitHub `LuisPuertollano`). He writes in Spanish; reply in Spanish.
  Code, comments, commit messages, README and docs for the public project: **English**.
- Never assume — verify on the real system, or ask. Say clearly what was tested and what was not.
- Do not start long-running or invasive operations on his machines without an explicit go-ahead.
- The project must stay **100 % free** (MIT, no paid services, no telemetry, no accounts).
  It will be published on GitHub with README/landing page, screenshots and install instructions.

## Git workflow

- Always work on a branch and open a pull request; never push to `main` directly.
- **Auto-merge (approved by Luis, 2026-11-14):** when every CI check on a PR you opened is green
  (the repo is public since 2026-11-14, so `windows-11-arm` must run and pass too) and it has no merge conflict,
  mark it ready and merge it without asking. Use a merge commit so per-step commits stay visible.
  Never merge with a red, pending or cancelled check, and never skip or disable a test to get green.
- After merging, report what was merged, which issues closed and what still needs testing on a real machine.

## Where things run

- Development: Claude Code in the cloud or on a home-lab VM (`claude-box`).
  Progress lives in this repo, never only on a laptop.
- Real Windows testing happens on Luis's laptop (Windows 11 **ARM64**, Claude Desktop from the
  Microsoft Store, PowerShell 7 Store build, Python 3.12 arm64). Treat ARM64 as first-class.
- macOS testing: GitHub Actions `macos-latest` runners (no Mac available).

## Current state

`prototype/windows/` is a **working** Windows prototype (PowerShell + a small Python helper),
installed and verified on Luis's laptop on 2026-11-14. The cross-platform Go core (`cmd/`, `internal/`)
reads Claude Code sessions and Claude Desktop notifications; its tray ran on the Windows ARM64 laptop on
2026-11-14 (dot, tooltip, menu and Quit fine; real notification DB read). Linux (x64, ARM64) and macOS are tested end-to-end in CI only;
Luis has no Linux or macOS machine, so never ask him to test there. Read `docs/FINDINGS.md` before changing
any detection logic — it records what was measured and what is known to be broken upstream.

## Roadmap

See `docs/ROADMAP.md`. Short version: (1) harden Windows, (2) cross-platform core for
Linux/macOS, (3) more agents (Gemini CLI, Codex, Copilot CLI…), (4) subscription usage view,
(5) public release with landing page.

## Rules for changes

- Keep detection sources pluggable: one module per source (Claude Code session files,
  Claude Desktop notifications, hooks of other CLIs…). The tray/menu UI only aggregates.
- Worst state wins across sessions: red > grey > orange > green (grey = a source cannot be read).
- Read-only towards the agents: never modify an agent's own config without the user asking.
  If hooks are needed, install them additively and make uninstall remove only ours.
- Undocumented data sources (session files, notification DB) must fail soft: if the format
  changes, show a clear "unknown" state, never crash and never show a wrong green.
