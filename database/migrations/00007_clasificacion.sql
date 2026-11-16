-- =====================================================================
-- URBIA · HU 06.3 · Migración 00007 — Confirmación de la clasificación
-- · reporte.origen_categoria: cómo se llegó a la categoría FINAL del reporte
--     'sugerida'  → el ciudadano aceptó la categoría que sugirió la IA
--     'corregida' → el ciudadano eligió otra distinta a la sugerida
--     'manual'    → no había sugerencia (IA caída, sin detección) o la eligió
--                   al crear el reporte, antes de ver la sugerencia
-- · clasificacion: bitácora de cada confirmación, para auditar la diferencia
--   entre lo que sugirió la IA y lo que decidió el ciudadano.
-- · La fila de deteccion NUNCA se modifica: aquí solo se referencia.
-- =====================================================================

-- +goose Up
SET LOCAL search_path = urbia, public;

ALTER TABLE reporte
    ADD COLUMN origen_categoria VARCHAR(10),
    ADD CONSTRAINT ck_reporte_origen_categoria
        CHECK (origen_categoria IN ('sugerida', 'corregida', 'manual'));

-- Los reportes que ya tienen categoría la recibieron al crearse: manual.
UPDATE reporte SET origen_categoria = 'manual' WHERE id_categoria IS NOT NULL;

-- Categoría y origen van juntos: o ambos o ninguno.
ALTER TABLE reporte
    ADD CONSTRAINT ck_reporte_categoria_origen
        CHECK ((id_categoria IS NULL) = (origen_categoria IS NULL));

CREATE TABLE clasificacion (
                               id_clasificacion      BIGINT GENERATED ALWAYS AS IDENTITY,
                               id_reporte            BIGINT       NOT NULL,
                               id_usuario            BIGINT       NOT NULL,
                               id_categoria          SMALLINT     NOT NULL, -- categoría final que confirmó el ciudadano
                               id_categoria_anterior SMALLINT,              -- la que tenía el reporte antes (NULL si no tenía)
                               id_categoria_sugerida SMALLINT,              -- la que sugirió la IA (NULL si no hubo sugerencia)
                               id_deteccion          BIGINT,                -- detección con la que se comparó
                               origen                VARCHAR(10)  NOT NULL,
                               fecha                 TIMESTAMPTZ  NOT NULL DEFAULT now(),

                               CONSTRAINT pk_clasificacion PRIMARY KEY (id_clasificacion),
                               CONSTRAINT fk_clasificacion_reporte FOREIGN KEY (id_reporte)
                                   REFERENCES reporte (id_reporte) ON DELETE RESTRICT,
                               CONSTRAINT fk_clasificacion_usuario FOREIGN KEY (id_usuario)
                                   REFERENCES usuario (id_usuario) ON DELETE RESTRICT,
                               CONSTRAINT fk_clasificacion_categoria FOREIGN KEY (id_categoria)
                                   REFERENCES categoria (id_categoria) ON DELETE RESTRICT,
                               CONSTRAINT fk_clasificacion_categoria_anterior FOREIGN KEY (id_categoria_anterior)
                                   REFERENCES categoria (id_categoria) ON DELETE RESTRICT,
                               CONSTRAINT fk_clasificacion_categoria_sugerida FOREIGN KEY (id_categoria_sugerida)
                                   REFERENCES categoria (id_categoria) ON DELETE RESTRICT,
                               CONSTRAINT fk_clasificacion_deteccion FOREIGN KEY (id_deteccion)
                                   REFERENCES deteccion (id_deteccion) ON DELETE RESTRICT,
                               CONSTRAINT ck_clasificacion_origen CHECK (origen IN ('sugerida', 'corregida', 'manual')),
    -- El origen debe ser coherente con la sugerencia guardada.
                               CONSTRAINT ck_clasificacion_coherencia CHECK (
                                   (origen = 'manual'    AND id_categoria_sugerida IS NULL) OR
                                   (origen = 'sugerida'  AND id_categoria = id_categoria_sugerida) OR
                                   (origen = 'corregida' AND id_categoria_sugerida IS NOT NULL
                                       AND id_categoria <> id_categoria_sugerida)
                                   ),
                               CONSTRAINT ck_clasificacion_sugerida_deteccion CHECK (
                                   id_categoria_sugerida IS NULL OR id_deteccion IS NOT NULL
                                   )
);

CREATE INDEX ix_clasificacion_id_reporte ON clasificacion (id_reporte, fecha DESC);
CREATE INDEX ix_clasificacion_origen     ON clasificacion (origen);

-- +goose Down
SET LOCAL search_path = urbia, public;

DROP TABLE IF EXISTS clasificacion;

ALTER TABLE reporte
DROP CONSTRAINT IF EXISTS ck_reporte_categoria_origen,
    DROP CONSTRAINT IF EXISTS ck_reporte_origen_categoria,
    DROP COLUMN IF EXISTS origen_categoria;