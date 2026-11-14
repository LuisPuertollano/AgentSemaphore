#Requires -Version 7
<#
.SYNOPSIS
  Installs AgentSemaphore for the current user (no admin rights needed).

.DESCRIPTION
  Copies the app to %LOCALAPPDATA%\AgentSemaphore, optionally adds an autostart
  shortcut and, only if asked, silences Claude's pop-ups while keeping them in
  the notification center (the semaphore reads them from there).

  Every change is recorded in install-manifest.json so uninstall.ps1 removes
  only what this script created and restores the notification settings that
  were there before. Use -WhatIf to see every change without making it.

  Today the payload is the PowerShell prototype (prototype/windows). When the
  Go build ships, the same script will install the binary instead.

.EXAMPLE
  ./install.ps1 -Autostart
.EXAMPLE
  ./install.ps1 -Autostart -SilentNotifications -WhatIf
#>
[CmdletBinding(SupportsShouldProcess)]
param(
  # Folder with the files to install (default: the prototype in this repo).
  [string]$Source = (Join-Path $PSScriptRoot '..\..\prototype\windows'),
  [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'AgentSemaphore'),
  # Start at sign-in (shortcut in the Startup folder).
  [switch]$Autostart,
  # Opt-in: no banner and no sound for Claude's toasts; they stay in the notification center.
  [switch]$SilentNotifications,
  # With -SilentNotifications: also silence Phone Link's mirror of the phone's Claude notifications.
  [switch]$IncludePhoneLink,
  # Stop and remove the earlier manual install in %LOCALAPPDATA%\SemaforoClaude.
  [switch]$RemoveLegacy,
  # Do not start the app after installing.
  [switch]$NoStart
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'common.ps1')

$Source = [IO.Path]::GetFullPath($Source)
$InstallDir = [IO.Path]::GetFullPath($InstallDir)
$payload = @('semaforo.ps1', 'avisos.py')
foreach ($f in $payload) {
  if (-not (Test-Path -LiteralPath (Join-Path $Source $f))) { throw "Missing $f in $Source" }
}

$manifest = Read-Manifest $InstallDir
if (-not $manifest) {
  $manifest = [ordered]@{ version = 1; installDir = $InstallDir; files = @(); shortcut = $null; notificationBackup = $null }
}

# --- Earlier manual install ------------------------------------------------
$legacyDir = Join-Path $env:LOCALAPPDATA 'SemaforoClaude'
if (Test-Path -LiteralPath $legacyDir) {
  if ($RemoveLegacy) {
    Stop-Semaphore $legacyDir
    $legacyLnk = Join-Path ([Environment]::GetFolderPath('Startup')) 'Semaforo Claude.lnk'
    if ((Test-Path -LiteralPath $legacyLnk) -and (Test-ShortcutPointsTo $legacyLnk $legacyDir)) {
      if ($PSCmdlet.ShouldProcess($legacyLnk, 'Remove legacy autostart shortcut')) { Remove-Item -LiteralPath $legacyLnk }
    }
    if ($PSCmdlet.ShouldProcess($legacyDir, 'Remove legacy install folder')) { Remove-Item -LiteralPath $legacyDir -Recurse -Force }
  } else {
    Write-Warning "An earlier install exists in $legacyDir. While it runs, the new one will not start (same single-instance lock). Re-run with -RemoveLegacy to remove it."
  }
}

# --- Files -------------------------------------------------------------------
Stop-Semaphore $InstallDir
if (-not (Test-Path -LiteralPath $InstallDir)) {
  if ($PSCmdlet.ShouldProcess($InstallDir, 'Create install folder')) { New-Item -ItemType Directory -Path $InstallDir | Out-Null }
}
$toCopy = @($payload | ForEach-Object { Join-Path $Source $_ }) + @(
  (Join-Path $PSScriptRoot 'uninstall.ps1'),
  (Join-Path $PSScriptRoot 'common.ps1'))
