#Requires -Version 7
<#
  End-to-end test of the Go core on a real Windows desktop (CI runners,
  windows-11-arm and windows-latest). No Claude installation is needed:
  - fake "claude" processes (a renamed system program) and fake session files
    drive the Claude Code source through every colour;
  - the tray binary is started twice to check the single-instance lock;
  - a screenshot of the desktop is saved for review (not an assertion).
  It writes to the runner user's profile, so it refuses to run outside CI.
#>
param([Parameter(Mandatory)][string]$Exe, [string]$OutDir = (Join-Path $env:RUNNER_TEMP 'e2e'))
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if (-not $env:CI) { throw 'Refusing to run outside CI: this test writes to your profile.' }
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

function Check([bool]$cond, [string]$what) { if (-not $cond) { throw "FAILED: $what" }; Write-Host "ok  $what" }
function Once {
  $json = & $Exe -once | Out-String
  if ($LASTEXITCODE -ne 0) { throw "agentsemaphore -once exited with $LASTEXITCODE" }
  $json | ConvertFrom-Json
}

# --- fake Claude Code sessions ---------------------------------------------------
$sesDir = Join-Path $env:USERPROFILE '.claude\sessions'
New-Item -ItemType Directory -Force -Path $sesDir | Out-Null
$fakeDir = Join-Path $OutDir 'fake'
New-Item -ItemType Directory -Force -Path $fakeDir | Out-Null
$fakeExe = Join-Path $fakeDir 'claude.exe'
Copy-Item (Join-Path $env:windir 'System32\PING.EXE') $fakeExe
$procs = 1..3 | ForEach-Object { Start-Process $fakeExe -ArgumentList '-n 600 127.0.0.1' -WindowStyle Hidden -PassThru }
function Write-Session($p, [string]$name, [string]$status, [string]$waitingFor = '') {
  $o = [ordered]@{ pid = $p.Id; name = $name; status = $status; kind = 'interactive' }
  if ($waitingFor) { $o.waitingFor = $waitingFor }
  $o | ConvertTo-Json | Set-Content (Join-Path $sesDir "$($p.Id).json") -Encoding utf8
}

try {
  $s = Once
  Check ($s.state -eq 'green') 'no sessions -> green'
  Check (($s.menu -join "`n") -match 'Claude Code') 'menu names the Claude Code source'

  Write-Session $procs[0] 'e2e-wait' 'waiting' 'permission prompt'
  Write-Session $procs[1] 'e2e-busy' 'busy'
  Write-Session $procs[2] 'e2e-idle' 'idle'
  $s = Once
  Check ($s.state -eq 'red') 'waiting session -> red'
  Check (@($s.sessions).Count -eq 3) 'three live sessions seen (process name check works on this OS)'
  Check (($s.menu -join "`n") -match 'e2e-wait.*permission prompt') 'menu shows the waiting session and why'

  Write-Session $procs[0] 'e2e-wait' 'idle'
  Check ((Once).state -eq 'orange') 'busy session -> orange'

  Write-Session $procs[1] 'e2e-busy' 'dreaming'
  Check ((Once).state -eq 'grey') 'unknown status -> grey'

  Stop-Process -Id $procs[1].Id -Force
  Start-Sleep -Milliseconds 500
  Check ((Once).state -eq 'green') 'session of an exited process is ignored -> green'

  # --- tray: single instance and screenshot -----------------------------------------
  $log = Join-Path $env:APPDATA 'AgentSemaphore\agentsemaphore.log'
  Write-Session $procs[0] 'e2e-wait' 'waiting' 'permission prompt'
  $tray = Start-Process $Exe -PassThru
  Start-Sleep -Seconds 4
  Check (-not $tray.HasExited) 'tray app keeps running'
  $second = Start-Process $Exe -PassThru
  Check ($second.WaitForExit(10000)) 'second copy exits on its own (single instance)'
  Check ((Get-Content $log -Raw) -match 'already running') 'second copy logged "already running"'

  try {
    Add-Type -AssemblyName System.Windows.Forms, System.Drawing
    $b = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
    $bmp = [System.Drawing.Bitmap]::new($b.Width, $b.Height)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.CopyFromScreen($b.Location, [System.Drawing.Point]::Empty, $b.Size)
    $bmp.Save((Join-Path $OutDir 'desktop.png'), [System.Drawing.Imaging.ImageFormat]::Png)
    Write-Host "screenshot saved ($($b.Width)x$($b.Height))"
  } catch { Write-Warning "no screenshot: $($_.Exception.Message)" }

  Stop-Process -Id $tray.Id -Force
  Copy-Item $log $OutDir -ErrorAction SilentlyContinue
  Write-Host 'All Windows end-to-end checks passed.'
} finally {
  $procs | ForEach-Object { Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue }
  Get-ChildItem $sesDir -Filter *.json | Remove-Item -Force
}
