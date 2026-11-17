# Pruebas de integración y regresión (HU 09.4)

Validan el recorrido completo de un reporte, **desde la solicitud hasta la persistencia y
la consulta**, con los servicios reales (API Go, PostgreSQL + PostGIS, STORAGE_DIR y el
servicio de IA en Python). Se corren antes de cada entrega para confirmar que nada de lo
que ya funcionaba se rompió.

## El recorrido

| # | Paso | Qué se comprueba |
|---|---|---|
| 1 | Registrar un reporte con imagen y coordenadas | 201, `Location`, estado `pendiente`, ubicación GPS con precisión |
| 2 | Persistencia de reporte, archivo y metadatos | Fila en `reporte` con autor, descripción y **coordenadas exactas** (PostGIS); fila en `archivo` con variante, MIME, tamaño y SHA-256 **iguales al archivo en disco**; la descarga devuelve la misma foto; evento de creación en `historial_estado` |
| 3 | Se guarda la detección original | Una fila en `deteccion` con resultado, modelo y versión de Python, enlazada desde la sugerencia |
| 4 | Corregir la categoría | `reporte.id_categoria` cambia, `clasificacion` registra la corrección y la sugerencia, y **la fila de `deteccion` queda idéntica** (id, clase, confianza y fecha) |
| 5 | Cambiar estado e historial | Ciudadano → 403; administrador pendiente → en_revision → en_proceso → resuelto; resuelto es final (409); historial de 4 eventos a nombre del administrador; `historial_estado` no se puede modificar ni desde la cuenta de la API |
| 6 | Listado, detalle y estadísticas | "Mis reportes", filtro por estado y categoría, detalle del autor (exacto) y de un vecino (aproximado, sin foto), estadísticas del día +1 |
| 7 | Reinicio | Tras reiniciar la API (y en Bruno también Python y PostgreSQL), el reporte, la detección, el historial y la foto siguen iguales; los tokens siguen sirviendo; la IA vuelve a responder |

## Cómo correrlas

Requisitos: PostgreSQL con las migraciones, `backend-go/.env` válido y Python
(`python -m scripts.servidor_grpc`) apuntando al mismo `STORAGE_DIR` que Go.

### 1. Automática (Go)

```powershell
cd backend-go
go test -tags integracion ./internal/integracion/ -run Regresion -v -count=1   # solo el recorrido
go test -tags integracion ./internal/integracion/ -v -count=1                  # toda la integración (08.2 + 09.4)
```

Con una foto real (`$env:URBIA_IT_FOTO_BACHE = "C:\...\bache.jpeg"`) la IA sugiere `bache` y el
paso 4 es una **corrección**; con la foto sintética normalmente no hay sugerencia y queda como
elección **manual**. El paso 7 arma una API nueva con su propio pool y almacenamiento: es lo
mismo que reiniciar el proceso.

### 2. Manual con el modelo real (Bruno)

Colección `local`, carpeta **`09.4 Integración y regresión`** (usa `ana@test.mx` como
ciudadana y `juan@test.mx` como administrador):

1. Corre de la **00 a la 17** en orden con el Runner.
2. Reinicia todo:
   ```powershell
   # Ctrl+C en la terminal de la API y en la de Python
   Get-Service postgresql*                      # confirma el nombre del servicio
   Restart-Service postgresql-x64-18            # PowerShell como administrador
   # Levanta Python (python -m scripts.servidor_grpc) y luego la API (go run ./cmd/api)
   ```
3. Espera ~15 s y corre de la **18 a la 21**.

### 3. Regresión completa antes de una entrega

```powershell
cd backend-go;        go vet ./... ; go test ./... -cover
                      go test -tags integracion ./internal/integracion/ -v -count=1
cd ../inference-python; python -m pytest
```

Y en Bruno, las carpetas de la 06.1 a la 09.4. La 09.1 al final, porque su última prueba
bloquea el login 5 minutos.

## Registro de ejecuciones

| Fecha | `go test ./...` | Integración Go | `pytest` | Bruno 09.4 (00–17) | Reinicio (18–21) | Observaciones |
|---|---|---|---|---|---|---|
| 2026-11-17 | ok (response 100 %, middleware 91.1 %, handlers 33.1 %, services 40 %) | ok en 6.1 s (08.2 + regresión 7/7; reporte 97) | 224 passed, 2 skipped | OK| OK| Con `bache.jpeg` la IA sugirió bache y el paso 4 fue una corrección a basura. Tras reiniciar API, Python y PostgreSQL, el reporte nuevo (105) recibió `sugerida/basura` con confianza 0.8585. La 02 se corrigió: el sandbox de Bruno no tiene `Intl` para la zona horaria |