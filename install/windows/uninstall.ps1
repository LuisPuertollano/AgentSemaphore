#Requires -Version 7
<#
.SYNOPSIS
  Removes AgentSemaphore and undoes only what install.ps1 changed.

.DESCRIPTION
  Stops the app, restores the notification settings saved by
  install.ps1 -SilentNotifications, removes our autostart shortcut(s) and the
  install folder. Refuses to delete a folder that has no install manifest.
  Use -WhatIf to see every change without making it.
#>
[CmdletBinding(SupportsShouldProcess)]
param(
  # Default: the folder this script runs from if it holds a manifest, else %LOCALAPPDATA%\AgentSemaphore.
  [string]$InstallDir
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'common.ps1')

if (-not $InstallDir) {
  $InstallDir = if (Test-Path -LiteralPath (Join-Path $PSScriptRoot $ManifestName)) { $PSScriptRoot }
                else { Join-Path $env:LOCALAPPDATA 'AgentSemaphore' }
}
$InstallDir = [IO.Path]::GetFullPath($InstallDir)
$manifest = Read-Manifest $InstallDir
if (-not $manifest) { throw "No $ManifestName in $InstallDir — nothing installed there by install.ps1, so nothing is removed." }

Stop-Semaphore $InstallDir
Stop-GoSemaphore $InstallDir
Remove-RunValue $InstallDir

# Notification settings back to what they were before the install.
$backupPath = Join-Path $InstallDir 'notification-backup.json'
if (Test-Path -LiteralPath $backupPath) {
  $backup = Get-Content -LiteralPath $backupPath -Raw | ConvertFrom-Json -AsHashtable
  foreach ($entry in @($backup.entries)) {
    if ($PSCmdlet.ShouldProcess($entry.key, 'Restore notification settings')) { Restore-NotificationValues $entry }
  }
}

# Our autostart shortcut, plus the one the prototype's own menu creates, but
# only if it points into this install folder.
$startup = [Environment]::GetFolderPath('Startup')
$shortcuts = @()
if ($manifest.shortcut) { $shortcuts += $manifest.shortcut }
$shortcuts += Join-Path $startup 'Semaforo Claude.lnk'
foreach ($lnk in ($shortcuts | Select-Object -Unique)) {
  if ((Test-Path -LiteralPath $lnk) -and (Test-ShortcutPointsTo $lnk $InstallDir)) {
    if ($PSCmdlet.ShouldProcess($lnk, 'Remove autostart shortcut')) { Remove-Item -LiteralPath $lnk }
  }
}

# The folder last; we may be running from inside it, so step out first.
Set-Location -LiteralPath $HOME
if ($PSCmdlet.ShouldProcess($InstallDir, 'Remove install folder')) {
  Remove-Item -LiteralPath $InstallDir -Recurse -Force
}
Write-Host 'AgentSemaphore removed.'
$data = Join-Path $env:APPDATA 'AgentSemaphore'
if (Test-Path -LiteralPath $data) { Write-Host "Your settings and log are kept in $data (delete the folder to remove them too)." }
