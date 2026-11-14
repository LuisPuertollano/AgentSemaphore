# Windows prototype

Requirements: Windows 10/11, PowerShell 7 (`pwsh`), Python 3 on `PATH` (only for the
Claude Desktop chat/Cowork source). Tested on Windows 11 ARM64.

## Install (manual, for now)

```powershell
$dir = "$env:LOCALAPPDATA\SemaforoClaude"
New-Item -ItemType Directory $dir -Force | Out-Null
Copy-Item .\semaforo.ps1, .\estado.ps1, .\avisos.py, .\avisos-textos.json $dir
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
- `estado.ps1` — the logic without UI: colours, session files, parsing `avisos.py` output (tested in CI).
- `avisos.py` — reads a copy of the Windows notification DB and returns Claude toasts that may need you.
- `avisos-textos.json` — known toast phrases per language (`pide` = needs you, `ignorar` = does not).

Grey dot = a source cannot be read (Python missing, notification DB format changed, unreadable or
unknown session file). Priority: red > grey > orange > green. The tooltip says which source.
- `cambios.log` (created at runtime) — one line per colour change with the reason.

## Uninstall

Tray menu → **Salir**, then delete `%LOCALAPPDATA%\SemaforoClaude` and
`shell:startup\Semaforo Claude.lnk`.
