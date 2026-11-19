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

type DeteccionRepository struct{ db *pgxpool.Pool }

func NewDeteccionRepository(db *pgxpool.Pool) *DeteccionRepository {
	return &DeteccionRepository{db: db}
}

// Crear guarda el resultado de la IA para un reporte (también los errores).
func (r *DeteccionRepository) Crear(ctx context.Context, d *models.Deteccion) error {
	const q = `
		INSERT INTO deteccion (id_reporte, modelo, version_modelo, clase_detectada, confianza,
		                       resultado, umbral_confianza, codigo_error, milisegundos)
		VALUES ($1, $2, $3, $4, $5::float8, $6, $7::float8, $8, $9::float8)
		RETURNING id_deteccion, fecha`
	err := r.db.QueryRow(ctx, q,
		d.IDReporte, d.Modelo, d.VersionModelo, d.ClaseDetectada, d.Confianza,
		string(d.Resultado), d.UmbralConfianza, d.CodigoError, d.Milisegundos,
	).Scan(&d.ID, &d.Fecha)
	if err != nil {
		return fmt.Errorf("crear detección: %w", err)
	}
	return nil
}

// Ultima devuelve la detección más reciente de un reporte (ErrNoEncontrado si no tiene).
// Solo lectura: la detección original nunca se modifica.
func (r *DeteccionRepository) Ultima(ctx context.Context, idReporte int64) (*models.Deteccion, error) {
	var d models.Deteccion
	var resultado string
	err := r.db.QueryRow(ctx, `
		SELECT id_deteccion, id_reporte, resultado, clase_detectada, confianza::float8
		FROM deteccion
		WHERE id_reporte = $1
		ORDER BY fecha DESC, id_deteccion DESC
		LIMIT 1`, idReporte).
		Scan(&d.ID, &d.IDReporte, &resultado, &d.ClaseDetectada, &d.Confianza)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("leer detección: %w", err)
	}
	d.Resultado = dominio.ResultadoDeteccion(resultado)
	return &d, nil
}
