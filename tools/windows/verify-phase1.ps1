#Requires -Version 7
<#
.SYNOPSIS
  Runs the Phase 1 checks on this machine and writes one report to paste back.

.DESCRIPTION
  Read-only towards Claude: it never edits Claude's settings or files.

  1. Toast texts (-SkipToasts to skip): searches the installed Claude Desktop
     package for the known Spanish toast phrases and lists the same messages in
     every language the package contains (English, German, ...).
  2. Claude Code CLI (-SkipCli to skip). Run the script from a normal PowerShell
     window, not from inside a Claude session. It opens short Claude Code sessions in new
     windows, each with a prompt that should make it wait for you, records what
     its session file says, then closes it. Nothing to answer, except possibly
     once "Do you trust this folder?" -> Yes.
  3. Claude Desktop Code tab (-Desktop to include, guided): copies a prompt to
     the clipboard, you paste it into a Code-tab session on a LOCAL folder and
     press Enter; the script records the state, then tells you to answer.

  Report: %USERPROFILE%\agentsemaphore-verify.md (also copied to the clipboard).
#>
param(
  [switch]$SkipToasts,
  [switch]$SkipCli,
  [switch]$Desktop,
  [int]$TimeoutSec = 120,
  [string]$WorkDir = (Join-Path $env:USERPROFILE 'agentsemaphore-verify-work')
)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'verify-lib.ps1')

$sesDir = Join-Path $env:USERPROFILE '.claude\sessions'
$report = [System.Collections.Generic.List[string]]::new()
function Say([string]$s) { Write-Host $s; $report.Add($s) }
function Note([string]$s) { Write-Host $s -ForegroundColor Cyan }

Say "# AgentSemaphore Phase 1 verification"
Say ""
Say "- When: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss zzz')"
Say "- Windows: $([Environment]::OSVersion.VersionString), arch $env:PROCESSOR_ARCHITECTURE"
Say "- PowerShell: $($PSVersionTable.PSVersion)"
$claudeCmd = Get-Command claude -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
$cliVersion = if ($claudeCmd) { try { (& $claudeCmd.Source --version 2>$null) -join ' ' } catch { 'unknown' } } else { 'not found' }
Say "- Claude Code CLI: $cliVersion ($(if ($claudeCmd) { $claudeCmd.Source } else { '-' }))"

# --- 1. Toast texts ------------------------------------------------------------
if (-not $SkipToasts) {
  Say ""
  Say "## 1. Toast texts in the Claude Desktop package"
  $root = $null
  $proc = Get-Process claude -ErrorAction SilentlyContinue | Where-Object { $_.Path -like '*\WindowsApps\Claude_*' } | Select-Object -First 1
  if ($proc) { $root = ($proc.Path -replace '(\\WindowsApps\\Claude_[^\\]+).*', '$1') }
  if (-not $root) {
    try { $root = (powershell.exe -NoProfile -Command "(Get-AppxPackage | Where-Object PackageFamilyName -eq 'Claude_pzs8sxrjxfjjc' | Select-Object -First 1).InstallLocation") } catch {}
  }
  if (-not $root -or -not (Test-Path -LiteralPath $root)) {
    Say "Claude Desktop package not found (start Claude Desktop and run again)."
  } else {
    $pkgVersion = Split-Path $root -Leaf
    Say "Package: $pkgVersion"
    Note "Searching the Claude Desktop package (read-only, may take a minute)..."
    # Exact Spanish texts of the toasts seen on a real machine (FINDINGS §3).
    $phrases = @('Claude necesita tu respuesta para continuar', 'Claude está esperando tu respuesta', 'La tarea programada necesita tu aprobación', 'Tarea programada completada')
    $found = @(Find-LocalisedStrings -Root $root -Phrases $phrases)
    if (-not $found.Count) { Say "No phrase found in .json/.asar/.js files under the package." }
    foreach ($f in $found) {
      Say ""
      Say "Key ``$($f.Key)`` (matched: $($f.Phrase))"
      foreach ($v in $f.Values.GetEnumerator()) { Say "- ``$($v.Key)``  — $($v.Value)" }
    }
  }
}

