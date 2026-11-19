-- =====================================================================
-- URBIA · HU 02.2 · Script 01 — Base de datos, roles y permisos mínimos
-- Ejecutar como: postgres (superusuario), conectado a la BD "postgres"
-- Cambia las contraseñas antes de ejecutarlo y NO las subas al repo.
-- =====================================================================

-- Rol dueño del esquema: crea/modifica tablas (lo usan las migraciones).
CREATE ROLE urbia_owner WITH LOGIN PASSWORD 'CAMBIAR_owner_pass';

-- Rol de la aplicación (backend): solo lee y escribe datos, no cambia estructura.
CREATE ROLE urbia_app WITH LOGIN PASSWORD 'CAMBIAR_app_pass';

CREATE DATABASE urbia
    WITH OWNER = urbia_owner
         ENCODING = 'UTF8'
         TEMPLATE = template0;

-- Nadie más que estos roles se conecta a la BD.
REVOKE ALL ON DATABASE urbia FROM PUBLIC;
GRANT CONNECT ON DATABASE urbia TO urbia_app;
