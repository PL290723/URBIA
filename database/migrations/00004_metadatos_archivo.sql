-- =====================================================================
-- URBIA · HU 04.1 · Migración 00004 — Metadatos de archivo
-- · variante: foto original del ciudadano o imagen procesada por el modelo
-- · sha256: huella del contenido (integridad y detección de duplicados)
-- · ruta: solo rutas relativas generadas por el backend (sin traversal)
-- =====================================================================

-- +goose Up
SET LOCAL search_path = urbia, public;

ALTER TABLE archivo
    ADD COLUMN variante VARCHAR(20) NOT NULL DEFAULT 'original',
    ADD COLUMN sha256   CHAR(64)    NOT NULL,
    ADD CONSTRAINT ck_archivo_variante CHECK (variante IN ('original', 'procesada')),
    ADD CONSTRAINT ck_archivo_sha256   CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    ADD CONSTRAINT ck_archivo_ruta     CHECK (
        ruta ~ '^(originales|procesadas)/[0-9]{4}/[0-9]{2}/[a-z2-7]{26}\.(jpg|png|webp)$'
    ),
    ADD CONSTRAINT ck_archivo_ruta_variante CHECK (
        (variante = 'original'  AND ruta LIKE 'originales/%') OR
        (variante = 'procesada' AND ruta LIKE 'procesadas/%')
    );

CREATE INDEX ix_archivo_sha256 ON archivo (sha256);

-- +goose Down
SET LOCAL search_path = urbia, public;

DROP INDEX IF EXISTS ix_archivo_sha256;
ALTER TABLE archivo
DROP CONSTRAINT IF EXISTS ck_archivo_ruta_variante,
    DROP CONSTRAINT IF EXISTS ck_archivo_ruta,
    DROP CONSTRAINT IF EXISTS ck_archivo_sha256,
    DROP CONSTRAINT IF EXISTS ck_archivo_variante,
    DROP COLUMN IF EXISTS sha256,
    DROP COLUMN IF EXISTS variante;