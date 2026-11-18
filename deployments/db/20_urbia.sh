#!/bin/sh
# URBIA (HU 10.3): base, roles y PostGIS en el primer arranque del contenedor db.
# Equivale a database/scripts/01 y 02, con las contraseñas tomadas del entorno.
# Solo corre cuando el volumen de datos está vacío.

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
     -v owner_pass="$URBIA_OWNER_PASSWORD" \
     -v app_pass="$URBIA_APP_PASSWORD" <<'SQL' || exit 1
CREATE ROLE urbia_owner WITH LOGIN PASSWORD :'owner_pass';
CREATE ROLE urbia_app   WITH LOGIN PASSWORD :'app_pass';

CREATE DATABASE urbia
    WITH OWNER = urbia_owner
         ENCODING = 'UTF8'
         TEMPLATE = template0;

REVOKE ALL ON DATABASE urbia FROM PUBLIC;
GRANT CONNECT ON DATABASE urbia TO urbia_app;
SQL

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname urbia \
     -f /urbia-sql/02_habilitar_postgis.sql || exit 1