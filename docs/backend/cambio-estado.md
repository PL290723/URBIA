# Cambio de estado administrativo (HU 07.2)

El administrador avanza el ciclo de vida de un reporte. Cada cambio queda en
`historial_estado` con el estado anterior, el nuevo, la fecha y **quién** lo hizo.

## Endpoint

`PATCH /api/v1/reportes/{id}/estado` · `Authorization: Bearer <token de administrador>`

```json
{ "estado": "en_revision" }
```

| Respuesta | Cuándo |
|---|---|
| `200` | Cambio aplicado |
| `400 VALIDACION_FALLIDA` | `estado` vacío o fuera de la lista (`pendiente`, `en_revision`, `en_proceso`, `resuelto`) |
| `400 SOLICITUD_INVALIDA` | JSON mal formado o id no numérico |
| `401` | Sin token o token vencido |
| `403` | El usuario no es administrador (`dominio.Rol.PuedeCambiarEstado`) |
| `404` | El reporte no existe |
| `409 CONFLICTO` | Transición no permitida, mismo estado o reporte ya `resuelto` |

## Transiciones permitidas

```
pendiente ──► en_revision ──► en_proceso ──► resuelto (final)
```

Solo hacia adelante y de uno en uno. Están en la tabla `transicion_estado`
(migración 00003) y se validan **dos veces**:

1. En Go (`dominio.MotivoCambioInvalido`), para responder un 409 claro.
2. En la BD (trigger `tg_reporte_transicion_estado`), por si alguien cambia el estado
   por otro camino. Si el trigger lo rechaza, la API responde 409.

El 409 explica qué hacer:

```json
{
  "success": false,
  "error": {
    "code": "CONFLICTO",
    "message": "No se puede pasar de pendiente a resuelto; desde pendiente solo se permite: en_revision",
    "details": {
      "estado_actual": "pendiente",
      "estado_solicitado": "resuelto",
      "estados_permitidos": ["en_revision"]
    }
  }
}
```

## Cómo se guarda

Todo en **una transacción** (`EstadoRepository.Cambiar`):

1. `SELECT … FOR UPDATE` bloquea el reporte y lee el estado actual.
2. Se valida la transición contra ese estado.
3. `UPDATE reporte SET estado = …`; el trigger pone `fecha_actualizacion = now()`.
4. `INSERT INTO historial_estado (id_reporte, id_usuario, estado_anterior, estado_nuevo)`
   con el administrador del token.

Por el bloqueo, si dos administradores cambian el mismo reporte a la vez, el segundo
espera y se valida contra el estado que dejó el primero: el historial nunca queda con
un estado anterior equivocado.

## Respuesta

```json
{
  "success": true,
  "data": {
    "id_reporte": 70,
    "estado_anterior": "pendiente",
    "estado_nuevo": "en_revision",
    "id_historial_estado": 145,
    "cambiado_por": 6,
    "fecha": "2026-11-17T02:10:00Z",
    "fecha_actualizacion": "2026-11-17T02:10:00Z",
    "estados_siguientes": ["en_proceso"]
  }
}
```

## Efecto en otras HU

- **06.3:** en cuanto el reporte sale de `pendiente`, el ciudadano ya no puede cambiar su categoría (409).
- **06.4:** el detalle del reporte muestra el nuevo registro en `historial`.
- **07.1:** un reporte `resuelto` ya no admite cálculo de prioridad (409).