package repositories

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
)

type ConsultaReporteRepository struct{ db *pgxpool.Pool }

func NewConsultaReporteRepository(db *pgxpool.Pool) *ConsultaReporteRepository {
	return &ConsultaReporteRepository{db: db}
}

// selectReporte trae el reporte con su categoría, autor y foto original más reciente.
// Lee todo; el service recorta según quién consulta.
const selectReporte = `
	SELECT r.id_reporte, r.id_usuario, u.nombre, u.correo, r.estado::text, r.prioridad::text,
	       r.descripcion, r.id_categoria, c.nombre, r.origen_categoria,
	       ST_Y(r.ubicacion), ST_X(r.ubicacion), r.origen_ubicacion, r.precision_m::float8,
	       r.fecha_creacion, r.fecha_actualizacion,
	       f.id_archivo, f.mime, f.tamano, f.fecha_subida
	FROM reporte r
	JOIN usuario u ON u.id_usuario = r.id_usuario
	LEFT JOIN categoria c ON c.id_categoria = r.id_categoria
	LEFT JOIN LATERAL (
		SELECT a.id_archivo, a.mime, a.tamano, a.fecha_subida
		FROM archivo a
		WHERE a.id_reporte = r.id_reporte AND a.variante = 'original'
		ORDER BY a.fecha_subida DESC, a.id_archivo DESC
		LIMIT 1
	) f ON true`

func scanReporte(row pgx.Row) (*models.FilaReporte, error) {
	var f models.FilaReporte
	var estado string
	var idCategoria *int16
	var nombreCategoria *string
	var idFoto, tamanoFoto *int64
	var mimeFoto *string
	var fechaFoto *time.Time
	err := row.Scan(&f.ID, &f.IDUsuario, &f.NombreAutor, &f.CorreoAutor, &estado, &f.Prioridad,
		&f.Descripcion, &idCategoria, &nombreCategoria, &f.OrigenCategoria,
		&f.Latitud, &f.Longitud, &f.OrigenUbicacion, &f.PrecisionM,
		&f.FechaCreacion, &f.FechaActualizacion,
		&idFoto, &mimeFoto, &tamanoFoto, &fechaFoto)
	if err != nil {
		return nil, err
	}
	f.Estado = dominio.Estado(estado)
	if idCategoria != nil && nombreCategoria != nil {
		f.Categoria = &models.Categoria{ID: *idCategoria, Nombre: *nombreCategoria}
	}
	if idFoto != nil {
		f.Foto = &models.Archivo{ID: *idFoto, IDReporte: f.ID, MIME: *mimeFoto, Tamano: *tamanoFoto}
		if fechaFoto != nil {
			f.Foto.FechaSubida = *fechaFoto
		}
	}
	return &f, nil
}

// filtrosSQL arma el WHERE con parámetros numerados ($1, $2…); nunca concatena valores.
type filtrosSQL struct {
	condiciones []string
	args        []any
}

func (q *filtrosSQL) param(v any) string {
	q.args = append(q.args, v)
	return "$" + strconv.Itoa(len(q.args))
}

func (q *filtrosSQL) where() string {
	if len(q.condiciones) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(q.condiciones, " AND ")
}

func construirFiltros(f dominio.FiltrosReporte, idUsuario int64) *filtrosSQL {
	q := &filtrosSQL{}
	if len(f.Estados) > 0 {
		estados := make([]string, len(f.Estados))
		for i, e := range f.Estados {
			estados[i] = string(e)
		}
		q.condiciones = append(q.condiciones, "r.estado::text = ANY("+q.param(estados)+"::text[])")
	}
	if f.Categoria != nil {
		q.condiciones = append(q.condiciones, "r.id_categoria = "+q.param(*f.Categoria))
	}
	if f.Desde != nil {
		q.condiciones = append(q.condiciones, "r.fecha_creacion >= "+q.param(*f.Desde))
	}
	if f.Hasta != nil {
		q.condiciones = append(q.condiciones, "r.fecha_creacion < "+q.param(*f.Hasta))
	}
	if f.BBox != nil {
		// Usa el índice GIST ix_reporte_ubicacion (sobre ubicacion::geography).
		b := f.BBox
		q.condiciones = append(q.condiciones, fmt.Sprintf(
			"ST_Intersects(r.ubicacion::geography, ST_MakeEnvelope(%s::float8, %s::float8, %s::float8, %s::float8, 4326)::geography)",
			q.param(b.MinLon), q.param(b.MinLat), q.param(b.MaxLon), q.param(b.MaxLat)))
	}
	if f.SoloMios {
		q.condiciones = append(q.condiciones, "r.id_usuario = "+q.param(idUsuario))
	}
	return q
}

