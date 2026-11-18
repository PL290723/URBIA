# Seguridad de la API y de los archivos (HU 09.1)

Revisión hecha **antes de exponer el backend a dispositivos de la red local**.
Criterio de aceptación: las pruebas básicas de seguridad no permiten acceso anónimo a
operaciones privadas ni subida de archivos no permitidos.

Evidencia: carpeta de Bruno `09.1 Seguridad API y archivos` (ver [Cómo probar](#cómo-probar)).

## Resumen de hallazgos

| # | Hallazgo | Riesgo | Estado |
|---|---|---|---|
| 1 | `JWT_SECRET` de `.env.example` mide más de 32 caracteres, así que la API arrancaba con él. Es público (está en GitHub): cualquiera podía firmar un token de **administrador**. | Alto | **Corregido**: `config.Load` rechaza cualquier secreto con `CAMBIAR` |
| 2 | No había CORS: el frontend en otro origen no podía usar la API, y la tentación sería abrirlo con `*`. | Medio | **Corregido**: lista blanca `CORS_ALLOWED_ORIGINS`, `*` prohibido |
| 3 | Las rutas JSON no tenían límite de cuerpo: un JSON de cientos de MB se leía completo. | Medio | **Corregido**: 64 KiB por defecto, 413 `CUERPO_DEMASIADO_GRANDE` |
| 4 | El login no tenía límite de intentos (fuerza bruta). | Medio | **Corregido**: 10 fallos por IP en 5 min → 429 con `Retry-After` |
| 5 | El log de peticiones escribía el query string completo (`bbox` = zona exacta consultada). | Bajo | **Corregido**: parámetros sensibles se escriben como `***` |
| 6 | `X-Request-ID` del cliente se copiaba al log tal cual (cualquier texto ≤ 64). | Bajo | **Corregido**: solo `[A-Za-z0-9._-]{1,64}`; si no, se genera uno |
| 7 | El servidor solo tenía `ReadHeaderTimeout`: un cliente lento podía retener conexiones. | Bajo | **Corregido**: `ReadTimeout`, `WriteTimeout`, `IdleTimeout` y `MaxHeaderBytes` |
| 8 | Sin encabezados defensivos (nosniff, frame, CSP, caché). | Bajo | **Corregido**: middleware `CabecerasSeguridad` |
| 9 | Las fotos pueden mostrar **rostros y placas**. | Medio | **Mitigado** (fotos privadas); difuminado automático fuera del MVP |

Lo que ya estaba bien y se verificó: contraseñas con bcrypt (costo 12), JWT HS256 con
algoritmo fijo (rechaza `alg: none`), emisor y expiración obligatorios, mismo mensaje para
correo inexistente y contraseña incorrecta, consultas 100 % parametrizadas (`$1, $2…`),
fotos validadas por contenido real y re-codificadas sin EXIF, almacenamiento con `os.Root`
y nombres aleatorios, y errores 500 sin detalles internos.

## Autenticación y autorización por ruta

Todas las rutas privadas pasan por `middleware.RequireAuth` (401 sin token o con token
inválido). Los permisos por rol usan `RequirePermiso`/`RequireRol` (403). Lo que depende
del dueño del recurso lo decide el *service* y responde **404** para no revelar que existe.

| Método | Ruta | Sin token | Quién puede |
|---|---|---|---|
| GET | `/api/health`, `/api/v1/health`, `/api/v1/health/live` | 200 (pública) | Cualquiera — solo estados y latencias, nunca hosts ni errores |
| GET | `/api/v1/openapi.yaml`, `/api/v1/docs` | 200 (pública) | Cualquiera — solo el contrato |
| POST | `/api/v1/auth/registro` | 201 (pública) | Cualquiera — siempre crea `ciudadano` |
| POST | `/api/v1/auth/login` | 200/401 (pública) | Cualquiera — con límite de intentos |
| GET | `/api/v1/auth/perfil` | 401 | Autenticado |
| GET | `/api/v1/admin/usuarios` | 401 | Administrador (403) |
| POST | `/api/v1/reportes` | 401 | Ciudadano (403 para admin) |
| GET | `/api/v1/reportes` | 401 | Autenticado; ajenos con datos recortados |
| GET | `/api/v1/reportes/{id}` | 401 | Autenticado; ajeno → ubicación aproximada, sin foto ni autor |
| GET | `/api/v1/reportes/{id}/historial` | 401 | Autor o administrador (otro → 403) |
| POST | `/api/v1/reportes/{id}/archivos` | 401 | Autor del reporte o administrador (otro → 404) |
| GET | `/api/v1/archivos/{id}` | 401 | Autor del reporte o administrador (otro → 404) |
| DELETE | `/api/v1/archivos/{id}` | 401 | Autor del reporte o administrador (otro → 404) |
| PUT | `/api/v1/reportes/{id}/categoria` | 401 | Ciudadano autor (403 rol, 404 ajeno) |
| PATCH | `/api/v1/reportes/{id}/estado` | 401 | Administrador (403) |
| POST/GET | `/api/v1/reportes/{id}/prioridad` | 401 | Administrador (403) |
| GET | `/api/v1/reportes/{id}/cercanos` | 401 | Administrador (403) |
| GET | `/api/v1/estadisticas` | 401 | Administrador (403) |

`router/openapi_test.go` ya falla si una ruta documentada como protegida responde algo
distinto de 401 sin token. Al agregar una ruta nueva hay que agregarla aquí y en Bruno.

Los administradores **no** se crean por la API: se promueven a mano en la base
(ver [autenticacion.md](autenticacion.md)).

## Entradas y consultas

- JSON: `binding` con `required`, `max`, `email`; JSON mal formado → 400 sin eco del contenido.
- Formularios y query params: `dominio.Parse*` valida cada campo y responde 400 `VALIDACION_FALLIDA`.
- Ids de ruta: enteros positivos (`parseID`), si no 400.
- SQL: siempre con parámetros de pgx. El único SQL armado con texto es el `WHERE` dinámico de
  `consulta_reporte_repository.go`, y solo concatena **placeholders** (`q.param(...)`), nunca valores.

## Archivos

| Control | Dónde |
|---|---|
| Máx. 10 MB por foto; el cuerpo multipart se corta en 11 MB | `handlers`, `middleware.LimiteCuerpo` |
| Formato por **contenido** (firma + decodificador coinciden): solo JPEG, PNG, WebP. SVG, GIF, HTML, PDF, texto → 415 | `imagenes.Saneador` |
| Dimensiones 200–8000 px y ≤ 25 MP antes de decodificar (bombas de descompresión) → 422 | `imagenes.Saneador` |
| Vacío → 400; dañado → 422 | `imagenes.Saneador` |
| Re-codificación a JPEG: se pierden EXIF (incluido GPS), ICC y cualquier carga oculta | `imagenes.Saneador` |
| Nombre y extensión del cliente se ignoran; se guarda con nombre aleatorio | `storage.Local` |
| Todo acceso a disco pasa por `os.Root` + patrón de ruta (sin `../` ni symlinks) | `storage.Local` |
| Nunca se sirven como estáticos; descarga con `nosniff`, `no-store` y permiso de autor/admin | `handlers.ArchivoHandler` |

## Logs

- Nunca se escriben contraseñas, hashes, tokens ni el encabezado `Authorization`.
- Query string: `token`, `contrasena`, `password`, `correo`, `email`, `lat`, `lon`, `lng`,
  `latitud`, `longitud`, `bbox`… se escriben como `***`.
- `config.ResumenSeguro` oculta la contraseña de `DATABASE_URL` y no imprime `JWT_SECRET`.
- Los 500 solo dicen "Ocurrió un error inesperado"; el detalle queda en el log con el `request_id`.
- Se registra la IP del cliente (necesaria para el límite de login). Son IP de la red local.

## CORS

`CORS_ALLOWED_ORIGINS` = orígenes exactos del frontend, separados por coma:

```dotenv
CORS_ALLOWED_ORIGINS=http://localhost:5173,http://192.168.1.50:5173
```

- Sin `Origin` (Bruno, curl, apps nativas) no aplica: CORS lo hace cumplir el navegador.
- Origen permitido → `Access-Control-Allow-Origin: <ese origen>` y `Vary: Origin`.
- Preflight de otro origen → **403**. Petición simple de otro origen → sin encabezados CORS
  (el navegador no deja leer la respuesta).
- No se envía `Access-Control-Allow-Credentials`: la sesión viaja en `Authorization`, no en cookies.

## Secretos fuera del repositorio

- `.gitignore` excluye `.env`, `.env.*` (salvo `.env.example`), `*.pem`, `*.key`, `secrets/`,
  `storage/`, pesos del modelo y `*.log`.
- Los `.env.example` solo tienen valores `CAMBIAR_…`, y la API **se niega a arrancar** si
  `JWT_SECRET` conserva el de ejemplo.
- Revisión rápida antes de cada entrega:

```powershell
git ls-files | Select-String -Pattern '\.env$|\.pem$|\.key$|storage/'
git log --all -p | Select-String -Pattern 'JWT_SECRET=|DATABASE_URL=|password=' | Select-Object -Unique
```

Solo deben aparecer los `.env.example` con `CAMBIAR_…`. Si alguna vez se sube un secreto real,
**se cambia** (borrarlo del historial no basta).

## Imágenes, coordenadas, rostros y placas

- **Imágenes:** privadas (autor o administrador). Sin EXIF: la foto no revela el GPS del celular.
- **Coordenadas:** exactas solo para el autor y el administrador; los demás ciudadanos ven la
  ubicación redondeada a 3 decimales (~110 m). El log no las guarda.
- **Rostros y placas:** el MVP **no** los difumina. Mitigaciones: fotos privadas, la app debe
  pedir al ciudadano que encuadre solo el problema, y en la demostración se usan fotos propias o
  de prueba sin personas ni placas legibles. El difuminado automático queda como mejora futura.

## Red de la feria

La API está pensada para una **red local controlada**, no para Internet:

1. Usar un router o hotspot **propio del equipo**, con WPA2/WPA3 y contraseña que solo
   tenga el equipo. No usar la Wi-Fi abierta del evento.
2. No abrir puertos en el router (sin *port forwarding*, sin DMZ, sin UPnP).
3. En el firewall de Windows, permitir el puerto de la API **solo en el perfil Privado**.
   PostgreSQL del contenedor solo escucha en `127.0.0.1:5433` y la inferencia (gRPC 50051,
   salud 8000) no publica puertos. Procedimiento completo: [deployments/feria.md](../../deployments/feria.md).
4. `CORS_ALLOWED_ORIGINS` con la IP local exacta del frontend; nada de `*`.
5. `APP_ENV=production` y `GIN_MODE=release` el día de la demostración (logs JSON, sin modo debug).
6. Cuentas de demostración con datos ficticios; un `JWT_SECRET` nuevo para la feria.
7. Al terminar, apagar la API y cambiar la contraseña de la red.

HTTP sin TLS es aceptable **solo** dentro de esa red cerrada; el token viaja en claro.

Con Docker Desktop todos los dispositivos llegan a la API con la IP del proxy de Docker,
así que el límite de login (hallazgo 4) es compartido: en la feria se sube con
`LOGIN_MAX_INTENTOS` (ver `deployments/feria.md`).

## Limitaciones conocidas

- El límite de login vive en memoria: se reinicia con la API y no se comparte entre instancias.
- El registro no tiene límite de cuentas por IP.
- No hay revocación de tokens: un JWT robado vale hasta su expiración (`JWT_TTL`, 2 h).
- Sin TLS (ver red de la feria) y sin difuminado de rostros/placas.

## Cómo probar

Colección de Bruno, carpeta `09.1 Seguridad API y archivos`. Corre en orden: crea dos
ciudadanos y un reporte, prueba las rutas privadas sin token, con token falsificado, con rol
incorrecto y como otro usuario; sube archivos no permitidos; revisa límites, encabezados y
CORS; y al final la fuerza bruta del login (deja la IP bloqueada `LOGIN_VENTANA`).
