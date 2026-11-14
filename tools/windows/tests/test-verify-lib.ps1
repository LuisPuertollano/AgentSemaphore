#Requires -Version 7
# Tests for verify-lib.ps1. Runs on any OS with pwsh; throws on the first failure.
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '..\verify-lib.ps1')
function Check([bool]$cond, [string]$what) { if (-not $cond) { throw "FAILED: $what" }; Write-Host "ok  $what" }

$root = Join-Path ([IO.Path]::GetTempPath()) "as-pkg-$([guid]::NewGuid())"
New-Item -ItemType Directory -Path (Join-Path $root 'locales') -Force | Out-Null
Set-Content (Join-Path $root 'locales/es.json') '{"toast.needsReply": "Claude necesita tu respuesta para continuar", "other": "x"}' -Encoding utf8
Set-Content (Join-Path $root 'locales/en.json') '{"toast.needsReply": "Claude needs your response to continue", "other": "y"}' -Encoding utf8
Set-Content (Join-Path $root 'app.asar') ('BINARY' + [char]0 + '{"toast.needsReply":"Claude braucht deine Antwort, um fortzufahren","x":"Tarea programada completada"}') -Encoding utf8
Set-Content (Join-Path $root 'readme.txt') '"toast.needsReply": "ignored, wrong extension"' -Encoding utf8

$r = @(Find-LocalisedStrings -Root $root -Phrases @('necesita tu respuesta', 'Tarea programada completada', 'no such phrase'))
$needs = $r | Where-Object Key -eq 'toast.needsReply'
Check ($null -ne $needs) 'key found from the Spanish phrase'
Check ($needs.Values.Count -eq 3) 'same key found in es, en and the asar (de)'
Check ($needs.Values.Contains('Claude needs your response to continue')) 'English value returned'
Check ($needs.Values.Contains('Claude braucht deine Antwort, um fortzufahren')) 'German value from inside the asar'
Check (($r | Where-Object Key -eq 'x').Values.Count -eq 1) 'second phrase found too'
Check (-not ($r | Where-Object Phrase -eq 'no such phrase')) 'unknown phrase yields nothing'

$ses = Join-Path $root 'sessions'
New-Item -ItemType Directory $ses | Out-Null
Set-Content (Join-Path $ses '1.json') '{"pid":1,"status":"waiting","waitingFor":"permission prompt","entrypoint":"cli","procStart":"5"}' -Encoding utf8
Set-Content (Join-Path $ses '2.json') '{"pid":2,"status":"idle"}' -Encoding utf8
Set-Content (Join-Path $ses '3.json') '{broken' -Encoding utf8
$snap = Get-SessionSnapshot $ses
Check ($snap.Count -eq 2) 'broken file skipped'
Check ($snap['1.json'].WaitingFor -eq 'permission prompt' -and $snap['1.json'].HasProcStart) 'fields read'
Check ($null -eq $snap['2.json'].Entrypoint -and -not $snap['2.json'].HasProcStart) 'missing fields are null'
Remove-Item $root -Recurse -Force
Write-Host 'All verify-lib checks passed.'
