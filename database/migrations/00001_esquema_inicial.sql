-- =====================================================================
-- URBIA · HU 02.3 · Migración 00001 — Esquema inicial
-- Se aplica con: goose up   (usuario urbia_owner, BD urbia)
-- =====================================================================

-- +goose Up
SET LOCAL search_path = urbia, public;

CREATE EXTENSION IF NOT EXISTS postgis;

CREATE DOMAIN estado_reporte AS VARCHAR(20)
    CONSTRAINT ck_estado_reporte
        CHECK (VALUE IN ('pendiente', 'en_revision', 'en_proceso', 'resuelto', 'rechazado'));

CREATE TABLE usuario (
                         id_usuario          BIGINT GENERATED ALWAYS AS IDENTITY,
                         nombre              VARCHAR(100) NOT NULL,
                         correo              VARCHAR(150) NOT NULL,
                         hash_contrasena     VARCHAR(255) NOT NULL,
                         rol                 VARCHAR(20)  NOT NULL DEFAULT 'ciudadano',
                         fecha_registro      TIMESTAMPTZ  NOT NULL DEFAULT now(),
                         fecha_actualizacion TIMESTAMPTZ  NOT NULL DEFAULT now(),
                         CONSTRAINT pk_usuario PRIMARY KEY (id_usuario),
                         CONSTRAINT ck_usuario_nombre CHECK (length(trim(nombre)) > 0),
                         CONSTRAINT ck_usuario_correo CHECK (correo LIKE '%_@_%._%'),
                         CONSTRAINT ck_usuario_rol    CHECK (rol IN ('ciudadano', 'administrador')),
                         CONSTRAINT ck_usuario_fechas CHECK (fecha_actualizacion >= fecha_registro)
);
CREATE UNIQUE INDEX uq_usuario_correo ON usuario (lower(correo));

CREATE TABLE categoria (
                           id_categoria SMALLINT GENERATED ALWAYS AS IDENTITY,
                           nombre       VARCHAR(60)  NOT NULL,
                           descripcion  VARCHAR(255),
                           activo       BOOLEAN      NOT NULL DEFAULT TRUE,
                           CONSTRAINT pk_categoria PRIMARY KEY (id_categoria)
);
CREATE UNIQUE INDEX uq_categoria_nombre ON categoria (lower(nombre));

CREATE TABLE reporte (
                         id_reporte          BIGINT GENERATED ALWAYS AS IDENTITY,
                         id_usuario          BIGINT         NOT NULL,
                         id_categoria        SMALLINT,
                         descripcion         TEXT           NOT NULL,
                         ubicacion           GEOMETRY(Point, 4326) NOT NULL,
                         estado              estado_reporte NOT NULL DEFAULT 'pendiente',
                         prioridad           VARCHAR(10)    NOT NULL DEFAULT 'media',
                         fecha_creacion      TIMESTAMPTZ    NOT NULL DEFAULT now(),
                         fecha_actualizacion TIMESTAMPTZ    NOT NULL DEFAULT now(),
                         CONSTRAINT pk_reporte PRIMARY KEY (id_reporte),
                         CONSTRAINT fk_reporte_usuario FOREIGN KEY (id_usuario)
                             REFERENCES usuario (id_usuario) ON DELETE RESTRICT,
                         CONSTRAINT fk_reporte_categoria FOREIGN KEY (id_categoria)
                             REFERENCES categoria (id_categoria) ON DELETE RESTRICT,
                         CONSTRAINT ck_reporte_descripcion CHECK (length(trim(descripcion)) BETWEEN 10 AND 1000),
                         CONSTRAINT ck_reporte_ubicacion CHECK (
                             ST_X(ubicacion) BETWEEN -180 AND 180
                                 AND ST_Y(ubicacion) BETWEEN -90 AND 90
                             ),
                         CONSTRAINT ck_reporte_prioridad CHECK (prioridad IN ('baja', 'media', 'alta')),
                         CONSTRAINT ck_reporte_fechas    CHECK (fecha_actualizacion >= fecha_creacion)
);

