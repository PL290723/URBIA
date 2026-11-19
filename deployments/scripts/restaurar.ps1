# Restaura un respaldo hecho con respaldar.ps1.
# REEMPLAZA la base de datos actual y, si se indica -Fotos, las fotografias.
#
# Uso (desde la raiz del repo):
#   powershell -ExecutionPolicy Bypass -File deployments\scripts\restaurar.ps1 -Bd <archivo.dump> [-Fotos <archivo.tar.gz>]
param(
    [Parameter(Mandatory = $true)][string]$Bd,
    [string]$Fotos
)
$ErrorActionPreference = "Stop"

function Invoke-Paso([string]$Descripcion, [scriptblock]$Comando) {
    Write-Host "==> $Descripcion"
    & $Comando
    if ($LASTEXITCODE -ne 0) { throw "Fallo: $Descripcion (codigo $LASTEXITCODE)" }
}

$Bd = (Resolve-Path $Bd).Path
if ($Fotos) { $Fotos = (Resolve-Path $Fotos).Path }

Write-Host "Se reemplazara la base de datos 'urbia' con: $Bd"
if ($Fotos) { Write-Host "Se reemplazaran las fotografias con: $Fotos" }
$respuesta = Read-Host "Escribe SI para continuar"
if ($respuesta -cne "SI") { Write-Host "Cancelado."; exit 1 }

Push-Location (Join-Path $PSScriptRoot "..")
try {
    Invoke-Paso "Detener API e inferencia" { docker compose stop api inferencia }
    Invoke-Paso "Copiar el respaldo al contenedor" { docker compose cp $Bd db:/tmp/urbia.dump }
    Invoke-Paso "Borrar la base actual" {
        docker compose exec -T db psql -U postgres -d postgres -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS urbia WITH (FORCE);"
    }
    Invoke-Paso "Restaurar la base (con permisos y configuracion de roles)" {
        docker compose exec -T db pg_restore -U postgres -d postgres --create --exit-on-error /tmp/urbia.dump
    }
    Invoke-Paso "Borrar la copia temporal" { docker compose exec -T db rm -f /tmp/urbia.dump }
    if ($Fotos) {
        $carpeta = Split-Path $Fotos -Parent
        $nombre = Split-Path $Fotos -Leaf
        Invoke-Paso "Restaurar las fotografias" {
            docker run --rm -v urbia_storage:/datos -v "${carpeta}:/respaldo:ro" alpine:3.22 sh -c "find /datos -mindepth 1 -delete && tar xzf /respaldo/$nombre -C /datos"
        }
    }
}
finally {
    Write-Host "==> Arrancar API e inferencia"
    docker compose start api inferencia | Out-Null
    Pop-Location
}

Write-Host ""
Write-Host "Restauracion terminada. Revisa: docker compose ps"