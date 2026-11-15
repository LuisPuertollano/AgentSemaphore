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

# --- Go build -----------------------------------------------------------------
$GoExeName = 'agentsemaphore.exe'
$RunKey    = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
$RunName   = 'AgentSemaphore'   # same name the app's own "Start with the system" menu uses
$PrototypeFiles = @('semaforo.ps1', 'estado.ps1', 'avisos.py', 'avisos-textos.json', 'cambios.log', 'ajustes.json')

# Stops the Go build running from $dir.
function Stop-GoSemaphore {
  [CmdletBinding(SupportsShouldProcess)]
  param([string]$dir)
  $exe = Join-Path $dir $GoExeName
  foreach ($p in Get-Process -Name 'agentsemaphore*' -ErrorAction SilentlyContinue) {
    $path = $null; try { $path = $p.Path } catch {}
    if ($path -and $path -ieq $exe) {
      if ($PSCmdlet.ShouldProcess("PID $($p.Id)", "Stop AgentSemaphore running from $dir")) {
        Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
        $p.WaitForExit(5000) | Out-Null
      }
    }
  }
}

# The HKCU Run value, if any.
function Get-RunValue { (Get-ItemProperty -LiteralPath $RunKey -Name $RunName -ErrorAction SilentlyContinue)?.$RunName }

# Removes the Run value, but only if it starts something inside $dir.
function Remove-RunValue {
  [CmdletBinding(SupportsShouldProcess)]
  param([string]$dir)
  $v = Get-RunValue
  if ($v -and $v.Contains($dir, [StringComparison]::OrdinalIgnoreCase)) {
    if ($PSCmdlet.ShouldProcess("$RunKey\$RunName", 'Remove autostart entry')) {
      Remove-ItemProperty -LiteralPath $RunKey -Name $RunName
    }
  }
}

# Removes the prototype's autostart shortcuts that point into $dir.
function Remove-PrototypeShortcuts {
  [CmdletBinding(SupportsShouldProcess)]
  param([string]$dir, [string[]]$extra = @())
  $startup = [Environment]::GetFolderPath('Startup')
  $links = @($extra) + @((Join-Path $startup 'AgentSemaphore.lnk'), (Join-Path $startup 'Semaforo Claude.lnk'))
  foreach ($lnk in ($links | Where-Object { $_ } | Select-Object -Unique)) {
    if ((Test-Path -LiteralPath $lnk) -and (Test-ShortcutPointsTo $lnk $dir)) {
      if ($PSCmdlet.ShouldProcess($lnk, 'Remove autostart shortcut')) { Remove-Item -LiteralPath $lnk }
    }
  }
}

# OS architecture as used in release asset names.
function Get-GoArch {
  switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) {
    'Arm64' { 'arm64' } 'X64' { 'amd64' } default { throw "Unsupported architecture: $_" }
  }
}
