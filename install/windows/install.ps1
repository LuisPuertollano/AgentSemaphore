#Requires -Version 7
<#
.SYNOPSIS
  Installs AgentSemaphore for the current user (no admin rights needed).

.DESCRIPTION
  Two payloads, same folder (%LOCALAPPDATA%\AgentSemaphore):
  - the Go build (recommended): -Exe <path to agentsemaphore.exe>, or
    -FromRelease to download the newest release for this machine (x64/ARM64;
    pre-releases included while there is no stable one), checked against
    the release's SHA256SUMS.txt;
  - the PowerShell prototype (default when neither is given).
  Installing one removes what the other left in that folder, so switching is clean.

  Optional: start at sign-in (-Autostart) and, only if asked, silence Claude's
  pop-ups while keeping them in the notification center (-SilentNotifications;
  the semaphore reads them from there).

  Every change is recorded in install-manifest.json so uninstall.ps1 removes
  only what this script created and restores the notification settings that
  were there before. Use -WhatIf to see every change without making it.

.EXAMPLE
  ./install.ps1 -Exe .\agentsemaphore-windows-arm64.exe -Autostart
.EXAMPLE
  ./install.ps1 -FromRelease -Autostart -WhatIf
.EXAMPLE
  ./install.ps1 -FromRelease -Version v0.1.0-alpha.1 -Autostart
.EXAMPLE
  ./install.ps1 -Autostart                      # the PowerShell prototype
#>
[CmdletBinding(SupportsShouldProcess)]
param(
  # Go build: path to a downloaded agentsemaphore(-windows-<arch>).exe.
  [string]$Exe,
  # Go build: download the newest GitHub release (pre-releases included) for this machine's architecture.
  [switch]$FromRelease,
  # With -FromRelease: this release tag instead of the newest, e.g. v0.1.0-alpha.1.
  [string]$Version,
  # Prototype: folder with its files (default: the prototype in this repo).
  [string]$Source = (Join-Path $PSScriptRoot '..\..\prototype\windows'),
  [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'AgentSemaphore'),
  # Start at sign-in (Go: HKCU Run entry; prototype: shortcut in the Startup folder).
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

if ($Version -and -not $FromRelease) { throw '-Version only works together with -FromRelease.' }
if ($Version -and $Version -notmatch '^v\d+\.\d+\.\d+(-[0-9A-Za-z.]+)?$') { throw "Not a release tag: $Version (expected e.g. v0.1.0-alpha.1)" }
$mode = if ($Exe -or $FromRelease) { 'go' } else { 'prototype' }
$InstallDir = [IO.Path]::GetFullPath($InstallDir)

# --- What to install -------------------------------------------------------------
if ($mode -eq 'go') {
  if ($FromRelease) {
    $assetName = "agentsemaphore-windows-$(Get-GoArch).exe"
    $Exe = Join-Path ([IO.Path]::GetTempPath()) $assetName
    $what = if ($Version) { "release $Version" } else { 'the newest release' }
    if ($PSCmdlet.ShouldProcess("$assetName from $what", 'Download and check SHA-256')) {
      $rel = Get-Release $Version
      Save-ReleaseAsset $rel $assetName $Exe
      Write-Host "Downloaded $assetName from $($rel.tag_name) (SHA-256 checked)."
    }
  }
  if (-not $WhatIfPreference -and -not (Test-Path -LiteralPath $Exe)) { throw "Not found: $Exe" }
  $Exe = [IO.Path]::GetFullPath($Exe)
  $payload = @()
} else {
  $Source = [IO.Path]::GetFullPath($Source)
  $payload = @('semaforo.ps1', 'estado.ps1', 'avisos.py', 'avisos-textos.json')
  foreach ($f in $payload) {
    if (-not (Test-Path -LiteralPath (Join-Path $Source $f))) { throw "Missing $f in $Source" }
  }
}

$manifest = Read-Manifest $InstallDir
if (-not $manifest) {
  $manifest = [ordered]@{ version = 1; installDir = $InstallDir; files = @(); shortcut = $null; notificationBackup = $null }
}
foreach ($k in 'mode', 'runValue') { if (-not $manifest.Contains($k)) { $manifest[$k] = $null } }

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
  } elseif ($mode -eq 'prototype') {
    Write-Warning "An earlier install exists in $legacyDir. While it runs, the new one will not start (same single-instance lock). Re-run with -RemoveLegacy to remove it."
  }
}

