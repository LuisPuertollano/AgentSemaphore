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

**Static reading of the 2.1.325 binary (2026-11-14, not a runtime test).** The interactive UI
derives the state like this: `waiting` if an MCP elicitation is queued (`waitingFor: "input needed"`),
else if the top dialog declares a reason (permission dialogs default to `"permission prompt"`;
about 25 other dialogs use `"dialog open"`; also `"sandbox request"`, `"goal proposal"`), else if a
worker or sandbox request is pending (`"worker request"`, `"sandbox request"`); otherwise `busy`
while a query runs, else `idle`. AskUserQuestion and plan approval are not named explicitly; they
are expected to arrive as permission dialogs. This logic lives in the terminal UI, so the Desktop
**Code** tab (which draws its own dialogs) still has to be checked on a real machine
([issue #11](https://github.com/LuisPuertollano/AgentSemaphore/issues/11)).

**Verified on the laptop (2026-11-14, Windows 11 ARM64, Desktop Code tab on a local folder, bundled
Claude Code 2.1.325, read-only probe):**

| Prompt | `status` / `waitingFor` | Timing |
|---|---|---|
| Tool permission dialog (permission mode "Ask") | `waiting` / `permission prompt` | set when the dialog appeared, back to `busy` on approval |
| AskUserQuestion | `waiting` / `input needed` | set within ~0.1 s of the question |
| Same session in **Auto** permission mode | no dialog, so no `waiting` (expected) | — |
| Plan approval | not tested yet | — |

- Windows session files carry `procStart` and `pidDomain` too.
- Claude Desktop creates **short-lived session files with an empty `status`** (< 1 s) while sessions start
  (e.g. `luisp-6f`, `financedata-24`). An empty status is therefore ignored for 10 s before it counts
  as unknown (grey), so the tray does not flash grey.
- **CLI 2.1.235 (npm install, Windows):** four sessions started from a script ran 2 min each and wrote only
  `<pid>.<hash>.key`, **no `.json`**. Not yet known whether that older version does not write session state
  or the sessions were stuck at a startup prompt. To recheck after updating the CLI.

**Documented alternative:** `claude agents --json` ("print active sessions … as a JSON array … for
scripting; does not require a TTY") returns the local sessions with `pid`, `cwd`, `kind`, `startedAt`,
`sessionId`, `name`, `status` (checked on Linux, 2.1.325). Local sessions only. A candidate to replace
reading the files, and the natural command to run over SSH on other machines (#24). Its start-up cost
per poll has not been measured yet.

**Linux uses the same format** (checked in a cloud container, 2.1.325): `~/.claude/sessions/<pid>.json`
with the same fields plus `procStart` (process start time, useful against PID reuse), `pidDomain`,
`nameSource`, `peerProtocol`, `messagingSocketPath`. A `<pid>.<hash>.key` file sits next to it.

**Gap observed (2026-11-14):** the claude.ai session list reported a Remote Control session
("Proxmox SysAdmin") as "Waiting on permission: AskUserQuestion" while the laptop tray showed only
"1 idle". Checked on the laptop the same day: `~/.claude/sessions` held a single file
(`status: idle`, `entrypoint: claude-desktop`, process `claude`) — the waiting session was **not
running on the laptop**, so the tray was right about the laptop. It was a Claude Desktop **Code** tab
session in an **SSH environment** (`root@<LAN host>`, Remote Control on): Desktop runs Claude Code on
the remote host, so the session file lives there. Sessions on other machines (SSH environments,
Remote Control) and in the cloud are invisible to local sources; see the ideas in
[issue #19](https://github.com/LuisPuertollano/AgentSemaphore/issues/19) and
[issue #24](https://github.com/LuisPuertollano/AgentSemaphore/issues/24).

For the same reason, test point A in `docs/VERIFY.md` with a **local** Code-tab session.

Side note: `cambios.log` is UTF-8 without BOM; Windows PowerShell 5.1's `Get-Content` shows accents
garbled (`semÃ¡foro`). Use `pwsh` or `Get-Content -Encoding UTF8`.

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

Localisation: the phrases live in `prototype/windows/avisos-textos.json` per language; only Spanish is
verified, English/German are **not yet verified** (procedure in `docs/VERIFY.md`). A Claude toast that
matches no known phrase counts as red (better a red too many than a wrong green) and is logged.

**Cloud sessions do not post toasts (live test, 2026-11-14):** a Claude Code session on claude.ai/code,
opened from Claude Desktop, waited on AskUserQuestion with Desktop in the background. No Windows toast arrived
and nothing appeared in the notification center. Cloud sessions are invisible to every local source
([issue #22](https://github.com/LuisPuertollano/AgentSemaphore/issues/22)).

**Toast texts in other languages:** the Desktop package (`Claude_5.11492.5.0_arm64`, file `ion-dist.asar`)
ships its UI strings in at least en, es, de, fr, it, pt, hi, id, ja and ko. `tools/windows/verify-phase1.ps1`
extracts them without switching the app's language. The exact keys of the four toast texts are still to be picked
out of the results (the phrase "Tarea programada" alone matched 262 keys).

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
