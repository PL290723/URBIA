package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PL290723/URBIA/backend-go/internal/models"
)

var (
	ErrUsuarioNoExiste   = errors.New("el usuario no existe")
	ErrCategoriaNoExiste = errors.New("la categoría no existe")
)

type ReporteRepository struct{ db *pgxpool.Pool }

func NewReporteRepository(db *pgxpool.Pool) *ReporteRepository { return &ReporteRepository{db: db} }

// Crear guarda en UNA transacción el reporte, su foto y la primera entrada del
// historial de estados. Si algo falla no queda nada a medias en la base.
// La foto ya debe estar escrita en disco (foto.Ruta); si esto falla, quien llama la borra.
func (r *ReporteRepository) Crear(ctx context.Context, rep *models.Reporte, foto *models.Archivo) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacción: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no hace nada si ya se hizo Commit

	var idCategoria *int16
	if rep.Categoria != nil {
		idCategoria = &rep.Categoria.ID
	}

	// ST_MakePoint recibe (x, y) = (longitud, latitud). SRID 4326 = WGS 84 (lo que da el GPS).
	const qReporte = `
		INSERT INTO reporte (id_usuario, id_categoria, descripcion, ubicacion, estado,
		                     origen_ubicacion, precision_m)
		VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($4::float8, $5::float8), 4326), $6::varchar, $7, $8::float8)
		RETURNING id_reporte, prioridad, fecha_creacion`
	u := rep.Ubicacion
	err = tx.QueryRow(ctx, qReporte,
		rep.IDUsuario, idCategoria, rep.Descripcion, u.Longitud, u.Latitud,
		string(rep.Estado), string(u.Origen), u.PrecisionM,
	).Scan(&rep.ID, &rep.Prioridad, &rep.FechaCreacion)
	if err != nil {
		return traducirErrorReporte(err)
	}

	foto.IDReporte = rep.ID
	if err := insertarArchivo(ctx, tx, foto); err != nil {
		return err
	}

	// Primera entrada del historial: sin estado anterior, la hace el propio autor.
	const qHistorial = `
		INSERT INTO historial_estado (id_reporte, id_usuario, estado_anterior, estado_nuevo)
		VALUES ($1, $2, NULL, $3::varchar)`
	if _, err := tx.Exec(ctx, qHistorial, rep.ID, rep.IDUsuario, string(rep.Estado)); err != nil {
		return fmt.Errorf("registrar historial inicial: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("confirmar reporte: %w", err)
	}
	return nil
}

func traducirErrorReporte(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
		switch pgErr.ConstraintName {
		case "fk_reporte_usuario":
			return ErrUsuarioNoExiste
		case "fk_reporte_categoria":
			return ErrCategoriaNoExiste
		}
	}
	return fmt.Errorf("crear reporte: %w", err)
}
