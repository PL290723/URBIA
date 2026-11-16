package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PL290723/URBIA/backend-go/internal/models"
)

type CategoriaRepository struct{ db *pgxpool.Pool }

func NewCategoriaRepository(db *pgxpool.Pool) *CategoriaRepository {
	return &CategoriaRepository{db: db}
}

// BuscarActiva devuelve la categoría solo si existe y está activa; si no, ErrNoEncontrado.
func (r *CategoriaRepository) BuscarActiva(ctx context.Context, id int16) (*models.Categoria, error) {
	var c models.Categoria
	err := r.db.QueryRow(ctx,
		`SELECT id_categoria, nombre FROM categoria WHERE id_categoria = $1 AND activo`, id).
		Scan(&c.ID, &c.Nombre)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("leer categoría: %w", err)
	}
	return &c, nil
}
