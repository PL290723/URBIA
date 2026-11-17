-- =====================================================================
-- URBIA · HU 07.1 · Migración 00008 — Cálculo de prioridad explicable
-- · calculo_prioridad: bitácora de cada cálculo con el resultado y los
--   factores usados (JSONB), para poder explicar cualquier prioridad.
-- · Las evaluaciones del administrador (gravedad y afectación) se guardan
--   en columnas para reutilizarlas al recalcular.
-- · reporte.origen_prioridad: 'por_defecto' (el 'media' con que nace el
--   reporte, NO calculado) o 'calculada' (asignada por la fórmula).
-- · Los pesos son PROVISIONALES (version_formula 'v1-provisional').
-- =====================================================================

-- +goose Up
SET LOCAL search_path = urbia, public;

ALTER TABLE reporte
    ADD COLUMN origen_prioridad VARCHAR(12) NOT NULL DEFAULT 'por_defecto',
    ADD CONSTRAINT ck_reporte_origen_prioridad
        CHECK (origen_prioridad IN ('por_defecto', 'calculada'));

CREATE TABLE calculo_prioridad (
                                   id_calculo_prioridad     BIGINT GENERATED ALWAYS AS IDENTITY,
                                   id_reporte               BIGINT       NOT NULL,
                                   id_usuario               BIGINT       NOT NULL, -- administrador que lo calculó
                                   version_formula          VARCHAR(20)  NOT NULL,
    -- Entradas (lo que se sabía al calcular)
                                   gravedad_observable      SMALLINT,              -- 1-5, evaluada por el administrador
                                   afectacion_estimada      SMALLINT,              -- 1-5, evaluada por el administrador
                                   justificacion_afectacion VARCHAR(500),          -- obligatoria si hay afectación
                                   id_categoria             SMALLINT,              -- categoría del reporte al calcular
                                   recurrencias             INTEGER,               -- NULL: no verificable (sin categoría)
                                   dias_antiguedad          NUMERIC(7,1) NOT NULL,
    -- Resultado
                                   cobertura                NUMERIC(3,2) NOT NULL, -- suma de pesos de los factores disponibles
                                   cobertura_minima         NUMERIC(3,2) NOT NULL, -- la exigida al calcular
                                   puntuacion               NUMERIC(5,1) NOT NULL, -- 0-100
                                   nivel                    VARCHAR(10),           -- NULL: datos insuficientes
                                   factores                 JSONB        NOT NULL, -- explicación factor por factor
                                   resumen                  VARCHAR(300) NOT NULL,
                                   prioridad_asignada       BOOLEAN      NOT NULL, -- si actualizó reporte.prioridad
                                   fecha                    TIMESTAMPTZ  NOT NULL DEFAULT now(),

                                   CONSTRAINT pk_calculo_prioridad PRIMARY KEY (id_calculo_prioridad),
                                   CONSTRAINT fk_calculo_prioridad_reporte FOREIGN KEY (id_reporte)
                                       REFERENCES reporte (id_reporte) ON DELETE CASCADE,
                                   CONSTRAINT fk_calculo_prioridad_usuario FOREIGN KEY (id_usuario)
                                       REFERENCES usuario (id_usuario) ON DELETE RESTRICT,
                                   CONSTRAINT fk_calculo_prioridad_categoria FOREIGN KEY (id_categoria)
                                       REFERENCES categoria (id_categoria) ON DELETE RESTRICT,
                                   CONSTRAINT ck_calculo_prioridad_gravedad   CHECK (gravedad_observable BETWEEN 1 AND 5),
                                   CONSTRAINT ck_calculo_prioridad_afectacion CHECK (afectacion_estimada BETWEEN 1 AND 5),
    -- La afectación no se deduce solo de la foto: exige justificación.
                                   CONSTRAINT ck_calculo_prioridad_justificacion CHECK (
                                       afectacion_estimada IS NULL OR length(trim(justificacion_afectacion)) >= 10
                                       ),
                                   CONSTRAINT ck_calculo_prioridad_recurrencias CHECK (recurrencias >= 0),
                                   CONSTRAINT ck_calculo_prioridad_dias       CHECK (dias_antiguedad >= 0),
                                   CONSTRAINT ck_calculo_prioridad_cobertura  CHECK (cobertura BETWEEN 0 AND 1),
                                   CONSTRAINT ck_calculo_prioridad_cob_minima CHECK (cobertura_minima BETWEEN 0 AND 1),
                                   CONSTRAINT ck_calculo_prioridad_puntuacion CHECK (puntuacion BETWEEN 0 AND 100),
                                   CONSTRAINT ck_calculo_prioridad_nivel      CHECK (nivel IN ('baja', 'media', 'alta')),
                                   CONSTRAINT ck_calculo_prioridad_asignada   CHECK (prioridad_asignada = (nivel IS NOT NULL)),
                                   CONSTRAINT ck_calculo_prioridad_factores   CHECK (jsonb_typeof(factores) = 'array')
);

CREATE INDEX ix_calculo_prioridad_id_reporte ON calculo_prioridad (id_reporte, fecha DESC);

-- +goose Down
SET LOCAL search_path = urbia, public;

DROP TABLE IF EXISTS calculo_prioridad;

ALTER TABLE reporte
DROP CONSTRAINT IF EXISTS ck_reporte_origen_prioridad,
    DROP COLUMN IF EXISTS origen_prioridad;