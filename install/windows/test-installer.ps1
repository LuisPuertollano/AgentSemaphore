#Requires -Version 7
<#
  End-to-end test of install.ps1 / uninstall.ps1 for CI (GitHub windows runners).
  It changes the current user's registry and Startup folder, so run it only on a
  throwaway machine. Throws on the first failed check.
#>
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'common.ps1')

if (-not $env:CI) { throw 'Refusing to run outside CI: this test changes your notification settings.' }

function Check([bool]$cond, [string]$what) {
  if (-not $cond) { throw "FAILED: $what" }
  Write-Host "ok  $what"
}
function Get-Val([string]$key, [string]$name) {
  (Get-ItemProperty -LiteralPath $key -Name $name -ErrorAction SilentlyContinue)?.$name
}
function Has-Val([string]$key, [string]$name) {
  (Test-Path -LiteralPath $key) -and ((Get-Item -LiteralPath $key).GetValueNames() -contains $name)
}

$dir = Join-Path $env:RUNNER_TEMP 'AgentSemaphore-test'
$key = "$NotifSettingsRoot\$ClaudeAumid"
$lnk = Join-Path ([Environment]::GetFolderPath('Startup')) 'AgentSemaphore.lnk'
$install = Join-Path $PSScriptRoot 'install.ps1'

# The user's original settings: banner on (1), no custom sound value.
New-Item -Path $key -Force | Out-Null
New-ItemProperty -LiteralPath $key -Name ShowBanner -PropertyType DWord -Value 1 -Force | Out-Null
Remove-ItemProperty -LiteralPath $key -Name SoundFile -ErrorAction SilentlyContinue

# 1. -WhatIf changes nothing.
& $install -InstallDir $dir -Autostart -SilentNotifications -NoStart -WhatIf
Check (-not (Test-Path $dir)) 'WhatIf: no install folder'
Check (-not (Test-Path $lnk)) 'WhatIf: no shortcut'
Check ((Get-Val $key ShowBanner) -eq 1) 'WhatIf: ShowBanner untouched'

# 2. Real install.
& $install -InstallDir $dir -Autostart -SilentNotifications -NoStart
foreach ($f in 'semaforo.ps1', 'estado.ps1', 'avisos.py', 'avisos-textos.json', 'uninstall.ps1', 'common.ps1', $ManifestName, 'notification-backup.json') {
  Check (Test-Path (Join-Path $dir $f)) "installed $f"
}
Check (Test-Path $lnk) 'autostart shortcut created'
Check ((Get-Val $key ShowBanner) -eq 0) 'ShowBanner = 0'
Check ((Has-Val $key SoundFile) -and (Get-Val $key SoundFile) -eq '') 'SoundFile = empty'

# 3. Re-install must keep the ORIGINAL backup, not back up our own values.
& $install -InstallDir $dir -Autostart -SilentNotifications -NoStart
$backup = Get-Content (Join-Path $dir 'notification-backup.json') -Raw | ConvertFrom-Json -AsHashtable
$saved = @($backup.entries | Where-Object { $_.key -eq $key })
Check ($saved.Count -eq 1) 'one backup entry for the Claude key'
Check ($saved[0].items.ShowBanner.existed -and $saved[0].items.ShowBanner.data -eq 1) 'backup still holds ShowBanner = 1'
Check (-not $saved[0].items.SoundFile.existed) 'backup still says SoundFile did not exist'

# 4. Uninstall from the install folder restores everything.
& (Join-Path $dir 'uninstall.ps1')
Check (-not (Test-Path $dir)) 'install folder removed'
Check (-not (Test-Path $lnk)) 'shortcut removed'
Check ((Get-Val $key ShowBanner) -eq 1) 'ShowBanner restored to 1'
Check (-not (Has-Val $key SoundFile)) 'SoundFile removed again'

# 5. Uninstall refuses a folder it did not create.
$foreign = Join-Path $env:RUNNER_TEMP 'not-ours'
New-Item -ItemType Directory $foreign -Force | Out-Null
$refused = $false
try { & (Join-Path $PSScriptRoot 'uninstall.ps1') -InstallDir $foreign } catch { $refused = $true }
Check ($refused -and (Test-Path $foreign)) 'uninstall refuses a folder without manifest'

