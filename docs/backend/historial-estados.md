# Historial de estados (HU 07.3)

Trazabilidad de las acciones administrativas: la secuencia completa de estados de un
reporte, quién hizo cada cambio y cuándo. **Solo lectura**: el historial no se modifica
desde la API ni puede reescribirse en la base de datos.

## Endpoint

`GET /api/v1/reportes/{id}/historial` · `Authorization: Bearer <token>`

| Quién consulta | Respuesta |
|---|---|
| Administrador | `200` con todos los eventos y el actor completo (`id_usuario`, `nombre`, `rol`) |
| Autor del reporte | `200` con los eventos; del actor solo ve el `rol` |
| Otro ciudadano | `403` |
| Sin token | `401` |
| Reporte inexistente | `404` · id no numérico: `400` |
| `POST`, `PUT`, `PATCH`, `DELETE` | `405` (la ruta solo admite `GET`) |

```json
{
  "success": true,
  "data": {
    "id_reporte": 80,
    "estado_actual": "en_proceso",
    "total": 3,
    "solo_lectura": true,
    "eventos": [
      {
        "id_historial_estado": 160, "tipo": "creacion",
        "estado_anterior": null, "estado_nuevo": "pendiente",
        "fecha": "2026-11-17T03:00:00Z",
        "actor": { "id_usuario": 4, "nombre": "Ana", "rol": "ciudadano" }
      },
      {
        "id_historial_estado": 161, "tipo": "cambio_estado",
        "estado_anterior": "pendiente", "estado_nuevo": "en_revision",
        "fecha": "2026-11-17T03:05:00Z",
        "actor": { "id_usuario": 6, "nombre": "Juan", "rol": "administrador" }
      }
    ]
  }
}
```

- **Orden cronológico:** `ORDER BY fecha, id_historial_estado` (el id desempata eventos del mismo instante).
- **`tipo`:** `creacion` cuando `estado_anterior` es `null`; `cambio_estado` en los demás (HU 07.2).
- **Correos:** nunca se incluyen en el historial.

El **detalle** del reporte (`GET /api/v1/reportes/{id}`, HU 06.4) también trae `historial`
con las mismas reglas: el administrador ve el actor completo, el autor solo el rol y la
vista pública no trae historial.

## Inmutabilidad (migración `00009_historial_inmutable.sql`)

| Capa | Protección |
|---|---|
| API | Solo existe `GET` sobre el historial; el único que escribe es el cambio de estado (07.2), y solo **inserta**. |
| Permisos | `REVOKE UPDATE, DELETE, TRUNCATE ON historial_estado FROM urbia_app`: el backend no puede modificar ni borrar. |
| Trigger | `tg_historial_estado_sin_update` y `tg_historial_estado_sin_truncate` rechazan `UPDATE` y `TRUNCATE` para **cualquier** rol, incluido el dueño. |
| FK | `fk_historial_estado_reporte ON DELETE RESTRICT`: no se puede borrar un reporte con historial. |

Una corrección nunca edita un registro: se registra un nuevo cambio de estado.