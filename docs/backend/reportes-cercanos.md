# Reportes cercanos o posibles duplicados (HU 06.5)

Ayuda al administrador a detectar reportes que probablemente describen el **mismo
problema** (varios vecinos reportan el mismo bache). Son **sugerencias internas**:
la API no elimina, fusiona ni modifica ningún reporte; decidir qué hacer es del administrador.

## Endpoint

`GET /api/v1/reportes/{id}/cercanos` · `Authorization: Bearer <token de administrador>`

| Parámetro | Por defecto | Rango | Qué hace |
|---|---|---|---|
| `radio_m` | `DUPLICADOS_RADIO_M` (50) | 1 – 1000 | Distancia máxima en metros (filtro) |
| `ventana_dias` | `DUPLICADOS_VENTANA_DIAS` (30) | 1 – 365 | Diferencia de días que se considera "al mismo tiempo" (señal) |
| `limite` | 20 | 1 – 50 | Máximo de coincidencias devueltas |

- Solo `administrador` (`dominio.Rol.PuedeRevisarDuplicados`); un ciudadano recibe 403.
- `404` si el reporte no existe; `400 VALIDACION_FALLIDA` si algún parámetro está fuera de rango.
- Los valores por defecto se configuran en el `.env`; la API no arranca si están fuera de rango.

## Cómo se buscan

1. **Filtro geográfico (PostGIS):** `ST_DWithin(r.ubicacion::geography, base::geography, radio_m)`.
   Mide en **metros reales** y usa el índice GIST `ix_reporte_ubicacion`
   (`ubicacion::geography`). La distancia exacta sale de `ST_Distance` sobre geography.
   Se leen como máximo los 200 más cercanos.
2. **Señales complementarias** (no filtran, solo puntúan):
   - **Categoría:** misma categoría final (`misma_categoria`; `null` si alguno no tiene).
   - **Tiempo:** días entre las fechas de creación; `dentro_ventana` si son ≤ `ventana_dias`.
   - `mismo_autor` se informa, pero no suma puntos.
3. **Puntuación** (0 a 1) = 0.50 · cercanía + 0.35 · misma categoría + 0.15 · cercanía en el tiempo.
   - cercanía = `1 − distancia / radio`
   - cercanía en el tiempo = `1 − días / ventana` (0 fuera de la ventana)
4. **Nivel:** `alta` ≥ 0.70 · `media` ≥ 0.40 · `baja` el resto.
   Orden: puntuación descendente; a igual puntuación, el más cercano.

Ejemplos con radio 50 m y ventana 30 días:

| Caso | Puntuación | Nivel |
|---|---|---|
| Mismo punto, misma categoría, mismo día | 1.00 | alta |
| 25 m, misma categoría, 3 días después | 0.73 | alta |
| Mismo punto, otra categoría, mismo día | 0.65 | media |
| 40 m, misma categoría, hace 2 meses | 0.45 | media |
| 50 m, otra categoría, hace un año | 0.00 | baja |

## Respuesta

```json
{
  "success": true,
  "data": {
    "reporte": {
      "id_reporte": 40, "estado": "pendiente",
      "categoria": { "id_categoria": 1, "nombre": "bache" },
      "ubicacion": { "latitud": 20.5235, "longitud": -100.8157, "aproximada": false },
      "fecha_creacion": "2026-11-16T19:00:00Z"
    },
    "parametros": { "radio_m": 50, "ventana_dias": 30, "limite": 20 },
    "coincidencias": [
      {
        "id_reporte": 41, "estado": "pendiente",
        "categoria": { "id_categoria": 1, "nombre": "bache" },
        "descripcion": "Bache grande frente a la escuela",
        "ubicacion": { "latitud": 20.52355, "longitud": -100.81572, "aproximada": false },
        "fecha_creacion": "2026-11-16T19:05:00Z",
        "distancia_m": 6.1, "dias_diferencia": 0,
        "misma_categoria": true, "dentro_ventana": true, "mismo_autor": false,
        "puntuacion": 0.94, "nivel": "alta"
      }
    ],
    "total": 1
  }
}
```

`dias_diferencia` es negativo si el candidato es **anterior** al reporte revisado.
`total` cuenta los reportes dentro del radio antes de aplicar `limite`.

## Configuración

```env
DUPLICADOS_RADIO_M=50
DUPLICADOS_VENTANA_DIAS=30
```