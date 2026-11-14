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
foreach ($f in 'semaforo.ps1', 'avisos.py', 'uninstall.ps1', 'common.ps1', $ManifestName, 'notification-backup.json') {
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

Write-Host 'All installer checks passed.'
