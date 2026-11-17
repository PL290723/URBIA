package services

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

const nombreSinCategoria = "sin_categoria"

// estadosEnOrden: el panel siempre recibe los 4 estados, aunque alguno valga 0.
var estadosEnOrden = []dominio.Estado{
	dominio.EstadoPendiente, dominio.EstadoEnRevision, dominio.EstadoEnProceso, dominio.EstadoResuelto,
}

// EstadisticasService entrega los agregados del panel administrativo (HU 07.4).
type EstadisticasService struct {
	repo  *repositories.EstadisticasRepository
	log   *slog.Logger
	ahora func() time.Time
}

func NewEstadisticasService(repo *repositories.EstadisticasRepository, log *slog.Logger) *EstadisticasService {
	return &EstadisticasService{repo: repo, log: log, ahora: time.Now}
}

// Ahora expone el reloj para que el handler valide los filtros con la misma hora.
func (s *EstadisticasService) Ahora() time.Time { return s.ahora() }

func (s *EstadisticasService) Calcular(ctx context.Context, sesion auth.Sesion,
	f dominio.FiltrosEstadisticas) (*models.Estadisticas, error) {
	if !sesion.Rol.PuedeVerEstadisticas() { // la ruta ya lo filtra; defensa adicional
		return nil, response.Forbidden("Solo un administrador puede ver las estadísticas")
	}
	datos, err := s.repo.Calcular(ctx, f)
	if err != nil {
		return nil, err
	}
	est := armarEstadisticas(f, datos)
	s.log.Info("estadísticas calculadas", "id_usuario", sesion.UsuarioID, "desde", est.Filtros.Desde,
		"hasta", est.Filtros.Hasta, "agrupacion", est.Filtros.Agrupacion, "total", est.Total, "geo", f.Geo)
	return est, nil
}

// armarEstadisticas convierte los conteos crudos en la respuesta. Función pura.
func armarEstadisticas(f dominio.FiltrosEstadisticas, d *models.DatosEstadisticas) *models.Estadisticas {
	e := &models.Estadisticas{
		Filtros: models.FiltrosAplicados{
			Desde: f.DesdeFecha(), Hasta: f.HastaFecha(), Dias: f.Dias,
			ZonaHoraria: dominio.ZonaHorariaEstadisticas, Agrupacion: f.Agrupacion,
		},
		PorEstado:    make([]models.ConteoEstado, 0, len(estadosEnOrden)),
		PorCategoria: armarCategorias(d.PorCategoria),
		PorPeriodo:   d.PorPeriodo,
	}
	for _, estado := range estadosEnOrden {
		n := d.PorEstado[estado]
		e.PorEstado = append(e.PorEstado, models.ConteoEstado{Estado: estado, Total: n})
		e.Total += n
	}
	if e.PorPeriodo == nil {
		e.PorPeriodo = []models.ConteoPeriodo{}
	}
	if f.Geo {
		e.Geografica = filtrarCeldas(d.Celdas, f.CeldaGrados)
	}
	return e
}

// armarCategorias junta las filas (categoría, estado) en una entrada por categoría,
// con los 4 estados siempre presentes. Orden: más reportes primero; "sin_categoria" al final.
func armarCategorias(filas []models.FilaCategoriaEstado) []models.ConteoCategoria {
	porClave := map[int16]*models.ConteoCategoria{}
	var sinCategoria *models.ConteoCategoria
	const claveSinCategoria = int16(-1)

	for _, fila := range filas {
		clave := claveSinCategoria
		if fila.IDCategoria != nil {
			clave = *fila.IDCategoria
		}
		c, ok := porClave[clave]
		if !ok {
			c = &models.ConteoCategoria{PorEstado: map[dominio.Estado]int64{}, Activa: fila.Activa}
			for _, estado := range estadosEnOrden {
				c.PorEstado[estado] = 0
			}
			if fila.IDCategoria != nil {
				id := *fila.IDCategoria
				c.IDCategoria = &id
				if fila.Nombre != nil {
					c.Nombre = *fila.Nombre
				}
			} else {
				c.Nombre = nombreSinCategoria
				sinCategoria = c
			}
			porClave[clave] = c
		}
		if fila.Estado != nil {
			c.PorEstado[*fila.Estado] += fila.Total
			c.Total += fila.Total
		}
	}

	out := make([]models.ConteoCategoria, 0, len(porClave)+1)
	for clave, c := range porClave {
		if clave != claveSinCategoria {
			out = append(out, *c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return *out[i].IDCategoria < *out[j].IDCategoria
	})
	if sinCategoria == nil { // siempre presente, aunque sea 0
		sinCategoria = &models.ConteoCategoria{Nombre: nombreSinCategoria, Activa: true,
			PorEstado: map[dominio.Estado]int64{}}
		for _, estado := range estadosEnOrden {
			sinCategoria.PorEstado[estado] = 0
		}
	}
	return append(out, *sinCategoria)
}

// filtrarCeldas oculta las celdas con menos de MinReportesPorCelda reportes para que
// el mapa de calor no permita ubicar a un ciudadano. Función pura.
func filtrarCeldas(celdas []models.CeldaGeo, celdaGrados float64) *models.DistribucionGeografica {
	g := &models.DistribucionGeografica{
		CeldaGrados:    celdaGrados,
		MinimoPorCelda: dominio.MinReportesPorCelda,
		Celdas:         []models.CeldaGeo{},
	}
	for _, c := range celdas {
		if c.Total < dominio.MinReportesPorCelda {
			g.CeldasOcultas++
			g.ReportesOcultos += c.Total
			continue
		}
		g.Celdas = append(g.Celdas, c)
	}
	return g
}
