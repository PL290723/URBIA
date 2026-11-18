# Instalación

Guía para que cualquier integrante del equipo levante el backend completo de URBIA desde cero.
Tiempo aproximado: 30 a 45 minutos la primera vez, casi todo descargando imágenes.

## 1. Requisitos

### Software
| Herramienta | Versión | Para qué |
|---|---|---|
| Windows 10 22H2 / 11 con **WSL 2** | — | Base de Docker Desktop |
| **Docker Desktop** | Engine 24+, Compose 2.20+ | Construir y ejecutar todo el backend |
| **Git** | 2.40+ | Obtener el código |
| Bruno | cualquiera | Probar la API (colección `local`) |
| DataGrip o `psql` | cualquiera | Revisar la base (opcional) |

Solo para desarrollar **sin** Docker: Go 1.27.3, Python 3.11, PostgreSQL 18 + PostGIS 3.6 y
goose v3.26 (ver [database/README.md](../database/README.md) y [docs/vision/servicio-inferencia.md](vision/servicio-inferencia.md)).

### Hardware
| Recurso | Mínimo | Recomendado |
|---|---|---|
| CPU | 4 núcleos x86-64 con virtualización activada | 8 núcleos |
| RAM | 8 GB | 16 GB (Docker Desktop + modelo YOLO) |
| Disco libre | 15 GB | 25 GB si se usa el perfil GPU |
| GPU | No necesaria | NVIDIA con 4 GB+ de VRAM (opcional) |
| Red (feria) | Router o hotspot propio con WPA2/WPA3 | — |

Equipo de referencia: Acer Nitro AN515-58, i5-12500H, 16 GB, RTX 3050 Ti 4 GB.

## 2. Instalar Docker Desktop
1. PowerShell como administrador: `wsl --install` (si no está) y `wsl --update`. Reiniciar.
2. Administrador de tareas → Rendimiento → CPU: "Virtualización: Habilitado". Si no, activarla en la BIOS.
3. Instalar Docker Desktop desde https://docs.docker.com/desktop/setup/install/windows-install/
   con "Use WSL 2 instead of Hyper-V".
4. Verificar:
   ```powershell
   docker version
   docker compose version
   docker run --rm hello-world
   ```

## 3. Obtener el código
```powershell
git clone https://github.com/PL290723/URBIA.git
cd URBIA
git checkout develop
```

## 4. Variables de entorno

Toda la configuración del despliegue vive en **`deployments/.env`** (git lo ignora). Se crea
a partir de la plantilla:

```powershell
cd deployments
Copy-Item .env.example .env
```

| Variable | Obligatoria | Ejemplo / por defecto | Descripción |
|---|---|---|---|
| `POSTGRES_PASSWORD` | sí | — | Superusuario `postgres` del contenedor |
| `URBIA_OWNER_PASSWORD` | sí | — | Rol de migraciones (`urbia_owner`) |
| `URBIA_APP_PASSWORD` | sí | — | Rol del backend (`urbia_app`, solo CRUD) |
| `JWT_SECRET` | sí | — | Firma de tokens, 32+ caracteres; la API rechaza el de ejemplo |
| `JWT_TTL` | no | `2h` | Vigencia del token |
| `LOG_LEVEL` | no | `info` | `debug`, `info`, `warn`, `error` |
| `CORS_ALLOWED_ORIGINS` | no | `http://localhost:5173` | Orígenes exactos del frontend, separados por coma |
| `LOGIN_MAX_INTENTOS` | no | `10` (feria: `50`) | Fallos de login antes de 429 (3-100) |
| `LOGIN_VENTANA` | no | `5m` (feria: `2m`) | Duración del bloqueo |
| `API_PUERTO` | no | `8080` | Puerto de la API en Windows |
| `DB_PUERTO_LOCAL` | no | `5433` | Puerto de PostgreSQL en `127.0.0.1` (DataGrip) |
| `URBIA_VERSION` | no | `0.1.0` | Tag de las imágenes y versión que reporta la API |

Generar valores (PowerShell):

