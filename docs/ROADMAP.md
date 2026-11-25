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
  Linux via D-Bus StatusNotifierItem; macOS built on `macos-latest`). Chosen over Python + pystray
  (runtime to install) and Rust + Tauri (heavier toolchain for a tray-only app).
- State priority: **red > grey > orange > green**. Grey = a source could not be read; it must
  never be masked by orange or shown as green.
- macOS: no paid notarization; unsigned build with a documented manual step.
- Order for Phase 1: settings, i18n and installer first, then the verifications and the grey state.

## Status (2026-11-18)
- Pre-releases from v0.1.0-alpha.1 on, all on the
  [releases page](https://github.com/LuisPuertollano/AgentSemaphore/releases): Windows x64/ARM64
  (tray and console build), Linux x64/ARM64, macOS Intel/Apple silicon, `SHA256SUMS.txt` and a Scoop
  manifest. Install scripts for Windows (`install/windows`) and Linux/macOS (`install/unix`). Every
  release is installed and checked by CI right after publishing.
- Real-machine testing so far: Windows 11 ARM64 only. On 2026-11-22, v0.1.0-alpha.5 installed from the
  release: a real Claude Desktop notification turned the dot red, and a left click opened Claude.
  Claude Desktop flashes its own taskbar button on a reply, so the tray's own flash could not be told
  apart there (see FINDINGS). Linux and macOS are tested in CI only (no maintainer
  machine), so reports from users of those systems are very welcome.

## Phase 0 — Windows prototype ✅ (2026-11-14)
- Tray dot red/orange/green, tooltip and menu with per-session state.
- Claude Code sessions (CLI and Desktop "Code" tab) via `~/.claude/sessions/*.json`.
- Claude chat / Cowork / scheduled tasks → red via Desktop toasts in the notification DB.
- Taskbar flash on transition to red; autostart; transition log (`cambios.log`).

## Phase 1 — Harden Windows ✅ (2026-11-14)
- Verify which Claude Code prompts set `status: waiting` (permission, AskUserQuestion, plan approval).
- Verify toast texts in English and German UI.
- "Unknown" (grey) state when a source fails or its format changes.
- Settings in a small JSON file; UI in seven languages (en, es, de, fr, it, zh, hi).
- Installer/uninstaller (`install/windows`), with the silent-notification tweak as an explicit
  opt-in with backup/restore.

## Phase 2 — Cross-platform core (Go) 🚧
Done:
- ✅ Go core: pluggable sources (Claude Code session files on every OS; Claude Desktop
  notifications on Windows), worst-state aggregation, tray with tooltip and menu.
- ✅ Windows: left click opens Claude, taskbar flash, autostart, single instance, installer
  (`-FromRelease` with SHA-256 check). End-to-end tests on Windows 11 ARM64 and x64 in CI.
- ✅ Releases: eight binaries (incl. the Windows console build) + `SHA256SUMS.txt`, published from a tag or by hand; each release is
  installed and checked on Windows, Linux and macOS right after publishing.
- ✅ PID reuse on Linux: `procStart` (verified format, see FINDINGS) tells a live session from an
  old file whose pid now belongs to a newer process.
- ✅ Linux and macOS end-to-end tests in CI (x64 and ARM64 Linux, Apple silicon and Intel macOS): fake
  sessions through every colour; on Linux the tray's dot, tooltip and menu are read back over
  D-Bus as a panel would see them.

- ✅ Linux and macOS install script (`install/unix`): downloads the release, checks SHA-256,
  autostart through the app itself, clean uninstall; tested in CI on the same three systems.
- ✅ macOS: a click shows the menu, whose "Open Claude" runs `open -a Claude` (a left click cannot
  open Claude there without hiding the menu, see FINDINGS); no Dock icon (checked in CI); a red
  state is shown by the dot only (no Dock bounce, no extra notification).
  Not yet tried on a real Mac.

Next:
- ✅ GNOME in CI: GNOME Shell 50 runs headless with Ubuntu's AppIndicator extension; the dot is
  checked in the real top bar (registered, visible, red in a screenshot). See FINDINGS.
- ✅ KDE Plasma in CI: a Plasma 6.6 X11 session (kwin_x11, kded, plasmashell) on Xvfb; the dot is
  checked in the default panel's system tray (registered, red in a screenshot). Plasma on Wayland could
  not be screenshotted on a runner without a GPU (see FINDINGS).
- Linux: other desktops (Cinnamon, XFCE as a full session…), real GNOME and Plasma sessions, Plasma on
  Wayland. In CI where possible, otherwise by user reports.

## Phase 3 — More agents 🚧
- ✅ One generic hook source (`internal/source/hooks`): the agent runs
  `agentsemaphore hook <agent> <event>`; one file per session; the agent's process is recorded so a
  crashed session disappears; a pure observer (no output, exit 0).
- ✅ GitHub Copilot CLI and OpenAI Codex CLI: `agentsemaphore hooks install|uninstall|status`.
  Copilot gets its own hook file; for Codex our entries are added next to the user's in `hooks.json`.
  Gemini CLI: the snippet is printed, because its hooks live in the user's main `settings.json`.
- ✅ Event meanings from the vendors' documentation, with sources and gaps in
  [docs/AGENTS.md](AGENTS.md). Tested in CI with simulated hook calls under fake agent processes.
- Next: reports from real Copilot, Codex and Gemini CLI users; Codex questions (`request_user_input`);
  more agents.

## Phase 4 — Subscription usage (long term)
- Optional panel with plan usage per provider, only from data the user's own tools already hold locally.
- Researched 2026-11-18: the only documented local source of plan usage is Claude Code's status line
  (`rate_limits` 5-hour and 7-day). See [docs/USAGE.md](USAGE.md).
- ✅ Claude Code: `agentsemaphore usage install claude` (opt-in; your own status line is still shown;
  exact restore). A "Plan usage" menu group; it never changes the dot. Tested end-to-end on Linux,
  macOS and Windows.
- ✅ "Limit reached" in the menu from two Claude Code hooks (`StopFailure` `rate_limit`,
  `quota_auto_resume_*`), installed with the status line, additive, removed exactly. Not yet seen with a
  real usage limit.
- Later: Copilot (OpenTelemetry file export) if requested.

## Phase 5 — Public release
- ✅ Repo public, MIT, README with visuals, contributing guide, pre-releases.
- ✅ Website (GitHub Pages), real screenshots (Linux, XFCE; `tools/screenshots/linux.sh`), test-report form.
- ✅ Scoop manifest with every release (Windows), installed and checked in CI.
- Next: Homebrew (needs a tap kept in step with each release) and winget (needs a pull request to
  microsoft/winget-pkgs per release); Windows/macOS screenshots from testers.
- Everything free; no telemetry.

## Ideas (not scheduled)
- Cloud and Remote Control sessions from the user's own claude.ai session list (undocumented; opt-in).
- Detect sessions that ended their turn with a plain-text question ("idle but needs you").
- Claude Code sessions that Claude Desktop runs on SSH hosts.
