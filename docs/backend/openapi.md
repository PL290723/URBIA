# Documentación OpenAPI (HU 08.1)

Contrato de la API para el frontend: endpoints, parámetros, cuerpos JSON, respuestas,
códigos HTTP, autenticación y permisos, con ejemplos.

| Qué | Dónde |
|---|---|
| Especificación (fuente) | `backend-go/internal/apidocs/openapi.yaml` (OpenAPI 3.1) |
| Especificación publicada | `GET /api/v1/openapi.yaml` |
| Visor interactivo (Swagger UI) | `GET /api/v1/docs` |
| Prueba de alineación | `backend-go/internal/router/openapi_test.go` |

Las dos rutas son públicas: el contrato no contiene datos, secretos ni hosts internos.
El YAML se compila dentro del binario con `go:embed`, así que no hay archivos sueltos que
copiar al desplegar. El visor carga Swagger UI desde jsDelivr (el navegador necesita
internet); la especificación funciona sin conexión.

## Cómo usarla

- **Navegador:** abre `http://localhost:8080/api/v1/docs`, pulsa **Authorize** y pega el
  token de `/auth/login` (sin la palabra `Bearer`).
- **Bruno / Postman:** *Import collection → OpenAPI* con la URL
  `http://localhost:8080/api/v1/openapi.yaml` o con el archivo.
- **Generar un cliente** (opcional, frontend): cualquier generador OpenAPI 3.1, por
  ejemplo `npx openapi-typescript http://localhost:8080/api/v1/openapi.yaml -o api.d.ts`.

## Convenciones del contrato

- Sobre único: `{"success": true, "data": ...}` / `{"success": false, "error": {...}}`.
- `error.code` es estable y está en `components.schemas.CodigoError`.
- Cada operación dice en **Permisos** qué rol la puede usar. `security: []` = pública;
  sin `security` hereda `bearerAuth` (token obligatorio).
- 401 = sin token o token inválido; 403 = rol sin permiso; 404 también cuando el recurso
  es ajeno y revelarlo filtraría datos (fotos y categoría de reportes de otros).
- Los campos que un rol no puede ver **no aparecen** (no van como `null`); los campos
  que sí aparecen pero pueden faltar van como `type: [X, 'null']`.

## Mantenerla alineada con la implementación

`go test ./internal/router/...` falla si:

1. Hay una ruta en Gin que no está en `openapi.yaml`, o una documentada que no existe.
2. Una operación protegida no responde 401 sin token (o una "pública" sí lo hace).
3. Falta `operationId` (o se repite), `summary`, `tags`, `**Permisos:**`, una respuesta
   2xx, el 401 en rutas protegidas o el 400 en rutas con `{id}`.
4. Algún `$ref` apunta a un componente que no existe.
5. El enum `CodigoError` no coincide con las constantes `response.Code*`.

**Regla para cada HU nueva:** si agregas o cambias un endpoint, actualiza
`openapi.yaml` en el mismo PR. Lo que la prueba no puede revisar (la forma exacta de los
JSON) se revisa en el PR comparando el modelo Go con su esquema.

Validación completa opcional (con Node):

```bash
npx @redocly/cli lint backend-go/internal/apidocs/openapi.yaml
```
