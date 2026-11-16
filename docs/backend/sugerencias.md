# Sugerencia de categoría con IA (HU 06.2)

Al crear un reporte (`POST /api/v1/reportes`, HU 06.1), la API pide al servicio de
inferencia (Python, gRPC, HU 05.4/05.5) una **sugerencia de categoría** para la foto.
La sugerencia se guarda en `deteccion` y se devuelve en la respuesta.

## Decisión: la inferencia va DESPUÉS de guardar el reporte

```
validar campos → validar y sanear foto → TRANSACCIÓN (reporte + archivo + historial) → COMMIT
                                                                                         │
                                                       ClienteDeteccion.Detectar ◀───────┘
                                                       guardar fila en deteccion
                                                       201 con data.sugerencia
```

- El reporte **nunca se pierde** por la IA: cuando se llama al modelo ya está confirmado.
- Si la IA falla, tarda demasiado o no está configurada, la respuesta sigue siendo **201**
  con `sugerencia.estado = "no_disponible"`, y en `deteccion` queda una fila `error`
  con su `codigo_error` (para reintentarla después).
- La llamada usa `context.WithoutCancel`: si el cliente cierra la conexión, la
  detección se termina y se guarda igual. El límite de tiempo es
  `INFERENCE_GRPC_TIMEOUT` (10 s); Python corta antes (`URBIA_TIMEOUT_INFERENCIA`, 8 s).
- Se descartó inferir **antes** del commit: un fallo o una demora del modelo
  bloquearía o tumbaría el registro del reporte.

## La categoría del ciudadano no se toca

- `reporte.id_categoria` **nunca** se modifica con la sugerencia.
- Si el ciudadano eligió categoría, se devuelve `coincide_con_categoria` (true/false)
  solo como información.
- Si no eligió, el reporte queda sin categoría; la app puede mostrar la sugerencia y
  que el ciudadano o un administrador la confirmen después.

## Respuesta: `data.sugerencia`

| Campo | Descripción |
|---|---|
| `estado` | `sugerida` · `sin_deteccion` · `no_disponible` |
| `categoria` | `{id_categoria, nombre}` sugerida, o `null` |
| `confianza` | 0–1 (4 decimales), o `null` |
| `coincide_con_categoria` | `true`/`false` si hay categoría del ciudadano y sugerida; si no, `null` |
| `modelo`, `version_modelo` | p. ej. `urbia-yolo26s` / `1.0.0`; `null` si la IA no respondió |
| `id_deteccion` | fila guardada en `deteccion`; `null` si no se pudo guardar |

| `estado` | Cuándo |
|---|---|
| `sugerida` | El modelo detectó una clase y existe una categoría activa con ese nombre |
| `sin_deteccion` | Ninguna caja superó el umbral, o la clase no corresponde a una categoría activa |
| `no_disponible` | Error de la IA o `INFERENCE_GRPC_ADDR` vacía |

## Tabla `deteccion` (migración 00006)

Se guarda la detección **original** del modelo, aunque después cambie la categoría:

| Columna | `detectado` | `no_detectado` | `error` |
|---|---|---|---|
| `resultado` | `detectado` | `no_detectado` | `error` |
| `clase_detectada`, `confianza` | valor | NULL | NULL |
| `modelo`, `version_modelo` | valor | valor | NULL |
| `umbral_confianza`, `milisegundos` | valor | valor | NULL |
| `codigo_error` | NULL | NULL | `SERVICIO_NO_DISPONIBLE`, `TIEMPO_AGOTADO`, `NO_CONFIGURADO`, … |

Restricciones nuevas: `ck_deteccion_modelo` (sin modelo solo con `error`),
`ck_deteccion_error` (`codigo_error` si y solo si `error`), `ck_deteccion_umbral`,
`ck_deteccion_milisegundos`, e índice parcial `ix_deteccion_error`.

Reportes cuya inferencia falló:

```sql
SELECT r.id_reporte, d.codigo_error, d.fecha
FROM urbia.deteccion d JOIN urbia.reporte r USING (id_reporte)
WHERE d.resultado = 'error' ORDER BY d.fecha DESC;
```

## Configuración

| Variable (Go) | Valor | Nota |
|---|---|---|
| `INFERENCE_GRPC_ADDR` | `localhost:50051` | Vacía = sin IA (todo `no_disponible`) |
| `INFERENCE_GRPC_TIMEOUT` | `10s` | Mayor que `URBIA_TIMEOUT_INFERENCIA` de Python |

Python debe leer **la misma carpeta** que Go: `STORAGE_DIR` del servicio Python =
`STORAGE_DIR` de la API (en local, la ruta absoluta de `backend-go/storage`).