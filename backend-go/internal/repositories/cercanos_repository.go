package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
)

// MaxCandidatosCercanos acota cuántos reportes dentro del radio se leen para
// puntuarlos en Go (los más cercanos primero).
const MaxCandidatosCercanos = 200

// CercanosRepository solo LEE: nunca modifica, fusiona ni borra reportes (HU 06.5).
type CercanosRepository struct{ db *pgxpool.Pool }

func NewCercanosRepository(db *pgxpool.Pool) *CercanosRepository {
	return &CercanosRepository{db: db}
}

const columnasCercano = `
	r.id_reporte, r.id_usuario, r.estado::text, r.descripcion, r.id_categoria, c.nombre,
	ST_Y(r.ubicacion), ST_X(r.ubicacion), r.fecha_creacion`

func scanCercano(row pgx.Row, extra ...any) (*models.ReporteCercanoFila, error) {
	var f models.ReporteCercanoFila
	var estado string
	var idCat *int16
	var nombreCat *string
	destinos := append([]any{&f.ID, &f.IDUsuario, &estado, &f.Descripcion, &idCat, &nombreCat,
		&f.Latitud, &f.Longitud, &f.FechaCreacion}, extra...)
	if err := row.Scan(destinos...); err != nil {
		return nil, err
	}
	f.Estado = dominio.Estado(estado)
	if idCat != nil && nombreCat != nil {
		f.Categoria = &models.Categoria{ID: *idCat, Nombre: *nombreCat}
	}
	return &f, nil
}

// Base devuelve el reporte que se revisa (ErrNoEncontrado si no existe).
func (r *CercanosRepository) Base(ctx context.Context, id int64) (*models.ReporteCercanoFila, error) {
	f, err := scanCercano(r.db.QueryRow(ctx, `
		SELECT`+columnasCercano+`
		FROM reporte r
		LEFT JOIN categoria c ON c.id_categoria = r.id_categoria
		WHERE r.id_reporte = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("leer reporte base: %w", err)
	}
	return f, nil
}

// Candidatos devuelve los reportes a menos de radioM metros del reporte base,
// del más cercano al más lejano. ST_DWithin sobre geography usa el índice GIST
// ix_reporte_ubicacion y mide en metros reales (no en grados).
func (r *CercanosRepository) Candidatos(ctx context.Context, idBase int64, radioM int) ([]models.ReporteCercanoFila, error) {
	rows, err := r.db.Query(ctx, `
		SELECT`+columnasCercano+`,
		       ST_Distance(r.ubicacion::geography, b.ubicacion::geography) AS distancia_m
		FROM reporte b
		JOIN reporte r
		  ON r.id_reporte <> b.id_reporte
		 AND ST_DWithin(r.ubicacion::geography, b.ubicacion::geography, $2::float8)
		LEFT JOIN categoria c ON c.id_categoria = r.id_categoria
		WHERE b.id_reporte = $1
		ORDER BY distancia_m, r.id_reporte
		LIMIT $3`, idBase, radioM, MaxCandidatosCercanos)
	if err != nil {
		return nil, fmt.Errorf("buscar reportes cercanos: %w", err)
	}
	defer rows.Close()

	candidatos := []models.ReporteCercanoFila{}
	for rows.Next() {
		var distancia float64
		f, err := scanCercano(rows, &distancia)
		if err != nil {
			return nil, fmt.Errorf("leer reporte cercano: %w", err)
		}
		f.DistanciaM = distancia
		candidatos = append(candidatos, *f)
	}
	return candidatos, rows.Err()
}
