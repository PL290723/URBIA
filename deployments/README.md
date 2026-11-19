# Despliegue con Docker Compose

## Servicios
| Servicio | Imagen | Qué hace | Puerto en el equipo |
|---|---|---|---|
| `db` | `postgis/postgis:18-3.6` | PostgreSQL 18 + PostGIS 3.6 | `127.0.0.1:5433` (solo DataGrip) |
| `migraciones` | `urbia-migraciones` | `goose up` con `urbia_owner` y termina | — |
| `api` | `urbia-api` | API Go | `8080` |
| `inferencia` | `urbia-inferencia` | YOLO + gRPC (CPU) | — |

## Orden de arranque
`db` (healthy) -> `migraciones` (termina con código 0) -> `api` -> `inferencia`.
La API arranca aunque la inferencia aún esté cargando el modelo: `/api/v1/health`
responde `degradado` hasta que la IA está lista (unos 90 s la primera vez).

## Redes
- `interna` (sin internet): los cuatro servicios se hablan por nombre (`db`, `inferencia`).
- `publica`: solo para publicar el puerto de la API y el de la base en `127.0.0.1`.

## Volúmenes
| Volumen | Contenido | Montaje |
|---|---|---|
| `urbia_pgdata` | Datos de PostgreSQL | `db:/var/lib/postgresql` |
| `urbia_storage` | Fotografías | `api:/app/storage` (escritura), `inferencia:/app/storage` (solo lectura) |

## Primer arranque
```powershell
cd deployments
Copy-Item .env.example .env   # y cambiar todos los CAMBIAR
docker compose up -d --build
docker compose ps
```
La primera vez, `db` ejecuta `db/20_urbia.sh` (roles, base `urbia`, PostGIS y esquema).
Después solo corren las migraciones pendientes.

## Comandos
| Comando | Qué hace |
|---|---|
| `docker compose up -d` | Levanta todo (o lo que falte) |
| `docker compose up -d --build` | Reconstruye imágenes tras cambios de código |
| `docker compose ps` | Estado y healthchecks |
| `docker compose logs -f api` | Logs de un servicio |
| `docker compose stop` | Apagado ordenado, conserva contenedores y datos |
| `docker compose down` | Borra contenedores y redes, **conserva** volúmenes |
| `docker compose down -v` | Borra también la base y las fotos (reinicio desde cero) |

## Administradores
```powershell
docker compose exec db psql -U postgres -d urbia -c "UPDATE urbia.usuario SET rol = 'administrador', fecha_actualizacion = now() WHERE lower(correo) = lower('correo@ejemplo.mx');"
```

## DataGrip
Host `localhost`, puerto `5433`, base `urbia`, usuario `urbia_owner` o `urbia_app`.

## Secretos
Ninguna contraseña está en `compose.yaml` ni en las imágenes: todo sale de
`deployments/.env`, que está en `.gitignore`.

## Feria y red local
Cómo dar acceso a celulares de la misma red, firewall, CORS, límite de login y la
alternativa CPU/GPU: [feria.md](feria.md).

## Scripts de operación
| Script | Qué hace |
|---|---|
| `scripts/respaldar.ps1` | Respalda base y fotos en `$HOME\urbia-respaldos` |
| `scripts/restaurar.ps1` | Restaura un respaldo (reemplaza base y fotos) |
| `scripts/cuentas-demo.ps1` | Crea las cuentas de prueba y promueve al administrador |

Uso y detalles: [docs/operacion.md](../docs/operacion.md).