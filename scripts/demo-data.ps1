<#
.SYNOPSIS
    Genera el set de datos ficticio para demos: plantilla Excel + PDFs "escaneados".

.DESCRIPTION
    Envuelve `go run ./cmd/demodata`. Por defecto borra la carpeta de salida antes
    de generar, así cada grabación parte del mismo estado (útil si ya renombraste
    los PDFs en una toma anterior).

    Todos los correos usan el dominio reservado example.com: un envío real por
    error nunca llega a nadie.

.PARAMETER Out
    Carpeta de salida, relativa a la raíz del repo. Por defecto: demo

.PARAMETER Count
    Total de destinatarios (12-5000). Los primeros 12 son fijos e incluyen 2
    errores sembrados (un correo inválido y un PDF faltante).

.PARAMETER Keep
    No borrar la carpeta de salida antes de generar.

.EXAMPLE
    .\scripts\demo-data.ps1
    .\scripts\demo-data.ps1 -Count 100
    .\scripts\demo-data.ps1 -Out C:\Demo -Keep
#>
[CmdletBinding()]
param(
    [string]$Out = "demo",
    [ValidateRange(12, 5000)][int]$Count = 12,
    [switch]$Keep
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "No se encontró 'go' en el PATH. Instala Go 1.25+ desde https://go.dev/dl/"
    }

    $target = if ([System.IO.Path]::IsPathRooted($Out)) { $Out } else { Join-Path $root $Out }
    if ((Test-Path $target) -and -not $Keep) {
        Write-Host "Limpiando $target ..." -ForegroundColor DarkGray
        try {
            Remove-Item -Recurse -Force -Confirm:$false $target
        }
        catch {
            throw "No se pudo limpiar '$target'. ¿Hay algún PDF o el Excel abierto en otra aplicación? Ciérralo y reintenta (o usa -Keep). Detalle: $($_.Exception.Message)"
        }
    }

    go run ./cmd/demodata -out $target -count $Count
    if ($LASTEXITCODE -ne 0) { throw "La generación falló (código $LASTEXITCODE)." }

    Write-Host ""
    Write-Host "Listo. En la app:" -ForegroundColor Green
    Write-Host "  1. Conexión  -> Modo demo"
    Write-Host "  2. Datos     -> $(Join-Path $target 'plantilla_envio.xlsx')"
    Write-Host "  4. Adjuntos  -> carpeta $(Join-Path $target 'documentos_origen')"
    Write-Host "                  renombrar con  CERT-{{documento_id}}_{{nombre}}{{ext}}"
    Write-Host "                  patrón         CERT-{{documento_id}}_*.pdf"
}
finally {
    Pop-Location
}