CREATE TABLE deteccion (
                           id_deteccion    BIGINT GENERATED ALWAYS AS IDENTITY,
                           id_reporte      BIGINT       NOT NULL,
                           modelo          VARCHAR(60)  NOT NULL,
                           version_modelo  VARCHAR(20)  NOT NULL,
                           clase_detectada VARCHAR(60),
                           confianza       NUMERIC(5,4),
                           resultado       VARCHAR(20)  NOT NULL,
                           fecha           TIMESTAMPTZ  NOT NULL DEFAULT now(),
                           CONSTRAINT pk_deteccion PRIMARY KEY (id_deteccion),
                           CONSTRAINT fk_deteccion_reporte FOREIGN KEY (id_reporte)
                               REFERENCES reporte (id_reporte) ON DELETE CASCADE,
                           CONSTRAINT ck_deteccion_confianza CHECK (confianza BETWEEN 0 AND 1),
                           CONSTRAINT ck_deteccion_resultado CHECK (resultado IN ('detectado', 'no_detectado', 'error')),
                           CONSTRAINT ck_deteccion_consistencia CHECK (
                               resultado <> 'detectado' OR (clase_detectada IS NOT NULL AND confianza IS NOT NULL)
                               )
);

CREATE TABLE archivo (
                         id_archivo   BIGINT GENERATED ALWAYS AS IDENTITY,
                         id_reporte   BIGINT       NOT NULL,
                         tipo         VARCHAR(20)  NOT NULL,
                         ruta         VARCHAR(500) NOT NULL,
                         mime         VARCHAR(50)  NOT NULL,
                         tamano       INTEGER      NOT NULL,
                         fecha_subida TIMESTAMPTZ  NOT NULL DEFAULT now(),
                         CONSTRAINT pk_archivo PRIMARY KEY (id_archivo),
                         CONSTRAINT fk_archivo_reporte FOREIGN KEY (id_reporte)
                             REFERENCES reporte (id_reporte) ON DELETE CASCADE,
                         CONSTRAINT uq_archivo_ruta   UNIQUE (ruta),
                         CONSTRAINT ck_archivo_tipo   CHECK (tipo IN ('imagen', 'video')),
                         CONSTRAINT ck_archivo_mime   CHECK (mime IN ('image/jpeg', 'image/png', 'image/webp', 'video/mp4')),
                         CONSTRAINT ck_archivo_tamano CHECK (tamano > 0 AND tamano <= 10485760)
);

CREATE TABLE historial_estado (
                                  id_historial_estado BIGINT GENERATED ALWAYS AS IDENTITY,
                                  id_reporte          BIGINT         NOT NULL,
                                  id_usuario          BIGINT         NOT NULL,
                                  estado_anterior     estado_reporte,
                                  estado_nuevo        estado_reporte NOT NULL,
                                  fecha               TIMESTAMPTZ    NOT NULL DEFAULT now(),
                                  CONSTRAINT pk_historial_estado PRIMARY KEY (id_historial_estado),
                                  CONSTRAINT fk_historial_estado_reporte FOREIGN KEY (id_reporte)
                                      REFERENCES reporte (id_reporte) ON DELETE RESTRICT,
                                  CONSTRAINT fk_historial_estado_usuario FOREIGN KEY (id_usuario)
                                      REFERENCES usuario (id_usuario) ON DELETE RESTRICT,
                                  CONSTRAINT ck_historial_estado_cambio CHECK (
                                      estado_anterior IS NULL OR estado_anterior <> estado_nuevo
                                      )
);

CREATE INDEX ix_reporte_id_usuario     ON reporte (id_usuario);
CREATE INDEX ix_reporte_id_categoria   ON reporte (id_categoria);
CREATE INDEX ix_reporte_estado         ON reporte (estado);
CREATE INDEX ix_reporte_fecha_creacion ON reporte (fecha_creacion DESC);
CREATE INDEX ix_reporte_ubicacion      ON reporte USING GIST ((ubicacion::geography));
CREATE INDEX ix_deteccion_id_reporte   ON deteccion (id_reporte);
CREATE INDEX ix_archivo_id_reporte     ON archivo (id_reporte);
CREATE INDEX ix_historial_estado_id_reporte ON historial_estado (id_reporte, fecha);
CREATE INDEX ix_historial_estado_id_usuario ON historial_estado (id_usuario);

-- +goose Down
SET LOCAL search_path = urbia, public;

DROP TABLE IF EXISTS historial_estado;
DROP TABLE IF EXISTS archivo;
DROP TABLE IF EXISTS deteccion;
DROP TABLE IF EXISTS reporte;
DROP TABLE IF EXISTS categoria;
DROP TABLE IF EXISTS usuario;
DROP DOMAIN IF EXISTS estado_reporte;