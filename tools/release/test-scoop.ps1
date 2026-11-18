#Requires -Version 7
<#
  CI only: installs AgentSemaphore with Scoop from a manifest and checks it.
  -Manifest is a local file or URL; -Version is the tag it must report.
  Installs Scoop itself first if it is missing (CI runners run as admin,
  hence -RunAsAdmin). Refuses to run outside CI.
#>
param(
  [Parameter(Mandatory)][string]$Manifest,
  [Parameter(Mandatory)][string]$Version
)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if (-not $env:CI) { throw 'Refusing to run outside CI: this installs Scoop for the current user.' }
function Check([bool]$cond, [string]$what) { if (-not $cond) { throw "FAILED: $what" }; Write-Host "ok  $what" }

if (-not (Get-Command scoop -ErrorAction SilentlyContinue)) {
  Invoke-Expression "& {$(Invoke-RestMethod https://get.scoop.sh)} -RunAsAdmin"
  $env:PATH = "$env:USERPROFILE\scoop\shims;$env:PATH"
}
scoop install $Manifest
Check ($LASTEXITCODE -eq 0) "scoop install $Manifest"
$got = (& agentsemaphore -version | Out-String).Trim()
Check ($got -eq "agentsemaphore $Version") "the 'agentsemaphore' command runs the console build ($got)"
$app = Join-Path $env:USERPROFILE 'scoop\apps\agentsemaphore\current'
Check (Test-Path (Join-Path $app 'agentsemaphore.exe')) 'the tray program is installed'
$lnk = Join-Path ([Environment]::GetFolderPath('StartMenu')) 'Programs\Scoop Apps\AgentSemaphore.lnk'
Check (Test-Path $lnk) 'Start menu shortcut created'
scoop uninstall agentsemaphore
Check (-not (Test-Path $app)) 'scoop uninstall removes it'
Write-Host 'All Scoop checks passed.'
