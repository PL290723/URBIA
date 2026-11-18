# Checklist de aceptación del backend (HU 11.1)

Comprueba que las funciones comprometidas del MVP funcionan **en el despliegue de entrega**
(Docker Compose, base nueva) antes de cerrar el backend.

- Entorno: `deployments/compose.yaml` (CPU), base creada desde cero, cuentas de
  [demostración](operacion.md#3-cuentas-de-prueba-no-productivas).
- Evidencia: carpeta de Bruno `11.1 Aceptacion` (colección `local`), salidas de los comandos
  y pruebas automáticas.
- Un criterio se acepta solo si **todas** sus verificaciones pasan.

## Procedimiento

Comandos `docker compose` desde `deployments/`; scripts desde la raíz del repo.
No cerrar Bruno entre fases: guarda ids y tokens en variables de ejecución.

### Fase 0. Pruebas automáticas (sin Docker)
```powershell
cd backend-go; go vet ./... ; go test ./... -count=1
cd ../inference-python; python -m pytest
```

### Fase 1. Despliegue desde cero
```powershell
cd deployments
docker compose down -v
docker compose up -d --build
docker compose ps
docker compose logs migraciones
docker compose run --rm migraciones status
docker compose exec db psql -U postgres -d urbia -c "SELECT postgis_full_version();"
cd ..
powershell -ExecutionPolicy Bypass -File deployments\scripts\cuentas-demo.ps1 -ContrasenaAdmin "<contraseña>"
```

### Fase 2. Flujo principal
Bruno, entorno `local`, Runner sobre `11.1 Aceptacion / A Flujo principal` (01-24). Después:
```powershell
cd deployments
docker compose exec api find /app/storage -type f
```

### Fase 3. Falla de la IA
```powershell
docker compose stop inferencia
```
Esperar 20 s. Runner sobre `B IA caida` (25-27). Después:
```powershell
docker compose start inferencia
```

### Fase 4. Reinicio sin pérdida de datos
```powershell
docker compose restart
docker compose down
docker compose up -d
docker compose ps
```
Esperar a que todo esté `healthy` (hasta ~90 s). Runner sobre `C Tras reinicio` (28-30).

### Fase 5. Red local
Seguir [deployments/feria.md](../deployments/feria.md) (IP, red privada, firewall). Runner sobre
`C Tras reinicio` con el entorno `feria` (`baseUrl = http://<IP>:8080`) y abrir en un celular de
la misma red `http://<IP>:8080/api/v1/health/live`.

## Checklist

## Checklist

| # | Criterio | Verificación | Resultado |
|---|---|---|---|
| 1 | API inicia y healthcheck responde | F1: `api` `healthy`. Bruno 01: 200, `status: ok`, base e inferencia `activo` | [x] |
| 2 | Base PostgreSQL/PostGIS inicia y migraciones aplican desde cero | F1: tras `down -v`, `db` `healthy`; `migraciones` `exited (0)` en versión 9; `status` sin pendientes; PostGIS 3.6 sobre PostgreSQL 18 | [x] |
| 3 | Autenticación y roles funcionan | Bruno 02-06: login de ambos roles, perfil con rol, 401 sin token, 403 ciudadano en ruta de administrador | [x] |
| 4 | Se puede crear un reporte con imagen y ubicación | Bruno 07: 201, `Location`, `pendiente`, foto y coordenadas GPS | [x] |
| 5 | La imagen queda en almacenamiento persistente | Bruno 08: descarga `image/jpeg`. F2: archivo en `/app/storage/originales`. Bruno 30: sigue tras `down`/`up` | [x] |
| 6 | La inferencia produce sugerencia o resultado no concluyente | Bruno 07: sugerencia `sugerida` o `sin_deteccion`. Bruno 10: detección `detectado` o `no_detectado` con modelo y versión | [x] |
| 7 | El reporte se conserva cuando la IA falla | F3: Bruno 25 `degradado`; 26: 201 con sugerencia `no_disponible`; 27: el reporte existe con su foto | [x] |
| 8 | El usuario puede confirmar o corregir la categoría | Bruno 09: 200, categoría nueva, origen `corregida` (o `manual` si no hubo sugerencia) | [x] |
| 9 | Se conserva la detección original | Bruno 09: `id_deteccion` igual al de la creación. Bruno 10: misma detección y clase tras la corrección | [x] |
| 10 | Se puede consultar listado y detalle | Bruno 10-12: detalle de administrador y del autor (ubicación exacta), "mis reportes" | [x] |
| 11 | La búsqueda geográfica funciona | Bruno 13-15: `bbox` que contiene el reporte lo lista, `bbox` lejano no; cercanos responde con parámetros | [x] |
| 12 | La prioridad tiene explicación y factores | Bruno 16-17: 4 factores con explicación, resumen, versión de fórmula; consulta con vigencia | [x] |
| 13 | El administrador puede cambiar estado y consultar historial | Bruno 18-23: ciudadano 403; pendiente → en_revision → en_proceso → resuelto; resuelto final (409); historial de 4 eventos | [x] |
| 14 | Las estadísticas básicas responden | Bruno 24: 200, total y desgloses | [x] |
| 15 | Los contenedores se reinician sin pérdida de datos | F4: `restart` y `down`/`up`; Bruno 28-30: reporte, categoría, detección, estado y foto intactos; tokens válidos | [x] |
| 16 | La API puede consumirse desde otro dispositivo de la red local | F5: Bruno 28-30 por la IP local y `/health/live` desde un celular | [ ] |

## Registro de ejecución

## Registro de ejecución

| Campo | Valor |
|---|---|
| Fecha | 2026-11-18 |
| Commit de `develop` | `9910f74` |
| Equipo | Acer Nitro AN515-58 (i5-12500H, 16 GB), Windows + Docker Desktop (WSL 2), inferencia en CPU |
| `go test ./...` | ok: 13 paquetes con pruebas sin fallos (auth, clients, config, dominio, handlers, imagenes, middleware, repositories, response, router, services, storage); 6 sin archivos de prueba |
| `pytest` | 225 passed, 1 skipped (enlaces simbólicos, no aplica en Windows) en 59 s; Python 3.11.9 |
| Despliegue desde cero | `down -v` + `up -d --build`: `db`, `api` e `inferencia` healthy; migraciones 00001-00009 aplicadas en 75 ms en total, versión 9, sin pendientes; PostGIS 3.6.4 sobre PostgreSQL 18 (GEOS 3.14.1, PROJ 9.8.1) |
| Health inicial | `ok`; base de datos `activo` (0 ms), inferencia `activo` (4 ms); versión 0.1.0 |
| Almacenamiento | Fotos originales en el volumen: `/app/storage/originales/2026/11/<id aleatorio de 26 caracteres>.jpg` |
| Bruno A (01-24) | 24/24 |
| Bruno B (25-27) | 3/3 |
| Bruno C (28-30) local / feria | 3/3 / pendiente |
| Reporte de prueba (id) / sugerencia | 2 / `sugerida`: bache (corregida a basura en la prueba 09) |
| Resultado | Pendiente: criterio 16 (red local) |
| Observaciones | En los logs de `migraciones` aparece una segunda ejecución a las 10:29 con "no migrations to run": es normal, el servicio corre `goose up` en cada `docker compose up` y es idempotente. El reporte de prueba es el 2 porque ya existía uno de un intento anterior; por eso hay dos fotos en el volumen. |

## Pendientes y limitaciones aceptadas
Las limitaciones conocidas del MVP están en [solucion-problemas.md](solucion-problemas.md#3-limitaciones-conocidas-del-mvp)
y no bloquean la aceptación. Cualquier criterio que falle se registra aquí con su issue o tarea.
