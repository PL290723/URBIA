package models

import (
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
)

// ReporteCercanoFila es lo que lee el repositorio para cada reporte (base o candidato).
type ReporteCercanoFila struct {
	ID            int64
	IDUsuario     int64
	Estado        dominio.Estado
	Descripcion   string
	Categoria     *Categoria
	Latitud       float64
	Longitud      float64
	FechaCreacion time.Time
	DistanciaM    float64 // solo candidatos: metros al reporte base (geography)
}

// ReporteBaseVista es el reporte que se está revisando.
type ReporteBaseVista struct {
	ID            int64          `json:"id_reporte"`
	Estado        dominio.Estado `json:"estado"`
	Categoria     *Categoria     `json:"categoria"`
	Ubicacion     UbicacionVista `json:"ubicacion"`
	FechaCreacion time.Time      `json:"fecha_creacion"`
}

// Coincidencia es un posible reporte relacionado: una sugerencia interna para el
// administrador. Nada se fusiona ni se modifica.
type Coincidencia struct {
	ID             int64                     `json:"id_reporte"`
	Estado         dominio.Estado            `json:"estado"`
	Categoria      *Categoria                `json:"categoria"`
	Descripcion    string                    `json:"descripcion"`
	Ubicacion      UbicacionVista            `json:"ubicacion"`
	FechaCreacion  time.Time                 `json:"fecha_creacion"`
	DistanciaM     float64                   `json:"distancia_m"`
	DiasDiferencia float64                   `json:"dias_diferencia"` // negativo: es anterior al base
	MismaCategoria *bool                     `json:"misma_categoria"` // null si alguno no tiene categoría
	DentroVentana  bool                      `json:"dentro_ventana"`
	MismoAutor     bool                      `json:"mismo_autor"`
	Puntuacion     float64                   `json:"puntuacion"`
	Nivel          dominio.NivelCoincidencia `json:"nivel"`
}

// ReportesCercanos es la respuesta de GET /api/v1/reportes/{id}/cercanos.
type ReportesCercanos struct {
	Reporte       ReporteBaseVista           `json:"reporte"`
	Parametros    dominio.ParametrosCercania `json:"parametros"`
	Coincidencias []Coincidencia             `json:"coincidencias"`
	Total         int                        `json:"total"` // candidatos dentro del radio antes de aplicar el límite
}
