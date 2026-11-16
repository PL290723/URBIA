package models

import (
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
)

type Categoria struct {
	ID     int16  `json:"id_categoria"`
	Nombre string `json:"nombre"`
}

// Reporte es también el resumen que se devuelve al crearlo (HU 06.1).
type Reporte struct {
	ID            int64             `json:"id_reporte"`
	Estado        dominio.Estado    `json:"estado"`
	Prioridad     string            `json:"prioridad"`
	Descripcion   string            `json:"descripcion"`
	Categoria     *Categoria        `json:"categoria"` // null si el ciudadano no la eligió
	Ubicacion     dominio.Ubicacion `json:"ubicacion"`
	Foto          *Archivo          `json:"foto"`
	FechaCreacion time.Time         `json:"fecha_creacion"`

	IDUsuario int64 `json:"-"` // autor; sale del token, nunca del cliente
}
