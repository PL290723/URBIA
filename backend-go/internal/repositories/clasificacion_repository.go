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

// ErrReporteNoEditable: el reporte ya no está pendiente (alguien lo empezó a revisar).
var ErrReporteNoEditable = errors.New("el reporte ya no permite cambiar la categoría")

type ClasificacionRepository struct{ db *pgxpool.Pool }

func NewClasificacionRepository(db *pgxpool.Pool) *ClasificacionRepository {
	return &ClasificacionRepository{db: db}
}

// EstadoReporte devuelve el autor y el estado de un reporte (ErrNoEncontrado si no existe).
func (r *ClasificacionRepository) EstadoReporte(ctx context.Context, idReporte int64) (autor int64, estado dominio.Estado, err error) {
	var e string
	err = r.db.QueryRow(ctx,
		`SELECT id_usuario, estado FROM reporte WHERE id_reporte = $1`, idReporte).Scan(&autor, &e)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", ErrNoEncontrado
	}
	if err != nil {
		return 0, "", fmt.Errorf("leer reporte: %w", err)
	}
	return autor, dominio.Estado(e), nil
}

// Confirmar guarda la categoría final del reporte y registra la confirmación,
// todo en UNA transacción. La fila de deteccion no se toca.
// Bloquea el reporte (FOR UPDATE) y vuelve a comprobar que siga pendiente y que
// sea del mismo autor: si un administrador lo movió de estado entre la lectura y
// este punto, devuelve ErrReporteNoEditable.
func (r *ClasificacionRepository) Confirmar(ctx context.Context, c *models.Clasificacion) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacción: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var anterior *int16
	var nombreAnterior *string
	err = tx.QueryRow(ctx, `
		SELECT r.id_categoria, c.nombre
		FROM reporte r
		LEFT JOIN categoria c ON c.id_categoria = r.id_categoria
		WHERE r.id_reporte = $1 AND r.id_usuario = $2 AND r.estado = $3
		FOR UPDATE OF r`,
		c.IDReporte, c.IDUsuario, string(dominio.EstadoPendiente)).Scan(&anterior, &nombreAnterior)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrReporteNoEditable
	}
	if err != nil {
		return fmt.Errorf("bloquear reporte: %w", err)
	}
	if anterior != nil && nombreAnterior != nil {
		c.CategoriaAnterior = &models.Categoria{ID: *anterior, Nombre: *nombreAnterior}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE reporte
		SET id_categoria = $1, origen_categoria = $2, fecha_actualizacion = now()
		WHERE id_reporte = $3`,
		c.Categoria.ID, string(c.Origen), c.IDReporte); err != nil {
		return fmt.Errorf("actualizar categoría del reporte: %w", err)
	}

	var sugerida *int16
	if c.CategoriaSugerida != nil {
		sugerida = &c.CategoriaSugerida.ID
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO clasificacion (id_reporte, id_usuario, id_categoria, id_categoria_anterior,
		                           id_categoria_sugerida, id_deteccion, origen)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id_clasificacion, fecha`,
		c.IDReporte, c.IDUsuario, c.Categoria.ID, anterior, sugerida, c.IDDeteccion, string(c.Origen),
	).Scan(&c.ID, &c.Fecha)
	if err != nil {
		return fmt.Errorf("registrar clasificación: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("confirmar clasificación: %w", err)
	}
	return nil
}
