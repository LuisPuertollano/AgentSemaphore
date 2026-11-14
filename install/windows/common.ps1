# Shared helpers for install.ps1 and uninstall.ps1 (dot-sourced).

$ClaudeAumid          = 'Claude_pzs8sxrjxfjjc!Claude'
$PhoneLinkClaudeAumid = 'Microsoft.YourPhone_8wekyb3d8bbwe!YourPhoneNotifications_com.anthropic.claude'
$NotifSettingsRoot    = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Notifications\Settings'
$NotifValueNames      = @('ShowBanner', 'SoundFile', 'ShowInActionCenter')
$ManifestName         = 'install-manifest.json'

function Read-Manifest([string]$dir) {
  $p = Join-Path $dir $ManifestName
  if (Test-Path -LiteralPath $p) { return Get-Content -LiteralPath $p -Raw | ConvertFrom-Json -AsHashtable }
  return $null
}

function Write-Manifest([string]$dir, $manifest) {
  $manifest | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $dir $ManifestName) -Encoding utf8
}

# Captures the current state of our notification values under $key so it can
# be restored exactly: whether each value existed, its registry kind and data.
function Get-NotificationValues([string]$key) {
  $entry = [ordered]@{ key = $key; keyExisted = (Test-Path -LiteralPath $key); items = [ordered]@{} }
  $item = if ($entry.keyExisted) { Get-Item -LiteralPath $key } else { $null }
  foreach ($n in $NotifValueNames) {
    if ($item -and ($item.GetValueNames() -contains $n)) {
      $entry.items[$n] = [ordered]@{ existed = $true; kind = $item.GetValueKind($n).ToString(); data = $item.GetValue($n, $null, 'DoNotExpandEnvironmentNames') }
    } else {
      $entry.items[$n] = [ordered]@{ existed = $false }
    }
  }
  $entry
}

# Puts the values back as recorded: rewrites the ones that existed, deletes the
# ones that did not. Never deletes the key itself (Windows owns it).
function Restore-NotificationValues($entry) {
  $key = $entry.key
  if (-not (Test-Path -LiteralPath $key)) { return }
  foreach ($n in @($entry.items.Keys)) {
    $v = $entry.items[$n]
    if ($v.existed) {
      New-ItemProperty -LiteralPath $key -Name $n -PropertyType $v.kind -Value $v.data -Force | Out-Null
    } else {
      Remove-ItemProperty -LiteralPath $key -Name $n -ErrorAction SilentlyContinue
    }
  }
}

function Test-ShortcutPointsTo([string]$lnk, [string]$dir) {
  $s = (New-Object -ComObject WScript.Shell).CreateShortcut($lnk)
  return "$($s.Arguments) $($s.TargetPath)".Contains($dir, [StringComparison]::OrdinalIgnoreCase)
}

# Stops semaphore processes started from $dir (pwsh running its semaforo.ps1).
function Stop-Semaphore {
  [CmdletBinding(SupportsShouldProcess)]
  param([string]$dir)
  $entryScript = Join-Path $dir 'semaforo.ps1'
  $procs = Get-CimInstance Win32_Process -Filter "Name = 'pwsh.exe'" -ErrorAction SilentlyContinue |
    Where-Object { $_.CommandLine -and $_.CommandLine.Contains($entryScript, [StringComparison]::OrdinalIgnoreCase) }
  foreach ($p in $procs) {
    if ($PSCmdlet.ShouldProcess("PID $($p.ProcessId)", "Stop AgentSemaphore running from $dir")) {
      Stop-Process -Id $p.ProcessId -Force -ErrorAction SilentlyContinue
    }
  }
}
