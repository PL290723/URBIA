-- =====================================================================
-- URBIA · HU 06.2 · Migración 00006 — Detalle de la detección
-- · Se registra también cuando la IA falla (resultado 'error'), para saber
--   qué reportes quedaron sin sugerencia y poder reintentarlos después.
-- · umbral_confianza: umbral con el que el modelo decidió.
-- · codigo_error: código de URBIA (SERVICIO_NO_DISPONIBLE, TIEMPO_AGOTADO, …).
-- · milisegundos: tiempo de la inferencia dentro del servicio Python.
-- · Con 'error' puede no conocerse el modelo, por eso modelo y versión pasan
--   a ser opcionales SOLO en ese caso (ck_deteccion_modelo).
-- =====================================================================

-- +goose Up
SET LOCAL search_path = urbia, public;

ALTER TABLE deteccion
    ALTER COLUMN modelo         DROP NOT NULL,
ALTER COLUMN version_modelo DROP NOT NULL,
    ADD COLUMN umbral_confianza NUMERIC(5,4),
    ADD COLUMN codigo_error     VARCHAR(40),
    ADD COLUMN milisegundos     NUMERIC(8,1),
    ADD CONSTRAINT ck_deteccion_umbral       CHECK (umbral_confianza BETWEEN 0 AND 1),
    ADD CONSTRAINT ck_deteccion_milisegundos CHECK (milisegundos >= 0),
    ADD CONSTRAINT ck_deteccion_modelo       CHECK (
        resultado = 'error' OR (modelo IS NOT NULL AND version_modelo IS NOT NULL)
    ),
    ADD CONSTRAINT ck_deteccion_error        CHECK (
        (resultado = 'error') = (codigo_error IS NOT NULL)
    );

-- Para consultar rápido los reportes cuya inferencia falló (reintentos).
CREATE INDEX ix_deteccion_error ON deteccion (fecha) WHERE resultado = 'error';

-- +goose Down
SET LOCAL search_path = urbia, public;

DROP INDEX IF EXISTS ix_deteccion_error;

-- Las detecciones de error sin modelo no caben en el esquema anterior.
DELETE FROM deteccion WHERE modelo IS NULL OR version_modelo IS NULL;

ALTER TABLE deteccion
DROP CONSTRAINT IF EXISTS ck_deteccion_error,
    DROP CONSTRAINT IF EXISTS ck_deteccion_modelo,
    DROP CONSTRAINT IF EXISTS ck_deteccion_milisegundos,
    DROP CONSTRAINT IF EXISTS ck_deteccion_umbral,
    DROP COLUMN IF EXISTS milisegundos,
    DROP COLUMN IF EXISTS codigo_error,
    DROP COLUMN IF EXISTS umbral_confianza,
    ALTER COLUMN version_modelo SET NOT NULL,
    ALTER COLUMN modelo         SET NOT NULL;