# --- helpers for 2 and 3 -------------------------------------------------------
# Watches session files and returns the timeline of status changes for the
# session selected by $pick, until $done says so or the timeout passes.
function Watch-Session([scriptblock]$pick, [scriptblock]$done, [int]$timeout) {
  $t0 = Get-Date
  $timeline = [System.Collections.Generic.List[object]]::new()
  $last = $null
  while (((Get-Date) - $t0).TotalSeconds -lt $timeout) {
    $s = & $pick (Get-SessionSnapshot $sesDir)
    if ($s) {
      $sig = "$($s.Status)|$($s.WaitingFor)"
      if ($sig -ne $last) {
        $timeline.Add([pscustomobject]@{ T = [math]::Round(((Get-Date) - $t0).TotalSeconds, 1); Status = $s.Status; WaitingFor = $s.WaitingFor; Session = $s })
        $last = $sig
      }
      if (& $done $timeline ((Get-Date) - $t0).TotalSeconds) { break }
    }
    Start-Sleep -Milliseconds 250
  }
  , $timeline
}
function Format-Timeline($timeline) {
  if (-not $timeline.Count) { return '(no session seen)' }
  ($timeline | ForEach-Object { "$($_.T)s $($_.Status)$(if ($_.WaitingFor) { " [$($_.WaitingFor)]" })" }) -join ' → '
}
# Done when the expected status has held for $hold seconds (for idle: only after it worked).
function New-DoneCheck([string]$expect, [double]$hold) {
  {
    param($tl, [double]$elapsed)
    if (-not $tl.Count) { return $false }
    $lastEntry = $tl[$tl.Count - 1]
    if ($lastEntry.Status -ne $expect) { return $false }
    if ($expect -eq 'idle' -and -not ($tl | Where-Object Status -eq 'busy')) { return $false }
    return $elapsed -ge ($lastEntry.T + $hold)
  }.GetNewClosure()
}

$scenarios = @(
  [pscustomobject]@{ Id = 'A1'; What = 'permission dialog';      Mode = 'default'; Expect = 'waiting'; Prompt = 'Run exactly this shell command and nothing else: New-Item -ItemType File -Path agentsemaphore-probe.txt' }
  [pscustomobject]@{ Id = 'A2'; What = 'AskUserQuestion';        Mode = 'default'; Expect = 'waiting'; Prompt = 'Use the AskUserQuestion tool to ask me which colour I prefer: red, green or blue. Do nothing else.' }
  [pscustomobject]@{ Id = 'A3'; What = 'plan approval';          Mode = 'plan';    Expect = 'waiting'; Prompt = 'Plan in one line how to create an empty file hello.txt in this folder and present the plan for approval. Do not read any files.' }
  [pscustomobject]@{ Id = 'A5'; What = 'plain-text question';    Mode = 'default'; Expect = 'idle';    Prompt = 'Ask me in plain text what my favourite food is, then stop. Do not use any tool.' }
)

