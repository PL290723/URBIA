# Confirmación y corrección de la categoría (HU 06.3)

Después de crear el reporte (06.1) y recibir la sugerencia de la IA (06.2), el ciudadano
**confirma** la categoría sugerida o la **corrige** por otra. La decisión del ciudadano es
la que queda en el reporte; la detección original de la IA se conserva sin cambios.

## Endpoint

`PUT /api/v1/reportes/{id}/categoria` · `Authorization: Bearer <token>` · JSON

```json
{ "id_categoria": 3 }
```

- Solo el rol `ciudadano` (`dominio.Rol.PuedeConfirmarCategoria`) y solo **su propio**
  reporte. Un reporte ajeno responde 404, igual que uno inexistente.
- Solo mientras el reporte está `pendiente` (`dominio.Estado.PermiteCambiarCategoria`).
  Cuando un administrador lo pasa a revisión, la categoría ya no se cambia desde aquí.
- Se puede confirmar varias veces mientras siga pendiente; cada vez queda registrada.

## Origen de la categoría

| `origen` | Cuándo |
|---|---|
| `sugerida` | La categoría elegida es la que sugirió la IA en su última detección |
| `corregida` | La IA sugirió una categoría y el ciudadano eligió otra |
| `manual` | No había sugerencia: la IA falló, no detectó nada o su clase no es una categoría activa. También se usa al **crear** el reporte con categoría (la eligió antes de ver la sugerencia) |

La sugerencia vigente es la **última** fila de `deteccion` del reporte con
`resultado = 'detectado'` cuya clase corresponde a una categoría activa.

## Datos (migración 00007)

- `reporte.origen_categoria` (`sugerida` / `corregida` / `manual`). Va junto con
  `id_categoria`: ambos o ninguno (`ck_reporte_categoria_origen`).
- Tabla `clasificacion`: una fila por confirmación, con la categoría final, la anterior,
  la sugerida, la detección con la que se comparó y el origen.
  `ck_clasificacion_coherencia` impide guardar un origen que no cuadre con la sugerencia.
- `deteccion` **no se modifica**: `clasificacion.id_deteccion` solo la referencia.
- Todo en una transacción con `SELECT … FOR UPDATE`: si el reporte cambia de estado
  mientras tanto, se responde 409 y no se guarda nada.

## Respuestas

| Código | Cuándo |
|---|---|
| 200 | Confirmada; `data` = la clasificación registrada |
| 400 `VALIDACION_FALLIDA` | Falta `id_categoria`, no es > 0, o la categoría no existe o no está activa (`no_existe`) |
| 400 `SOLICITUD_INVALIDA` | El cuerpo no es JSON válido o el id del reporte no es un entero positivo |
| 401 | Sin token o token inválido |
| 403 | El rol no es `ciudadano` |
| 404 | El reporte no existe o es de otro ciudadano |
| 409 `CONFLICTO` | El reporte ya no está `pendiente` |

```json
{
  "success": true,
  "data": {
    "id_clasificacion": 4,
    "id_reporte": 12,
    "categoria": { "id_categoria": 1, "nombre": "bache" },
    "categoria_anterior": null,
    "categoria_sugerida": { "id_categoria": 3, "nombre": "basura" },
    "id_deteccion": 9,
    "origen": "corregida",
    "fecha": "2026-11-16T08:30:00Z"
  }
}
```

## Auditoría

Diferencias entre la IA y los ciudadanos:

```sql
SELECT cl.id_reporte, cl.origen, sug.nombre AS sugerida, fin.nombre AS final,
       d.confianza, d.version_modelo, cl.fecha
FROM urbia.clasificacion cl
JOIN urbia.categoria fin ON fin.id_categoria = cl.id_categoria
LEFT JOIN urbia.categoria sug ON sug.id_categoria = cl.id_categoria_sugerida
LEFT JOIN urbia.deteccion d   ON d.id_deteccion   = cl.id_deteccion
ORDER BY cl.fecha DESC;
```

Tasa de aciertos de la IA por versión del modelo:

```sql
SELECT d.version_modelo,
       count(*) FILTER (WHERE cl.origen = 'sugerida')  AS aceptadas,
       count(*) FILTER (WHERE cl.origen = 'corregida') AS corregidas
FROM urbia.clasificacion cl
JOIN urbia.deteccion d ON d.id_deteccion = cl.id_deteccion
WHERE cl.origen IN ('sugerida', 'corregida')
GROUP BY d.version_modelo;
```