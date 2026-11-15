# Windows installer

Per-user install into `%LOCALAPPDATA%\AgentSemaphore`. No admin rights needed. Requires PowerShell 7 (`pwsh`).

## Go build (recommended)

A single executable. No Python needed. It installs on x64 and ARM64.

```powershell
# from a downloaded binary (CI artifact or release asset)
pwsh -File install/windows/install.ps1 -Exe .\agentsemaphore-windows-arm64.exe -Autostart
# or download the latest release for this machine
pwsh -File install/windows/install.ps1 -FromRelease -Autostart
```

`-Autostart` adds the same `HKCU\…\Run` entry the app's own "Start with the system" menu uses.

## PowerShell prototype

```powershell
pwsh -File install/windows/install.ps1 -Autostart
```

## Switches

| Switch | Effect |
|---|---|
| `-WhatIf` | Show every change, make none |
| `-Exe <path>` / `-FromRelease` | Install the Go build. Without either, the prototype is installed |
| `-Autostart` | Start at sign-in |
| `-SilentNotifications` | Claude toasts arrive with no banner and no sound, but stay in the notification center, where the semaphore reads them. The previous values are backed up and restored on uninstall |
| `-IncludePhoneLink` | Same for Phone Link's mirror of the phone's Claude notifications |
| `-RemoveLegacy` | Stops and removes the earlier manual install in `%LOCALAPPDATA%\SemaforoClaude` |
| `-NoStart` | Do not start after installing |

Installing one build removes what the other left in the folder: files, shortcut and Run entry. Switching
back and forth is therefore clean.

## Uninstall

```powershell
pwsh -File "$env:LOCALAPPDATA\AgentSemaphore\uninstall.ps1"
```

The uninstaller:
- removes only what the installer created;
- leaves alone a Run entry that points somewhere else;
- restores the notification settings.

The app's settings and log in `%APPDATA%\AgentSemaphore` are kept.

`test-installer.ps1` is the end-to-end test used by CI. It changes registry settings, so it refuses to run outside CI.
