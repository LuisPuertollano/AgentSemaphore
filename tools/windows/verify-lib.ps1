# Helpers for verify-phase1.ps1 (dot-sourced). Kept separate so CI can test them.

# Searches files under $Root for the given phrases and, for every "key": "value"
# pair whose value contains a phrase, returns that key's value in every file that
# has it. In a localised app this yields the same message in all languages.
function Find-LocalisedStrings {
  param(
    [Parameter(Mandatory)] [string]$Root,
    [Parameter(Mandatory)] [string[]]$Phrases,
    [string[]]$Extensions = @('.json', '.asar', '.js'),
    [long]$MaxBytes = 400MB
  )
  $files = @(Get-ChildItem -LiteralPath $Root -Recurse -File -ErrorAction SilentlyContinue |
    Where-Object { $_.Extension -in $Extensions -and $_.Length -le $MaxBytes })
  $texts = @{}
  foreach ($f in $files) {
    try { $texts[$f.FullName] = [Text.Encoding]::UTF8.GetString([IO.File]::ReadAllBytes($f.FullName)) } catch {}
  }
  $str = '(?:[^"\\]|\\.)'          # one character inside a JSON string
  $keys = [ordered]@{}
  foreach ($phrase in $Phrases) {
    $p = [regex]::Escape($phrase)
    $rx = [regex]::new("`"($str{1,200})`"\s*:\s*`"($str*?$p$str*)`"", 'IgnoreCase')
    foreach ($kv in $texts.GetEnumerator()) {
      foreach ($m in $rx.Matches($kv.Value)) { $keys[$m.Groups[1].Value] = $phrase }
    }
  }
  foreach ($key in $keys.Keys) {
    $rx = [regex]::new("`"$([regex]::Escape($key))`"\s*:\s*`"($str*)`"")
    $values = [ordered]@{}
    foreach ($kv in $texts.GetEnumerator()) {
      foreach ($m in $rx.Matches($kv.Value)) {
        $v = $m.Groups[1].Value
        if (-not $values.Contains($v)) { $values[$v] = [IO.Path]::GetRelativePath($Root, $kv.Key) }
      }
    }
    [pscustomobject]@{ Key = $key; Phrase = $keys[$key]; Values = $values }
  }
}

# Reads all Claude Code session files. Returns a hashtable: file name -> object
# with the fields we care about (missing fields are $null).
function Get-SessionSnapshot([string]$Dir) {
  $out = @{}
  foreach ($f in Get-ChildItem -LiteralPath $Dir -Filter *.json -File -ErrorAction SilentlyContinue) {
    try { $j = [IO.File]::ReadAllText($f.FullName) | ConvertFrom-Json -ErrorAction Stop } catch { continue }
    $get = { param($n) $p = $j.PSObject.Properties[$n]; if ($p) { $p.Value } }
    $out[$f.Name] = [pscustomobject]@{
      File            = $f.Name
      Pid             = & $get 'pid'
      Name            = & $get 'name'
      Status          = & $get 'status'
      WaitingFor      = & $get 'waitingFor'
      Entrypoint      = & $get 'entrypoint'
      StatusUpdatedAt = & $get 'statusUpdatedAt'
      HasProcStart    = $null -ne (& $get 'procStart')
    }
  }
  $out
}
