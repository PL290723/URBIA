-- =====================================================================
-- URBIA · HU 07.3 · Migración 00009 — Historial de estados inmutable
-- historial_estado es una bitácora de SOLO INSERCIÓN:
-- · urbia_app (el backend) ya no tiene UPDATE, DELETE ni TRUNCATE sobre ella.
-- · Un trigger rechaza UPDATE y TRUNCATE para CUALQUIER rol (también el dueño),
--   así nadie reescribe la trazabilidad por error desde una consola.
-- · DELETE ya está frenado por fk_historial_estado_reporte (ON DELETE RESTRICT)
--   y por el REVOKE; el dueño puede borrar solo para limpiar datos de prueba.
-- =====================================================================

-- +goose Up
SET LOCAL search_path = urbia, public;

REVOKE UPDATE, DELETE, TRUNCATE ON historial_estado FROM urbia_app;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION fn_historial_estado_inmutable() RETURNS trigger
    LANGUAGE plpgsql AS
$$
BEGIN
    RAISE EXCEPTION 'historial_estado es de solo inserción: no se permite %', TG_OP
        USING ERRCODE = 'insufficient_privilege',
              HINT = 'Registra un nuevo cambio de estado en lugar de modificar el historial';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER tg_historial_estado_sin_update
    BEFORE UPDATE ON historial_estado
    FOR EACH ROW EXECUTE FUNCTION fn_historial_estado_inmutable();

CREATE TRIGGER tg_historial_estado_sin_truncate
    BEFORE TRUNCATE ON historial_estado
    FOR EACH STATEMENT EXECUTE FUNCTION fn_historial_estado_inmutable();

-- +goose Down
SET LOCAL search_path = urbia, public;

DROP TRIGGER IF EXISTS tg_historial_estado_sin_truncate ON historial_estado;
DROP TRIGGER IF EXISTS tg_historial_estado_sin_update ON historial_estado;
DROP FUNCTION IF EXISTS fn_historial_estado_inmutable();

GRANT UPDATE, DELETE ON historial_estado TO urbia_app;