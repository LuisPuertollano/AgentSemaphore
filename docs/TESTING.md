# How to test AgentSemaphore

Thank you for trying it. This page says what to install, what to look at and how to report it.
It is kept up to date with every release; the section **What to check now** changes most often.

A report takes about five minutes. "All fine" is a useful report too:
**[send a test report](https://github.com/LuisPuertollano/AgentSemaphore/issues/new?template=test-report.yml)**.

## What to check now

Newest pre-release: **v0.1.0-alpha.6** (2026-11-22). Already checked on a real machine: Windows 11 ARM64
with Claude Desktop and Claude Code. Everything else has been tested in CI only, so it needs you most:

| Where | What we most want to know |
|---|---|
| **macOS** (Apple silicon or Intel) | Does the dot appear in the menu bar? Does a **click open its menu** (new in alpha.6)? Does it turn red while Claude Code waits for you? Is there **no** Dock icon? Intel Macs: does the binary run at all? (never run anywhere yet) |
| **Linux desktops** (GNOME, KDE Plasma, Cinnamon, XFCE, …) | Does the dot appear in the panel, with the right colour, tooltip and menu? |
| **Copilot CLI, Codex CLI, Gemini CLI** (any system) | After `agentsemaphore hooks install <agent>`: red when the agent asks you something, orange while it works, green when it is done ([docs/AGENTS.md](AGENTS.md)) |
| **Windows x64** | The same as on ARM64: dot, red on a Claude Desktop notification, left click opens Claude |

New in alpha.6:
- **macOS:** a click on the dot shows its menu; "Open Claude" is in the menu. In alpha.5 a click opened
  Claude and the menu could not be opened at all
  ([FINDINGS](FINDINGS.md#macos-clicks-and-the-dock-2026-11-18-code-reading-and-ci-no-real-mac)).
- **Every system:** the menu has **"Send a test report…"**, which opens the report form with the title
  (system and architecture) and the version already filled in. The menu also shows the version.
- **Windows:** the flash option is now called "Flash Claude in the taskbar when it turns red". Claude Desktop
  also flashes its own button when a reply arrives, so you may see orange with the option off too.

## 1. Install

The installers fetch the newest release for your machine and check its SHA-256. Nothing needs admin
rights or sudo. Uninstalling removes everything except your settings and log.

**macOS and Linux:**

```sh
curl -fsSL https://raw.githubusercontent.com/LuisPuertollano/AgentSemaphore/main/install/unix/install.sh | bash -s -- --autostart
```

Remove it again with the same line and `--uninstall` instead of `--autostart`.
Details: [install/unix](../install/unix/README.md).

**Windows** (PowerShell 7), from a clone of the repository:

```powershell
git clone https://github.com/LuisPuertollano/AgentSemaphore
pwsh -File AgentSemaphore/install/windows/install.ps1 -FromRelease -Autostart
```

Or with [Scoop](https://scoop.sh), using the newest release tag:
`scoop install https://github.com/LuisPuertollano/AgentSemaphore/releases/download/v0.1.0-alpha.5/agentsemaphore.json`.
Details: [install/windows](../install/windows/README.md).

### macOS notes

- The binary is not notarized: notarization needs a paid Apple account, and the project stays free.
  Files downloaded with `curl`, as the install line does, are normally not quarantined, so Gatekeeper
  lets them run. If macOS still blocks it, run
  `xattr -d com.apple.quarantine ~/.local/bin/agentsemaphore` and start it again.
- The dot sits in the menu bar, near the clock. There is no Dock icon and no window, by design.
- macOS sees Claude Code (CLI and the Desktop app's **Code** tab) and the CLIs with hooks.
  Claude Desktop **chat, Cowork and scheduled tasks are not supported on macOS yet**: on Windows they
  are seen through Windows notifications, which macOS does not offer in the same way.

### Linux notes

- On GNOME the dot needs the AppIndicator extension (Ubuntu has it on by default).
  KDE Plasma shows it natively.

## 2. Use your agents as usual

Work as you normally would for a while, ideally at least once with each of these:
- an agent that **asks you something** or **waits for a permission**: the dot should turn **red**;
- an agent that **works on its own**: **orange**;
- **nothing running**: **green**. **Grey** means a source could not be read (the menu says which).

Hover the dot for a summary; open its menu for the list of sessions and what each one waits for.

## 3. Report

Use the **[test report form](https://github.com/LuisPuertollano/AgentSemaphore/issues/new?template=test-report.yml)**.
It asks for:

- **The version:** `agentsemaphore -version` (on Windows: `agentsemaphore-cli -version`).
- **Optionally, the state as the app sees it**, while a problem is visible: `agentsemaphore -once`
  (Windows: `agentsemaphore-cli -once`). It prints JSON; edit out session or folder names you do not
  want to share.
- **Optionally, the log:** `agentsemaphore.log` in the settings folder:

  | System | Settings folder |
  |---|---|
  | macOS | `~/Library/Application Support/AgentSemaphore` |
  | Linux | `~/.config/AgentSemaphore` |
  | Windows | `%APPDATA%\AgentSemaphore` |

Screenshots of the dot and the menu are very welcome, especially from macOS and Linux desktops not
seen yet.

## Privacy

AgentSemaphore is local only: no accounts, no servers, no telemetry. It never sends anything anywhere,
including your test report: you write that yourself, on GitHub.
