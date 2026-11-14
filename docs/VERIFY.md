# Verification on a real Windows machine (Phase 1, points A and B)

## Quick way: one script

`tools/windows/verify-phase1.ps1` does most of it automatically and writes one report
(`%USERPROFILE%\agentsemaphore-verify.md`, also copied to the clipboard):

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File tools\windows\verify-phase1.ps1            # toast texts + CLI
pwsh -NoProfile -ExecutionPolicy Bypass -File tools\windows\verify-phase1.ps1 -Desktop   # + guided Code-tab steps
```

- **Toast texts:** read from the installed Claude Desktop package in every language it ships.
  No need to switch the app's language.
- **CLI:** short sessions open in new windows and close by themselves. Run the script from a normal
  PowerShell window, not from inside a Claude session: windows opened from a Claude session become
  "child sessions" and do not save their state.
- **Code tab:** cannot be driven from outside (no UI Automation tree). With `-Desktop`, the script
  puts each prompt in the clipboard; you paste it and answer when told.

The manual steps below are the fallback.

## Manual steps

Both tools are read-only. They never write to Claude's files or to the notification database.
Results go into `docs/FINDINGS.md`. Before publishing, remove conversation titles.

## A — Which prompts set `status: waiting`? ([#11](https://github.com/LuisPuertollano/AgentSemaphore/issues/11))

1. Start the probe in its own window:
   `pwsh -File tools\windows\probe-sessions.ps1`
   Windows PowerShell 5.1 works too. It prints every change and appends it to
   `%TEMP%\agentsemaphore-probe.log`.
2. Run each step below twice: once in the **Claude Code CLI** (Windows Terminal, `claude`) and once
   in the **Claude Desktop Code tab**. Use the default permission mode (asks before running commands).
   In the Code tab, pick a **local** folder as the environment, not an SSH host: an SSH session runs
   on the remote machine and writes its session file there.
   Wait about 3 s at each prompt before answering, so the probe records it.

| # | What to type / do | Expected `status` / `waitingFor` | CLI | Desktop Code |
|---|---|---|---|---|
| 1 | "Run the command `whoami`" → permission dialog appears | `waiting` / `permission prompt` | | |
| 2 | "Use the AskUserQuestion tool to ask me which colour I prefer" | `waiting` / ? | | |
| 3 | Switch to plan mode (Shift+Tab), ask "plan how to create hello.txt" → plan approval dialog | `waiting` / ? | | |
| 4 | "Count to 5, running `Start-Sleep 2` between numbers" (approve) | `busy` while it runs | | |
| 5 | "Ask me in plain text what my favourite food is, then stop" | `idle` expected — that is the known gap ([#20](https://github.com/LuisPuertollano/AgentSemaphore/issues/20)) | | |
| 6 | Leave the session doing nothing | `idle` | | |

3. Stop the probe (Ctrl+C) and share the log file.
4. Fill in for each row: the status actually seen, the `waitingFor` text, and the delay from the
   dialog appearing to the change in the log.
   Note whether the `procStart` / `pidDomain` fields exist on Windows (shown as `fields+=`).

## B — Claude Desktop toast texts in English and German ([#12](https://github.com/LuisPuertollano/AgentSemaphore/issues/12))

For each language, **English** and then **German**:

1. Claude Desktop → Settings → Language → switch to the language. Restart the app if it asks.
2. Make Claude need you while its window is **in the background** (switch to another window first):
   - Chat: ask something long, switch away, and wait for the answer to finish.
   - Chat: ask Claude to ask you a question before continuing, then switch away.
   - Cowork: give a task that needs a permission (e.g. "create a file on my desktop"), then switch away.
   - Scheduled task that needs approval, if you have one.
3. Dump what Windows stored:
   `python -I tools\windows\dump-toasts.py 2 > toasts-<lang>.txt`
4. Share the files. For each toast, note what you had done when it arrived.

Switch Claude Desktop back to your usual language afterwards.

With the results:
- the phrases in `prototype/windows/avisos-textos.json` become verified (`"verificado": true`);
- texts that do not need you go to `ignorar`;
- we check whether the `launch` attribute gives a language-independent marker.

Until then, an unrecognised Claude toast counts as **red** and is written to `cambios.log`.
