# Windows installer

Per-user install, no admin rights. PowerShell 7 (`pwsh`) required.

```powershell
# from a clone of the repo
pwsh -File install/windows/install.ps1 -Autostart
# see every change first, change nothing
pwsh -File install/windows/install.ps1 -Autostart -SilentNotifications -WhatIf
```

| Switch | Effect |
|---|---|
| `-Autostart` | Shortcut in the Startup folder |
| `-SilentNotifications` | Claude toasts: no banner, no sound, still kept in the notification center (the semaphore reads them there). The previous values are backed up and restored on uninstall. |
| `-IncludePhoneLink` | Same for Phone Link's mirror of the phone's Claude notifications |
| `-RemoveLegacy` | Stops and removes the earlier manual install in `%LOCALAPPDATA%\SemaforoClaude` |
| `-NoStart` | Do not start after installing |

Uninstall (removes only what the installer created, restores notification settings):

```powershell
pwsh -File "$env:LOCALAPPDATA\AgentSemaphore\uninstall.ps1"
```

Today this installs the PowerShell prototype; it will install the Go build once it ships.
`test-installer.ps1` is the CI end-to-end test — it changes registry settings and refuses to run
outside CI.
