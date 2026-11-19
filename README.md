# URBIA

**Sistema inteligente de detección y gestión de problemas urbanos**

URBIA recibe reportes ciudadanos geolocalizados con foto (baches, basura y otras incidencias),
sugiere la categoría con un modelo de visión por computadora y permite a los administradores
darles seguimiento. MVP para la Feria de Proyectos TecNM Celaya 2026.

## Versión actual

**v0.1.0-rc.1**, versión candidata para la Feria de Proyectos. Qué incluye, cómo se verificó y
qué defectos conocidos tiene: [notas de la versión](docs/versiones/v0.1.0-rc.1.md).

| Componente | Versión |
|---|---|
| Backend (API Go e imágenes Docker) | 0.1.0 |
| Contrato de la API (OpenAPI) | 1.0.0 (`/api/v1`) |
| Frontend (PWA) | 0.1.0 |
| Modelo de IA | `urbia-yolo26s` 1.0.0 (Release `modelo-v1.0.0`) |

## Arquitectura

| Componente | Tecnología | Carpeta |
|---|---|---|
| Frontend (PWA instalable) | React 19 + TypeScript + Vite, Tailwind, Leaflet, TanStack Query | `frontend/` |
| API principal | Go 1.27 + Gin | `backend-go/` |
| Servicio de inferencia | Python 3.11 + YOLO26 (Ultralytics) | `inference-python/` |
| Comunicación interna | gRPC + Protocol Buffers | `proto/` |
| Base de datos | PostgreSQL 18 + PostGIS 3.6, migraciones con goose | `database/` |
| Fotografías | Volumen local compartido (solo lectura para la IA) | — |
| Orquestación | Docker Compose | `deployments/` |

## Inicio rápido

Requisitos: Docker Desktop con WSL 2, Git, 8 GB de RAM (16 recomendados) y 15 GB libres.

```powershell
git clone https://github.com/PL290723/URBIA.git
cd URBIA/deployments
Copy-Item .env.example .env        # completar las contraseñas y JWT_SECRET
docker compose up -d --build
curl.exe http://localhost:8080/api/v1/health
```

## Documentación

| Tema | Documento |
|---|---|
| Frontend: instalación, celulares en la feria, HTTPS | [frontend/README.md](frontend/README.md) |
| Requisitos, variables, base, migraciones y pesos YOLO | [docs/instalacion.md](docs/instalacion.md) |
| Comandos, endpoints, cuentas de prueba y respaldos | [docs/operacion.md](docs/operacion.md) |
| Problemas comunes y limitaciones del MVP | [docs/solucion-problemas.md](docs/solucion-problemas.md) |
| Docker Compose (servicios, redes, volúmenes) | [deployments/README.md](deployments/README.md) |
| Red local y feria | [deployments/feria.md](deployments/feria.md) |
| Seguridad | [docs/backend/seguridad.md](docs/backend/seguridad.md) |
| Modelo de datos | [docs/base-de-datos/modelo-er.md](docs/base-de-datos/modelo-er.md) |
| Modelo de IA | [docs/vision/modelo.md](docs/vision/modelo.md) |
| Contrato gRPC | [docs/vision/contrato-grpc.md](docs/vision/contrato-grpc.md) |
| Funcionalidad por historia de usuario | `docs/backend/` y `docs/vision/` |
| Pruebas de la API (colección de Bruno) | [pruebas/bruno/README.md](pruebas/bruno/README.md) |
| Checklist de aceptación del backend | [docs/aceptacion.md](docs/aceptacion.md) |
| Defectos conocidos | [docs/defectos-conocidos.md](docs/defectos-conocidos.md) |
| Evidencias de la versión candidata | [docs/evidencias/v0.1.0-rc.1/](docs/evidencias/v0.1.0-rc.1/README.md) |

## Flujo de trabajo

Ramas `feature/<nombre>` desde `develop`, fusionadas por PR. Commits con estilo
*conventional commits* (`feat(go): ...`, `docs: ...`). `main` solo recibe versiones, por PR
desde `develop`, y cada una lleva una etiqueta `vX.Y.Z` (las candidatas, `vX.Y.Z-rc.N`).