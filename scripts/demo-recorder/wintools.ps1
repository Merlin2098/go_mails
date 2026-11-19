# Utilidades de ventanas Win32 para el grabador (record-gmail.mjs).
# Playwright controla páginas web, no ventanas del sistema: esto cubre el resto
# (ubicar el Explorador, ocultar la ventana nativa de Wails, aceptar el
# diálogo de confirmación nativo).
#
#   wintools.ps1 explorer -Path <carpeta> -X 0 -Y 0 -W 400 -H 800
#   wintools.ps1 hide     -ProcessId <pid>
#   wintools.ps1 confirm  -Title "Confirmar envío" [-HoldMs 1800]
#   wintools.ps1 close-explorer -Path <carpeta>
param(
  [Parameter(Mandatory)][ValidateSet("explorer", "hide", "confirm", "close-explorer")][string]$Action,
  [string]$Path,
  [int]$ProcessId,
  [string]$Title,
  [int]$X, [int]$Y, [int]$W, [int]$H,
  [int]$HoldMs = 1800,
  [int]$TimeoutMs = 30000
)
$ErrorActionPreference = "Stop"

Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class W32 {
  [DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr h, IntPtr after, int x, int y, int cx, int cy, uint flags);
  [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr h, int cmd);
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
  [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern IntPtr FindWindow(string cls, string title);
  [DllImport("user32.dll")] public static extern IntPtr SendMessage(IntPtr h, uint msg, IntPtr w, IntPtr l);
  [DllImport("user32.dll")] public static extern bool IsWindow(IntPtr h);
}
"@

function Wait-Until([scriptblock]$probe) {
  $deadline = (Get-Date).AddMilliseconds($TimeoutMs)
  while ((Get-Date) -lt $deadline) {
    $r = & $probe
    if ($r) { return $r }
    Start-Sleep -Milliseconds 150
  }
  throw "Tiempo agotado esperando ($Action)"
}

function Find-Explorer([string]$folder) {
  $full = (Resolve-Path $folder).Path.TrimEnd("\")
  foreach ($w in (New-Object -ComObject Shell.Application).Windows()) {
    try { if ($w.Document.Folder.Self.Path -eq $full) { return $w } } catch {}
  }
  return $null
}

switch ($Action) {
  "explorer" {
    Start-Process explorer.exe -ArgumentList "`"$((Resolve-Path $Path).Path)`""
    $win = Wait-Until { Find-Explorer $Path }
    $win.Document.CurrentViewMode = 4          # Detalles
    $h = [IntPtr]$win.HWND
    [W32]::ShowWindow($h, 9) | Out-Null        # SW_RESTORE (por si abrió maximizada)
    [W32]::SetWindowPos($h, [IntPtr]::Zero, $X, $Y, $W, $H, 0x0040) | Out-Null
  }
  "close-explorer" {
    $win = Find-Explorer $Path
    if ($win) { $win.Quit() }
  }
  "hide" {
    $p = Get-Process -Id $ProcessId
    $h = Wait-Until { $p.Refresh(); if ($p.MainWindowHandle -ne 0) { $p.MainWindowHandle } }
    [W32]::ShowWindow([IntPtr]$h, 0) | Out-Null # SW_HIDE
  }
  "confirm" {
    $h = Wait-Until { $x = [W32]::FindWindow($null, $Title); if ($x -ne [IntPtr]::Zero) { $x } }
    [W32]::SetForegroundWindow($h) | Out-Null
    Start-Sleep -Milliseconds $HoldMs           # que se lea en el video
    # WM_COMMAND con IDYES (6); si el diálogo fuera Aceptar/Cancelar, IDOK (1).
    [W32]::SendMessage($h, 0x0111, [IntPtr]6, [IntPtr]::Zero) | Out-Null
    Start-Sleep -Milliseconds 300
    if ([W32]::IsWindow($h)) { [W32]::SendMessage($h, 0x0111, [IntPtr]1, [IntPtr]::Zero) | Out-Null }
  }
}
