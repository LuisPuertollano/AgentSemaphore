# Roadmap

## Phase 0 — Windows prototype ✅ (2026-11-14)
- Tray dot red/orange/green, tooltip and menu with per-session state.
- Claude Code sessions (CLI and Desktop "Code" tab) via `~/.claude/sessions/*.json`.
- Claude chat / Cowork / scheduled tasks → red via Desktop toasts in the notification DB.
- Taskbar flash on transition to red; autostart; transition log (`cambios.log`).

## Phase 1 — Harden Windows
- Verify which Claude Code prompts set `status: waiting` (permission, AskUserQuestion, plan approval).
- Verify toast texts in English and German UI.
- "Unknown" (grey) state when a source fails or its format changes.
- Settings in a small JSON file; English UI strings with Spanish/German translations.
- Installer/uninstaller script (`irm … | iex` style), including the silent-notification tweak
  as an explicit opt-in with backup/restore.
- Decide packaging: keep PowerShell, or move the core to the cross-platform implementation (Phase 2).

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

## Phase 5 — Public release
- Repo public, MIT, README + GitHub Pages landing page, screenshots/GIFs, install instructions per OS.
- Everything free; no telemetry.
