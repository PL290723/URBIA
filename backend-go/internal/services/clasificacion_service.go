package services

import (
	"context"
	"errors"
	"log/slog"

	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

const msgNoEditable = "El reporte ya está en revisión; su categoría ya no se puede cambiar"

type ClasificacionService struct {
	clasificaciones *repositories.ClasificacionRepository
	detecciones     *repositories.DeteccionRepository
	categorias      *repositories.CategoriaRepository
	log             *slog.Logger
}

func NewClasificacionService(clasificaciones *repositories.ClasificacionRepository,
	detecciones *repositories.DeteccionRepository, categorias *repositories.CategoriaRepository,
	log *slog.Logger) *ClasificacionService {
	return &ClasificacionService{clasificaciones: clasificaciones, detecciones: detecciones,
		categorias: categorias, log: log}
}

// Confirmar guarda la categoría final que decide el ciudadano (HU 06.3) y
// registra si fue la sugerida por la IA, una corrección o una elección manual.
// La detección original de la IA se conserva intacta.
func (s *ClasificacionService) Confirmar(ctx context.Context, sesion auth.Sesion, idReporte int64,
	idCategoria int16) (*models.Clasificacion, error) {
	if !sesion.Rol.PuedeConfirmarCategoria() { // la ruta ya lo filtra; defensa adicional
		return nil, response.Forbidden("Solo el ciudadano que creó el reporte puede confirmar su categoría")
	}

	autor, estado, err := s.clasificaciones.EstadoReporte(ctx, idReporte)
	if errors.Is(err, repositories.ErrNoEncontrado) || (err == nil && autor != sesion.UsuarioID) {
		return nil, response.NotFound(msgReporteNoEncontrado) // no revela reportes ajenos
	}
	if err != nil {
		return nil, err
	}
	if !estado.PermiteCambiarCategoria() {
		return nil, response.Conflict(msgNoEditable)
	}

	categoria, err := s.categorias.BuscarActiva(ctx, idCategoria)
	if errors.Is(err, repositories.ErrNoEncontrado) {
		return nil, errCategoriaInvalida
	}
	if err != nil {
		return nil, err
	}

	c := &models.Clasificacion{
		IDReporte: idReporte,
		IDUsuario: sesion.UsuarioID,
		Categoria: *categoria,
	}
	c.CategoriaSugerida, c.IDDeteccion, err = s.sugerenciaVigente(ctx, idReporte)
	if err != nil {
		return nil, err
	}
	var sugerida *int16
	if c.CategoriaSugerida != nil {
		sugerida = &c.CategoriaSugerida.ID
	}
	c.Origen = dominio.OrigenDeClasificacion(categoria.ID, sugerida)

	if err := s.clasificaciones.Confirmar(ctx, c); err != nil {
		if errors.Is(err, repositories.ErrReporteNoEditable) {
			return nil, response.Conflict(msgNoEditable) // cambió de estado mientras tanto
		}
		return nil, err
	}

	s.log.Info("categoría confirmada", "id_reporte", idReporte, "id_usuario", sesion.UsuarioID,
		"id_categoria", categoria.ID, "origen", c.Origen, "id_deteccion", c.IDDeteccion)
	return c, nil
}

// sugerenciaVigente devuelve la categoría que sugirió la IA en su última detección
// y el id de esa detección. Si la IA falló, no detectó nada o la clase ya no
// corresponde a una categoría activa, no hay sugerencia (la elección será "manual"),
// pero se devuelve igual el id de la detección consultada para la auditoría.
func (s *ClasificacionService) sugerenciaVigente(ctx context.Context, idReporte int64) (*models.Categoria, *int64, error) {
	det, err := s.detecciones.Ultima(ctx, idReporte)
	if errors.Is(err, repositories.ErrNoEncontrado) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	idDeteccion := &det.ID
	if det.Resultado != dominio.ResultadoDetectado || det.ClaseDetectada == nil {
		return nil, idDeteccion, nil
	}

	cat, err := s.categorias.BuscarActivaPorNombre(ctx, *det.ClaseDetectada)
	if errors.Is(err, repositories.ErrNoEncontrado) {
		return nil, idDeteccion, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return cat, idDeteccion, nil
}
