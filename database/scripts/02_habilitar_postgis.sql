-- =====================================================================
-- URBIA · HU 02.2 · Script 02 — Esquema, PostGIS y permisos
-- Ejecutar como: postgres (superusuario), conectado a la BD "urbia"
-- (PostGIS no es una extensión "trusted": solo un superusuario la crea)
-- =====================================================================

CREATE EXTENSION IF NOT EXISTS postgis;

-- Esquema propio del proyecto, propiedad de urbia_owner.
CREATE SCHEMA IF NOT EXISTS urbia AUTHORIZATION urbia_owner;

-- Que los roles encuentren primero las tablas de urbia y luego las funciones de PostGIS (public).
ALTER ROLE urbia_owner IN DATABASE urbia SET search_path = urbia, public;
ALTER ROLE urbia_app   IN DATABASE urbia SET search_path = urbia, public;

-- Permisos mínimos para la app: usar el esquema y hacer CRUD, nada de DDL.
GRANT USAGE ON SCHEMA urbia TO urbia_app;

-- Aplica a las tablas y secuencias que urbia_owner cree de aquí en adelante.
ALTER DEFAULT PRIVILEGES FOR ROLE urbia_owner IN SCHEMA urbia
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO urbia_app;
ALTER DEFAULT PRIVILEGES FOR ROLE urbia_owner IN SCHEMA urbia
    GRANT USAGE, SELECT ON SEQUENCES TO urbia_app;

-- Verificación
SELECT postgis_full_version();
