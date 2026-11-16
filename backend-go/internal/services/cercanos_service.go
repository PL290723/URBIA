package services

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"sort"

	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

// CercanosService sugiere al administrador reportes posiblemente relacionados
// (HU 06.5). Es de solo lectura: no elimina, fusiona ni modifica nada.
type CercanosService struct {
	repo    *repositories.CercanosRepository
	defecto dominio.ParametrosCercania // DUPLICADOS_RADIO_M y DUPLICADOS_VENTANA_DIAS
	log     *slog.Logger
}

func NewCercanosService(repo *repositories.CercanosRepository, defecto dominio.ParametrosCercania,
	log *slog.Logger) *CercanosService {
	return &CercanosService{repo: repo, defecto: defecto, log: log}
}

// Defecto son los parámetros configurados (el handler los usa como base).
func (s *CercanosService) Defecto() dominio.ParametrosCercania { return s.defecto }

// Buscar devuelve los reportes cercanos al reporte id, ordenados de la
// coincidencia más fuerte a la más débil.
func (s *CercanosService) Buscar(ctx context.Context, sesion auth.Sesion, id int64,
	p dominio.ParametrosCercania) (*models.ReportesCercanos, error) {
	if !sesion.Rol.PuedeRevisarDuplicados() { // la ruta ya lo filtra; defensa adicional
		return nil, response.Forbidden("Solo un administrador puede revisar posibles duplicados")
	}

	base, err := s.repo.Base(ctx, id)
	if errors.Is(err, repositories.ErrNoEncontrado) {
		return nil, response.NotFound(msgReporteNoEncontrado)
	}
	if err != nil {
		return nil, err
	}

	candidatos, err := s.repo.Candidatos(ctx, id, p.RadioM)
	if err != nil {
		return nil, err
	}

	resultado := armarCercanos(base, candidatos, p)
	s.log.Info("revisión de duplicados", "id_reporte", id, "id_usuario", sesion.UsuarioID,
		"radio_m", p.RadioM, "ventana_dias", p.VentanaDias, "candidatos", resultado.Total)
	return resultado, nil
}

// armarCercanos puntúa y ordena los candidatos. Función pura: se prueba sin BD.
func armarCercanos(base *models.ReporteCercanoFila, candidatos []models.ReporteCercanoFila,
	p dominio.ParametrosCercania) *models.ReportesCercanos {
	var catBase *int16
	if base.Categoria != nil {
		catBase = &base.Categoria.ID
	}

	coincidencias := make([]models.Coincidencia, 0, len(candidatos))
	for _, c := range candidatos {
		var catCand *int16
		if c.Categoria != nil {
			catCand = &c.Categoria.ID
		}
		dias := c.FechaCreacion.Sub(base.FechaCreacion).Hours() / 24
		senales := dominio.SenalesCercania{
			DistanciaM:     c.DistanciaM,
			MismaCategoria: dominio.MismaCategoria(catBase, catCand),
			DiasDiferencia: dias,
		}
		ev := dominio.EvaluarCoincidencia(senales, p)

		coincidencias = append(coincidencias, models.Coincidencia{
			ID:             c.ID,
			Estado:         c.Estado,
			Categoria:      c.Categoria,
			Descripcion:    c.Descripcion,
			Ubicacion:      models.UbicacionVista{Latitud: c.Latitud, Longitud: c.Longitud},
			FechaCreacion:  c.FechaCreacion,
			DistanciaM:     math.Round(c.DistanciaM*10) / 10,
			DiasDiferencia: math.Round(dias*10) / 10,
			MismaCategoria: senales.MismaCategoria,
			DentroVentana:  ev.DentroVentana,
			MismoAutor:     c.IDUsuario == base.IDUsuario,
			Puntuacion:     ev.Puntuacion,
			Nivel:          ev.Nivel,
		})
	}

	// Más probable primero; a igual puntuación, el más cercano.
	sort.SliceStable(coincidencias, func(i, j int) bool {
		if coincidencias[i].Puntuacion != coincidencias[j].Puntuacion {
			return coincidencias[i].Puntuacion > coincidencias[j].Puntuacion
		}
		return coincidencias[i].DistanciaM < coincidencias[j].DistanciaM
	})

	total := len(coincidencias)
	if len(coincidencias) > p.Limite {
		coincidencias = coincidencias[:p.Limite]
	}

	return &models.ReportesCercanos{
		Reporte: models.ReporteBaseVista{
			ID:            base.ID,
			Estado:        base.Estado,
			Categoria:     base.Categoria,
			Ubicacion:     models.UbicacionVista{Latitud: base.Latitud, Longitud: base.Longitud},
			FechaCreacion: base.FechaCreacion,
		},
		Parametros:    p,
		Coincidencias: coincidencias,
		Total:         total,
	}
}
