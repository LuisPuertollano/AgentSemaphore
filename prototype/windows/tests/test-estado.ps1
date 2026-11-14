#Requires -Version 7
# Tests for estado.ps1 (no UI). Runs on any OS with pwsh; throws on the first failure.
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '..\estado.ps1')

function Check([bool]$cond, [string]$what) {
  if (-not $cond) { throw "FAILED: $what" }
  Write-Host "ok  $what"
}

# Colours and priority
Check ((Get-Color 'waiting') -eq 'rojo') 'waiting -> rojo'
Check ((Get-Color 'busy') -eq 'naranja' -and (Get-Color 'shell') -eq 'naranja') 'busy/shell -> naranja'
Check ((Get-Color 'idle') -eq 'verde') 'idle -> verde'
Check ((Get-Color 'thinking-hard') -eq 'gris') 'unknown status -> gris, never verde'
Check ((Get-Color $null) -eq 'gris') 'missing status -> gris'
Check ((Get-Peor @()) -eq 'verde') 'nothing -> verde'
Check ((Get-Peor @('naranja', 'gris')) -eq 'gris') 'gris beats naranja'
Check ((Get-Peor @('gris', 'rojo', 'naranja')) -eq 'rojo') 'rojo beats everything'
Check ((Get-Peor @('verde', 'morado')) -eq 'gris') 'unknown colour -> gris'

# Session files
$dir = Join-Path ([IO.Path]::GetTempPath()) "as-test-$([guid]::NewGuid())"
New-Item -ItemType Directory $dir | Out-Null
$vivos = { param($id) $id -in 101, 102, 103 }
function Write-Ses([string]$name, [string]$json) { Set-Content -LiteralPath (Join-Path $dir $name) -Value $json -Encoding utf8 }

Write-Ses '101.json' '{"pid":101,"name":"a","status":"waiting","waitingFor":"permission prompt"}'
Write-Ses '102.json' '{"pid":102,"cwd":"C:\\work\\b","status":"busy"}'
Write-Ses '103.json' '{"pid":103,"name":"c","status":"dreaming"}'
Write-Ses '999.json' '{"pid":999,"name":"dead","status":"waiting"}'
$r = @(Read-Sesiones $dir $vivos)
Check ($r.Count -eq 3) 'dead pid is ignored'
Check (($r | Where-Object Nombre -eq 'a').Color -eq 'rojo') 'waiting session is red'
Check (($r | Where-Object Nombre -eq 'b').Color -eq 'naranja') 'name falls back to cwd leaf'
Check (($r | Where-Object Nombre -eq 'c').Color -eq 'gris') 'unknown status session is grey'

# A half-written file keeps the last good reading, then goes grey after 3 failures without one
Write-Ses '101.json' '{"pid":101,"name":"a","sta'
$r = @(Read-Sesiones $dir $vivos)
Check (($r | Where-Object Nombre -eq 'a').Color -eq 'rojo') 'torn write uses last good reading'
Write-Ses '101.json' '{"pid":101,"name":"a","status":"waiting"}'   # repaired
Write-Ses '104.json' '{not json'
$null = @(Read-Sesiones $dir $vivos)
Check (-not (@(Read-Sesiones $dir $vivos) | Where-Object Nombre -eq '104.json')) 'no grey after 2 failed reads (may be a torn write)'
Check ((@(Read-Sesiones $dir $vivos) | Where-Object Nombre -eq '104.json').Color -eq 'gris') 'unreadable file -> gris after 3 failed reads'
Write-Ses '101.json' '{"pid":101,"na'
1..2 | ForEach-Object { $null = @(Read-Sesiones $dir $vivos) }
Check ((@(Read-Sesiones $dir $vivos) | Where-Object Nombre -eq '101.json').Color -eq 'gris') 'persistently broken file -> gris even with a last good reading'
Remove-Item -LiteralPath (Join-Path $dir '101.json'), (Join-Path $dir '104.json')
# A starting session (empty status) is ignored during the grace period, grey after it
Write-Ses '107.json' '{"pid":103,"name":"starting","status":""}'
Check (-not (@(Read-Sesiones $dir $vivos) | Where-Object Nombre -eq 'starting')) 'empty status ignored while starting (no grey flicker)'
$script:graciaSeg = 0
Check ((@(Read-Sesiones $dir $vivos) | Where-Object Nombre -eq 'starting').Color -eq 'gris') 'empty status after the grace period -> gris'
$script:graciaSeg = 10
Remove-Item -LiteralPath (Join-Path $dir '107.json')
Write-Ses '105.json' '{"sessionId":"x","status":"idle"}'
Check ((@(Read-Sesiones $dir $vivos) | Where-Object Nombre -eq '105.json').Color -eq 'gris') 'file without pid -> gris'
Remove-Item -LiteralPath $dir -Recurse -Force
Check (@(Read-Sesiones $dir $vivos).Count -eq 0) 'no sessions folder -> nothing (source absent, not an error)'

# avisos.py output
$ok = ConvertFrom-AvisosSalida '{"ok":true,"avisos":[{"id":7,"epoch":1794657600,"nombre":"Chat","textos":["Chat","x"],"tipo":"pide"}]}' 0
Check ($ok.Ok -and $ok.Avisos.Count -eq 1 -and $ok.Avisos[0].Nombre -eq 'Chat') 'avisos ok parsed'
$empty = ConvertFrom-AvisosSalida '{"ok":true,"avisos":[]}' 0
Check ($empty.Ok -and $empty.Avisos.Count -eq 0) 'empty list parsed'
$err = ConvertFrom-AvisosSalida '{"ok":false,"error":"OperationalError: no such table: Notification"}' 1
Check ((-not $err.Ok) -and $err.Error -like '*no such table*') 'error message kept'
$junk = ConvertFrom-AvisosSalida 'Traceback (most recent call last)' 1
Check ((-not $junk.Ok) -and $junk.Error) 'unparseable output is an error'
$old = ConvertFrom-AvisosSalida '[]' 0
Check (-not $old.Ok) 'old output format is an error, not an empty list'

Write-Host 'All estado checks passed.'
