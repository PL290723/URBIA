package models

import (
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
)

type Archivo struct {
	ID          int64                   `json:"id_archivo"`
	IDReporte   int64                   `json:"id_reporte"`
	Tipo        string                  `json:"tipo"`
	Variante    dominio.VarianteArchivo `json:"variante"`
	MIME        string                  `json:"mime"`
	Tamano      int64                   `json:"tamano"`
	FechaSubida time.Time               `json:"fecha_subida"`

	Ruta           string `json:"-"` // ruta interna en disco: nunca sale en el JSON
	SHA256         string `json:"-"`
	IDAutorReporte int64  `json:"-"` // para aplicar la política de acceso
}
