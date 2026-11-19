# Pruebas de integración Go–Python–PostgreSQL (HU 08.2)

Verifican el flujo completo de un reporte con servicios **reales** (sin dobles de prueba):

```
Bruno / prueba ──HTTP──▶ API Go ──SQL──▶ PostgreSQL + PostGIS
                           │  └──escribe──▶ STORAGE_DIR/originales/AAAA/MM/<id>.jpg
                           └──gRPC Detectar(ruta)──▶ Python ──lee (solo lectura)──┘
```

## Qué se prueba

| Subtarea | Prueba (`backend-go/internal/integracion/`) | Qué demuestra |
|---|---|---|
| Go ↔ PostgreSQL | `TestPostgres_GoSeConecta` | Usuario `urbia_app` (no superusuario), esquema `urbia`, migraciones aplicadas, PostGIS con `geography`, categorías activas, credenciales incorrectas fallan sin exponer la contraseña |
| Go ↔ Python (gRPC) | `TestGRPC_GoInvocaPython` | Mismo contrato (`1.0.0`), modelo y umbral informados, **cada clase del modelo existe como categoría activa en la BD** |
| Imágenes compartidas | `TestAlmacenamiento_CompartidoEntreGoYPython` | Lo que Go escribe en su `STORAGE_DIR` lo lee Python en el suyo con la misma ruta relativa, mismas dimensiones, y Python no modifica el archivo |
| Inferencia exitosa | `TestReporte_InferenciaExitosa` | `POST /reportes` → 201 con sugerencia, `Location`, `X-Request-ID`; fila en `deteccion` con modelo/versión de Python; foto en disco; el detalle muestra lo mismo |
| IA apagada | `TestReporte_IAApagada` | Sin configurar o con Python detenido: el reporte **se guarda** (201) con `no_disponible`, la detección queda como `error` con su código, no espera el timeout completo; `/health` queda `degradado` (no `caido`) |
| Errores | `TestErrores_BaseDeDatos`, `TestErrores_Timeout`, `TestErrores_ArchivoInexistente` | BD apagada al arrancar (falla en ≤ 5 s); BD caída en caliente (`/health` 503, login 500 sin 401 engañoso, no quedan fotos huérfanas); timeout → `TIEMPO_AGOTADO` reintentable y reporte guardado; archivo inexistente → `IMAGEN_NO_ENCONTRADA`, ruta maliciosa → `RUTA_INVALIDA`, foto borrada del disco → 404 controlado. Ninguna respuesta filtra detalles internos |
| Recorrido completo | `TestRegresion_RecorridoCompletoDeUnReporte` | Reporte -> persistencia -> detección -> corrección -> estados e historial -> consultas -> reinicio. Ver [regresion.md](regresion.md) |
Las pruebas arman la API con `internal/app.NuevoRouter`, **el mismo ensamblado que
`cmd/api`**; solo cambian las dependencias externas (pool, detector, monitor).

## Requisitos

1. PostgreSQL 18 + PostGIS con las migraciones aplicadas (`goose up`).
2. `backend-go/.env` válido (el mismo de la API).
3. El servidor de inferencia corriendo (`inference-python`, `python scripts/servidor_grpc.py`).
4. **`STORAGE_DIR` de Go y de Python apuntando a la misma carpeta.** Por defecto ya
   coinciden: Go usa `./storage` desde `backend-go/` y Python `../backend-go/storage`
   desde `inference-python/`. En Docker (Módulo 10) será el mismo volumen, montado en
   solo lectura para Python.

## Cómo correrlas

```powershell
cd backend-go
go test -tags integracion ./internal/integracion/ -v -count=1
```

- `-tags integracion`: sin la etiqueta estas pruebas no se compilan; `go test ./...` no las corre.
- `-count=1`: evita la caché de Go (el resultado depende de servicios externos).
- Sin `INFERENCE_GRPC_ADDR` las pruebas que necesitan a Python se marcan `SKIP`.

Variables opcionales:

| Variable | Para qué |
|---|---|
| `URBIA_IT_FOTO_BACHE` | Ruta a una foto real de bache; exige que la sugerencia sea `bache` |
| `URBIA_IT_LOG=1` | Muestra los logs de la API durante las pruebas |

Una sola prueba: `go test -tags integracion ./internal/integracion/ -run TestReporte_IAApagada -v -count=1`.

## Seguimiento entre servicios

Cada reporte de prueba se manda con un `X-Request-ID` que empieza con `it082-`. Go lo
envía a Python como `id_solicitud`, así que la misma petición aparece en los logs de
los dos servicios (`id_solicitud=it082-exitosa-…`).

## Datos que dejan

Usuarios `it-<n>@urbia.test` y reportes con la descripción `[IT 08.2] …`, más sus fotos
y detecciones. El historial es de solo inserción, así que no se borran desde las
pruebas. Para limpiarlos en una base de desarrollo, como `urbia_owner`:

```sql
SET search_path = urbia, public;
SELECT count(*) FROM reporte WHERE descripcion LIKE '[IT 08.2]%';
```

## Pruebas manuales (Bruno)

Carpeta `08.2 Integración Go-Python-PostgreSQL` de la colección `local`. Las requests
05–11 necesitan detener o reconfigurar un servicio antes; cada una lo indica en su
primera línea de *tests*.
