package models

import (
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
)

// FilaReporte es TODO lo que lee el repositorio. Nunca sale tal cual en el JSON:
// el service decide qué parte ve cada quién (dominio.Vista).
type FilaReporte struct {
	ID                 int64
	IDUsuario          int64
	NombreAutor        string
	CorreoAutor        string
	Estado             dominio.Estado
	Prioridad          string
	Descripcion        string
	Categoria          *Categoria
	OrigenCategoria    *string
	Latitud, Longitud  float64
	OrigenUbicacion    string
	PrecisionM         *float64
	FechaCreacion      time.Time
	FechaActualizacion time.Time
	Foto               *Archivo          // foto original más reciente
	Deteccion          *Deteccion        // última detección de la IA (solo en el detalle)
	CategoriaSugerida  *Categoria        // categoría activa que corresponde a la clase detectada
	Historial          []HistorialEstado // solo en el detalle
}

// ---------------------------------------------------------------------
// Lo que sí sale en el JSON (HU 06.4)
// ---------------------------------------------------------------------

type UbicacionVista struct {
	Latitud    float64 `json:"latitud"`
	Longitud   float64 `json:"longitud"`
	Aproximada bool    `json:"aproximada"` // true: redondeada a ~110 m (reporte ajeno)
}

// FotoVista son los metadatos de la foto; la imagen se descarga en Url
// con el mismo control de acceso (autor o administrador).
type FotoVista struct {
	IDArchivo   int64     `json:"id_archivo"`
	MIME        string    `json:"mime"`
	Tamano      int64     `json:"tamano"`
	FechaSubida time.Time `json:"fecha_subida"`
	URL         string    `json:"url"`
}

type SugerenciaVista struct {
	Estado    dominio.EstadoSugerencia `json:"estado"`
	Categoria *Categoria               `json:"categoria"`
	Confianza *float64                 `json:"confianza"`
}

type HistorialEstado struct {
	ID             int64              `json:"id_historial_estado"`
	Tipo           dominio.TipoEvento `json:"tipo"` // creacion | cambio_estado
	EstadoAnterior *dominio.Estado    `json:"estado_anterior"`
	EstadoNuevo    dominio.Estado     `json:"estado_nuevo"`
	Fecha          time.Time          `json:"fecha"`
	Actor          *ActorHistorial    `json:"actor,omitempty"` // recortado según la vista (HU 07.3)
}

// ActorHistorial es quién hizo el cambio. El administrador ve id y nombre;
// el autor del reporte solo ve el rol.
type ActorHistorial struct {
	IDUsuario *int64      `json:"id_usuario,omitempty"`
	Nombre    *string     `json:"nombre,omitempty"`
	Rol       dominio.Rol `json:"rol"`
}

// HistorialReporte es la respuesta de GET /api/v1/reportes/:id/historial (HU 07.3).
type HistorialReporte struct {
	IDReporte    int64             `json:"id_reporte"`
	EstadoActual dominio.Estado    `json:"estado_actual"`
	Total        int               `json:"total"`
	SoloLectura  bool              `json:"solo_lectura"` // el historial no se modifica desde la API
	Eventos      []HistorialEstado `json:"eventos"`      // orden cronológico (fecha, id)
}

type AutorReporte struct {
	IDUsuario int64  `json:"id_usuario"`
	Nombre    string `json:"nombre"`
	Correo    string `json:"correo"`
}

type DeteccionVista struct {
	IDDeteccion    int64                      `json:"id_deteccion"`
	Resultado      dominio.ResultadoDeteccion `json:"resultado"`
	ClaseDetectada *string                    `json:"clase_detectada"`
	Confianza      *float64                   `json:"confianza"`
	Modelo         *string                    `json:"modelo"`
	VersionModelo  *string                    `json:"version_modelo"`
	CodigoError    *string                    `json:"codigo_error"`
	Fecha          time.Time                  `json:"fecha"`
}

// DatosAdministrativos solo aparece para administradores.
type DatosAdministrativos struct {
	Autor           AutorReporte    `json:"autor"`
	Prioridad       string          `json:"prioridad"`
	OrigenUbicacion string          `json:"origen_ubicacion"`
	PrecisionM      *float64        `json:"precision_m"`
	OrigenCategoria *string         `json:"origen_categoria"`
	Deteccion       *DeteccionVista `json:"deteccion,omitempty"` // solo en el detalle
}

// ReporteResumen es cada elemento del listado (mapa y lista).
type ReporteResumen struct {
	ID            int64                 `json:"id_reporte"`
	Estado        dominio.Estado        `json:"estado"`
	Categoria     *Categoria            `json:"categoria"`
	Descripcion   string                `json:"descripcion"`
	Ubicacion     UbicacionVista        `json:"ubicacion"`
	FechaCreacion time.Time             `json:"fecha_creacion"`
	EsPropio      bool                  `json:"es_propio"`
	IDFoto        *int64                `json:"id_foto,omitempty"`        // autor o administrador
	Admin         *DatosAdministrativos `json:"administrativo,omitempty"` // administrador
}

// ReporteDetalle es GET /api/v1/reportes/{id}.
type ReporteDetalle struct {
	ID                 int64                 `json:"id_reporte"`
	Estado             dominio.Estado        `json:"estado"`
	Categoria          *Categoria            `json:"categoria"`
	Descripcion        string                `json:"descripcion"`
	Ubicacion          UbicacionVista        `json:"ubicacion"`
	FechaCreacion      time.Time             `json:"fecha_creacion"`
	FechaActualizacion time.Time             `json:"fecha_actualizacion"`
	EsPropio           bool                  `json:"es_propio"`
	Foto               *FotoVista            `json:"foto,omitempty"`       // autor o administrador
	Sugerencia         *SugerenciaVista      `json:"sugerencia,omitempty"` // autor o administrador
	Historial          []HistorialEstado     `json:"historial,omitempty"`  // autor o administrador
	Admin              *DatosAdministrativos `json:"administrativo,omitempty"`
}

type Paginacion struct {
	Pagina       int   `json:"pagina"`
	PorPagina    int   `json:"por_pagina"`
	Total        int64 `json:"total"`
	TotalPaginas int   `json:"total_paginas"`
}

type ListadoReportes struct {
	Elementos  []ReporteResumen `json:"elementos"`
	Paginacion Paginacion       `json:"paginacion"`
}
