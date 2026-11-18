# Contributing

AgentSemaphore is **pre-alpha**: help of any size is welcome, and testing on your own machine is one of the
most useful things you can do.

## Ways to help

- **Try it and report what you see.** Use the [test report](https://github.com/LuisPuertollano/AgentSemaphore/issues/new?template=test-report.yml)
  form: your OS and architecture, the agents and versions, what the tray showed and what the agent was actually
  doing. Reports from macOS, Linux desktops and from Copilot, Codex or Gemini CLI users are the most needed.
- **Share findings.** How an agent stores its live state, which notifications it posts, what changed in a
  new version. `docs/FINDINGS.md` records what has been measured so far; corrections are very welcome.
- **Add a source for another agent.** Copilot, Codex and Gemini CLI go through `internal/source/hooks`
  (see `docs/AGENTS.md`); other agents may need their own package. One package per source under
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

## Releases

Maintainers only. Either push a tag `vX.Y.Z` (or `vX.Y.Z-alpha.N` for a pre-release), or run the
**Release** workflow by hand on `main` (Actions → Release → Run workflow) with the version to create.
The workflow builds the six binaries, publishes them with `SHA256SUMS.txt`, then installs the new
release on Windows x64 and ARM64 with `install.ps1 -FromRelease` and checks the version it reports.
