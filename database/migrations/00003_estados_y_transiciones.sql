-- =====================================================================
-- URBIA · HU 02.4 · Migración 00003 — Estados y transiciones
-- · Estados finales: pendiente, en_revision, en_proceso, resuelto
-- · Tabla transicion_estado con las transiciones permitidas
-- · Trigger que rechaza cambios de estado no permitidos en reporte
-- =====================================================================

-- +goose Up
SET LOCAL search_path = urbia, public;

-- Estados definitivos (se quita 'rechazado')
ALTER DOMAIN estado_reporte DROP CONSTRAINT ck_estado_reporte;
ALTER DOMAIN estado_reporte ADD CONSTRAINT ck_estado_reporte
    CHECK (VALUE IN ('pendiente', 'en_revision', 'en_proceso', 'resuelto'));

-- Transiciones permitidas: de qué estado a qué estado puede pasar un reporte
CREATE TABLE transicion_estado (
                                   estado_origen  estado_reporte NOT NULL,
                                   estado_destino estado_reporte NOT NULL,
                                   CONSTRAINT pk_transicion_estado PRIMARY KEY (estado_origen, estado_destino),
                                   CONSTRAINT ck_transicion_estado_distinto CHECK (estado_origen <> estado_destino)
);

INSERT INTO transicion_estado (estado_origen, estado_destino) VALUES
                                                                  ('pendiente',   'en_revision'),
                                                                  ('en_revision', 'en_proceso'),
                                                                  ('en_proceso',  'resuelto');

-- +goose StatementBegin
CREATE FUNCTION fn_validar_transicion_estado() RETURNS trigger AS $$
BEGIN
    IF NEW.estado IS DISTINCT FROM OLD.estado THEN
        IF NOT EXISTS (
            SELECT 1 FROM urbia.transicion_estado
            WHERE estado_origen = OLD.estado AND estado_destino = NEW.estado
        ) THEN
            RAISE EXCEPTION 'Transición de estado no permitida: % → %', OLD.estado, NEW.estado
                USING ERRCODE = 'check_violation';
END IF;
        NEW.fecha_actualizacion := now();
END IF;
RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER tg_reporte_transicion_estado
    BEFORE UPDATE OF estado ON reporte
    FOR EACH ROW EXECUTE FUNCTION fn_validar_transicion_estado();

GRANT SELECT ON transicion_estado TO urbia_app;

-- +goose Down
SET LOCAL search_path = urbia, public;

DROP TRIGGER IF EXISTS tg_reporte_transicion_estado ON reporte;
DROP FUNCTION IF EXISTS fn_validar_transicion_estado();
DROP TABLE IF EXISTS transicion_estado;

ALTER DOMAIN estado_reporte DROP CONSTRAINT ck_estado_reporte;
ALTER DOMAIN estado_reporte ADD CONSTRAINT ck_estado_reporte
    CHECK (VALUE IN ('pendiente', 'en_revision', 'en_proceso', 'resuelto', 'rechazado'));