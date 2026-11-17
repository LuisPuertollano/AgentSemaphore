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

# --- GitHub releases -------------------------------------------------------------
$ReleasesApi = 'https://api.github.com/repos/LuisPuertollano/AgentSemaphore/releases'

# The release with tag $tag, or the newest published one. Pre-releases count:
# GitHub's own "latest" link skips them, and early versions are all pre-releases.
# The API comes first (with $env:GITHUB_TOKEN if set). Anonymous API calls are
# rate-limited per IP, so without an answer the tag comes from the public
# releases page and the asset links are built from it.
function Get-Release([string]$tag) {
  $api = if ($env:AGENTSEMAPHORE_RELEASES_API) { $env:AGENTSEMAPHORE_RELEASES_API } else { $ReleasesApi }
  $headers = @{ Accept = 'application/vnd.github+json'; 'User-Agent' = 'AgentSemaphore-installer' }
  if ($env:GITHUB_TOKEN) { $headers.Authorization = "Bearer $env:GITHUB_TOKEN" }
  try {
    if ($tag) { return Invoke-RestMethod -Uri "$api/tags/$tag" -Headers $headers }
    $all = Invoke-RestMethod -Uri "$api`?per_page=20" -Headers $headers
    $rel = $all | Where-Object { -not $_.draft } | Sort-Object { [datetime]$_.published_at } -Descending | Select-Object -First 1
    if ($rel) { return $rel }
  } catch {
    Write-Host "The GitHub API did not answer ($($_.Exception.Message)); using the releases page."
  }
  if (-not $tag) {
    $page = (Invoke-WebRequest -Uri 'https://github.com/LuisPuertollano/AgentSemaphore/releases' -UseBasicParsing).Content
    $m = [regex]::Match([string]$page, '/LuisPuertollano/AgentSemaphore/releases/tag/(v\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?)')
    if (-not $m.Success) { throw 'No published release found.' }
    $tag = $m.Groups[1].Value
    Write-Host "Found $tag on the releases page."
  }
  $base = "https://github.com/LuisPuertollano/AgentSemaphore/releases/download/$tag"
  [pscustomobject]@{
    tag_name = $tag
    assets   = @('agentsemaphore-windows-amd64.exe', 'agentsemaphore-windows-arm64.exe', 'SHA256SUMS.txt') |
      ForEach-Object { [pscustomobject]@{ name = $_; browser_download_url = "$base/$_" } }
  }
}

# Downloads asset $name of release $rel to $dest and checks it against the
# release's SHA256SUMS.txt; deletes the download and throws on a mismatch.
function Save-ReleaseAsset($rel, [string]$name, [string]$dest) {
  $asset = @($rel.assets | Where-Object { $_.name -eq $name })
  if (-not $asset) { throw "Release $($rel.tag_name) has no $name." }
  $sums = @($rel.assets | Where-Object { $_.name -eq 'SHA256SUMS.txt' })
  if (-not $sums) { throw "Release $($rel.tag_name) has no SHA256SUMS.txt; not installing an unchecked download." }
  # Saved to a file first: release assets come as application/octet-stream,
  # so Invoke-WebRequest's .Content would be bytes, not text.
  $sumsFile = "$dest.sha256sums"
  Invoke-WebRequest -Uri $sums[0].browser_download_url -OutFile $sumsFile -UseBasicParsing
  $line = @(Get-Content -LiteralPath $sumsFile) |
    Where-Object { $_ -match "^([0-9a-fA-F]{64})\s+\*?$([regex]::Escape($name))\s*$" } | Select-Object -First 1
  Remove-Item -LiteralPath $sumsFile -Force
  if (-not $line) { throw "SHA256SUMS.txt of $($rel.tag_name) does not list $name." }
  $want = ($line -split '\s+')[0].ToLowerInvariant()
  Invoke-WebRequest -Uri $asset[0].browser_download_url -OutFile $dest -UseBasicParsing
  $got = (Get-FileHash -LiteralPath $dest -Algorithm SHA256).Hash.ToLowerInvariant()
  if ($got -ne $want) {
    Remove-Item -LiteralPath $dest -Force
    throw "SHA-256 mismatch for $name (expected $want, got $got); download deleted."
  }
  Unblock-File -LiteralPath $dest
}