# --- Stop what runs from the folder, remove the other mode's leftovers ----------
Stop-Semaphore $InstallDir
Stop-GoSemaphore $InstallDir
if ($mode -eq 'go') {
  foreach ($f in $PrototypeFiles) {
    $p = Join-Path $InstallDir $f
    if (Test-Path -LiteralPath $p) {
      if ($PSCmdlet.ShouldProcess($p, 'Remove prototype file')) { Remove-Item -LiteralPath $p -Force }
    }
  }
  $manifest.files = @($manifest.files | Where-Object { $_ -notin $PrototypeFiles })
  Remove-PrototypeShortcuts $InstallDir -extra @($manifest.shortcut)
  $manifest.shortcut = $null
} else {
  $goExe = Join-Path $InstallDir $GoExeName
  if (Test-Path -LiteralPath $goExe) {
    if ($PSCmdlet.ShouldProcess($goExe, 'Remove Go build')) { Remove-Item -LiteralPath $goExe -Force }
  }
  $manifest.files = @($manifest.files | Where-Object { $_ -ne $GoExeName })
  Remove-RunValue $InstallDir
  $manifest.runValue = $null
}
$manifest.mode = $mode

# --- Files -------------------------------------------------------------------
if (-not (Test-Path -LiteralPath $InstallDir)) {
  if ($PSCmdlet.ShouldProcess($InstallDir, 'Create install folder')) { New-Item -ItemType Directory -Path $InstallDir | Out-Null }
}
$copies = [ordered]@{}
foreach ($f in $payload) { $copies[$f] = Join-Path $Source $f }
if ($mode -eq 'go') { $copies[$GoExeName] = $Exe }
$copies['uninstall.ps1'] = Join-Path $PSScriptRoot 'uninstall.ps1'
$copies['common.ps1'] = Join-Path $PSScriptRoot 'common.ps1'
foreach ($name in $copies.Keys) {
  $dst = Join-Path $InstallDir $name
  if ($PSCmdlet.ShouldProcess($dst, "Copy from $($copies[$name])")) {
    Copy-Item -LiteralPath $copies[$name] -Destination $dst -Force
  }
  if ($manifest.files -notcontains $name) { $manifest.files = @($manifest.files) + $name }
}

# --- Autostart -------------------------------------------------------------
if ($mode -eq 'go') {
  $entry = Join-Path $InstallDir $GoExeName
  if ($Autostart -and $PSCmdlet.ShouldProcess("$RunKey\$RunName", "Start $entry at sign-in")) {
    if (-not (Test-Path -LiteralPath $RunKey)) { New-Item -Path $RunKey -Force | Out-Null }
    New-ItemProperty -LiteralPath $RunKey -Name $RunName -PropertyType String -Value "`"$entry`"" -Force | Out-Null
    $manifest.runValue = $true
  }
} else {
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
if ($mode -eq 'prototype' -and -not (Get-Command python -ErrorAction SilentlyContinue)) {
  Write-Warning 'Python is not on PATH: Claude chat/Cowork notifications will not be detected (Claude Code sessions still are).'
}
$other = Get-RunValue
if ($mode -eq 'go' -and -not $Autostart -and $other -and -not $other.Contains($InstallDir, [StringComparison]::OrdinalIgnoreCase)) {
  Write-Warning "Another AgentSemaphore starts at sign-in from: $other. Re-run with -Autostart to point it to this install."
}
if (-not $NoStart) {
  if ($PSCmdlet.ShouldProcess($entry, 'Start AgentSemaphore')) {
    if ($mode -eq 'go') { Start-Process -FilePath $entry -WorkingDirectory $InstallDir }
    else { Start-Process -FilePath (Join-Path $env:windir 'System32\conhost.exe') -ArgumentList $launchArgs -WorkingDirectory $InstallDir }
  }
}
if (-not $WhatIfPreference) {
  Write-Host "AgentSemaphore ($mode) installed in $InstallDir. To uninstall: pwsh -File `"$(Join-Path $InstallDir 'uninstall.ps1')`""
}