```powershell
# Contraseñas de la base: solo letras y números (van dentro de una URL y de un DSN)
-join ((48..57) + (65..90) + (97..122) | Get-Random -Count 32 | ForEach-Object { [char]$_ })
# JWT_SECRET
[Convert]::ToBase64String([Security.Cryptography.RandomNumberGenerator]::GetBytes(48))
```

Validar sin imprimir secretos: `docker compose config --quiet` (sin salida = correcto).

Compose fija el resto de la configuración de los servicios (`APP_ENV=production`,
`GIN_MODE=release`, direcciones internas `db:5432` e `inferencia:50051`, `STORAGE_DIR=/app/storage`,
`URBIA_DEVICE=cpu`). Referencia completa de cada servicio:
[backend-go/.env.example](../backend-go/.env.example), [docs/backend/salud.md](backend/salud.md),
[docs/vision/servidor-inferencia.md](vision/servidor-inferencia.md).

> Las contraseñas de la base solo se aplican cuando el volumen de datos está vacío (primer
> arranque). Para cambiarlas después, ver [solucion-problemas.md](solucion-problemas.md).

## 5. Base de datos y migraciones

Con Compose no hay pasos manuales:

1. **Primer arranque** (volumen `urbia_pgdata` vacío): `deployments/db/20_urbia.sh` crea los
   roles `urbia_owner` y `urbia_app`, la base `urbia`, PostGIS y el esquema
   (equivale a `database/scripts/01` y `02`).
2. **Cada `docker compose up`**: el servicio `migraciones` ejecuta `goose up` con `urbia_owner`
   sobre `database/migrations` y termina. La API no arranca hasta que termina con código 0.

Comandos de goose a través de Compose (desde `deployments/`):

```powershell
docker compose run --rm migraciones status   # aplicadas y pendientes
docker compose run --rm migraciones up       # aplicar pendientes
docker compose run --rm migraciones down     # revertir la última (cuidado)
```

Agregar una migración: crear `database/migrations/000NN_nombre.sql` con `-- +goose Up` y
`-- +goose Down` (reglas en [database/README.md](../database/README.md)) y reconstruir la
imagen de migraciones, porque las lleva dentro:

```powershell
docker compose up -d --build migraciones
docker compose logs migraciones
```

Empezar con una base vacía (**borra todos los datos y fotos**): `docker compose down -v`.

## 6. Pesos del modelo YOLO

- Los pesos (`.pt`) **no** están en git. `inference-python/config/modelo.json` (sí versionado)
  indica el archivo (`models/urbia-yolo26s-v1.0.0.pt`) y su **SHA-256**.
- Se publican como GitHub Release `modelo-v<version>`.
- Al construir la imagen de inferencia se descargan del Release y se verifica el SHA-256;
  si no coincide, la construcción falla. Quedan dentro de la imagen en `/app/models`, así que en
  la feria no hace falta internet.
- Al arrancar, el servicio vuelve a verificar el SHA-256 y las clases (`bache`, `basura`).
- Publicar una versión nueva: `python -m scripts.publicar_modelo` (ver [docs/vision/modelo.md](vision/modelo.md)),
  crear el Release, subir `modelo.json` y reconstruir: `docker compose build inferencia`.
- Probar otros pesos sin reconstruir: montarlos en `/app/models` (ver [docs/vision/docker.md](vision/docker.md)).
- Desarrollo sin Docker: `cd inference-python` y `python -m scripts.descargar_modelo`.

Comprobar qué modelo está cargado:

```powershell
docker compose exec api wget -qO- http://inferencia:8000/health
```

## 7. Primer arranque y verificación

```powershell
cd deployments
docker compose up -d --build
docker compose ps
```

| Servicio | Estado esperado |
|---|---|
| `db` | `healthy` |
| `migraciones` | `exited (0)` (termina a propósito) |
| `api` | `healthy` |
| `inferencia` | `healthy` (hasta ~90 s) |

```powershell
curl.exe http://localhost:8080/api/v1/health
```

Debe responder `"status": "ok"` con base de datos e inferencia `activo`. Siguiente paso:
crear las cuentas de prueba ([operacion.md](operacion.md#3-cuentas-de-prueba-no-productivas)).