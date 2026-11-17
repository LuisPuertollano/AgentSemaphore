# Linux and macOS installer

Per-user install into `~/.local/bin`. No sudo. Needs `bash` and `curl`.

```sh
# newest release for this machine (pre-releases included), SHA-256 checked, start at sign-in
curl -fsSL https://raw.githubusercontent.com/LuisPuertollano/AgentSemaphore/main/install/unix/install.sh | bash -s -- --autostart
```

From a clone of this repo: `bash install/unix/install.sh --autostart`.

## Options

| Option | Effect |
|---|---|
| `--dry-run` | Show every change, make none |
| `--version <tag>` | That release instead of the newest, e.g. `v0.1.0-alpha.1` |
| `--exe <file>` | Install a binary you already have (no checksum to compare) |
| `--dir <folder>` | Install somewhere else than `~/.local/bin` |
| `--autostart` | Start at sign-in. It creates the same entry as "Start with the system" in the app's menu: `~/.config/autostart/agentsemaphore.desktop` on Linux, `~/Library/LaunchAgents/io.github.luispuertollano.agentsemaphore.plist` on macOS |
| `--no-start` | Do not start it now |
| `--uninstall` | Stop it, remove the binary and our autostart entry. Settings and log stay |

## Notes

- **Linux:** the dot is a StatusNotifierItem (the tray protocol KDE Plasma uses natively). GNOME
  shows it only with the AppIndicator extension; Ubuntu enables that by default. Other desktops
  are untested so far.
- **macOS:** the binary is not notarized (that needs a paid Apple account). Files downloaded with
  `curl`, as this script does, are normally not quarantined, so Gatekeeper does not block them. If you
  download a binary with a browser instead, run it once with right-click → Open, or
  `xattr -d com.apple.quarantine agentsemaphore`.
- `--autostart` needs a build newer than `v0.1.0-alpha.1`. With that version, use "Start with the
  system" in the menu instead.
- Tested in CI on Linux x64, Linux ARM64 and macOS (Apple silicon). Not yet on a real desktop:
  reports are welcome.
