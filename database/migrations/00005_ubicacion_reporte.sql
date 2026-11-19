-- =====================================================================
-- URBIA · HU 06.1 · Migración 00005 — Origen y precisión de la ubicación
-- · origen_ubicacion: 'gps' (lo tomó el celular) o 'manual' (lo marcó el
--   ciudadano en el mapa porque el GPS no estaba disponible o era impreciso)
-- · precision_m: radio de precisión del GPS en metros (solo si es 'gps')
-- =====================================================================

-- +goose Up
SET LOCAL search_path = urbia, public;

-- El DEFAULT solo sirve para rellenar los reportes que ya existen;
-- después se quita para que el backend siempre lo mande explícito.
ALTER TABLE reporte
    ADD COLUMN origen_ubicacion VARCHAR(10)  NOT NULL DEFAULT 'manual',
    ADD COLUMN precision_m      NUMERIC(6,2),
    ADD CONSTRAINT ck_reporte_origen_ubicacion CHECK (origen_ubicacion IN ('gps', 'manual')),
    ADD CONSTRAINT ck_reporte_precision        CHECK (precision_m IS NULL OR (precision_m > 0 AND precision_m <= 100)),
    ADD CONSTRAINT ck_reporte_precision_origen CHECK (
        (origen_ubicacion = 'gps'    AND precision_m IS NOT NULL) OR
        (origen_ubicacion = 'manual' AND precision_m IS NULL)
    );

ALTER TABLE reporte ALTER COLUMN origen_ubicacion DROP DEFAULT;

-- +goose Down
SET LOCAL search_path = urbia, public;

ALTER TABLE reporte
DROP CONSTRAINT IF EXISTS ck_reporte_precision_origen,
    DROP CONSTRAINT IF EXISTS ck_reporte_precision,
    DROP CONSTRAINT IF EXISTS ck_reporte_origen_ubicacion,
    DROP COLUMN IF EXISTS precision_m,
    DROP COLUMN IF EXISTS origen_ubicacion;