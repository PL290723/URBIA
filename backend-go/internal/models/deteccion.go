package models

import (
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
)

// Deteccion es una fila de la tabla deteccion: el resultado ORIGINAL de la IA,
// guardado tal como llegó aunque después cambie la categoría del reporte.
type Deteccion struct {
	ID              int64
	IDReporte       int64
	Resultado       dominio.ResultadoDeteccion
	ClaseDetectada  *string  // NULL si no hubo detección
	Confianza       *float64 // NULL si no hubo detección
	Modelo          *string  // NULL solo con resultado 'error'
	VersionModelo   *string
	UmbralConfianza *float64
	CodigoError     *string // solo con resultado 'error'
	Milisegundos    *float64
	Fecha           time.Time
}

// Sugerencia es lo que recibe el cliente. Solo informa: no cambia el reporte.
type Sugerencia struct {
	Estado               dominio.EstadoSugerencia `json:"estado"`
	Categoria            *Categoria               `json:"categoria"`
	Confianza            *float64                 `json:"confianza"`
	CoincideConCategoria *bool                    `json:"coincide_con_categoria"` // null si no hay con qué comparar
	Modelo               *string                  `json:"modelo"`
	VersionModelo        *string                  `json:"version_modelo"`
	IDDeteccion          *int64                   `json:"id_deteccion"` // null si no se pudo guardar
}
