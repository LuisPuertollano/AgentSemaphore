<#
.SYNOPSIS
  Read-only probe: prints every change in Claude Code's session files.

.DESCRIPTION
  Watches %USERPROFILE%\.claude\sessions\*.json and prints one line each time a
  session appears, disappears or changes status / waitingFor. Also appends the
  lines to a log in %TEMP%. Never writes to Claude's files. Stop with Ctrl+C.
  Works in Windows PowerShell 5.1 and PowerShell 7.
#>
param([int]$IntervalMs = 250)

$dir = Join-Path $env:USERPROFILE '.claude\sessions'
$log = Join-Path $env:TEMP 'agentsemaphore-probe.log'
$before = @{}

function Out-Line([string]$s) {
  $line = '{0:HH:mm:ss.fff}  {1}' -f (Get-Date), $s
  Write-Host $line
  Add-Content -LiteralPath $log -Value $line -Encoding UTF8
}
function Get-Field($o, [string]$n) {
  $p = $o.PSObject.Properties[$n]
  if ($p) { $p.Value } else { $null }
}

Out-Line "probe started; watching $dir; log: $log"
while ($true) {
  $now = @{}
  foreach ($f in Get-ChildItem -LiteralPath $dir -Filter *.json -File -ErrorAction SilentlyContinue) {
    try { $j = [IO.File]::ReadAllText($f.FullName) | ConvertFrom-Json } catch { $now[$f.Name] = 'UNREADABLE'; continue }
    $sesPid = Get-Field $j 'pid'
    $p = $null
    if ($sesPid) { $p = Get-Process -Id $sesPid -ErrorAction SilentlyContinue }
    $alive = if ($p) { $p.ProcessName } else { 'dead' }
    $extra = @()
    foreach ($n in 'procStart', 'pidDomain') { if ($null -ne (Get-Field $j $n)) { $extra += $n } }
    $now[$f.Name] = 'name="{0}" status={1} waitingFor="{2}" entrypoint={3} kind={4} proc={5} fields+={6}' -f `
      (Get-Field $j 'name'), (Get-Field $j 'status'), (Get-Field $j 'waitingFor'), (Get-Field $j 'entrypoint'),
      (Get-Field $j 'kind'), $alive, ($extra -join ',')
  }
  foreach ($k in $now.Keys) {
    if (-not $before.ContainsKey($k)) { Out-Line "NEW     $k  $($now[$k])" }
    elseif ($before[$k] -ne $now[$k]) { Out-Line "CHANGE  $k  $($now[$k])" }
  }
  foreach ($k in $before.Keys) { if (-not $now.ContainsKey($k)) { Out-Line "GONE    $k" } }
  $before = $now
  Start-Sleep -Milliseconds $IntervalMs
}
