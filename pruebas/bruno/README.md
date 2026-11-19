# Pruebas de la API con Bruno

Colección de [Bruno](https://www.usebruno.com/) con las pruebas manuales y automáticas de la API
de URBIA, una carpeta por historia de usuario. Cada request trae sus `tests`, así que el Runner de
Bruno marca qué pasó y qué falló. Es la evidencia ejecutable de la versión candidata
(ver `docs/aceptacion.md`).

## Preparación

1. Abrir en Bruno: **Open Collection** → carpeta `pruebas/bruno` del repositorio.
2. Entornos: en Bruno, clonar el entorno **`ejemplo`** con el nombre **`local`**
   (Environments → `ejemplo` → Clone). `baseUrl = http://localhost:8080/api/v1`.
   - `demoAdminPass`: contraseña de `admin.demo@urbia.test` (la que se usó con
     `deployments/scripts/cuentas-demo.ps1`).
   - Para otro equipo de la red local: otro clon llamado `feria` con
     `baseUrl = http://<IP>:8080/api/v1`.
3. Fotos reales: poner las cuatro de `fotos/README.md` (no se suben al repositorio).
4. Backend levantado (`deployments/`, `docker compose up -d`) con todo `healthy`.

Los tokens, ids y coordenadas que crean las pruebas se guardan en el archivo del entorno al
correrlas. Por eso `environments/local.yml` y `feria.yml` están en `.gitignore` y solo se
versiona la plantilla limpia `environments/ejemplo.yml`.

## Cuentas de prueba (ficticias, no productivas)

| Cuenta | Contraseña | Rol | La crea |
|---|---|---|---|
| `ana@test.mx` | `NuevaClave123` | ciudadano | `00 Preparacion/01` |
| `juan@test.mx` | `NuevaClave123` | administrador | `00 Preparacion/02` + SQL (abajo) |
| `luis@test.mx` | `NuevaClave123` | ciudadano | `00 Preparacion/03` |
| `marta@test.mx` | `Marta2026!` | ciudadano | `07.1/P0` |
| `*.091@ejemplo.com` | `Segura091!` | ciudadano | `09.1` (correos únicos por corrida) |
| `ciudadano.demo@urbia.test` | `DemoUrbia2026` | ciudadano | `cuentas-demo.ps1` |
| `admin.demo@urbia.test` | `{{demoAdminPass}}` | administrador | `cuentas-demo.ps1` |

Solo existen en bases de prueba o de demostración. Nunca usarlas con datos reales; para la feria,
el administrador usa una contraseña nueva que no está en el repositorio.

Promover a `juan@test.mx` (una vez por base, desde `deployments/`):

```powershell
docker compose exec db psql -U postgres -d urbia -c "UPDATE urbia.usuario SET rol = 'administrador', fecha_actualizacion = now() WHERE lower(correo) = 'juan@test.mx';"
```

## Orden de ejecución

1. `00 Preparacion` (registros, logins y un reporte de otro ciudadano).
2. Carpetas `06.1` a `08.2`, en orden.
3. `09.3`, `09.4` y `11.1` (siguen sus propias fases, ver abajo).
4. `09.1 Seguridad API y archivos` **al final**: su última prueba bloquea el login de la IP
   durante `LOGIN_VENTANA` (con Docker Desktop, la de todos los dispositivos).

Pruebas que piden un paso manual antes (está escrito en el comentario de sus `tests`):

| Prueba | Paso manual |
|---|---|
| `06.2/05` y `06.2/06` | Detener y volver a levantar la inferencia |
| `09.3/09` | Inferencia sin modelo |
| `09.4/18` a `21` | Reiniciar todo (ver `docs/backend/regresion.md`) |
| `11.1/B` | `docker compose stop inferencia` (y `start` al terminar) |
| `11.1/C` | `docker compose restart`, `down` y `up -d` |

Con Docker Compose, "detener la inferencia" es `docker compose stop inferencia` y "levantarla"
es `docker compose start inferencia` (desde `deployments/`).

## Archivos de prueba

- `09.1 .../archivos/` y `09.3 .../archivos/`: archivos sintéticos para validar el rechazo
  (texto o HTML disfrazados de imagen, SVG con script, GIF, imagen diminuta, archivo vacío,
  JPEG truncado). No tienen datos personales.
- `06.1 .../archivos/foto.jpg.txt`: texto con nombre de foto.
- `fotos/`: fotos reales, locales (ver `fotos/README.md`).
