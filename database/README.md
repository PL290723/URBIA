# Base de datos de URBIA

PostgreSQL 18 + PostGIS 3.6. El modelo de datos está documentado en
[`docs/base-de-datos/modelo-er.md`](../docs/base-de-datos/modelo-er.md).

## Estructura

```
database/
├── README.md
└── scripts/
    ├── 01_crear_bd_y_roles.sql     # BD urbia, roles urbia_owner y urbia_app
    ├── 02_habilitar_postgis.sql    # extensión PostGIS, esquema urbia, permisos
    ├── 03_esquema.sql              # tablas, restricciones e índices
    └── 04_prueba_espacial.sql      # prueba de consulta espacial (no deja datos)
```

## Requisitos

- PostgreSQL 18
- PostGIS 3.6 para PostgreSQL 18
  - Windows (instalador EDB): Stack Builder → Spatial Extensions → PostGIS 3.6 Bundle
  - Ubuntu: `sudo apt install postgresql-18-postgis-3`

## Cómo levantar la base

Ejecuta los scripts **en orden**, cada uno con el usuario y la base indicados:

| # | Script | Usuario | Base |
|---|---|---|---|
| 1 | `01_crear_bd_y_roles.sql` | `postgres` | `postgres` |
| 2 | `02_habilitar_postgis.sql` | `postgres` | `urbia` |
| 3 | `03_esquema.sql` | `urbia_owner` | `urbia` |
| 4 | `04_prueba_espacial.sql` | `urbia_app` | `urbia` |

Con `psql`:

```bash
psql -U postgres -d postgres -f database/scripts/01_crear_bd_y_roles.sql
psql -U postgres -d urbia    -f database/scripts/02_habilitar_postgis.sql
psql -U urbia_owner -d urbia -f database/scripts/03_esquema.sql
psql -U urbia_app   -d urbia -f database/scripts/04_prueba_espacial.sql
```

## Roles

| Rol | Uso | Puede |
|---|---|---|
| `urbia_owner` | Migraciones | Crear y modificar tablas del esquema `urbia` |
| `urbia_app` | Backend | Solo `SELECT`, `INSERT`, `UPDATE`, `DELETE` |

## Migraciones (goose)

Instalar: `go install github.com/pressly/goose/v3/cmd/goose@latest`

Configurar: dentro de `database/`, copiar `.env.example` a `.env` y poner la contraseña de `urbia_owner`.

| Comando | Qué hace |
|---|---|
| `goose status` | Muestra migraciones aplicadas y pendientes |
| `goose up` | Aplica todas las pendientes |
| `goose down` | Revierte la última |
| `goose down-to 0` | Revierte todas |
| `goose -s create nombre sql` | Crea una migración nueva |

Reglas:
- Nunca editar una migración que ya está en `develop`; crear una nueva.
- Toda migración lleva `-- +goose Up` y `-- +goose Down`.
- `03_esquema.sql` se reemplazó por `migrations/00001_esquema_inicial.sql`.