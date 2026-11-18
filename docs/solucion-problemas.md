# Solución de problemas y limitaciones (HU 10.5)

## 1. Diagnóstico rápido

```powershell
cd deployments
docker compose ps
docker compose logs --tail 50 <servicio>
curl.exe http://localhost:8080/api/v1/health
```

El log de la API está en JSON; el campo `error` dice la causa. Errores de configuración se
listan todos juntos al arrancar.

## 2. Problemas conocidos

### Docker y construcción
| Síntoma | Causa | Solución |
|---|---|---|
| `error during connect ... dockerDesktopLinuxEngine` | Docker Desktop cerrado | Abrir Docker Desktop y esperar a "Engine running" |
| `WSL 2 installation is incomplete` | WSL desactualizado | `wsl --update` y reiniciar |
| La construcción de `inferencia` tarda 10-20 min en `pip install torch` | Descarga grande (CPU ~200 MB, GPU ~2.5 GB); no muestra avance | Esperar; revisar tráfico de red. Queda en caché |
| Falla la etapa `pesos`: `SHA-256 distinto` o `404` | Release no publicado o `modelo.json` desactualizado | Revisar el Release `modelo-v<version>` (instalacion.md §6) |
| `no space left on device` | Caché de construcción llena | `docker builder prune` y `docker image prune` |

### Arranque
| Síntoma | Causa | Solución |
|---|---|---|
| `Falta JWT_SECRET en deployments/.env` (u otra variable) | `.env` incompleto | Completar con `.env.example` |
| `port is already allocated` (8080 o 5433) | API corriendo en GoLand u otro servicio | Detenerlo o cambiar `API_PUERTO` / `DB_PUERTO_LOCAL` |
| `db` nunca queda `healthy`; log con `\r` o `set: Illegal option` | `deployments/db/20_urbia.sh` con CRLF | Guardarlo con LF, `docker compose down -v`, `up -d` |
| `migraciones` en `exited (1)` | Error SQL o contraseña de `urbia_owner` | `docker compose logs migraciones` |
| `api` en `Restarting`: `JWT_SECRET sigue con el valor de ejemplo` | Secreto de `.env.example` | Generar uno propio |
| `api` en `Restarting`: `mkdir /app/storage/originales: permission denied` | Volumen de fotos creado con dueño root (imágenes viejas) | `docker compose down`, `docker volume rm urbia_storage`, `docker compose up -d --build` |
| `password authentication failed` tras cambiar contraseñas en `.env` | Solo se aplican con el volumen vacío | `docker compose exec db psql -U postgres -c "ALTER ROLE urbia_app PASSWORD 'nueva';"` (igual con `urbia_owner`) y `docker compose up -d` |
| Aviso `volume "urbia_storage" already exists but was not created by Docker Compose` | Volumen de pruebas manuales (`docker run`) | Respaldar si hace falta y `docker volume rm urbia_storage` |

### Funcionamiento
| Síntoma | Causa | Solución |
|---|---|---|
| `/api/v1/health` → `degradado` | Inferencia cargando (hasta ~90 s) o caída | `docker compose ps`, `docker compose logs inferencia` |
| Reportes con sugerencia `no_disponible` | La IA no respondió a tiempo o no está lista | Igual que arriba; el reporte se guarda sin sugerencia |
| `inferencia` no arranca: `MODELO_INVALIDO` | Pesos distintos a `modelo.json` | Reconstruir: `docker compose build --no-cache inferencia` |
| Todos reciben 429 en el login | Límite compartido (todos llegan con la IP del proxy de Docker) | Esperar `LOGIN_VENTANA` o subir `LOGIN_MAX_INTENTOS` y `docker compose up -d` |
| El administrador recibe 403 | Token emitido antes de la promoción | Volver a iniciar sesión |
| Error de CORS en el navegador | Origen del frontend no listado | Agregarlo a `CORS_ALLOWED_ORIGINS` y `docker compose up -d` |
| El celular no conecta | Perfil de red público, firewall o IP incorrecta | [deployments/feria.md](../deployments/feria.md) §8 |
| `/api/v1/docs` en blanco | Swagger UI se carga desde un CDN y no hay internet | Usar `/api/v1/openapi.yaml` o Bruno |
| GPU: `could not select device driver` o `dispositivo=cpu` | Docker no ve la GPU | Volver a CPU: `docker compose up -d` |
| Falla un script `.ps1`: "la ejecución de scripts está deshabilitada" | Política de ejecución | Usar `powershell -ExecutionPolicy Bypass -File ...` |

## 3. Limitaciones conocidas del MVP

**Seguridad y red** (detalle en [docs/backend/seguridad.md](backend/seguridad.md))
- HTTP sin TLS: solo aceptable en la red cerrada del equipo; el token viaja en claro.
- No hay revocación de tokens ni cambio de contraseña: un JWT vale hasta `JWT_TTL`.
- Administradores se promueven a mano en la base.
- El límite de login vive en memoria y, con Docker Desktop, es compartido por todos los dispositivos.
- Las fotos pueden mostrar rostros o placas; no hay difuminado automático.

**IA** (detalle en [docs/vision/modelo.md](vision/modelo.md))
- Solo detecta `bache` y `basura`; las demás categorías se eligen a mano.
- En `test`: mAP50 0.79; recall de bache 0.65 (se le escapa uno de cada tres). Es una
  **sugerencia** que el ciudadano confirma o corrige.
- Una inferencia a la vez (Ultralytics no es *thread-safe*); suficiente para la demo.

**Datos y operación**
- Borrar un reporte no borra sus fotos del disco (ver [docs/backend/almacenamiento.md](backend/almacenamiento.md)).
- Pesos de la fórmula de prioridad provisionales (`v1-provisional`).
- Una sola instancia de cada servicio, en un solo equipo; sin alta disponibilidad.
- Respaldos manuales (scripts), no programados.
- `/api/v1/docs` necesita internet (Swagger UI desde CDN).
