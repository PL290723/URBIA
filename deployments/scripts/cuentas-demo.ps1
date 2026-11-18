# Crea las cuentas de demostracion de URBIA (NO productivas) y promueve al administrador.
# La contrasena del administrador no se guarda en el repositorio: se pasa al ejecutar.
#
# Uso (desde la raiz del repo, con el stack levantado):
#   powershell -ExecutionPolicy Bypass -File deployments\scripts\cuentas-demo.ps1 -ContrasenaAdmin <8 a 72 caracteres>
param(
    [Parameter(Mandatory = $true)][string]$ContrasenaAdmin,
    [string]$ContrasenaCiudadano = "DemoUrbia2026",
    [string]$Api = "http://localhost:8080"
)
$ErrorActionPreference = "Stop"

$correoCiudadano = "ciudadano.demo@urbia.test"
$correoAdmin = "admin.demo@urbia.test"

function Registrar([string]$Nombre, [string]$Correo, [string]$Contrasena) {
    $cuerpo = @{ nombre = $Nombre; correo = $Correo; contrasena = $Contrasena } | ConvertTo-Json
    try {
        Invoke-RestMethod -Method Post -Uri "$Api/api/v1/auth/registro" `
            -ContentType "application/json; charset=utf-8" -Body $cuerpo | Out-Null
        Write-Host "Creada: $Correo"
    }
    catch {
        $codigo = $_.Exception.Response.StatusCode.value__
        if ($codigo -eq 409) { Write-Host "Ya existia: $Correo (se conserva su contrasena)" }
        else { throw }
    }
}

Registrar "Ciudadano Demo" $correoCiudadano $ContrasenaCiudadano
Registrar "Administrador Demo" $correoAdmin $ContrasenaAdmin

Push-Location (Join-Path $PSScriptRoot "..")
try {
    docker compose exec -T db psql -U postgres -d urbia -v ON_ERROR_STOP=1 -c "UPDATE urbia.usuario SET rol = 'administrador', fecha_actualizacion = now() WHERE lower(correo) = lower('$correoAdmin');"
    if ($LASTEXITCODE -ne 0) { throw "No se pudo promover al administrador" }
}
finally {
    Pop-Location
}

Write-Host ""
Write-Host "Cuentas listas (solo para pruebas y demostracion):"
Write-Host "  ciudadano: $correoCiudadano"
Write-Host "  admin:     $correoAdmin (rol administrador)"