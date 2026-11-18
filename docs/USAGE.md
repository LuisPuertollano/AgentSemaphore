# Phase 4 — subscription usage: findings and proposed design

Status: **Claude Code part implemented (2026-11-18)**, opt-in. The maintainer agreed: install only on
request, exact restore, usage shown in the menu only, Copilot later. Researched from official
documentation only.

```sh
agentsemaphore usage install claude     # Windows: agentsemaphore-cli usage install claude
agentsemaphore usage status claude
agentsemaphore usage uninstall claude   # restores your previous status line exactly
```

## Ground rules

- **Local only.** Use what the user's own tools already keep or expose on this machine. AgentSemaphore never
  calls a vendor API with the user's credentials.
- **Never read conversation content.** Session transcripts and history files are off limits, even when
  they also hold token counts.
- **Opt-in, additive, reversible.** Anything that touches an agent's configuration is installed only on
  request, and uninstall restores exactly what was there before.
- **Usage never makes the dot red.** The dot stays about who needs you. Usage is shown in the menu, and
  later perhaps as an optional warning.

## What exists locally (official docs, 2026-11-18)

| Agent | Plan / limit usage available locally | How a separate program can read it | Usable? |
|---|---|---|---|
| **Claude Code** | `rate_limits.five_hour` and `rate_limits.seven_day`: `used_percentage`, `resets_at` (Pro, Max, Ultra; after the first response of a session) | The **status line** command gets them as JSON on stdin ([statusline docs](https://code.claude.com/docs/en/statusline)) | **Yes**, see the design below |
| Claude Code | "Limit reached" only: `StopFailure` with `error: "rate_limit"`, `Notification` types `quota_auto_resume_*` | Hooks ([hooks docs](https://code.claude.com/docs/en/hooks)) | Yes, as a "limit reached" note |
| Claude Code | `/usage` (`/cost` and `/stats` are aliases): bars and history | Interactive screen only, no JSON | No |
| Claude Code | Cost and tokens as OpenTelemetry metrics | Local OTLP or Prometheus endpoint; needs environment variables; no rate-limit metric | Not for plan usage |
| Copilot CLI | Premium requests / AI Credits per session (`github.copilot.total_premium_requests`) | OpenTelemetry **file** export (`COPILOT_OTEL_FILE_EXPORTER_PATH`), content off by default | Maybe later, opt-in |
| Copilot CLI | Remaining quota | Only through the SDK, which asks GitHub with the user's login | No (vendor call) |
| Codex CLI | Rate limits in `/status`; `account/rateLimits/read` in `codex app-server` | Interactive, or an app-server call that asks OpenAI with the user's login | No |
| Gemini CLI | Quota in `/stats model` | Interactive only; telemetry has tokens but no quota | No |

Transcripts (`~/.claude/projects/…`, `~/.codex/history.jsonl`, `~/.copilot/session-state/…`,
`~/.gemini/tmp/…/chats/`) are excluded: they hold the conversation, and Claude Code documents its format
as internal.

## Design (Claude Code; implemented)

1. **`agentsemaphore statusline`**, a status line wrapper.
   - Claude Code runs it with the status JSON on stdin.
   - It writes **only** these numbers to `<config dir>/usage/claude.json`: the two percentages, the two
     reset times, and when they were seen.
   - It then runs the user's own status line command, if there was one, with the same input, and prints
     that command's output unchanged. Without one, it prints a short line like `5h 23% · week 41%`.
   - It must never break the status line. On any problem it falls back to passing the input through.
2. **`agentsemaphore usage install claude` / `uninstall claude`.**
   - Install sets `statusLine.command` in `~/.claude/settings.json` to the wrapper and keeps the previous
     value in our own files.
   - Uninstall restores the previous value exactly, or removes the key if there was none.
   - A `settings.json` that is not plain JSON is left alone.
3. **Menu.** A "Usage" group appears only when there is data, for example:
   `Claude: 5 h 23 % (resets 14:00) · week 41 % (resets Mon 09:00)`.
   - Data older than its reset time is shown as "unknown (no session since …)", never as 0 %.
4. **Not yet:** a "limit reached" note from the hooks `StopFailure: rate_limit` (it needs Claude Code
   hooks, which AgentSemaphore does not install today).
5. **No extra setting.** Nothing shows until `usage install claude` has been run and Claude Code has
   reported usage.
6. **Tests.**
   - Unit tests: parsing, missing windows, stale data, wrapper pass-through and failure.
   - e2e: Claude Code is simulated by running the wrapper with sample JSON taken from the docs.

## Decisions (2026-11-18)

1. `usage install claude` may edit `~/.claude/settings.json` (only `statusLine`, opt-in, exact restore).
2. Usage stays in the menu. It never changes the dot.
3. Copilot's OpenTelemetry file export: later, if someone asks.

On Windows the user's previous status line command is run through `bash` when it is on the PATH (as Git
for Windows provides), otherwise through `cmd`. How Claude Code itself runs it on Windows is not
documented. If the two differ, the wrapper falls back to its own short line.