// Listar devuelve una página de reportes (más recientes primero) y el total que cumple los filtros.
func (r *ConsultaReporteRepository) Listar(ctx context.Context, f dominio.FiltrosReporte, idUsuario int64) ([]models.FilaReporte, int64, error) {
	q := construirFiltros(f, idUsuario)

	var total int64
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM reporte r"+q.where(), q.args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("contar reportes: %w", err)
	}
	if total == 0 {
		return []models.FilaReporte{}, 0, nil
	}

	sql := selectReporte + q.where() +
		" ORDER BY r.fecha_creacion DESC, r.id_reporte DESC" +
		" LIMIT " + q.param(f.PorPagina) + " OFFSET " + q.param(f.Offset())
	rows, err := r.db.Query(ctx, sql, q.args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listar reportes: %w", err)
	}
	defer rows.Close()

	filas := make([]models.FilaReporte, 0, f.PorPagina)
	for rows.Next() {
		fila, err := scanReporte(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("leer reporte: %w", err)
		}
		filas = append(filas, *fila)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("listar reportes: %w", err)
	}
	return filas, total, nil
}

// Detalle devuelve un reporte. Con completo=true agrega la última detección de la IA
// y el historial de estados (solo se piden si quien consulta puede verlos).
func (r *ConsultaReporteRepository) Detalle(ctx context.Context, id int64, completo bool) (*models.FilaReporte, error) {
	fila, err := scanReporte(r.db.QueryRow(ctx, selectReporte+" WHERE r.id_reporte = $1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("leer reporte: %w", err)
	}
	if !completo {
		return fila, nil
	}

	if err := r.ultimaDeteccion(ctx, fila); err != nil {
		return nil, err
	}
	if fila.Historial, err = r.historial(ctx, id); err != nil {
		return nil, err
	}
	return fila, nil
}

func (r *ConsultaReporteRepository) ultimaDeteccion(ctx context.Context, fila *models.FilaReporte) error {
	var d models.Deteccion
	var resultado string
	var idSug *int16
	var nombreSug *string
	err := r.db.QueryRow(ctx, `
		SELECT d.id_deteccion, d.resultado, d.clase_detectada, d.confianza::float8,
		       d.modelo, d.version_modelo, d.codigo_error, d.fecha,
		       cs.id_categoria, cs.nombre
		FROM deteccion d
		LEFT JOIN categoria cs ON lower(cs.nombre) = lower(d.clase_detectada) AND cs.activo
		WHERE d.id_reporte = $1
		ORDER BY d.fecha DESC, d.id_deteccion DESC
		LIMIT 1`, fila.ID).
		Scan(&d.ID, &resultado, &d.ClaseDetectada, &d.Confianza,
			&d.Modelo, &d.VersionModelo, &d.CodigoError, &d.Fecha, &idSug, &nombreSug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // reporte anterior a la 06.2: sin detección
	}
	if err != nil {
		return fmt.Errorf("leer detección: %w", err)
	}
	d.IDReporte = fila.ID
	d.Resultado = dominio.ResultadoDeteccion(resultado)
	fila.Deteccion = &d
	if idSug != nil && nombreSug != nil && d.Resultado == dominio.ResultadoDetectado {
		fila.CategoriaSugerida = &models.Categoria{ID: *idSug, Nombre: *nombreSug}
	}
	return nil
}

// Historial devuelve la secuencia de estados del reporte en orden cronológico,
// con el actor de cada evento (HU 07.3). El service decide cuánto del actor se muestra.
func (r *ConsultaReporteRepository) Historial(ctx context.Context, id int64) ([]models.HistorialEstado, error) {
	return r.historial(ctx, id)
}

func (r *ConsultaReporteRepository) historial(ctx context.Context, id int64) ([]models.HistorialEstado, error) {
	rows, err := r.db.Query(ctx, `
		SELECT h.id_historial_estado, h.estado_anterior::text, h.estado_nuevo::text, h.fecha,
		       u.id_usuario, u.nombre, u.rol::text
		FROM historial_estado h
		JOIN usuario u ON u.id_usuario = h.id_usuario
		WHERE h.id_reporte = $1
		ORDER BY h.fecha, h.id_historial_estado`, id)
	if err != nil {
		return nil, fmt.Errorf("leer historial: %w", err)
	}
	defer rows.Close()

	historial := []models.HistorialEstado{}
	for rows.Next() {
		var anterior *string
		var nuevo, nombre, rol string
		var idActor int64
		var h models.HistorialEstado
		if err := rows.Scan(&h.ID, &anterior, &nuevo, &h.Fecha, &idActor, &nombre, &rol); err != nil {
			return nil, fmt.Errorf("leer historial: %w", err)
		}
		if anterior != nil {
			e := dominio.Estado(*anterior)
			h.EstadoAnterior = &e
		}
		h.EstadoNuevo = dominio.Estado(nuevo)
		h.Tipo = dominio.TipoDeEvento(h.EstadoAnterior)
		h.Actor = &models.ActorHistorial{IDUsuario: &idActor, Nombre: &nombre, Rol: dominio.Rol(rol)}
		historial = append(historial, h)
	}
	return historial, rows.Err()
}
