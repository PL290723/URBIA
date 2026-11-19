# Genera los logs de evidencia de una version en docs/evidencias/<Version>/logs.
# Requiere el stack de Docker Compose levantado (todo healthy), Go, Node.js y el venv de
# inference-python (.venv). No guarda logs de la API (tienen IPs y datos de peticiones).
#
# Uso (desde la raiz del repo):
#   powershell -ExecutionPolicy Bypass -File deployments\scripts\evidencias.ps1 [-Version v0.1.0-rc.1]
param(
    [string]$Version = "v0.1.0-rc.1"
)
$ErrorActionPreference = "Stop"

$raiz = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$logs = Join-Path $raiz "docs\evidencias\$Version\logs"
New-Item -ItemType Directory -Force $logs | Out-Null

function Guardar([string]$Descripcion, [string]$Carpeta, [string]$Comando, [string]$Archivo) {
    Write-Host "==> $Descripcion"
    Push-Location (Join-Path $raiz $Carpeta)
    try {
        cmd /c "$Comando 2>&1" | Out-File -Encoding utf8 (Join-Path $logs $Archivo)
        if ($LASTEXITCODE -ne 0) { Write-Warning "$Descripcion termino con codigo $LASTEXITCODE (revisa $Archivo)" }
    }
    finally { Pop-Location }
}

git -C $raiz rev-parse HEAD | Out-File -Encoding utf8 (Join-Path $logs "commit.txt")

Guardar "Go: vet y pruebas" "backend-go" "go vet ./... && go test ./... -count=1" "go-test.txt"
Guardar "Python: pytest" "inference-python" ".venv\Scripts\python.exe -m pytest" "pytest.txt"
Guardar "Frontend: tipos, lint y build" "frontend" "npm ci --no-audit --no-fund && npm run typecheck && npm run lint && npm run build" "frontend.txt"
Guardar "Compose: servicios" "deployments" "docker compose ps" "compose-ps.txt"
Guardar "Compose: imagenes" "deployments" "docker compose images" "compose-images.txt"
Guardar "Migraciones" "deployments" "docker compose run --rm migraciones status" "migraciones-status.txt"
Guardar "PostGIS" "deployments" "docker compose exec -T db psql -U postgres -d urbia -At -c ""SELECT postgis_full_version();""" "postgis.txt"
Guardar "Health de la API" "deployments" "curl.exe -s http://localhost:8080/api/v1/health" "health.json"

Write-Host "==> Revision de datos sensibles"
$hallazgos = Select-String -Path (Join-Path $logs "*") -Pattern "eyJ[A-Za-z0-9_-]{10,}|Bearer |JWT_SECRET=|contrasena|password="
if ($hallazgos) {
    $hallazgos | ForEach-Object { Write-Warning $_ }
    throw "Hay posibles datos sensibles en los logs: revisalos antes de hacer commit."
}

Write-Host ""
Write-Host "Evidencias listas en $logs"
Get-ChildItem $logs | Format-Table Name, Length
