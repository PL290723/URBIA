# Consulta de reportes: listado y detalle (HU 06.4)

Datos para el mapa, el listado y la vista de detalle del frontend. Las rutas usan
`/reportes` (en español), igual que el resto de la API.

| Método | Ruta | Uso |
|---|---|---|
| GET | `/api/v1/reportes` | Listado paginado con filtros (mapa y lista) |
| GET | `/api/v1/reportes/{id}` | Detalle de un reporte |

Ambas requieren `Authorization: Bearer <token>`; las pueden usar ciudadanos y
administradores. Lo que cambia es **cuánto** ve cada quién.

## Qué ve cada quién (`dominio.Vista`)

| Dato | Reporte ajeno (`publica`) | Reporte propio (`autor`) | Administrador |
|---|---|---|---|
| id, estado, categoría, descripción, fechas | ✔ | ✔ | ✔ |
| Ubicación | **aproximada** (3 decimales ≈ 110 m, `aproximada: true`) | exacta | exacta |
| Foto (`id_foto` en el listado, metadatos + `url` en el detalle) | ✖ | ✔ | ✔ |
| Sugerencia de la IA (estado, categoría, confianza) | ✖ | ✔ | ✔ |
| Historial de estados | ✖ | ✔ | ✔ |
| `administrativo`: autor (nombre y correo), prioridad, origen/precisión de la ubicación, origen de la categoría | ✖ | ✖ | ✔ |
| `administrativo.deteccion`: resultado, clase, confianza, modelo, versión, código de error | ✖ | ✖ | ✔ (solo detalle) |

- Los campos no permitidos **no aparecen** en el JSON (no van como `null`).
- La imagen se descarga en `GET /api/v1/archivos/{id}`, que aplica la misma política
  (autor o administrador): los metadatos y el archivo tienen el mismo acceso controlado.
- El nombre y el correo del autor nunca se envían a otros ciudadanos.
- Con `bbox`, a los ciudadanos se les **agranda el rectángulo** a la misma cuadrícula de
  3 decimales: así no se puede averiguar la ubicación exacta de un reporte ajeno
  moviendo el mapa en pasos muy pequeños.

## Listado: parámetros

| Parámetro | Ejemplo | Regla |
|---|---|---|
| `pagina` | `2` | 1 – 10000 (por defecto 1) |
| `por_pagina` | `50` | 1 – 100 (por defecto 20) |
| `estado` | `pendiente,en_revision` | uno o varios estados válidos |
| `id_categoria` | `3` | entero positivo |
| `desde`, `hasta` | `2026-11-01` | `AAAA-MM-DD`, hora de Celaya (UTC-6); ambos días incluidos |
| `bbox` | `-100.85,20.50,-100.78,20.55` | `min_lon,min_lat,max_lon,max_lat` (orden de Leaflet) |
| `mios` | `true` | solo los reportes del usuario del token |

Orden: más recientes primero (`fecha_creacion DESC, id_reporte DESC`).
Errores de parámetros: 400 `VALIDACION_FALLIDA` con `details` por campo.

```json
{
  "success": true,
  "data": {
    "elementos": [
      {
        "id_reporte": 12,
        "estado": "pendiente",
        "categoria": { "id_categoria": 3, "nombre": "basura" },
        "descripcion": "Basura acumulada en la esquina",
        "ubicacion": { "latitud": 20.524, "longitud": -100.816, "aproximada": true },
        "fecha_creacion": "2026-11-16T16:00:00Z",
        "es_propio": false
      }
    ],
    "paginacion": { "pagina": 1, "por_pagina": 20, "total": 1, "total_paginas": 1 }
  }
}
```

## Detalle

`404` si el reporte no existe. Ejemplo para el **autor**:

```json
{
  "success": true,
  "data": {
    "id_reporte": 12,
    "estado": "pendiente",
    "categoria": { "id_categoria": 3, "nombre": "basura" },
    "descripcion": "Basura acumulada en la esquina",
    "ubicacion": { "latitud": 20.523512, "longitud": -100.815788, "aproximada": false },
    "fecha_creacion": "2026-11-16T16:00:00Z",
    "fecha_actualizacion": "2026-11-16T16:05:00Z",
    "es_propio": true,
    "foto": { "id_archivo": 30, "mime": "image/jpeg", "tamano": 184233,
              "fecha_subida": "2026-11-16T16:00:00Z", "url": "/api/v1/archivos/30" },
    "sugerencia": { "estado": "sugerida", "categoria": { "id_categoria": 3, "nombre": "basura" }, "confianza": 0.91 },
    "historial": [ { "estado_anterior": null, "estado_nuevo": "pendiente", "fecha": "2026-11-16T16:00:00Z" } ]
  }
}
```

## Índices que se aprovechan

`ix_reporte_estado`, `ix_reporte_id_categoria`, `ix_reporte_fecha_creacion`,
`ix_reporte_id_usuario` y `ix_reporte_ubicacion` (GIST sobre `ubicacion::geography`,
usado por el filtro `bbox` con `ST_Intersects`). No hace falta migración nueva.