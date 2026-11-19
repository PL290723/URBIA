# Imagen Docker del backend Go (HU 10.1)

## Construcción
Multi-stage: `golang:1.27-alpine` compila un binario estático (`CGO_ENABLED=0`) y
`alpine:3.22` solo lo ejecuta. `openapi.yaml` va embebido en el binario.

```powershell
docker build -t urbia-api:0.1.0 --build-arg VERSION=0.1.0 .\backend-go
```

## Ejecución
| Aspecto | Valor |
|---|---|
| Usuario | `urbia` (UID/GID 10001), no root |
| Puerto | 8080 (`PORT`) |
| Fotografías | `/app/storage` (montar un volumen) |
| Healthcheck | `GET /api/v1/health` cada 30 s (503 si PostgreSQL no responde) |
| Apagado | SIGTERM → apagado ordenado en `SHUTDOWN_TIMEOUT` |

Valores por defecto en la imagen: `APP_ENV=production`, `GIN_MODE=release`,
`LOG_LEVEL=info`, `STORAGE_DIR=/app/storage`. `DATABASE_URL` y `JWT_SECRET` son
obligatorias y se pasan al ejecutar; nunca se copian a la imagen (`.dockerignore`).

```powershell
docker run -d --name urbia-api -p 8080:8080 `
  --env-file .\backend-go\.env.docker `
  -v urbia_storage:/app/storage urbia-api:0.1.0
```

## PostgreSQL en Windows (fuera de Docker)
Desde el contenedor, `localhost` es el propio contenedor. Usar `host.docker.internal`:
`postgres://urbia_app:...@host.docker.internal:5432/urbia?sslmode=disable&search_path=urbia,public`.
Si PostgreSQL rechaza con `no pg_hba.conf entry for host "X"`, agregar esa IP en
`pg_hba.conf` y recargar con `SELECT pg_reload_conf();`.

## Notas para Compose (HU 10.2)
- El servicio de inferencia lee `/app/storage` en solo lectura: los archivos son 0640,
  así que debe correr con el GID 10001.
- `stop_grace_period` mayor que `SHUTDOWN_TIMEOUT` (10 s), por ejemplo 15 s.
