# Operación

Todos los comandos `docker compose` se ejecutan desde `deployments/`. Los scripts, desde la raíz del repo.

## 1. Comandos de Docker Compose

| Comando | Qué hace |
|---|---|
| `docker compose up -d` | Levanta todo, o recrea lo que cambió en `.env` o `compose.yaml` |
| `docker compose up -d --build` | Reconstruye imágenes después de cambiar código (o tras `git pull`) |
| `docker compose build api` | Reconstruye solo un servicio (`api`, `inferencia`, `migraciones`) |
| `docker compose ps` | Estado y healthchecks |
| `docker compose logs -f api` | Logs en vivo de un servicio (Ctrl+C para salir) |
| `docker compose logs --since 10m` | Logs de todos los servicios de los últimos 10 minutos |
| `docker compose restart api` | Reinicia un servicio |
| `docker compose exec db psql -U postgres -d urbia` | Consola SQL dentro del contenedor |
| `docker compose stop` | Apagado ordenado; conserva contenedores, base y fotos |
| `docker compose start` | Arranca lo que se detuvo con `stop` |
| `docker compose down` | Borra contenedores y redes; **conserva** volúmenes |
| `docker compose down -v` | Borra también la base y las fotos (reinicio desde cero) |
| `docker compose -f compose.yaml -f compose.gpu.yaml up -d --build` | Inferencia con GPU (opcional) |

Actualizar a la última versión de `develop`:

```powershell
git pull origin develop
cd deployments
docker compose up -d --build
docker compose ps
```

## 2. Endpoints

Base: `http://localhost:8080` (o `http://<IP>:8080` en la red local). Especificación completa:
`GET /api/v1/openapi.yaml`; visor interactivo: `/api/v1/docs` (necesita internet, ver limitaciones).

| Método | Ruta | Permisos |
|---|---|---|
| GET | `/api/v1/health` · `/api/v1/health/live` · `/api/health` | público |
| POST | `/api/v1/auth/registro` · `/api/v1/auth/login` | público |
| GET | `/api/v1/auth/perfil` | autenticado |
| POST | `/api/v1/reportes` (multipart con `foto`) | ciudadano |
| GET | `/api/v1/reportes` · `/api/v1/reportes/{id}` | autenticado |
| GET | `/api/v1/reportes/{id}/historial` | autor o administrador |
| POST | `/api/v1/reportes/{id}/archivos` | autor o administrador |
| GET / DELETE | `/api/v1/archivos/{id}` | autor o administrador |
| PUT | `/api/v1/reportes/{id}/categoria` | ciudadano autor |
| PATCH | `/api/v1/reportes/{id}/estado` | administrador |
| POST / GET | `/api/v1/reportes/{id}/prioridad` | administrador |
| GET | `/api/v1/reportes/{id}/cercanos` | administrador |
| GET | `/api/v1/estadisticas` · `/api/v1/admin/usuarios` | administrador |
| GET | `/api/v1/openapi.yaml` · `/api/v1/docs` | público |

Las rutas privadas usan `Authorization: Bearer <token>` (el token sale de `/auth/login`).
Pruebas por HU: colección de Bruno `local`, una carpeta por HU, variable `baseUrl`.

## 3. Cuentas de prueba (no productivas)

Solo para pruebas y demostración, en una base de demo. Nunca con datos reales.

| Cuenta | Correo | Contraseña | Rol |
|---|---|---|---|
| Ciudadano | `ciudadano.demo@urbia.test` | `DemoUrbia2026` | ciudadano |
| Administrador | `admin.demo@urbia.test` | la que se elija al crearla (no se documenta) | administrador |

Crearlas (con el stack levantado):

```powershell
powershell -ExecutionPolicy Bypass -File deployments\scripts\cuentas-demo.ps1 -ContrasenaAdmin "<elige una>"
```

- `.test` es un dominio reservado: esos correos no existen.
- El registro siempre crea ciudadanos; el script promueve al administrador directamente en la base.
- El administrador debe volver a iniciar sesión para que su token lleve el rol nuevo.
- Para la feria: base nueva, `JWT_SECRET` nuevo y contraseña de administrador nueva.

Promover a cualquier otro usuario:

```powershell
docker compose exec db psql -U postgres -d urbia -c "UPDATE urbia.usuario SET rol = 'administrador', fecha_actualizacion = now() WHERE lower(correo) = lower('correo@ejemplo.mx');"
```

## 4. Respaldos y restauración

Qué se respalda: la base `urbia` (`pg_dump` en formato personalizado, con permisos y la
configuración de los roles) y el volumen de fotos `urbia_storage` (tar.gz, con sus dueños).
Siempre juntos: la base guarda las rutas de las fotos.

```powershell
# Respaldo (pausa la API unos segundos). Destino por defecto: $HOME\urbia-respaldos
powershell -ExecutionPolicy Bypass -File deployments\scripts\respaldar.ps1

# Restauración (pide escribir SI; REEMPLAZA la base y las fotos actuales)
powershell -ExecutionPolicy Bypass -File deployments\scripts\restaurar.ps1 `
  -Bd "$HOME\urbia-respaldos\urbia-bd-20261118-1030.dump" `
  -Fotos "$HOME\urbia-respaldos\urbia-fotos-20261118-1030.tar.gz"
```

- Los respaldos se guardan **fuera** del repositorio (`*.dump` también está en `.gitignore`).
- Contienen correos y hashes de contraseñas: tratarlos como datos privados.
- Restaurar en un equipo nuevo: primero `docker compose up -d` (crea roles y volúmenes), luego el script.
- Recomendado: un respaldo antes de la feria, otro al terminar y antes de cualquier `down -v`.

## 5. Conexión a la base

| Herramienta | Configuración |
|---|---|
| DataGrip | Host `localhost`, puerto `5433`, base `urbia`, usuario `urbia_owner` o `urbia_app` |
| psql en el contenedor | `docker compose exec db psql -U postgres -d urbia` |

## 6. Red local y feria

Celulares en la misma red, firewall, CORS, IP local y alternativa CPU/GPU:
[deployments/feria.md](../deployments/feria.md).
