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
param(
  [Parameter(Mandatory)][string]$Exe,          # console build: used for -once
  [string]$TrayExe = $Exe,                       # GUI build (no console window) for the tray checks
  [string]$FakeAgent,                            # tools/e2e/fakeagent build: runs a hook as an agent would
  [string]$OutDir = (Join-Path $env:RUNNER_TEMP 'e2e')
)
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

  # --- other agents through their hooks ----------------------------------------------
  # Copilot runs a hook's "powershell" command through PowerShell, so the chain is
  # agent -> pwsh -> agentsemaphore. Hooks run the console build (agentsemaphore-cli.exe):
  # PowerShell does not wait for the GUI build (measured here, see docs/FINDINGS.md).
  if ($FakeAgent) {
    $copilot = Join-Path $fakeDir 'copilot.exe'; Copy-Item $FakeAgent $copilot
    $codex = Join-Path $fakeDir 'codex.exe'; Copy-Item $FakeAgent $codex
    $pwshExe = (Get-Process -Id $PID).Path
    $hookProcs = [System.Collections.Generic.List[object]]::new()
    function Start-Hook([string]$agentExe, [string]$agent, [string]$event, [string]$json) {
      $cmd = "'$json' | & '$Exe' hook $agent $event"
      $b64 = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($cmd))
      # Start-Process joins -ArgumentList with spaces and does not quote: quote the pwsh path
      # ("C:\Program Files\...") or the fake agent gets it split in two.
      $p = Start-Process $agentExe -ArgumentList @("`"$pwshExe`"", '-NoProfile', '-EncodedCommand', $b64) -WindowStyle Hidden -PassThru
      $hookProcs.Add($p)
      $p
    }
    function Menu { (Once).menu -join "`n" }
    $hookDir = Join-Path $env:APPDATA 'AgentSemaphore\hooks'
    $hookLog = Join-Path $env:APPDATA 'AgentSemaphore\hooks.log'
    function Show-HookState([string]$why) {
      Write-Host "--- $why"
      $names = Get-ChildItem $hookDir -Filter *.json -ErrorAction SilentlyContinue | ForEach-Object { $_.Name }
      Write-Host "hook files: $($names -join ', ')"
      if (Test-Path $hookLog) { Write-Host "hooks.log:"; Get-Content $hookLog -Tail 20 | ForEach-Object { Write-Host "  $_" } }
    }


    $h1 = Start-Hook $copilot 'copilot' 'notification' '{"sessionId":"cp1","cwd":"C:\\work\\site","notification_type":"permission_prompt"}'
    $h2 = Start-Hook $codex 'codex' 'UserPromptSubmit' '{"session_id":"cx1","cwd":"C:\\work\\api"}'
    Start-Sleep -Seconds 4
    $s = Once
    if ($s.state -ne 'red') { $s | ConvertTo-Json -Depth 5 | Write-Host; Show-HookState 'red expected' }
    Check ($s.state -eq 'red') 'Copilot permission prompt (hook via PowerShell) -> red'
    Check ((Menu) -match 'Copilot . site.*permission prompt') 'menu shows the Copilot session and why'
    Check ((Menu) -match 'Codex . api') 'menu shows the Codex session'

    $h3 = Start-Hook $copilot 'copilot' 'agentStop' '{"sessionId":"cp1","cwd":"C:\\work\\site"}'
    Start-Sleep -Seconds 4
    Check ((Once).state -eq 'orange') 'Copilot turn done -> green, Codex still working -> orange'

    Stop-Process -Id $h1.Id, $h3.Id -Force
    Start-Sleep -Milliseconds 800
    Check (-not ((Menu) -match 'Copilot . site')) 'a session whose agent process exited is dropped'

    $null = Start-Hook $codex 'codex' 'SessionEnd' '{"session_id":"cx1","cwd":"C:\\work\\api"}'
    Start-Sleep -Seconds 4
    Check ((Once).state -eq 'green') 'Codex session end -> session removed -> green'

    $out = '{"session_id":"g1"}' | & $Exe hook gemini SessionStart
    Check ("$out".Trim() -eq '{}') "Gemini hook prints {} (got '$out')"
    $null = 'garbage' | & $Exe hook codex Stop
    Check ($LASTEXITCODE -eq 0) 'bad hook input: exit 0'

    $hookFile = Join-Path $env:USERPROFILE '.copilot\hooks\agentsemaphore.json'
    & $Exe hooks install copilot | Out-Null
    Check (Test-Path $hookFile) 'hooks install copilot writes our own file'
    $ps = ((Get-Content $hookFile -Raw | ConvertFrom-Json).hooks.notification)[0].powershell
    Check ($ps -like "& `"$TrayExe`" hook copilot notification" -or $ps -like '& "*" hook copilot notification') "the Copilot hook has a PowerShell command ($ps)"
    & $Exe hooks uninstall copilot | Out-Null
    Check (-not (Test-Path $hookFile)) 'hooks uninstall copilot removes it'
    $hookProcs | ForEach-Object { Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue }
  }

  # --- plan usage through Claude Code's status line ------------------------------------
  $claudeSettings = Join-Path $env:USERPROFILE '.claude\settings.json'
  $hadSettings = Test-Path $claudeSettings
  if ($hadSettings) { $savedSettings = Get-Content $claudeSettings -Raw }
  try {
    New-Item -ItemType Directory -Force (Split-Path $claudeSettings) | Out-Null
    Set-Content $claudeSettings '{"model": "opus", "statusLine": {"type": "command", "command": "echo MY-LINE", "padding": 1}}' -Encoding utf8
    & $Exe usage install claude | Out-Null
    Check ((Get-Content $claudeSettings -Raw) -match '\\" statusline') 'usage install claude: status line points to agentsemaphore'
    $now = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
    $status = '{"rate_limits":{"five_hour":{"used_percentage":23.5,"resets_at":' + ($now + 9000) + '},"seven_day":{"used_percentage":41.2,"resets_at":' + ($now + 259200) + '}}}'
    $line = ($status | & $Exe statusline | Out-String).Trim()
    Check ($line -match 'MY-LINE') "your own status line is still shown (got '$line')"
    $m = Menu
    Check ($m -match 'Plan usage' -and $m -match '5 h 24 %.*week 41 %') 'menu shows the plan usage'
    # Our limit hooks, run the way Claude Code does (exec form: command + args, event JSON on stdin).
    $cfg = Get-Content $claudeSettings -Raw | ConvertFrom-Json
    $stopHook = @($cfg.hooks.StopFailure | ForEach-Object { $_.hooks[0] } | Where-Object { $_.args -and $_.args[0] -eq 'usage-event' })
    Check ($stopHook.Count -eq 1) 'usage install claude adds our StopFailure hook (exec form)'
    $out = '{"hook_event_name":"StopFailure","error":"rate_limit"}' | & $stopHook[0].command @($stopHook[0].args)
    Check ($LASTEXITCODE -eq 0 -and -not $out) 'the StopFailure hook prints nothing and exits 0'
    Check ((Menu) -match 'limit reached at') 'menu shows "limit reached" after a StopFailure rate_limit hook'
    '{"hook_event_name":"Notification","notification_type":"quota_auto_resume_fired"}' | & $stopHook[0].command @($stopHook[0].args) | Out-Null
    Check (-not ((Menu) -match 'limit reached')) '"limit reached" goes when Claude Code reports the reset'
    & $Exe usage uninstall claude | Out-Null
    $restored = Get-Content $claudeSettings -Raw | ConvertFrom-Json
    Check ($restored.statusLine.command -eq 'echo MY-LINE' -and $restored.statusLine.padding -eq 1 -and $restored.model -eq 'opus' -and -not $restored.hooks) 'usage uninstall restores the status line and removes our hooks exactly'
  } finally {
    if ($hadSettings) { Set-Content $claudeSettings $savedSettings -NoNewline } else { Remove-Item $claudeSettings -ErrorAction SilentlyContinue }
  }

  # --- tray: single instance and screenshot -----------------------------------------
  $log = Join-Path $env:APPDATA 'AgentSemaphore\agentsemaphore.log'
  Write-Session $procs[0] 'e2e-wait' 'waiting' 'permission prompt'
  $tray = Start-Process $TrayExe -PassThru
  Start-Sleep -Seconds 4
  Check (-not $tray.HasExited) 'tray app keeps running'
  $second = Start-Process $TrayExe -PassThru
  Check ($second.WaitForExit(10000)) 'second copy exits on its own (single instance)'
  Check ((Get-Content $log -Raw) -match 'already running') 'second copy logged "already running"'

  # For the screenshot only: Windows 11 puts new tray icons behind the ^ arrow.
  # Pin ours next to the clock on this throwaway runner (what a user would do by dragging it).
  # Runners without a desktop shell (seen on windows-11-arm: first-run setup screen) have no tray.
  try {
    $key = $null
    foreach ($i in 1..10) {
      $key = Get-ChildItem 'HKCU:\Control Panel\NotifyIconSettings' -ErrorAction SilentlyContinue |
        Where-Object { "$($_.GetValue('ExecutablePath'))" -like '*agentsemaphore*' } | Select-Object -First 1
      if ($key) { break }
      Start-Sleep -Seconds 1
    }
    if ($key) { Set-ItemProperty -LiteralPath $key.PSPath -Name IsPromoted -Value 1 -Type DWord; Start-Sleep -Seconds 3; Write-Host 'tray icon pinned' }
    else { Write-Host 'tray icon not registered with the shell (no taskbar on this runner?)' }
  } catch { Write-Warning "could not pin the tray icon: $($_.Exception.Message)" }

  try {
    Add-Type -AssemblyName System.Windows.Forms, System.Drawing
    $b = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
    $bmp = [System.Drawing.Bitmap]::new($b.Width, $b.Height)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.CopyFromScreen($b.Location, [System.Drawing.Point]::Empty, $b.Size)
    $bmp.Save((Join-Path $OutDir 'desktop.png'), [System.Drawing.Imaging.ImageFormat]::Png)
    $w = [Math]::Min(420, $b.Width); $h = [Math]::Min(70, $b.Height)
    $crop = $bmp.Clone([System.Drawing.Rectangle]::new($b.Width - $w, $b.Height - $h, $w, $h), $bmp.PixelFormat)
    $crop.Save((Join-Path $OutDir 'tray-corner.png'), [System.Drawing.Imaging.ImageFormat]::Png)
    Write-Host "screenshots saved ($($b.Width)x$($b.Height))"
  } catch { Write-Warning "no screenshot: $($_.Exception.Message)" }

  Stop-Process -Id $tray.Id -Force
  Copy-Item $log $OutDir -ErrorAction SilentlyContinue
  Write-Host 'All Windows end-to-end checks passed.'
} finally {
  $procs | ForEach-Object { Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue }
  Get-ChildItem $sesDir -Filter *.json | Remove-Item -Force
}
