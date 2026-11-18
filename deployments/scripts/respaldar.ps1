# Respaldo de URBIA: base de datos (pg_dump) y fotografias (volumen urbia_storage).
# Pausa la API unos segundos para que base y fotos queden consistentes.
#
# Uso (desde la raiz del repo):
#   powershell -ExecutionPolicy Bypass -File deployments\scripts\respaldar.ps1
#   powershell -ExecutionPolicy Bypass -File deployments\scripts\respaldar.ps1 -Destino D:\respaldos
param(
    [string]$Destino = (Join-Path $HOME "urbia-respaldos")
)
$ErrorActionPreference = "Stop"

function Invoke-Paso([string]$Descripcion, [scriptblock]$Comando) {
    Write-Host "==> $Descripcion"
    & $Comando
    if ($LASTEXITCODE -ne 0) { throw "Fallo: $Descripcion (codigo $LASTEXITCODE)" }
}

$fecha = Get-Date -Format "yyyyMMdd-HHmm"
New-Item -ItemType Directory -Force -Path $Destino | Out-Null
$Destino = (Resolve-Path $Destino).Path
$archivoBd = "urbia-bd-$fecha.dump"
$archivoFotos = "urbia-fotos-$fecha.tar.gz"

Push-Location (Join-Path $PSScriptRoot "..")
try {
    Invoke-Paso "Pausar la API" { docker compose stop api }
    Invoke-Paso "Respaldar la base de datos" {
        docker compose exec -T db pg_dump -U postgres -d urbia -Fc --create -f /tmp/urbia.dump
    }
    Invoke-Paso "Copiar el respaldo de la base" { docker compose cp db:/tmp/urbia.dump (Join-Path $Destino $archivoBd) }
    Invoke-Paso "Borrar la copia temporal" { docker compose exec -T db rm -f /tmp/urbia.dump }
    Invoke-Paso "Respaldar las fotografias" {
        docker run --rm -v urbia_storage:/datos:ro -v "${Destino}:/respaldo" alpine:3.22 tar czf "/respaldo/$archivoFotos" -C /datos .
    }
}
finally {
    Write-Host "==> Reanudar la API"
    docker compose start api | Out-Null
    Pop-Location
}

Write-Host ""
Write-Host "Respaldo listo en $Destino"
Write-Host "  $archivoBd"
Write-Host "  $archivoFotos"