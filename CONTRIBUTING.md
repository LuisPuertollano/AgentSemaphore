# Contributing

AgentSemaphore is **pre-alpha**: help of any size is welcome, and testing on your own machine is one of the
most useful things you can do.

## Ways to help

- **Try it and report what you see.** Open an issue with your OS (and architecture), the agent and version,
  what the tray showed and what the agent was actually doing.
- **Share findings.** How an agent stores its live state, which notifications it posts, what changed in a
  new version. `docs/FINDINGS.md` records what has been measured so far; corrections are very welcome.
- **Add a source for another agent** (Gemini CLI, Codex CLI, Copilot CLI, …). One package per source under
  `internal/source/`, read-only towards the agent, failing soft (unknown → grey, never a wrong green).
- **Translations.** UI strings live in `internal/i18n/locales/*.json`. A test checks every key exists in every language.

## Ground rules

- 100 % free: MIT, no paid services, no telemetry, no accounts.
- Read-only towards the agents. Never change an agent's own configuration unless the user asks. Hooks, if
  ever needed, are installed additively, and uninstall removes only ours.
- Before sharing logs or dumps, remove anything personal: conversation titles, user names, host names.
- Code, comments, commits and docs are in English.

## Development

```sh
go test ./...                                    # Go core
python3 -m unittest discover -s prototype/windows/tests   # Windows prototype helper
pwsh ./prototype/windows/tests/test-estado.ps1   # prototype logic (any OS with PowerShell 7)
go run ./cmd/agentsemaphore -once                # print the current state as JSON
```

CI runs all of this on Linux, Windows and macOS for every pull request.
