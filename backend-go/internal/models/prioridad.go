package models

import (
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
)

// CalcularPrioridadRequest es el cuerpo (opcional) de POST /api/v1/reportes/:id/prioridad.
// Un campo omitido reutiliza la evaluación anterior del administrador, si existe.
type CalcularPrioridadRequest struct {
	GravedadObservable      *int    `json:"gravedad_observable"`
	AfectacionEstimada      *int    `json:"afectacion_estimada"`
	JustificacionAfectacion *string `json:"justificacion_afectacion"`
}

// ReportePrioridadFila son los datos del reporte que necesita el cálculo.
type ReportePrioridadFila struct {
	ID              int64
	IDUsuario       int64
	Estado          dominio.Estado
	Categoria       *Categoria
	Prioridad       string
	OrigenPrioridad string
	FechaCreacion   time.Time
}

// CalculoPrioridad es una fila de calculo_prioridad y la respuesta del cálculo.
type CalculoPrioridad struct {
	ID        int64 `json:"id_calculo"`
	IDReporte int64 `json:"id_reporte"`
	dominio.ResultadoPrioridad
	PrioridadAsignada bool      `json:"prioridad_asignada"` // true: actualizó reporte.prioridad
	CalculadoPor      int64     `json:"calculado_por"`
	Fecha             time.Time `json:"fecha"`

	// Entradas guardadas: sirven para reutilizar la evaluación y detectar cambios.
	Evaluacion     dominio.EvaluacionAdmin `json:"-"`
	IDCategoria    *int16                  `json:"-"`
	Recurrencias   *int                    `json:"-"`
	DiasAntiguedad float64                 `json:"-"`
}

// PrioridadReporteVista es la prioridad que tiene hoy el reporte.
type PrioridadReporteVista struct {
	IDReporte       int64          `json:"id_reporte"`
	Estado          dominio.Estado `json:"estado"`
	Prioridad       string         `json:"prioridad"`
	OrigenPrioridad string         `json:"origen_prioridad"` // por_defecto | calculada
}

// VigenciaPrioridad indica si el último cálculo sigue reflejando los datos actuales.
type VigenciaPrioridad struct {
	Desactualizado bool     `json:"desactualizado"`
	Motivos        []string `json:"motivos"`
	// Lo que daría la fórmula ahora mismo con las mismas evaluaciones (no se guarda).
	PuntuacionActual float64                 `json:"puntuacion_actual"`
	NivelActual      *dominio.NivelPrioridad `json:"nivel_actual"`
}

// ConsultaPrioridad es la respuesta de GET /api/v1/reportes/:id/prioridad.
type ConsultaPrioridad struct {
	Reporte  PrioridadReporteVista `json:"reporte"`
	Calculo  *CalculoPrioridad     `json:"calculo"` // null si nunca se ha calculado
	Vigencia VigenciaPrioridad     `json:"vigencia"`
}
