package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
)

var ErrReporteNoExiste = errors.New("el reporte no existe")

const columnasArchivo = `a.id_archivo, a.id_reporte, a.tipo, a.variante, a.mime, a.tamano,
	a.fecha_subida, a.ruta, a.sha256, r.id_usuario`

type ArchivoRepository struct{ db *pgxpool.Pool }

func NewArchivoRepository(db *pgxpool.Pool) *ArchivoRepository { return &ArchivoRepository{db: db} }

func (r *ArchivoRepository) Crear(ctx context.Context, a *models.Archivo) error {
	const q = `INSERT INTO archivo (id_reporte, tipo, variante, ruta, mime, tamano, sha256)
	           VALUES ($1, $2, $3, $4, $5, $6, $7)
	           RETURNING id_archivo, fecha_subida`
	err := r.db.QueryRow(ctx, q, a.IDReporte, a.Tipo, string(a.Variante), a.Ruta, a.MIME, a.Tamano, a.SHA256).
		Scan(&a.ID, &a.FechaSubida)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation (fk_archivo_reporte)
		return ErrReporteNoExiste
	}
	if err != nil {
		return fmt.Errorf("crear archivo: %w", err)
	}
	return nil
}

func (r *ArchivoRepository) BuscarPorID(ctx context.Context, id int64) (*models.Archivo, error) {
	q := `SELECT ` + columnasArchivo + `
	      FROM archivo a
	      JOIN reporte r ON r.id_reporte = a.id_reporte
	      WHERE a.id_archivo = $1`
	var a models.Archivo
	var variante string
	err := r.db.QueryRow(ctx, q, id).Scan(&a.ID, &a.IDReporte, &a.Tipo, &variante, &a.MIME,
		&a.Tamano, &a.FechaSubida, &a.Ruta, &a.SHA256, &a.IDAutorReporte)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("leer archivo: %w", err)
	}
	if a.Variante, err = dominio.ParseVariante(variante); err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *ArchivoRepository) Eliminar(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM archivo WHERE id_archivo = $1`, id)
	if err != nil {
		return fmt.Errorf("eliminar archivo: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoEncontrado
	}
	return nil
}

// AutorDeReporte devuelve el id_usuario que creó el reporte.
// Se moverá a ReporteRepository cuando exista el módulo de reportes.
func (r *ArchivoRepository) AutorDeReporte(ctx context.Context, idReporte int64) (int64, error) {
	var idUsuario int64
	err := r.db.QueryRow(ctx, `SELECT id_usuario FROM reporte WHERE id_reporte = $1`, idReporte).Scan(&idUsuario)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNoEncontrado
	}
	if err != nil {
		return 0, fmt.Errorf("leer autor del reporte: %w", err)
	}
	return idUsuario, nil
}
