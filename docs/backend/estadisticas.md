# Estadísticas básicas (HU 07.4)

Agregados de los reportes para el panel administrativo. **Solo conteos**: la respuesta
nunca incluye ids de usuario, nombres, correos, descripciones, ids de reporte ni
coordenadas exactas.

## Endpoint

`GET /api/v1/estadisticas` · solo `administrador` (`dominio.Rol.PuedeVerEstadisticas`)

| Parámetro | Valores | Por defecto |
|---|---|---|
| `desde`, `hasta` | `AAAA-MM-DD` en hora de Celaya, ambos incluidos | Últimos 30 días hasta hoy. Solo `desde`: hasta hoy. Solo `hasta`: los 30 días que terminan ahí |
| `agrupacion` | `dia`, `semana` (lunes a domingo), `mes` | ≤ 31 días: `dia` · ≤ 120: `semana` · más: `mes` |
| `geo` | `true` / `false` | `false` |
| `celda_grados` | `0.005` a `0.1` (solo con `geo=true`) | `0.01` (≈ 1.1 km) |

- Rango máximo: **366 días**.
- Errores de validación → `400 VALIDACION_FALLIDA` (`fecha`, `orden`, `rango`, `valor_permitido`, `booleano`, `requiere_geo`).
- Ciudadano → `403` · sin token → `401` · otro método → `405`.

## Respuesta

```json
{
  "success": true,
  "data": {
    "filtros": { "desde": "2026-10-18", "hasta": "2026-11-16", "dias": 30,
                 "zona_horaria": "America/Mexico_City", "agrupacion": "dia" },
    "total": 42,
    "por_estado": [
      { "estado": "pendiente", "total": 20 },
      { "estado": "en_revision", "total": 9 },
      { "estado": "en_proceso", "total": 8 },
      { "estado": "resuelto", "total": 5 }
    ],
    "por_categoria": [
      { "id_categoria": 1, "nombre": "bache", "activa": true, "total": 18,
        "por_estado": { "pendiente": 9, "en_revision": 4, "en_proceso": 3, "resuelto": 2 } },
      { "id_categoria": null, "nombre": "sin_categoria", "activa": true, "total": 3,
        "por_estado": { "pendiente": 3, "en_revision": 0, "en_proceso": 0, "resuelto": 0 } }
    ],
    "por_periodo": [
      { "inicio": "2026-10-18", "total": 0 },
      { "inicio": "2026-10-19", "total": 3 }
    ],
    "distribucion_geografica": {
      "celda_grados": 0.01, "minimo_por_celda": 3,
      "celdas": [ { "latitud": 20.525, "longitud": -100.815, "total": 7 } ],
      "celdas_ocultas": 4, "reportes_ocultos": 6
    }
  }
}
```

| Bloque | Cómo se calcula |
|---|---|
| `total` | Reportes creados en el rango (`fecha_creacion`). Es la suma de `por_estado`, de `por_categoria` y de `por_periodo`. |
| `por_estado` | Siempre los 4 estados en orden del ciclo de vida, aunque valgan 0. |
| `por_categoria` | Una entrada por categoría con su desglose por estado. Incluye las categorías **activas sin reportes** (total 0) y las inactivas que sí tienen. Orden: más reportes primero; `sin_categoria` siempre al final. |
| `por_periodo` | Serie **completa** (periodos sin reportes con 0), agrupada en hora de Celaya. `inicio` es el día, el lunes de la semana o el día 1 del mes; en semana/mes el primer `inicio` puede ser anterior a `desde`, pero solo cuenta reportes dentro del rango. |
| `distribucion_geografica` | Solo con `geo=true`. Rejilla de `celda_grados`; cada celda trae su **centro**, no la ubicación de un reporte. |

## Datos personales

- Solo se devuelven conteos agrupados.
- **Celdas con menos de 3 reportes se ocultan** (`MinReportesPorCelda`): con 1 o 2 reportes
  una celda podría señalar la casa de quien reportó. La respuesta dice cuántas celdas y
  reportes se ocultaron, para que los totales sigan cuadrando:
  `Σ celdas.total + reportes_ocultos = total`.

## Consistencia

Todas las consultas corren en **una transacción de solo lectura `REPEATABLE READ`**: si
llega un reporte mientras se calculan, todos los bloques ven la misma foto de los datos.