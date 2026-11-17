package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
)

// MaxIDsRecurrentes acota cuántos ids de reportes recurrentes se guardan en la explicación.
const MaxIDsRecurrentes = 20

// PrioridadRepository lee los datos del cálculo y guarda cada resultado (HU 07.1).
type PrioridadRepository struct{ db *pgxpool.Pool }

func NewPrioridadRepository(db *pgxpool.Pool) *PrioridadRepository {
	return &PrioridadRepository{db: db}
}

// Reporte devuelve los datos del reporte que usa el cálculo (ErrNoEncontrado si no existe).
func (r *PrioridadRepository) Reporte(ctx context.Context, id int64) (*models.ReportePrioridadFila, error) {
	var f models.ReportePrioridadFila
	var estado string
	var idCat *int16
	var nombreCat *string
	err := r.db.QueryRow(ctx, `
		SELECT r.id_reporte, r.id_usuario, r.estado::text, r.id_categoria, c.nombre,
		       r.prioridad, r.origen_prioridad, r.fecha_creacion
		FROM reporte r
		LEFT JOIN categoria c ON c.id_categoria = r.id_categoria
		WHERE r.id_reporte = $1`, id).
		Scan(&f.ID, &f.IDUsuario, &estado, &idCat, &nombreCat, &f.Prioridad, &f.OrigenPrioridad, &f.FechaCreacion)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("leer reporte para prioridad: %w", err)
	}
	f.Estado = dominio.Estado(estado)
	if idCat != nil && nombreCat != nil {
		f.Categoria = &models.Categoria{ID: *idCat, Nombre: *nombreCat}
	}
	return &f, nil
}

