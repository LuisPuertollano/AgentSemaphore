# Semaforo de Claude Code en la bandeja de Windows (junto al reloj)
#   Rojo    = alguna sesion te espera (pregunta o permiso)
#   Naranja = alguna sesion esta trabajando, sin necesitarte
#   Verde   = todo en reposo (o no hay sesiones)
# Fuente: %USERPROFILE%\.claude\sessions\*.json, que escribe el propio Claude Code
# (campo "status": busy | shell | idle | waiting). No usa hooks ni toca settings.json.
# Creado por Claude el 14-11-2026 para Luis.

param([int]$IntervaloMs = 1000)

Add-Type -AssemblyName System.Windows.Forms, System.Drawing

# --- Una sola instancia ---------------------------------------------------
$script:creado = $false
$script:mutex = [System.Threading.Mutex]::new($true, 'Local\SemaforoClaude', [ref]$script:creado)
if (-not $script:creado) { return }

Add-Type -TypeDefinition @'
using System; using System.Runtime.InteropServices;
public static class SemWin {
  [StructLayout(LayoutKind.Sequential)] public struct FLASHWINFO { public uint cbSize; public IntPtr hwnd; public uint dwFlags; public uint uCount; public uint dwTimeout; }
  [DllImport("user32.dll")] static extern bool FlashWindowEx(ref FLASHWINFO pwfi);
  [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
  [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr h, int cmd);
  [DllImport("user32.dll")] public static extern bool IsIconic(IntPtr h);
  // Parpadea en la barra de tareas hasta que la ventana pasa a primer plano
  public static void Flash(IntPtr h) { var f = new FLASHWINFO(); f.cbSize = (uint)Marshal.SizeOf(f); f.hwnd = h; f.dwFlags = 0x2 | 0xC; FlashWindowEx(ref f); }
  public static void Front(IntPtr h) { if (IsIconic(h)) ShowWindow(h, 9); SetForegroundWindow(h); }
}
'@

# --- Ajustes persistentes -------------------------------------------------
$script:dirApp  = Split-Path -Parent $PSCommandPath
$script:fAjust  = Join-Path $script:dirApp 'ajustes.json'
$script:ajustes = [pscustomobject]@{ Parpadeo = $true }
if (Test-Path $script:fAjust) { try { $script:ajustes = Get-Content $script:fAjust -Raw | ConvertFrom-Json } catch {} }
function Save-Ajustes { $script:ajustes | ConvertTo-Json | Out-File $script:fAjust -Encoding utf8 }

$script:lnk = Join-Path ([Environment]::GetFolderPath('Startup')) 'Semaforo Claude.lnk'

# --- Iconos ---------------------------------------------------------------
function New-Punto([System.Drawing.Color]$c) {
  $bmp = [System.Drawing.Bitmap]::new(32, 32)
  $g = [System.Drawing.Graphics]::FromImage($bmp)
  $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
  $g.Clear([System.Drawing.Color]::Transparent)
  $br = [System.Drawing.SolidBrush]::new($c)
  $g.FillEllipse($br, 2, 2, 28, 28)
  $pen = [System.Drawing.Pen]::new([System.Drawing.Color]::FromArgb(170, 20, 20, 20), 2)
  $g.DrawEllipse($pen, 2, 2, 28, 28)
  $g.Dispose(); $br.Dispose(); $pen.Dispose()
  [System.Drawing.Icon]::FromHandle($bmp.GetHicon())
}
$script:iconos = @{
  rojo    = New-Punto ([System.Drawing.Color]::FromArgb(220, 38, 38))
  naranja = New-Punto ([System.Drawing.Color]::FromArgb(245, 140, 20))
  verde   = New-Punto ([System.Drawing.Color]::FromArgb(34, 170, 80))
}
$script:emoji = @{ rojo = [char]::ConvertFromUtf32(0x1F534); naranja = [char]::ConvertFromUtf32(0x1F7E0); verde = [char]::ConvertFromUtf32(0x1F7E2) }
$script:peso  = @{ rojo = 3; naranja = 2; verde = 1 }

# --- Lectura del estado ---------------------------------------------------
$script:dirSes = Join-Path $env:USERPROFILE '.claude\sessions'

function Get-Color([string]$status) {
  switch ($status) { 'waiting' { 'rojo' } 'busy' { 'naranja' } 'shell' { 'naranja' } default { 'verde' } }
}

function Get-Sesiones {
  $out = [System.Collections.Generic.List[object]]::new()
  if (-not (Test-Path $script:dirSes)) { return $out }
  foreach ($f in Get-ChildItem $script:dirSes -Filter *.json -File -ErrorAction SilentlyContinue) {
    try { $j = [IO.File]::ReadAllText($f.FullName) | ConvertFrom-Json } catch { continue }
    if (-not $j.pid) { continue }
    $p = Get-Process -Id $j.pid -ErrorAction SilentlyContinue
    if (-not $p -or $p.ProcessName -ne 'claude') { continue }   # sesion cerrada o pid reutilizado
    $nombre = if ($j.name) { $j.name } elseif ($j.cwd) { Split-Path $j.cwd -Leaf } else { "pid $($j.pid)" }
    $out.Add([pscustomobject]@{ Color = (Get-Color $j.status); Nombre = $nombre; Status = $j.status; Espera = $j.waitingFor })
  }
  $out
}

# --- Chat y Cowork: avisos de la app Claude en Windows ---------------------
# La app deja avisos tipo "Claude necesita tu respuesta para continuar" en la base de
# avisos de Windows. Un aviso cuenta como rojo hasta que Claude pasa a primer plano
# despues de que llegara (se da por visto), o hasta que desaparece de la base.
$script:python   = (Get-Command python -ErrorAction SilentlyContinue).Source
$script:fAvisos  = Join-Path $script:dirApp 'avisos.py'
$script:fWal     = Join-Path $env:LOCALAPPDATA 'Microsoft\Windows\Notifications\wpndatabase.db-wal'
$script:walMarca = [datetime]::MinValue
$script:avisos   = @()
$script:ultimoFoco = [datetime]::UtcNow      # lo anterior al arranque no cuenta
$script:ticksFoco  = 0

function Update-Avisos {
  if (-not $script:python -or -not (Test-Path $script:fAvisos)) { return }
  $m = (Get-Item $script:fWal -ErrorAction SilentlyContinue).LastWriteTimeUtc
  if ($m -and $m -eq $script:walMarca) { return }   # nada nuevo en la base
  $script:walMarca = $m
  try {
    $txt = & $script:python -I $script:fAvisos 2>$null
    if ($LASTEXITCODE -eq 0 -and $txt) {
      $script:avisos = @(@($txt | ConvertFrom-Json) | ForEach-Object {
        [pscustomobject]@{ Utc = [DateTimeOffset]::FromUnixTimeSeconds([int64]$_.epoch).UtcDateTime; Nombre = $_.nombre } })
    }
  } catch {}
}

function Get-AvisosRojos {
  foreach ($a in $script:avisos) {
    if ($a.Utc -gt $script:ultimoFoco) { [pscustomobject]@{ Color = 'rojo'; Nombre = $a.Nombre; Status = 'aviso'; Espera = 'chat/Cowork' } }
  }
}

function Get-VentanaClaude {
  Get-Process -Name claude -ErrorAction SilentlyContinue | Where-Object { $_.MainWindowHandle -ne 0 -and $_.Path -like '*WindowsApps*' } | Select-Object -First 1
}

# --- Bandeja --------------------------------------------------------------
$script:ni = [System.Windows.Forms.NotifyIcon]::new()
$script:ni.Icon = $script:iconos.verde
$script:ni.Text = 'Claude: iniciando...'
$script:ni.Visible = $true

$script:menu = [System.Windows.Forms.ContextMenuStrip]::new()
$script:sepSes = [System.Windows.Forms.ToolStripSeparator]::new()
$script:itAbrir = [System.Windows.Forms.ToolStripMenuItem]::new('Abrir Claude')
$script:itParp  = [System.Windows.Forms.ToolStripMenuItem]::new('Parpadeo naranja en la barra al ponerse en rojo')
$script:itIni   = [System.Windows.Forms.ToolStripMenuItem]::new('Arrancar con Windows')
$script:itSalir = [System.Windows.Forms.ToolStripMenuItem]::new('Salir')
$script:itParp.CheckOnClick = $true; $script:itParp.Checked = [bool]$script:ajustes.Parpadeo
$script:itIni.CheckOnClick = $true;  $script:itIni.Checked = (Test-Path $script:lnk)
[void]$script:menu.Items.AddRange(@($script:sepSes, $script:itAbrir, $script:itParp, $script:itIni, [System.Windows.Forms.ToolStripSeparator]::new(), $script:itSalir))
$script:ni.ContextMenuStrip = $script:menu

$script:abrir = { $w = Get-VentanaClaude; if ($w) { [SemWin]::Front($w.MainWindowHandle) } }
$script:itAbrir.add_Click($script:abrir)
$script:ni.add_DoubleClick($script:abrir)
$script:itParp.add_Click({ $script:ajustes.Parpadeo = $script:itParp.Checked; Save-Ajustes })
$script:itIni.add_Click({
  if ($script:itIni.Checked) {
    $sh = New-Object -ComObject WScript.Shell
    $s = $sh.CreateShortcut($script:lnk)
    $s.TargetPath = Join-Path $env:windir 'System32\conhost.exe'
    $s.Arguments = "--headless pwsh.exe -NoProfile -ExecutionPolicy Bypass -File `"$PSCommandPath`""
    $s.WorkingDirectory = $script:dirApp
    $s.Description = 'Semaforo de Claude Code en la bandeja'
    $s.Save()
  } else { Remove-Item $script:lnk -ErrorAction SilentlyContinue }
})

$script:anterior = ''
$script:tick = {
  try {
    # Si Claude esta en primer plano 2 s seguidos, los avisos anteriores se dan por vistos
    $wf = Get-VentanaClaude
    if ($wf -and [SemWin]::GetForegroundWindow() -eq $wf.MainWindowHandle) {
      $script:ticksFoco++; if ($script:ticksFoco -ge 2) { $script:ultimoFoco = [datetime]::UtcNow }
    } else { $script:ticksFoco = 0 }

    $ses = [System.Collections.Generic.List[object]]::new()
    foreach ($s in @(Get-Sesiones)) { $ses.Add($s) }
    Update-Avisos
    foreach ($s in @(Get-AvisosRojos)) { $ses.Add($s) }
    $color = 'verde'
    foreach ($s in $ses) { if ($script:peso[$s.Color] -gt $script:peso[$color]) { $color = $s.Color } }

    # Icono y tooltip (maximo 127 caracteres)
    $script:ni.Icon = $script:iconos[$color]
    if ($ses.Count -eq 0) { $txt = 'Claude: todo en reposo, nada te espera' }
    else {
      $n = @{ rojo = 0; naranja = 0; verde = 0 }; foreach ($s in $ses) { $n[$s.Color]++ }
      $txt = "Claude: $($n.rojo) te espera(n) · $($n.naranja) trabajando · $($n.verde) en reposo"
      $esperan = @($ses | Where-Object Color -eq 'rojo' | ForEach-Object Nombre)
      if ($esperan.Count) { $txt += "`n" + ($esperan -join ', ') }
    }
    if ($txt.Length -gt 127) { $txt = $txt.Substring(0, 124) + '...' }
    $script:ni.Text = $txt

    # Lista de sesiones en el menu
    while ($script:menu.Items.IndexOf($script:sepSes) -gt 0) { $script:menu.Items.RemoveAt(0) }
    $i = 0
    foreach ($s in ($ses | Sort-Object { -$script:peso[$_.Color] }, Nombre)) {
      $etq = "$($script:emoji[$s.Color])  $($s.Nombre)"
      if ($s.Color -eq 'rojo' -and $s.Espera) { $etq += "  ($($s.Espera))" }
      $it = [System.Windows.Forms.ToolStripMenuItem]::new($etq); $it.add_Click($script:abrir)
      $script:menu.Items.Insert($i, $it); $i++
    }
    if ($i -eq 0) { $v = [System.Windows.Forms.ToolStripMenuItem]::new('Nada te espera (sin sesiones activas)'); $v.Enabled = $false; $script:menu.Items.Insert(0, $v) }

    # Al pasar a rojo: parpadeo de la ventana de Claude en la barra de tareas
    if ($color -eq 'rojo' -and $script:anterior -ne 'rojo' -and $script:ajustes.Parpadeo) {
      $w = Get-VentanaClaude
      if ($w -and [SemWin]::GetForegroundWindow() -ne $w.MainWindowHandle) { [SemWin]::Flash($w.MainWindowHandle) }
    }
    if ($color -ne $script:anterior) {
      $motivo = (@($ses | Where-Object Color -eq $color | ForEach-Object { "$($_.Nombre) [$($_.Status)]" }) -join '; ')
      $fg = if ($wf -and [SemWin]::GetForegroundWindow() -eq $wf.MainWindowHandle) { 'Claude delante' } else { 'Claude detras' }
      "{0:yyyy-MM-dd HH:mm:ss}  {1,-7}  {2}  | {3}" -f (Get-Date), $color, $fg, $motivo | Out-File (Join-Path $script:dirApp 'cambios.log') -Append -Encoding utf8
    }
    $script:anterior = $color
  } catch { $script:ni.Text = 'Claude: error leyendo estado' }
}

$script:timer = [System.Windows.Forms.Timer]::new()
$script:timer.Interval = $IntervaloMs
$script:timer.add_Tick($script:tick)

$script:ctx = [System.Windows.Forms.ApplicationContext]::new()
$script:itSalir.add_Click({ $script:timer.Stop(); $script:ni.Visible = $false; $script:ni.Dispose(); $script:ctx.ExitThread() })

& $script:tick
$script:timer.Start()
[System.Windows.Forms.Application]::Run($script:ctx)
$script:mutex.ReleaseMutex()