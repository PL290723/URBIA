# Evidencias de v0.1.0-rc.1

Obtenidas sobre el commit de `logs/commit.txt`, con el stack de Docker Compose (CPU) en una
base creada desde cero y las cuentas de demostración. Resultado del checklist:
[docs/aceptacion.md](../../aceptacion.md).

Se generan con `deployments/scripts/evidencias.ps1`.

## Logs
| Archivo | Contenido |
|---|---|
| `logs/commit.txt` | Commit probado |
| `logs/go-test.txt` | `go vet ./...` y `go test ./... -count=1` |
| `logs/pytest.txt` | Pruebas del servicio de inferencia |
| `logs/frontend.txt` | `npm ci`, `typecheck`, `lint` y `build` del frontend |
| `logs/compose-ps.txt` / `logs/compose-images.txt` | Servicios, estado e imágenes |
| `logs/migraciones-status.txt` | Migraciones aplicadas (versión 9) |
| `logs/postgis.txt` | Versión de PostGIS y sus bibliotecas |
| `logs/health.json` | `GET /api/v1/health` con base e IA activas |

## Pruebas de la API (Bruno)
La colección completa está en [`pruebas/bruno/`](../../../pruebas/bruno/README.md): una carpeta
por historia de usuario con sus pruebas automáticas, más `11.1 Aceptacion` (30 requests:
flujo principal, IA caída y tras reinicio, en `local` y `feria`). Cualquiera puede volver a
ejecutarla siguiendo su README.

## Qué no se incluye (a propósito)
Archivos `.env`, tokens, entornos de trabajo de Bruno (`local`, `feria`), fotos reales de
prueba, logs de la API (IPs y datos de peticiones) y respaldos de la base.
