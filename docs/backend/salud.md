# Salud y configuración operativa

## Endpoints
| Ruta | Uso | Respuesta |
|---|---|---|
| GET /api/v1/health | Estado completo (readiness) | 200 `ok`/`degradado`, 503 `caido` |
| GET /api/v1/health/live | El proceso vive (liveness) | Siempre 200 |
| GET /api/health | Alias de /api/v1/health | Igual |

## Estados
- `ok`: PostgreSQL e inferencia responden (o la inferencia no está configurada).
- `degradado`: PostgreSQL responde, la inferencia no. La API sigue atendiendo.
- `caido`: PostgreSQL no responde → 503.

Dependencias: `activo`, `caido`, `no_configurado`, `sin_verificar`.

## Servicio de inferencia
La API consulta `INFERENCE_URL/health` cada `INFERENCE_CHECK_INTERVAL` (15 s) con timeout
`INFERENCE_TIMEOUT` (2 s), en segundo plano. **El servicio Python debe exponer
`GET /health` y responder 200 cuando el modelo esté cargado.**

## Qué NO se expone
URLs, hosts, usuarios, mensajes de error de PostgreSQL o del servicio Python, ni stack
traces. Esos detalles solo aparecen en los logs del servidor.

## Logs
- Arranque: versión y resumen de configuración (contraseña de la BD como `xxxxx`, sin JWT).
- Errores: se registra cuando PostgreSQL o la inferencia **cambian** de estado.
- Health checks exitosos: nivel `debug`.