# 6. Go build: switch from the prototype, autostart via HKCU Run, uninstall.
if (-not $env:AS_TEST_EXE) { throw 'AS_TEST_EXE (path to a built agentsemaphore.exe) is required' }
$runKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
Remove-ItemProperty -LiteralPath $runKey -Name AgentSemaphore -ErrorAction SilentlyContinue
& $install -InstallDir $dir -Autostart -NoStart                       # prototype first
Check (Test-Path $lnk) 'prototype installed with its shortcut'
if (-not $env:AS_TEST_CLI) { throw 'AS_TEST_CLI (path to a console build) is required' }
& $install -InstallDir $dir -Exe $env:AS_TEST_EXE -CliExe $env:AS_TEST_CLI -Autostart -NoStart  # then the Go build
Check (Test-Path (Join-Path $dir 'agentsemaphore.exe')) 'Go build installed'
Check (Test-Path (Join-Path $dir 'agentsemaphore-cli.exe')) 'console build installed next to it'
$hookShow = & (Join-Path $dir 'agentsemaphore-cli.exe') hooks show copilot | Out-String
Check ($hookShow -match [regex]::Escape((Join-Path $dir 'agentsemaphore-cli.exe').Replace('\', '\\'))) 'hooks point to the console build'
Check (-not (Test-Path (Join-Path $dir 'semaforo.ps1'))) 'prototype files removed when switching'
Check (-not (Test-Path $lnk)) 'prototype shortcut removed when switching'
$run = (Get-ItemProperty -LiteralPath $runKey -Name AgentSemaphore -ErrorAction SilentlyContinue)?.AgentSemaphore
Check ($run -eq ('"' + (Join-Path $dir 'agentsemaphore.exe') + '"')) 'HKCU Run entry points to the installed exe'
$m = Get-Content (Join-Path $dir $ManifestName) -Raw | ConvertFrom-Json
Check ($m.mode -eq 'go' -and $m.runValue) 'manifest records the Go mode and the Run entry'
& $install -InstallDir $dir -Autostart -NoStart   # back to the prototype: both Go programs go
Check (-not (Test-Path (Join-Path $dir 'agentsemaphore-cli.exe'))) 'switching to the prototype removes the console build too'
& $install -InstallDir $dir -Exe $env:AS_TEST_EXE -Autostart -NoStart

# Re-install while it runs: the running copy is stopped first, then replaced.
$proc = Start-Process (Join-Path $dir 'agentsemaphore.exe') -PassThru
Start-Sleep -Seconds 3
Check (-not $proc.HasExited) 'installed Go build runs'
& $install -InstallDir $dir -Exe $env:AS_TEST_EXE -Autostart -NoStart
Check ($proc.HasExited) 're-install stopped the running copy'

# A Run entry that points elsewhere is not ours: uninstall must leave it alone.
$proc = Start-Process (Join-Path $dir 'agentsemaphore.exe') -PassThru
Start-Sleep -Seconds 3
& (Join-Path $dir 'uninstall.ps1')
Check ($proc.HasExited) 'uninstall stopped the Go build'
Check (-not (Test-Path $dir)) 'install folder removed'
Check ((Get-Item -LiteralPath $runKey).GetValueNames() -notcontains 'AgentSemaphore') 'Run entry removed'
New-ItemProperty -LiteralPath $runKey -Name AgentSemaphore -Value '"C:\elsewhere\agentsemaphore.exe"' -PropertyType String -Force | Out-Null
& $install -InstallDir $dir -Exe $env:AS_TEST_EXE -NoStart
& (Join-Path $dir 'uninstall.ps1')
Check ((Get-ItemProperty -LiteralPath $runKey -Name AgentSemaphore).AgentSemaphore -eq '"C:\elsewhere\agentsemaphore.exe"') 'a Run entry pointing elsewhere is left alone'
Remove-ItemProperty -LiteralPath $runKey -Name AgentSemaphore

# 7. Finding the newest release: through the API, and through the releases page
#    when the API does not answer (anonymous API calls are rate-limited per IP).
$tagPattern = '^v\d+\.\d+\.\d+(-[0-9A-Za-z.]+)?$'
$rel = Get-Release ''
Check ($rel.tag_name -match $tagPattern) "newest release through the API: $($rel.tag_name)"
$env:AGENTSEMAPHORE_RELEASES_API = 'https://api.github.com/repos/LuisPuertollano/AgentSemaphore/no-such-endpoint'
try {
  $fallback = Get-Release ''
  Check ($fallback.tag_name -eq $rel.tag_name) "same release found on the releases page: $($fallback.tag_name)"
  $dl = Join-Path $env:RUNNER_TEMP 'fallback-download.exe'
  Save-ReleaseAsset $fallback 'agentsemaphore-windows-amd64.exe' $dl
  Check (Test-Path $dl) 'download through the page fallback passes the SHA-256 check'
} finally {
  Remove-Item Env:AGENTSEMAPHORE_RELEASES_API
}

Write-Host 'All installer checks passed.'