foreach ($src in $toCopy) {
  $name = Split-Path $src -Leaf
  if ($PSCmdlet.ShouldProcess((Join-Path $InstallDir $name), "Copy from $src")) {
    Copy-Item -LiteralPath $src -Destination $InstallDir -Force
  }
  if ($manifest.files -notcontains $name) { $manifest.files = @($manifest.files) + $name }
}

# --- Autostart -------------------------------------------------------------
$entry = Join-Path $InstallDir 'semaforo.ps1'
$launchArgs = "--headless pwsh.exe -NoProfile -ExecutionPolicy Bypass -File `"$entry`""
if ($Autostart) {
  $lnk = Join-Path ([Environment]::GetFolderPath('Startup')) 'AgentSemaphore.lnk'
  if ($PSCmdlet.ShouldProcess($lnk, 'Create autostart shortcut')) {
    $sh = New-Object -ComObject WScript.Shell
    $s = $sh.CreateShortcut($lnk)
    $s.TargetPath = Join-Path $env:windir 'System32\conhost.exe'
    $s.Arguments = $launchArgs
    $s.WorkingDirectory = $InstallDir
    $s.Description = 'AgentSemaphore'
    $s.Save()
    $manifest.shortcut = $lnk
  }
}

# --- Silent notifications (opt-in) -----------------------------------------
if ($SilentNotifications) {
  $aumids = @($ClaudeAumid)
  if ($IncludePhoneLink) { $aumids += $PhoneLinkClaudeAumid }
  $backupPath = Join-Path $InstallDir 'notification-backup.json'
  if (Test-Path -LiteralPath $backupPath) {
    # Never overwrite: the first backup holds the user's original settings.
    $backup = Get-Content -LiteralPath $backupPath -Raw | ConvertFrom-Json -AsHashtable
  } else {
    $backup = [ordered]@{ created = (Get-Date).ToString('o'); entries = @() }
  }
  foreach ($aumid in $aumids) {
    $key = "$NotifSettingsRoot\$aumid"
    if (-not ($backup.entries | Where-Object { $_.key -eq $key })) {
      $backup.entries = @($backup.entries) + (Get-NotificationValues $key)
    }
  }
  if ($PSCmdlet.ShouldProcess($backupPath, 'Save backup of the current notification settings')) {
    $backup | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $backupPath -Encoding utf8
    $manifest.notificationBackup = $backupPath
  }
  foreach ($aumid in $aumids) {
    $key = "$NotifSettingsRoot\$aumid"
    if ($PSCmdlet.ShouldProcess($key, 'No banner, no sound, keep in notification center')) {
      if (-not (Test-Path -LiteralPath $key)) { New-Item -Path $key -Force | Out-Null }
      New-ItemProperty -LiteralPath $key -Name ShowBanner -PropertyType DWord -Value 0 -Force | Out-Null
      New-ItemProperty -LiteralPath $key -Name SoundFile -PropertyType String -Value '' -Force | Out-Null
      # If 0, toasts are no longer stored and the semaphore goes blind.
      $sic = (Get-ItemProperty -LiteralPath $key -Name ShowInActionCenter -ErrorAction SilentlyContinue)
      if ($sic -and $sic.ShowInActionCenter -eq 0) {
        New-ItemProperty -LiteralPath $key -Name ShowInActionCenter -PropertyType DWord -Value 1 -Force | Out-Null
      }
    }
  }
}

if ($PSCmdlet.ShouldProcess((Join-Path $InstallDir $ManifestName), 'Write install manifest')) {
  Write-Manifest $InstallDir $manifest
}

# --- Checks and start --------------------------------------------------------
if (-not (Get-Command python -ErrorAction SilentlyContinue)) {
  Write-Warning 'Python is not on PATH: Claude chat/Cowork notifications will not be detected (Claude Code sessions still are).'
}
if (-not $NoStart) {
  if ($PSCmdlet.ShouldProcess($entry, 'Start AgentSemaphore')) {
    Start-Process -FilePath (Join-Path $env:windir 'System32\conhost.exe') -ArgumentList $launchArgs -WorkingDirectory $InstallDir
  }
}
Write-Host "AgentSemaphore installed in $InstallDir. To uninstall: pwsh -File `"$(Join-Path $InstallDir 'uninstall.ps1')`""
