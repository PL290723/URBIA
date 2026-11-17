package models

import "github.com/PL290723/URBIA/backend-go/internal/dominio"

// Estadisticas es la respuesta de GET /api/v1/estadisticas (HU 07.4).
// Solo agregados: ningún dato personal ni coordenada exacta.
type Estadisticas struct {
	Filtros      FiltrosAplicados        `json:"filtros"`
	Total        int64                   `json:"total"`
	PorEstado    []ConteoEstado          `json:"por_estado"`
	PorCategoria []ConteoCategoria       `json:"por_categoria"`
	PorPeriodo   []ConteoPeriodo         `json:"por_periodo"`
	Geografica   *DistribucionGeografica `json:"distribucion_geografica,omitempty"` // solo con geo=true
}

type FiltrosAplicados struct {
	Desde       string             `json:"desde"` // AAAA-MM-DD, incluido
	Hasta       string             `json:"hasta"` // AAAA-MM-DD, incluido
	Dias        int                `json:"dias"`
	ZonaHoraria string             `json:"zona_horaria"`
	Agrupacion  dominio.Agrupacion `json:"agrupacion"`
}

type ConteoEstado struct {
	Estado dominio.Estado `json:"estado"`
	Total  int64          `json:"total"`
}

// ConteoCategoria: IDCategoria nil = reportes sin categoría.
type ConteoCategoria struct {
	IDCategoria *int16                   `json:"id_categoria"`
	Nombre      string                   `json:"nombre"`
	Activa      bool                     `json:"activa"`
	Total       int64                    `json:"total"`
	PorEstado   map[dominio.Estado]int64 `json:"por_estado"`
}

// ConteoPeriodo: Inicio es el primer día del periodo (día, lunes de la semana o día 1 del mes).
type ConteoPeriodo struct {
	Inicio string `json:"inicio"`
	Total  int64  `json:"total"`
}

type DistribucionGeografica struct {
	CeldaGrados     float64    `json:"celda_grados"`
	MinimoPorCelda  int        `json:"minimo_por_celda"`
	Celdas          []CeldaGeo `json:"celdas"`
	CeldasOcultas   int        `json:"celdas_ocultas"`   // con menos del mínimo
	ReportesOcultos int64      `json:"reportes_ocultos"` // reportes de esas celdas
}

// CeldaGeo: Latitud/Longitud son el CENTRO de la celda, no la ubicación de un reporte.
type CeldaGeo struct {
	Latitud  float64 `json:"latitud"`
	Longitud float64 `json:"longitud"`
	Total    int64   `json:"total"`
}

// FilaCategoriaEstado es una fila cruda del conteo por categoría y estado.
// IDCategoria nil = sin categoría; Estado nil = categoría activa sin reportes.
type FilaCategoriaEstado struct {
	IDCategoria *int16
	Nombre      *string
	Activa      bool
	Estado      *dominio.Estado
	Total       int64
}

// DatosEstadisticas son los conteos crudos que lee el repositorio.
type DatosEstadisticas struct {
	PorEstado    map[dominio.Estado]int64
	PorCategoria []FilaCategoriaEstado
	PorPeriodo   []ConteoPeriodo
	Celdas       []CeldaGeo // sin filtrar por mínimo
}
