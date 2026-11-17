package repositories

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
)

// EstadisticasRepository calcula los agregados del panel (HU 07.4).
// Solo cuenta: ninguna consulta devuelve datos de usuarios ni coordenadas exactas.
type EstadisticasRepository struct{ db *pgxpool.Pool }

func NewEstadisticasRepository(db *pgxpool.Pool) *EstadisticasRepository {
	return &EstadisticasRepository{db: db}
}

// Calcular corre todas las consultas en UNA transacción de solo lectura
// REPEATABLE READ: los totales cuadran entre sí aunque lleguen reportes nuevos
// mientras se calculan.
func (r *EstadisticasRepository) Calcular(ctx context.Context, f dominio.FiltrosEstadisticas) (*models.DatosEstadisticas, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("iniciar transacción: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	d := &models.DatosEstadisticas{}
	if d.PorEstado, err = porEstado(ctx, tx, f); err != nil {
		return nil, err
	}
	if d.PorCategoria, err = porCategoria(ctx, tx, f); err != nil {
		return nil, err
	}
	if d.PorPeriodo, err = porPeriodo(ctx, tx, f); err != nil {
		return nil, err
	}
	if f.Geo {
		if d.Celdas, err = celdas(ctx, tx, f); err != nil {
			return nil, err
		}
	}
	return d, tx.Commit(ctx)
}

func porEstado(ctx context.Context, tx pgx.Tx, f dominio.FiltrosEstadisticas) (map[dominio.Estado]int64, error) {
	rows, err := tx.Query(ctx, `
		SELECT estado::text, count(*)
		FROM reporte
		WHERE fecha_creacion >= $1 AND fecha_creacion < $2
		GROUP BY estado`, f.Desde, f.HastaExcl)
	if err != nil {
		return nil, fmt.Errorf("contar por estado: %w", err)
	}
	defer rows.Close()

	conteo := map[dominio.Estado]int64{}
	for rows.Next() {
		var estado string
		var n int64
		if err := rows.Scan(&estado, &n); err != nil {
			return nil, fmt.Errorf("leer conteo por estado: %w", err)
		}
		conteo[dominio.Estado(estado)] = n
	}
	return conteo, rows.Err()
}

// porCategoria cuenta por categoría y estado. Incluye las categorías activas sin
// reportes (estado NULL, total 0) y los reportes sin categoría (id NULL).
func porCategoria(ctx context.Context, tx pgx.Tx, f dominio.FiltrosEstadisticas) ([]models.FilaCategoriaEstado, error) {
	rows, err := tx.Query(ctx, `
		SELECT c.id_categoria, c.nombre, c.activo, r.estado::text, count(r.id_reporte)
		FROM categoria c
		LEFT JOIN reporte r
		  ON r.id_categoria = c.id_categoria
		 AND r.fecha_creacion >= $1 AND r.fecha_creacion < $2
		WHERE c.activo OR r.id_reporte IS NOT NULL
		GROUP BY c.id_categoria, c.nombre, c.activo, r.estado
		UNION ALL
		SELECT NULL, NULL, TRUE, r.estado::text, count(*)
		FROM reporte r
		WHERE r.id_categoria IS NULL
		  AND r.fecha_creacion >= $1 AND r.fecha_creacion < $2
		GROUP BY r.estado`, f.Desde, f.HastaExcl)
	if err != nil {
		return nil, fmt.Errorf("contar por categoría: %w", err)
	}
	defer rows.Close()

	filas := []models.FilaCategoriaEstado{}
	for rows.Next() {
		var fila models.FilaCategoriaEstado
		var estado *string
		if err := rows.Scan(&fila.IDCategoria, &fila.Nombre, &fila.Activa, &estado, &fila.Total); err != nil {
			return nil, fmt.Errorf("leer conteo por categoría: %w", err)
		}
		if estado != nil {
			e := dominio.Estado(*estado)
			fila.Estado = &e
		}
		filas = append(filas, fila)
	}
	return filas, rows.Err()
}

// porPeriodo devuelve la serie COMPLETA (periodos sin reportes con total 0),
// agrupando por día/semana/mes en hora de Celaya.
func porPeriodo(ctx context.Context, tx pgx.Tx, f dominio.FiltrosEstadisticas) ([]models.ConteoPeriodo, error) {
	rows, err := tx.Query(ctx, `
		WITH conteo AS (
		    SELECT date_trunc($3::text, fecha_creacion AT TIME ZONE '`+dominio.ZonaHorariaEstadisticas+`') AS inicio,
		           count(*) AS total
		    FROM reporte
		    WHERE fecha_creacion >= $1 AND fecha_creacion < $2
		    GROUP BY 1
		)
		SELECT to_char(s.inicio, 'YYYY-MM-DD'), COALESCE(c.total, 0)
		FROM generate_series(date_trunc($3::text, $4::date::timestamp),
		                     date_trunc($3::text, $5::date::timestamp),
		                     ('1 ' || $3::text)::interval) AS s(inicio)
		LEFT JOIN conteo c ON c.inicio = s.inicio
		ORDER BY s.inicio`,
		f.Desde, f.HastaExcl, f.Agrupacion.UnidadSQL(), f.DesdeFecha(), f.HastaFecha())
	if err != nil {
		return nil, fmt.Errorf("contar por periodo: %w", err)
	}
	defer rows.Close()

	serie := []models.ConteoPeriodo{}
	for rows.Next() {
		var p models.ConteoPeriodo
		if err := rows.Scan(&p.Inicio, &p.Total); err != nil {
			return nil, fmt.Errorf("leer conteo por periodo: %w", err)
		}
		serie = append(serie, p)
	}
	return serie, rows.Err()
}

// celdas agrupa los reportes en una rejilla de celda_grados × celda_grados y
// devuelve el CENTRO de cada celda con su total. El mínimo por celda lo aplica el service.
func celdas(ctx context.Context, tx pgx.Tx, f dominio.FiltrosEstadisticas) ([]models.CeldaGeo, error) {
	rows, err := tx.Query(ctx, `
		SELECT round(((floor(ST_Y(ubicacion) / $3::float8) + 0.5) * $3::float8)::numeric, 5)::float8,
		       round(((floor(ST_X(ubicacion) / $3::float8) + 0.5) * $3::float8)::numeric, 5)::float8,
		       count(*)
		FROM reporte
		WHERE fecha_creacion >= $1 AND fecha_creacion < $2
		GROUP BY floor(ST_Y(ubicacion) / $3::float8), floor(ST_X(ubicacion) / $3::float8)
		ORDER BY 3 DESC, 1, 2`, f.Desde, f.HastaExcl, f.CeldaGrados)
	if err != nil {
		return nil, fmt.Errorf("agrupar por celdas: %w", err)
	}
	defer rows.Close()

	out := []models.CeldaGeo{}
	for rows.Next() {
		var c models.CeldaGeo
		if err := rows.Scan(&c.Latitud, &c.Longitud, &c.Total); err != nil {
			return nil, fmt.Errorf("leer celda: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
