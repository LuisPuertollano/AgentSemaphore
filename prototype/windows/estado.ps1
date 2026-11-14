# Logica del semaforo sin interfaz (la usa semaforo.ps1 y la prueba tests/test-estado.ps1).
#
# Colores y prioridad: rojo > gris > naranja > verde.
#   gris = una fuente no se pudo leer o cambio de formato: podria esconder un rojo,
#   asi que nunca se tapa con naranja ni se muestra como verde.

$script:peso = @{ rojo = 4; gris = 3; naranja = 2; verde = 1 }

# Lee una propiedad que puede no existir (los formatos no documentados cambian).
function Get-Campo($o, [string]$n) {
  if ($null -eq $o) { return $null }
  $p = $o.PSObject.Properties[$n]
  if ($p) { $p.Value } else { $null }
}

# status del fichero de sesion de Claude Code -> color. Lo que no se conoce es gris.
function Get-Color([string]$status) {
  switch ($status) {
    'waiting' { 'rojo' }
    'busy'    { 'naranja' }
    'shell'   { 'naranja' }
    'idle'    { 'verde' }
    default   { 'gris' }
  }
}

function Get-Peor([string[]]$colores) {
  $c = 'verde'
  foreach ($x in $colores) {
    if (-not $x -or -not $script:peso.ContainsKey($x)) { $x = 'gris' }
    if ($script:peso[$x] -gt $script:peso[$c]) { $c = $x }
  }
  $c
}

# --- Sesiones de Claude Code ---------------------------------------------------
# Un fichero que no se puede leer (p. ej. a medio escribir) usa la ultima lectura buena;
# si falla 3 veces seguidas, la sesion sale en gris.
$script:ultimoBueno = @{}
$script:fallos = @{}
# Claude crea ficheros de sesion con status vacio mientras arranca (visto en Windows:
# duran < 1 s). Durante esta gracia se ignoran; si siguen vacios despues, gris.
$script:graciaSeg = 10
$script:primeraVez = @{}

# $esClaude: scriptblock que recibe un pid y dice si es un proceso vivo de Claude Code.
function Read-Sesiones([string]$dir, [scriptblock]$esClaude) {
  if (-not (Test-Path -LiteralPath $dir)) { return }   # sin Claude Code: fuente ausente, no error
  $vistos = @{}
  foreach ($f in Get-ChildItem -LiteralPath $dir -Filter *.json -File -ErrorAction SilentlyContinue) {
    $k = $f.FullName
    $vistos[$k] = $true
    $j = $null
    try {
      $j = [IO.File]::ReadAllText($k) | ConvertFrom-Json -ErrorAction Stop
      $script:fallos[$k] = 0
      $script:ultimoBueno[$k] = $j
    } catch {
      $script:fallos[$k] = 1 + [int]$script:fallos[$k]
      $j = $script:ultimoBueno[$k]
    }
    if ($script:fallos[$k] -ge 3) {
      [pscustomobject]@{ Color = 'gris'; Nombre = $f.Name; Status = 'ilegible'; Espera = 'fichero de sesion ilegible' }
      continue
    }
    if (-not $j) { continue }
    $sesPid = Get-Campo $j 'pid'
    if (-not $sesPid) {
      [pscustomobject]@{ Color = 'gris'; Nombre = $f.Name; Status = 'formato'; Espera = 'fichero de sesion sin pid (formato nuevo?)' }
      continue
    }
    if (-not (& $esClaude $sesPid)) { continue }   # sesion cerrada o pid reutilizado
    $name = Get-Campo $j 'name'; $cwd = Get-Campo $j 'cwd'; $status = Get-Campo $j 'status'
    $nombre = if ($name) { $name } elseif ($cwd) { Split-Path $cwd -Leaf } else { "pid $sesPid" }
    if (-not $status) {
      if (-not $script:primeraVez.ContainsKey($k)) { $script:primeraVez[$k] = [datetime]::UtcNow }
      if (([datetime]::UtcNow - $script:primeraVez[$k]).TotalSeconds -lt $script:graciaSeg) { continue }   # arrancando
    }
    $color = Get-Color $status
    $espera = if ($color -eq 'gris') { "estado desconocido: '$status'" } else { Get-Campo $j 'waitingFor' }
    [pscustomobject]@{ Color = $color; Nombre = $nombre; Status = $status; Espera = $espera }
  }
  foreach ($k in @($script:fallos.Keys)) { if (-not $vistos.ContainsKey($k)) { $script:fallos.Remove($k) } }
  foreach ($k in @($script:ultimoBueno.Keys)) { if (-not $vistos.ContainsKey($k)) { $script:ultimoBueno.Remove($k) } }
  foreach ($k in @($script:primeraVez.Keys)) { if (-not $vistos.ContainsKey($k)) { $script:primeraVez.Remove($k) } }
}

# --- Avisos de Claude Desktop ---------------------------------------------------
# Interpreta la salida de avisos.py. Devuelve @{ Ok; Avisos; Error }.
function ConvertFrom-AvisosSalida($texto, [int]$codigo) {
  $j = $null
  try { $j = ($texto -join "`n") | ConvertFrom-Json -ErrorAction Stop } catch {}
  if ($codigo -ne 0 -or $null -eq $j -or $j -is [array] -or -not (Get-Campo $j 'ok')) {
    $e = if ($j -isnot [array]) { Get-Campo $j 'error' }
    $err = if ($e) { $e } elseif ($codigo -ne 0) { "avisos.py salio con codigo $codigo" } else { 'salida de avisos.py ilegible' }
    return @{ Ok = $false; Avisos = @(); Error = $err }
  }
  $lista = @(@(Get-Campo $j 'avisos') | Where-Object { $_ } | ForEach-Object {
    [pscustomobject]@{
      Id     = Get-Campo $_ 'id'
      Utc    = [DateTimeOffset]::FromUnixTimeSeconds([int64](Get-Campo $_ 'epoch')).UtcDateTime
      Nombre = Get-Campo $_ 'nombre'
      Tipo   = Get-Campo $_ 'tipo'
      Textos = @(Get-Campo $_ 'textos') -join ' / '
    }
  })
  @{ Ok = $true; Avisos = $lista; Error = $null }
}
