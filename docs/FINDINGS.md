# Technical findings (measured on Windows 11 ARM64, 2026-11-14)

Everything here was observed on a real machine unless marked otherwise.
Versions: Claude Desktop (Store, `Claude_pzs8sxrjxfjjc`), Claude Code CLI 2.1.235,
Claude Code bundled in Desktop 2.1.325, PowerShell 7.7, Python 3.12 arm64.

## 1. Claude Code writes its own live session state — no hooks needed

`%USERPROFILE%\.claude\sessions\<pid>.json` (one file per running Claude Code process,
including the sessions launched by Claude Desktop's **Code** tab, `entrypoint: "claude-desktop"`).

Relevant fields: `pid`, `sessionId`, `cwd`, `name`, `status`, `waitingFor`, `statusUpdatedAt`,
`updatedAt`, `entrypoint`, `kind`.

`status` ∈ `busy | shell | idle | waiting` (enumerated in the Claude Code binary as
`["busy","shell","idle","waiting"]`). `waitingFor` carries a short reason when `waiting`.

Mapping used: `waiting` → red · `busy`/`shell` → orange · `idle` → green.
Liveness: the file is ignored unless `pid` is alive **and** the process name is `claude`
(guards against stale files and PID reuse).

⚠️ Undocumented. Not yet verified: which prompts (permission vs AskUserQuestion vs plan
approval) set `waiting`. Verify before relying on it; fall back to hooks if needed.

## 2. Claude Code hooks are unreliable inside Claude Desktop

Open upstream issues report that `Notification` hooks (`permission_prompt`, `idle_prompt`)
never fire in Desktop "Code" sessions, while `Stop` does. That is why the prototype reads the
session files instead of relying on hooks. Hooks in `~/.claude/settings.json` never run in
cloud sessions.

## 3. Claude chat & Cowork (incl. cloud sessions) — red only, via Windows notifications

Chat and Cowork run on Anthropic's servers; nothing local reflects "working".
The Desktop window exposes **no** UI Automation tree (only min/max/close), so it cannot be read.

But Claude Desktop posts toasts when a conversation needs you. Windows stores them in
`%LOCALAPPDATA%\Microsoft\Windows\Notifications\wpndatabase.db` (SQLite, WAL mode).

- Handler (AUMID): `Claude_pzs8sxrjxfjjc!Claude`
- Texts seen (Spanish UI): `"<conversation title>" / "Claude necesita tu respuesta para continuar"`,
  `"Claude está esperando tu respuesta"`, `"La tarea programada necesita tu aprobación" / "<task>"`,
  `"Tarea programada completada"` (ignored).
- `ArrivalTime` is a Windows FILETIME (UTC).
- The DB is read from a **copy** (db + wal), never in place. Only re-read when the WAL mtime changes.
- Measured end-to-end: toast at 13:47:48 → tray red at 13:47:49 → green at 13:48:00 when Claude
  came to the foreground.

Clearing rule: a toast counts as red until the Claude window has been in the foreground for
≥ 2 s after it arrived ("seen"), or until it disappears from the DB.

Localisation: the regex covers ES/EN/DE phrases; English/German strings are **not yet verified**.

## 4. Silent notifications keep the data source alive

To cut notification noise without blinding the semaphore, set the Claude app to "no banner,
no sound, keep in notification center":

`HKCU\Software\Microsoft\Windows\CurrentVersion\Notifications\Settings\<AUMID>`
- `ShowBanner` DWORD `0` → no pop-up (toast still stored in the notification center)
- `SoundFile` REG_SZ empty → no sound (delete the value to restore the default sound)
- `ShowInActionCenter` must be absent or `1` (if `0`, toasts are no longer stored → semaphore blind)

Phone Link mirrors the phone's Claude notifications as a separate AUMID:
`Microsoft.YourPhone_8wekyb3d8bbwe!YourPhoneNotifications_com.anthropic.claude` (duplicates; silence too).

## 5. Windows practicalities

- `conhost.exe --headless pwsh.exe -File …` runs the tray script with no console window.
- Autostart: shortcut in `shell:startup`.
- Single instance: named mutex `Local\SemaforoClaude`.
- Taskbar flash: `FlashWindowEx` with `FLASHW_TRAY | FLASHW_TIMERNOFG` on the Desktop window
  (the `claude` process whose path contains `WindowsApps` and has a main window).
- `pwsh -NoProfile` starts in ~0.3 s on this machine; Windows PowerShell 5.1 takes ~3 s
  (do not use 5.1 for hooks).
- Toast display duration is a global setting (`HKCU\Control Panel\Accessibility\MessageDuration`,
  `SPI_SETMESSAGEDURATION`) — do not touch it from the app.

## 6. Prior art (checked 2026-11-14)

Windows tray indicators for Claude Code exist (Claudebar, claude-status-bar-windows, Code Light AI,
claude-sessions, ClaudePulse-win, ClaudeTaskbar) and several macOS menu-bar ones (Ampel, Pulse,
ccemaphore, Claude Status Bar). None covers Claude **chat/Cowork** from the Desktop app, most rely
on the `Notification` hook that fails in Desktop, and none offers multi-agent + cross-platform in one
free tool. That is the gap this project targets.
