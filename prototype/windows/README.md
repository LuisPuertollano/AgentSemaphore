# Windows prototype

Requirements: Windows 10/11, PowerShell 7 (`pwsh`), Python 3 on `PATH` (only for the
Claude Desktop chat/Cowork source). Tested on Windows 11 ARM64.

## Install (manual, for now)

```powershell
$dir = "$env:LOCALAPPDATA\SemaforoClaude"
New-Item -ItemType Directory $dir -Force | Out-Null
Copy-Item .\semaforo.ps1, .\avisos.py $dir
# start now, with no console window
Start-Process conhost.exe "--headless pwsh.exe -NoProfile -ExecutionPolicy Bypass -File `"$dir\semaforo.ps1`""
```

Right-click the tray dot → **Arrancar con Windows** to start it at sign-in.
On Windows 11 the new icon may be hidden behind the **^** arrow: drag it to the taskbar.

## Optional: silence Claude's pop-ups but keep the semaphore working

The semaphore reads Claude Desktop's notifications from the notification center, so they must
keep arriving there. This turns off only the pop-up and the sound:

```powershell
$k = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Notifications\Settings\Claude_pzs8sxrjxfjjc!Claude'
New-ItemProperty $k ShowBanner -PropertyType DWord -Value 0 -Force
New-ItemProperty $k SoundFile  -PropertyType String -Value '' -Force
```

Undo: `Remove-ItemProperty $k ShowBanner, SoundFile`.

## Files

- `semaforo.ps1` — tray app (WinForms NotifyIcon), polls once per second.
- `avisos.py` — reads a copy of the Windows notification DB and returns Claude toasts that ask for you.
- `cambios.log` (created at runtime) — one line per colour change with the reason.

## Uninstall

Tray menu → **Salir**, then delete `%LOCALAPPDATA%\SemaforoClaude` and
`shell:startup\Semaforo Claude.lnk`.
