-- =====================================================================
-- URBIA · HU 02.3 · Migración 00002 — Datos semilla (categorías)
-- Roles y estados no se insertan: sus valores están en los CHECK
-- (ck_usuario_rol) y en el DOMAIN estado_reporte.
-- =====================================================================

-- +goose Up
SET LOCAL search_path = urbia, public;

INSERT INTO categoria (nombre, descripcion) VALUES
                                                ('bache',             'Hundimiento o daño en el pavimento'),
                                                ('alumbrado_publico', 'Luminaria apagada, intermitente o dañada'),
                                                ('basura',            'Acumulación de basura o tiradero en vía pública'),
                                                ('fuga_agua',         'Fuga de agua potable o drenaje'),
                                                ('banqueta_danada',   'Banqueta rota, levantada o con obstáculos'),
                                                ('senalizacion',      'Señal de tránsito dañada, caída o faltante'),
                                                ('grafiti',           'Pintas en muros, mobiliario o señalización'),
                                                ('arbol_caido',       'Árbol o rama caída que obstruye el paso')
    ON CONFLICT DO NOTHING;

-- +goose Down
SET LOCAL search_path = urbia, public;

DELETE FROM categoria
WHERE nombre IN ('bache', 'alumbrado_publico', 'basura', 'fuga_agua',
                 'banqueta_danada', 'senalizacion', 'grafiti', 'arbol_caido');