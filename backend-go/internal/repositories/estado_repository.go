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

// ErrCambioRechazado: la BD (trigger tg_reporte_transicion_estado) rechazó la transición.
var ErrCambioRechazado = errors.New("la base de datos rechazó la transición de estado")

// EstadoRepository cambia el estado de un reporte y lo registra en historial_estado (HU 07.2).
type EstadoRepository struct{ db *pgxpool.Pool }

func NewEstadoRepository(db *pgxpool.Pool) *EstadoRepository {
	return &EstadoRepository{db: db}
}

// Cambiar hace TODO en una transacción:
//  1. bloquea el reporte (FOR UPDATE) y lee su estado actual,
//  2. llama a validar(actual): si devuelve error, no cambia nada,
//  3. actualiza reporte.estado (el trigger vuelve a validar y pone fecha_actualizacion),
//  4. registra en historial_estado el estado anterior, el nuevo y quién lo hizo.
//
// Con el bloqueo, dos administradores que cambian el mismo reporte a la vez no
// pueden registrar un estado anterior equivocado: el segundo espera y valida
// contra el estado que dejó el primero.
func (r *EstadoRepository) Cambiar(ctx context.Context, idReporte, idUsuario int64, destino dominio.Estado,
	validar func(actual dominio.Estado) error) (*models.CambioEstado, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("iniciar transacción: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var actual string
	err = tx.QueryRow(ctx,
		`SELECT estado::text FROM reporte WHERE id_reporte = $1 FOR UPDATE`, idReporte).Scan(&actual)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("bloquear reporte: %w", err)
	}
	if err := validar(dominio.Estado(actual)); err != nil {
		return nil, err
	}

	c := &models.CambioEstado{
		IDReporte:      idReporte,
		EstadoAnterior: dominio.Estado(actual),
		EstadoNuevo:    destino,
		CambiadoPor:    idUsuario,
	}
	err = tx.QueryRow(ctx, `
		UPDATE reporte SET estado = $1::varchar
		WHERE id_reporte = $2
		RETURNING fecha_actualizacion`, string(destino), idReporte).Scan(&c.FechaActualizacion)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" { // check_violation del trigger
			return nil, ErrCambioRechazado
		}
		return nil, fmt.Errorf("actualizar estado: %w", err)
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO historial_estado (id_reporte, id_usuario, estado_anterior, estado_nuevo)
		VALUES ($1, $2, $3::varchar, $4::varchar)
		RETURNING id_historial_estado, fecha`,
		idReporte, idUsuario, actual, string(destino)).Scan(&c.IDHistorial, &c.Fecha)
	if err != nil {
		return nil, fmt.Errorf("registrar historial de estado: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("confirmar cambio de estado: %w", err)
	}
	return c, nil
}
