-- =====================================================================
-- URBIA · HU 02.2 · Script 04 — Prueba espacial (evidencia del criterio)
-- Ejecutar como: urbia_app, conectado a la BD "urbia"
-- Todo va dentro de una transacción con ROLLBACK: no deja datos.
-- =====================================================================

BEGIN;

INSERT INTO usuario (nombre, correo, hash_contrasena)
VALUES ('Usuario Prueba', 'prueba@urbia.mx', 'hash_de_prueba');

INSERT INTO categoria (nombre, descripcion)
VALUES ('Bache', 'Daño en el pavimento');

-- ST_MakePoint(longitud, latitud)  ← el orden es lon, lat
INSERT INTO reporte (id_usuario, id_categoria, descripcion, ubicacion)
SELECT u.id_usuario, c.id_categoria, d.descripcion,
       ST_SetSRID(ST_MakePoint(d.lon, d.lat), 4326)
FROM usuario u, categoria c,
     (VALUES
        ('Bache frente al TecNM Celaya',        -100.8166, 20.5413),
        ('Bache en el Jardín Principal',         -100.8157, 20.5235),
        ('Bache en la salida a Querétaro',       -100.7700, 20.5560)
     ) AS d(descripcion, lon, lat)
WHERE u.correo = 'prueba@urbia.mx' AND c.nombre = 'Bache';

-- (ubicacion se guarda como geometry; ::geography hace que la distancia sea en metros)
-- 1) Reportes a menos de 3 km del TecNM Celaya, ordenados por distancia (metros)
SELECT id_reporte,
       descripcion,
       round(ST_Distance(ubicacion::geography,
             ST_SetSRID(ST_MakePoint(-100.8166, 20.5413), 4326)::geography)) AS distancia_m
FROM reporte
WHERE ST_DWithin(ubicacion::geography,
                 ST_SetSRID(ST_MakePoint(-100.8166, 20.5413), 4326)::geography,
                 3000)
ORDER BY distancia_m;

-- 2) La restricción de coordenadas debe RECHAZAR una latitud inválida (95°)
SAVEPOINT antes_invalido;
DO $$
BEGIN
    INSERT INTO reporte (id_usuario, descripcion, ubicacion)
    SELECT id_usuario, 'Coordenada inválida de prueba',
           ST_SetSRID(ST_MakePoint(-100.8, 95), 4326)
    FROM usuario WHERE correo = 'prueba@urbia.mx';
    RAISE NOTICE 'ERROR: se aceptó una latitud inválida';
EXCEPTION WHEN OTHERS THEN
    RAISE NOTICE 'OK: latitud inválida rechazada → %', SQLERRM;
END $$;

-- 3) El índice espacial existe y la consulta lo usa
SET LOCAL enable_seqscan = off;  -- con 3 filas el planificador preferiría leer toda la tabla
EXPLAIN (COSTS OFF)
SELECT id_reporte FROM reporte
WHERE ST_DWithin(ubicacion::geography,
                 ST_SetSRID(ST_MakePoint(-100.8166, 20.5413), 4326)::geography, 3000);

SELECT indexname, indexdef
FROM pg_indexes
WHERE tablename = 'reporte' AND indexname = 'ix_reporte_ubicacion';

ROLLBACK;