# --- 2. Claude Code CLI ----------------------------------------------------------
if (-not $SkipCli) {
  Say ""
  Say "## 2. Claude Code CLI (automatic)"
  if (-not $claudeCmd) {
    Say "Claude Code CLI not found on PATH; skipped."
  } else {
    New-Item -ItemType Directory -Force -Path $WorkDir | Out-Null
    # When this script itself runs inside a Claude Code session, the CLI windows it
    # opens inherit CLAUDE_CODE_* markers, become "child sessions" and do not save
    # their state (seen 2026-11-14: "Transcript saving is off - inherited
    # CLAUDE_CODE_CHILD_SESSION marker"). Start them with a clean environment.
    $inherited = @(Get-ChildItem Env: | Where-Object { $_.Name -like 'CLAUDE_CODE_*' -or $_.Name -eq 'CLAUDECODE' })
    if ($inherited.Count) {
      Note "Removing inherited Claude Code variables for the test windows: $($inherited.Name -join ', ')"
      foreach ($v in $inherited) { Remove-Item -LiteralPath "Env:$($v.Name)" }
    }
    Note "New windows will open and close by themselves. If one asks 'Do you trust this folder?', choose Yes. Do not answer anything else, and do not log in from those windows."
    if ((& $claudeCmd.Source auth status 2>$null) -match 'not logged in|Not logged') { Say "Warning: the CLI reports it is not logged in; run 'claude' once and /login first." }
    Say ""
    Say "| Step | Expected | Seen (time → status [waitingFor]) | procStart field |"
    Say "|---|---|---|---|"
    foreach ($sc in $scenarios) {
      Note "$($sc.Id) $($sc.What)..."
      $before = (Get-SessionSnapshot $sesDir).Keys
      $argLine = "--permission-mode $($sc.Mode) `"$($sc.Prompt)`""
      $p = Start-Process -FilePath $claudeCmd.Source -ArgumentList $argLine -WorkingDirectory $WorkDir -PassThru
      $pick = { param($snap) $snap.Values | Where-Object { $_.File -notin $before -and $_.Entrypoint -ne 'claude-desktop' } | Select-Object -First 1 }.GetNewClosure()
      $tl = Watch-Session $pick (New-DoneCheck $sc.Expect 3) $TimeoutSec
      $sesPid = if ($tl.Count) { $tl[0].Session.Pid }
      if ($sesPid) { Stop-Process -Id $sesPid -Force -ErrorAction SilentlyContinue }
      Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
      $ps = if ($tl.Count) { $tl[0].Session.HasProcStart } else { '-' }
      Say "| $($sc.Id) $($sc.What) | $($sc.Expect) | $(Format-Timeline $tl) | $ps |"
      Start-Sleep -Seconds 2
    }
  }
}

# --- 3. Claude Desktop Code tab (guided) ---------------------------------------------
if ($Desktop) {
  Say ""
  Say "## 3. Claude Desktop Code tab (guided)"
  Note "In Claude Desktop open the Code tab, start a session on a LOCAL folder (not SSH), permission mode 'Ask'."
  Read-Host "Press Enter here when that session is open" | Out-Null
  Say ""
  Say "| Step | Expected | Seen (time → status [waitingFor]) |"
  Say "|---|---|---|"
  foreach ($sc in $scenarios) {
    if ($sc.Mode -eq 'plan') { Note "Switch that Code-tab session to Plan mode first." }
    Set-Clipboard -Value $sc.Prompt
    Note "$($sc.Id) $($sc.What): the prompt is in your clipboard. Paste it (Ctrl+V) into the Code tab and press Enter. Do NOT answer the dialog yet."
    $base = Get-SessionSnapshot $sesDir
    $pick = {
      param($snap)
      $snap.Values | Where-Object { $_.Entrypoint -eq 'claude-desktop' -and (-not $base.ContainsKey($_.File) -or $base[$_.File].StatusUpdatedAt -ne $_.StatusUpdatedAt) } | Select-Object -First 1
    }.GetNewClosure()
    $tl = Watch-Session $pick (New-DoneCheck $sc.Expect 3) $TimeoutSec
    Say "| $($sc.Id) $($sc.What) | $($sc.Expect) | $(Format-Timeline $tl) |"
    Read-Host "Recorded. Now answer or close the dialog in Claude Desktop (any answer), switch back to the normal mode if needed, then press Enter here" | Out-Null
  }
}

$out = Join-Path $env:USERPROFILE 'agentsemaphore-verify.md'
$report | Set-Content -LiteralPath $out -Encoding utf8
try { Set-Clipboard -Value ($report -join "`n") } catch {}
Write-Host ""
Write-Host "Report saved to $out and copied to the clipboard." -ForegroundColor Green
