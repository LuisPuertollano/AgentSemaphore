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
| Plan approval | not tested in the Code tab (see CLI below) | — |

- Windows session files carry `procStart` and `pidDomain` too.
- Claude Desktop creates **short-lived session files with an empty `status`** (< 1 s) while sessions start
  (named like `<user>-6f` or `<project>-24`). An empty status is therefore ignored for 10 s before it counts
  as unknown (grey), so the tray does not flash grey.
- **CLI 2.1.235 (npm install, Windows):** four sessions started from a script ran 2 min each and wrote only
  `<pid>.<hash>.key`, **no `.json`**. The script had been started from inside a Claude Code session, and
  every CLI window showed *"Transcript saving is off — inherited `CLAUDE_CODE_CHILD_SESSION` marker"*.
  Likely cause: child sessions do not save their state. The script now clears `CLAUDE_CODE_*` variables
  before launching. To recheck from a normal PowerShell window.
- **Rerun with the CLI updated to 2.1.325, from a normal PowerShell window (2026-11-14):** every session
  wrote its `.json` (with `procStart`). Not known which of the two changes fixed it.

  | Prompt (CLI 2.1.325, Windows ARM64) | Timeline (s after start → status [waitingFor]) |
  |---|---|
  | Command that needs permission | 1.1 *(empty)* → 1.3 idle → 1.6 busy → 7.7 waiting [permission prompt] |
  | AskUserQuestion | 1.1 idle → 1.3 busy → 9.0 waiting [input needed] |
  | Plan approval (plan mode) | 0.8 *(empty)* → 1.1 idle → 1.3 busy → 16.1 waiting [**permission prompt**] |
  | Question in plain text, then stop | 0.8 *(empty)* → 1.1 idle → 1.3 busy → 8.2 **idle** (known gap, #20) |

  So all three kinds of prompt set `waiting`; plan approval reports itself as `permission prompt`. The CLI
  also starts with an empty `status` for ~0.3 s, which confirms the need for the starting grace period.
- In the CLI, `whoami` ran **without** a permission dialog (read-only commands are allowed automatically),
  while the Desktop Code tab asked. The test now uses a command that writes a file.

**Documented alternative:** `claude agents --json` ("print active sessions … as a JSON array … for
scripting; does not require a TTY") returns the local sessions with `pid`, `cwd`, `kind`, `startedAt`,
`sessionId`, `name`, `status` (checked on Linux, 2.1.325). Local sessions only. A candidate to replace
reading the files, and the natural command to run over SSH on other machines (#24). Its start-up cost
per poll has not been measured yet.

**Linux uses the same format** (checked in a cloud container, 2.1.325): `~/.claude/sessions/<pid>.json`
with the same fields plus `procStart` (process start time, useful against PID reuse), `pidDomain`,
`nameSource`, `peerProtocol`, `messagingSocketPath`. A `<pid>.<hash>.key` file sits next to it.

**Gap observed (2026-11-14):** the claude.ai session list reported a Remote Control session
as "Waiting on permission: AskUserQuestion" while the laptop tray showed only
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

**Claude Code sessions post a toast when their turn ends, not while a dialog waits (2026-11-14).**
- Live test: a cloud session (claude.ai/code, opened from Desktop) waited on AskUserQuestion with Desktop in
  the background. **No toast** arrived while the dialog was open.
- The same session later posted toasts **at the end of each turn**, with the session title and a one-line
  summary of the turn (e.g. what it needs from you). Desktop also posts them for local / SSH Code-tab
  sessions.
- These toasts carry language-independent markers in the payload:
  - `<header id="session-session_<id>" title="<session title>">` → a **cloud** session;
  - `<header id="session-local_<uuid>" …>` → a **Code-tab session, local or over SSH**.
- **Silence marks summaries that need nothing.** Every end-of-turn summary is either "what I did" or "what
  I need from you". Desktop posts the first kind with `<audio silent="true"/>`. Samples from 2026-11-14:

  | Summary (paraphrased) | Origin | Silent | Needed the user |
  |---|---|---|---|
  | "confirm step 4 (the reinstall)" | cloud | no | yes |
  | "deleted three containers; backups kept" | local | yes | no |
  | "phase 1 applied on a container: 4 cores…" | local | yes | no |
  | "PR merged; no open PRs being watched" | cloud | yes | no |
  | "safeguards flagged this session" | local | no | a safety notice |

  **Rule adopted (5 of 5 consistent):** a silent toast with a Code-session header does not count as red.
  A known "needs you" phrase still wins, and any other unrecognised Claude toast is still red. Each silent
  summary is logged, so a misclassification would show up ([issue #22](https://github.com/LuisPuertollano/AgentSemaphore/issues/22)).
- **Permission requests mid-turn do post a toast (2026-11-15).** A cloud session waiting for permission to use
  AskUserQuestion posted *"Approve or deny AskUserQuestion"*, not silent. A question asked directly (no permission
  step) posted nothing until the turn ended (2026-11-14 test).
- So cloud and SSH sessions are partly visible: when a turn ends (including turns that end with a question in
  plain text, #20) and when they ask for a permission; not while a direct question is open mid-turn.

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

## Linux tray: an icon set during start-up can be lost (2026-11-17)

Seen in CI on both Linux runners (x64 on Ubuntu 26.04, and ARM64); not reproduced locally on Ubuntu 24.04 x64. Over D-Bus
the tray's tooltip and menu said "1 waiting" while its `IconPixmap` stayed grey for 20 s. In
fyne.io/systray v1.12.2, `SetIcon` only stores the icon while the StatusNotifierItem properties are
still being exported. When it lands between building the property set and publishing it, the
icon is never sent. The tray now re-sends the icon on every tick for the first ten seconds and
then every 30 s, not only when the state changes. `tools/e2e/unix-e2e.sh` reads the icon back
over D-Bus and catches this.

## procStart and pidDomain on Linux (2026-11-17)

Checked against a live Claude Code 2.1.326 session on Linux (x64):

- `procStart` is a string holding the process start time exactly as `/proc/<pid>/stat` field 22
  gives it (clock ticks since boot). Here: `"205"` = field 22 of that pid.
- `pidDomain` is `linux:<machine-id>:pid:[<pid-namespace inode>]`, i.e. the content of
  `/etc/machine-id` and the target of `/proc/self/ns/pid`.

The Go core uses `procStart` only when the session's `pidDomain` equals its own: then a different
start time means the pid was reused by a newer process, and the old session file is ignored. On
other systems (Windows and macOS also write both fields, format not checked) and for sessions from
another machine or PID namespace, only the process-name check applies, as before.

## Windows: hooks cannot rely on the GUI build under PowerShell (2026-11-17)

Measured on the CI runners (PowerShell 7). The same hook JSON was piped with
`$json | & <exe> hook copilot agentStop`:

- into the console build (`go build`): the session file was written before the command returned;
- into the GUI build (`go build -ldflags -H=windowsgui`, what the tray uses): PowerShell does not wait for
  a GUI program. On `windows-latest` nothing had been written 0.5 s later, not even a `hooks.log`
  line. On `windows-11-arm` the file showed up a few seconds later.

A program that PowerShell does not wait for would turn hook events into a race: an "agent stop" can
be processed before the "permission prompt" that came first. The console build runs synchronously
and keeps the events in order. On Windows, Copilot CLI hooks are given as a `powershell` command.

So on Windows the agent hooks run a console program. Releases therefore also ship
`agentsemaphore-cli-windows-<arch>.exe`. The installer puts it next to the tray as `agentsemaphore-cli.exe`, and `hooks install` points the hooks at it. It
is also the one to use for commands in a terminal, because the GUI build prints nothing there.

## macOS: clicks and the Dock (2026-11-18; code reading and CI, no real Mac)

**Clicks.** In fyne.io/systray v1.12.2 (`systray_darwin.m`) an event monitor sends every mouse-down
on the menu bar item, right and Control clicks included, to the left-click handler (only clicks
with Command held are passed through). With a left-click handler set
(`SetOnTapped`), as on Windows and Linux to open Claude, the menu could not be opened at all on macOS.
So on macOS no handler is set: a click shows the menu, as menu bar items usually do, and Claude is
opened from the menu's "Open Claude". Found by reading the library code; not yet seen on a real Mac.

**Dock.** The tray is a bare executable, not an `.app` bundle, and fyne.io/systray does not set an activation
policy. On the `macos-latest` runner (Apple silicon) `lsappinfo` lists the running tray as
`type="BackgroundOnly"`: no Dock icon and no app menu, only the menu bar item. The macOS end-to-end
test now fails if it is ever listed as `Foreground`. Not yet seen on a real Mac.

On macOS a red state is shown by the dot only. There is no taskbar to flash, and bouncing a Dock icon
would need the Dock icon this app does not have; a notification would bring back the noise the project
exists to remove.

## Windows: Claude Desktop flashes its own taskbar button (2026-11-22, laptop)

On the Windows 11 ARM64 laptop (Claude Desktop 6.28899.6.0 from the Store), with the window minimized,
Claude Desktop's taskbar button turns orange when a reply arrives. It does so **also with AgentSemaphore's
"flash" option switched off**, so Claude Desktop flashes its own button. With the option on, the result
looks the same, so AgentSemaphore's own flash could not be told apart there.

Checked separately: `FlashWindowEx` (`FLASHW_TRAY | FLASHW_TIMERNOFG`) on the window that
`Process.MainWindowHandle` reports for the Store app's `Claude.exe` (title "Claude") turns the button
orange. That is the same call and the same kind of window the tray uses.

So for Claude Desktop chats the option adds little. It still matters for reds that Claude Desktop does
not flash for itself, for example a Claude Code CLI session waiting in a terminal. Not yet observed on a
real machine.

## Linux: the dot in a real GNOME top bar, headless in CI (2026-11-22)

`tools/e2e/gnome-e2e.sh` runs GNOME Shell 50.1 (Ubuntu 26.04 runner) with
`--headless --wayland --virtual-monitor 1280x800` and Ubuntu's AppIndicator extension, under a private
session bus with keyfile GSettings. Measured: the extension's `org.kde.StatusNotifierWatcher` lists the
tray, `Main.panel.statusArea` gets a visible, mapped `appindicator-…` button, and a screenshot of the top
bar shows the dot in its red (112 pixels of it).

- `Eval` and `org.gnome.Shell.Screenshot` are refused unless GNOME is in "unsafe mode". A test-only
  extension in the scratch HOME sets `global.context.unsafe_mode = true`. GNOME then shows a padlock
  in the top bar and a notification saying so; neither exists on a normal desktop.
- GNOME Shell needs the system bus with logind. It aborts at start-up in a container without them
  (seen in the development container: "Error calling StartServiceByName for org.freedesktop.login1").
  GitHub's Ubuntu runners have both.
