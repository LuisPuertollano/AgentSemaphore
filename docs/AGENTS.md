# Other agents: GitHub Copilot CLI, OpenAI Codex CLI, Gemini CLI

Claude Code is read from its own session files. The other CLIs below do not write a documented live
state, but all three have a **hook system**: they can run a command on events such as "turn started",
"permission requested" or "turn finished". AgentSemaphore can register itself there. The CLI then runs
`agentsemaphore hook <agent> <event>` with the event's JSON on stdin, and the dot follows.

> **Status (2026-11-17):** built from each vendor's documentation and tested in CI with simulated
> hook calls. **Not yet tested with the real CLIs.** If you use one of them, a short report is very
> welcome.

## Set up

Hooks are never installed automatically. You choose per agent:

```sh
agentsemaphore hooks install copilot    # writes ~/.copilot/hooks/agentsemaphore.json (our own file)
agentsemaphore hooks install codex      # adds our entries to ~/.codex/hooks.json, next to yours
agentsemaphore hooks show gemini        # prints the snippet to add to ~/.gemini/settings.json by hand
agentsemaphore hooks status copilot
agentsemaphore hooks uninstall copilot  # removes only what install added
```

- **Copilot CLI** loads every `*.json` file in `~/.copilot/hooks/` (or `$COPILOT_HOME/hooks/`), so ours
  is a separate file. Your own hooks and settings are not touched.
- **Codex CLI** keeps user hooks in `$CODEX_HOME/hooks.json` (`~/.codex/hooks.json`). Our entries are
  added next to yours and recognised later by their command. A file that is not plain JSON is left
  alone. **Codex runs a new hook only after you trust it:** open Codex, type `/hooks`, and trust the
  AgentSemaphore entries. You have to do this again whenever they change, for example after reinstalling
  AgentSemaphore somewhere else.
- **Gemini CLI** keeps hooks inside the user's main `settings.json`. AgentSemaphore does not rewrite that
  file: `hooks show gemini` prints the `hooks` part to add yourself.

The hooks call the program where it is installed. If you move it, run `hooks install` again.

**Windows:** run these commands with the console build, `agentsemaphore-cli.exe`, which the installer
puts next to `agentsemaphore.exe`. The hooks then use it too. The tray program is a GUI program, and
PowerShell does not pass hook input to a GUI program (see FINDINGS). Releases up to v0.1.0-alpha.2
do not have the console build yet.

## What each event means

| Agent | 🟠 working | 🔴 waiting for you | 🟢 idle | session removed |
|---|---|---|---|---|
| Copilot | `userPromptSubmitted`, `postToolUse`, `postToolUseFailure`, `subagentStart` | `notification` with `permission_prompt` or `elicitation_dialog` | `sessionStart`, `agentStop` | `sessionEnd` |
| Codex | `UserPromptSubmit`, `PostToolUse`, `SubagentStart` | `PermissionRequest` | `SessionStart`, `Stop`, `Interrupt` | `SessionEnd` |
| Gemini | `BeforeAgent`, `BeforeTool`, `AfterTool` | `Notification` with `ToolPermission`; `BeforeTool` for `ask_user` (unconfirmed) | `SessionStart`, `AfterAgent` | `SessionEnd` |

- **The agent's process id** is recorded with each session: the first parent of the hook process
  that is not a shell. When that process ends without a session-end event (a crash, a closed
  terminal), the session disappears from the menu. If it cannot be found, a session is dropped 12 h
  after its last event.
- **Pure observer.** The hook prints nothing and always exits 0. Gemini gets `{}`, its documented
  no-op. Problems are written to `hooks.log` next to the settings, never shown to the agent.
- **Not used on purpose:**
  - Copilot's `preToolUse` fails closed: a hook error would deny the tool call.
  - Copilot's `permissionRequest` fires before its own rules decide, so it also fires when no prompt
    is shown.
  - Codex's `notify` setting holds a single program, so ours would replace yours.

## Known gaps

- **Codex:** `PermissionRequest` fires "when Codex is about to ask for approval". If another hook or a
  rule approves it, the dot stays red until the next tool finishes (`PostToolUse` → orange). Whether a
  question to the user (`request_user_input`) reaches `PreToolUse` is not documented, so questions are
  not detected yet.
- **Gemini:** that the `ask_user` tool goes through `BeforeTool` is inferred from the tools reference,
  not stated. Also, per the project's own announcement
  (https://github.com/google-gemini/gemini-cli/discussions/27274, not re-checked here), Gemini CLI has
  served only enterprise licences and paid API keys since 2026-06-18.
- **Copilot:** with the session sandbox on, hooks run inside it, so writing our state file may be
  blocked (documented). Not tested.

## Sources

- Copilot CLI hooks reference: https://docs.github.com/en/copilot/reference/hooks-reference.
  Notification hook from 1.0.18, user hook folder from 0.0.422 ([changelog](https://github.com/github/copilot-cli/blob/main/changelog.md)).
- Codex hooks: https://developers.openai.com/codex/hooks (now learn.chatgpt.com/docs/hooks).
  PermissionRequest from 0.122.0, SessionEnd from 0.145.0, Interrupt from 0.150.0.
- Gemini CLI hooks: https://github.com/google-gemini/gemini-cli/blob/main/docs/hooks/reference.md.
  Hooks on by default from v0.26.0.

Checked against these pages on 2026-11-17. For Copilot (notification types, fail-open and fail-closed)
and Codex (hooks.json locations, trust step), the key points were re-read directly from the pages above.