// Recurrencias cuenta los reportes de OTROS ciudadanos con la MISMA categoría
// confirmada, a menos de radioM metros y creados a ±ventanaDias del reporte base.
// Cuenta un reporte por ciudadano (el más cercano en el tiempo), para que una
// sola persona no infle la recurrencia. Usa el índice GIST de ubicacion::geography.
func (r *PrioridadRepository) Recurrencias(ctx context.Context, idBase int64, radioM, ventanaDias int) (int, []int64, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (o.id_usuario) o.id_reporte
		FROM reporte b
		JOIN reporte o
		  ON o.id_reporte <> b.id_reporte
		 AND o.id_usuario <> b.id_usuario
		 AND o.id_categoria = b.id_categoria
		 AND o.origen_categoria IS NOT NULL
		 AND ST_DWithin(o.ubicacion::geography, b.ubicacion::geography, $2::float8)
		 AND abs(extract(epoch FROM o.fecha_creacion - b.fecha_creacion)) <= $3::float8 * 86400
		WHERE b.id_reporte = $1
		  AND b.id_categoria IS NOT NULL
		ORDER BY o.id_usuario, abs(extract(epoch FROM o.fecha_creacion - b.fecha_creacion)), o.id_reporte`,
		idBase, radioM, ventanaDias)
	if err != nil {
		return 0, nil, fmt.Errorf("contar reportes recurrentes: %w", err)
	}
	defer rows.Close()

	total := 0
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, nil, fmt.Errorf("leer reporte recurrente: %w", err)
		}
		total++
		if len(ids) < MaxIDsRecurrentes {
			ids = append(ids, id)
		}
	}
	return total, ids, rows.Err()
}

const columnasCalculo = `
	id_calculo_prioridad, id_reporte, id_usuario, version_formula,
	gravedad_observable, afectacion_estimada, justificacion_afectacion,
	id_categoria, recurrencias, dias_antiguedad::float8,
	cobertura::float8, cobertura_minima::float8, puntuacion::float8, nivel, factores, resumen,
	prioridad_asignada, fecha`

// Ultimo devuelve el cálculo más reciente del reporte (ErrNoEncontrado si no tiene).
func (r *PrioridadRepository) Ultimo(ctx context.Context, idReporte int64) (*models.CalculoPrioridad, error) {
	var c models.CalculoPrioridad
	var grav, afec *int16
	var nivel *string
	var factores []byte
	err := r.db.QueryRow(ctx, `
		SELECT`+columnasCalculo+`
		FROM calculo_prioridad
		WHERE id_reporte = $1
		ORDER BY fecha DESC, id_calculo_prioridad DESC
		LIMIT 1`, idReporte).Scan(
		&c.ID, &c.IDReporte, &c.CalculadoPor, &c.VersionFormula,
		&grav, &afec, &c.Evaluacion.JustificacionAfectacion,
		&c.IDCategoria, &c.Recurrencias, &c.DiasAntiguedad,
		&c.Cobertura, &c.CoberturaMinima, &c.Puntuacion, &nivel, &factores, &c.Resumen,
		&c.PrioridadAsignada, &c.Fecha)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("leer último cálculo de prioridad: %w", err)
	}
	c.Evaluacion.Gravedad = aEntero(grav)
	c.Evaluacion.Afectacion = aEntero(afec)
	if nivel != nil {
		n := dominio.NivelPrioridad(*nivel)
		c.Nivel = &n
	}
	if err := json.Unmarshal(factores, &c.Factores); err != nil {
		return nil, fmt.Errorf("leer factores de prioridad: %w", err)
	}
	// Lo que se deriva de lo guardado (no se almacena dos veces).
	c.Provisional = true
	c.Suficiente = c.Nivel != nil
	c.Faltantes = []dominio.FactorPrioridad{}
	for _, f := range c.Factores {
		if !f.Disponible {
			c.Faltantes = append(c.Faltantes, f.Factor)
		}
	}
	return &c, nil
}

// Guardar registra el cálculo y, si alcanzó para asignar nivel, actualiza la
// prioridad del reporte. Todo en UNA transacción. Bloquea el reporte (FOR UPDATE)
// para que dos cálculos simultáneos no se pisen.
func (r *PrioridadRepository) Guardar(ctx context.Context, c *models.CalculoPrioridad) error {
	factores, err := json.Marshal(c.Factores)
	if err != nil {
		return fmt.Errorf("serializar factores: %w", err)
	}
	var nivel *string
	if c.Nivel != nil {
		s := string(*c.Nivel)
		nivel = &s
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacción: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var bloqueado int64
	err = tx.QueryRow(ctx, `SELECT id_reporte FROM reporte WHERE id_reporte = $1 FOR UPDATE`,
		c.IDReporte).Scan(&bloqueado)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoEncontrado
	}
	if err != nil {
		return fmt.Errorf("bloquear reporte: %w", err)
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO calculo_prioridad (id_reporte, id_usuario, version_formula,
		    gravedad_observable, afectacion_estimada, justificacion_afectacion,
		    id_categoria, recurrencias, dias_antiguedad,
		    cobertura, cobertura_minima, puntuacion, nivel, factores, resumen, prioridad_asignada)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::float8, $10::float8, $11::float8, $12::float8,
		        $13, $14::jsonb, $15, $16)
		RETURNING id_calculo_prioridad, fecha`,
		c.IDReporte, c.CalculadoPor, c.VersionFormula,
		aSmallint(c.Evaluacion.Gravedad), aSmallint(c.Evaluacion.Afectacion), c.Evaluacion.JustificacionAfectacion,
		c.IDCategoria, c.Recurrencias, c.DiasAntiguedad,
		c.Cobertura, c.CoberturaMinima, c.Puntuacion, nivel, string(factores), c.Resumen, c.PrioridadAsignada,
	).Scan(&c.ID, &c.Fecha)
	if err != nil {
		return fmt.Errorf("registrar cálculo de prioridad: %w", err)
	}

	if c.PrioridadAsignada {
		if _, err := tx.Exec(ctx, `
			UPDATE reporte
			SET prioridad = $1, origen_prioridad = $2, fecha_actualizacion = now()
			WHERE id_reporte = $3`,
			*nivel, dominio.OrigenPrioridadCalculada, c.IDReporte); err != nil {
			return fmt.Errorf("asignar prioridad al reporte: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("confirmar cálculo de prioridad: %w", err)
	}
	return nil
}

func aEntero(v *int16) *int {
	if v == nil {
		return nil
	}
	n := int(*v)
	return &n
}

func aSmallint(v *int) *int16 {
	if v == nil {
		return nil
	}
	n := int16(*v)
	return &n
}
