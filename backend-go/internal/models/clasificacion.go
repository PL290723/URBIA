package models

import (
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
)

// ConfirmarCategoriaRequest es el cuerpo de PUT /api/v1/reportes/:id/categoria.
type ConfirmarCategoriaRequest struct {
	IDCategoria int16 `json:"id_categoria" binding:"required,gt=0"`
}

// Clasificacion es una confirmación de categoría (fila de la tabla clasificacion)
// y también la respuesta del endpoint (HU 06.3).
type Clasificacion struct {
	ID                int64                   `json:"id_clasificacion"`
	IDReporte         int64                   `json:"id_reporte"`
	Categoria         Categoria               `json:"categoria"`          // la final, confirmada por el ciudadano
	CategoriaAnterior *Categoria              `json:"categoria_anterior"` // null si el reporte no tenía
	CategoriaSugerida *Categoria              `json:"categoria_sugerida"` // null si la IA no sugirió nada
	IDDeteccion       *int64                  `json:"id_deteccion"`       // detección con la que se comparó
	Origen            dominio.OrigenCategoria `json:"origen"`
	Fecha             time.Time               `json:"fecha"`

	IDUsuario int64 `json:"-"` // sale del token
}
