# Roadmap

Live tracking is on GitHub: one issue per phase with sub-issues —
[Phase 1](https://github.com/LuisPuertollano/AgentSemaphore/issues/2) ·
[Phase 2](https://github.com/LuisPuertollano/AgentSemaphore/issues/3) ·
[Phase 3](https://github.com/LuisPuertollano/AgentSemaphore/issues/4) ·
[Phase 4](https://github.com/LuisPuertollano/AgentSemaphore/issues/5) ·
[Phase 5](https://github.com/LuisPuertollano/AgentSemaphore/issues/6) ·
[Ideas and future intentions](https://github.com/LuisPuertollano/AgentSemaphore/issues/7).
This file keeps the overview; the issues hold the details and progress.

## Decisions (2026-11-14)
- Core stack: **Go + fyne.io/systray** (single binary, no runtime; Windows x64/arm64 without cgo;
  Linux via DBus StatusNotifierItem; macOS built on `macos-latest`).
- State priority: **red > grey > orange > green**. Grey = a source could not be read; it must
  never be masked by orange or shown as green.
- macOS: no paid notarization; unsigned build with a documented manual step.
- Order for Phase 1: settings, i18n and installer first, then the verifications and the grey state.

## Phase 0 — Windows prototype ✅ (2026-11-14)
- Tray dot red/orange/green, tooltip and menu with per-session state.
- Claude Code sessions (CLI and Desktop "Code" tab) via `~/.claude/sessions/*.json`.
- Claude chat / Cowork / scheduled tasks → red via Desktop toasts in the notification DB.
- Taskbar flash on transition to red; autostart; transition log (`cambios.log`).

## Phase 1 — Harden Windows
- Verify which Claude Code prompts set `status: waiting` (permission, AskUserQuestion, plan approval).
- Verify toast texts in English and German UI.
- "Unknown" (grey) state when a source fails or its format changes.
- Settings in a small JSON file; English UI strings with Spanish/German translations
  (Go packages `internal/config`, `internal/i18n`).
- Installer/uninstaller script (`install/windows`), including the silent-notification tweak
  as an explicit opt-in with backup/restore. Installs the prototype today, the Go build later.
- Packaging decided: the core moves to Go (Phase 2); the PowerShell prototype stays as reference.

## Phase 2 — Cross-platform core (Linux, macOS)
- Candidate stacks: Python + pystray, Go + systray, Rust + Tauri. Pick for: no runtime install for
  end users, ARM64 + x64 builds, small binary, easy CI on GitHub Actions.
- Pluggable sources; the Claude Code session-file source is portable as-is.
- macOS: menu-bar item; Linux: StatusNotifierItem/AppIndicator.
- CI: build and smoke-test on `windows-latest`, `windows-11-arm`, `ubuntu-latest`, `macos-latest`.

## Phase 3 — More agents
- Gemini CLI, OpenAI Codex CLI, GitHub Copilot CLI, others with hooks or state files.
- One source module per agent; additive hook install/uninstall.

## Phase 4 — Subscription usage (long term)
- Optional panel with plan usage per provider, only from data the user's own tools already hold locally.

## Ideas (not scheduled)
- Cloud and Remote Control sessions from the user's own claude.ai session list (undocumented; opt-in).
- Detect sessions that ended their turn with a plain-text question ("idle but needs you").

## Phase 5 — Public release
- Repo public, MIT, README + GitHub Pages landing page, screenshots/GIFs, install instructions per OS.
- Everything free; no telemetry.
