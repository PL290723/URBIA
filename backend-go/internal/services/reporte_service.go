package services

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

var errCategoriaInvalida = response.Validation([]dominio.ErrorCampo{{
	Campo: "id_categoria", Regla: "no_existe", Mensaje: "La categoría no existe o no está activa",
}})

type ReporteService struct {
	reportes    *repositories.ReporteRepository
	categorias  *repositories.CategoriaRepository
	archivos    *ArchivoService
	sugerencias *SugerenciaService
	log         *slog.Logger
}

func NewReporteService(reportes *repositories.ReporteRepository, categorias *repositories.CategoriaRepository,
	archivos *ArchivoService, sugerencias *SugerenciaService, log *slog.Logger) *ReporteService {
	return &ReporteService{reportes: reportes, categorias: categorias, archivos: archivos,
		sugerencias: sugerencias, log: log}
}

// Crear registra un reporte ciudadano con su fotografía y ubicación (HU 06.1) y
// después pide a la IA una sugerencia de categoría (HU 06.2).
// Orden: lo barato primero (permiso y categoría), luego la imagen, la transacción
// y AL FINAL la inferencia: si la IA falla, el reporte ya quedó guardado.
func (s *ReporteService) Crear(ctx context.Context, sesion auth.Sesion, n dominio.NuevoReporte,
	foto io.Reader, idSolicitud string) (*models.Reporte, error) {
	if !sesion.Rol.PuedeCrearReporte() { // la ruta ya lo filtra; esto es defensa adicional
		return nil, response.Forbidden("Solo los ciudadanos pueden registrar reportes")
	}

	var categoria *models.Categoria
	if n.IDCategoria != nil {
		c, err := s.categorias.BuscarActiva(ctx, *n.IDCategoria)
		if errors.Is(err, repositories.ErrNoEncontrado) {
			return nil, errCategoriaInvalida
		}
		if err != nil {
			return nil, err
		}
		categoria = c
	}

	archivo, err := s.archivos.PrepararOriginal(ctx, sesion.UsuarioID, foto)
	if err != nil {
		return nil, err // ya viene como 400/413/415/422 según el problema de la imagen
	}

	rep := &models.Reporte{
		IDUsuario:   sesion.UsuarioID, // el autor sale del token, nunca del formulario
		Estado:      dominio.EstadoInicialReporte,
		Descripcion: n.Descripcion,
		Categoria:   categoria,
		Ubicacion:   n.Ubicacion,
	}
	if categoria != nil {
		// La eligió al crear, antes de ver la sugerencia de la IA (HU 06.3).
		manual := dominio.CategoriaManual
		rep.OrigenCategoria = &manual
	}
	if err := s.reportes.Crear(ctx, rep, archivo); err != nil {
		s.archivos.Descartar(archivo.Ruta) // compensación: sin reporte no debe quedar la foto
		switch {
		case errors.Is(err, repositories.ErrCategoriaNoExiste):
			return nil, errCategoriaInvalida
		case errors.Is(err, repositories.ErrUsuarioNoExiste):
			return nil, response.Unauthorized("La cuenta ya no existe; inicia sesión de nuevo")
		}
		return nil, err
	}
	rep.Foto = archivo

	s.log.Info("reporte creado",
		"id_reporte", rep.ID, "id_usuario", rep.IDUsuario, "id_archivo", archivo.ID,
		"origen_ubicacion", rep.Ubicacion.Origen, "con_categoria", categoria != nil)
	// Después del commit: la IA solo agrega información, nunca puede deshacer el reporte.
	rep.Sugerencia = s.sugerencias.Sugerir(ctx, rep, idSolicitud)
	return rep, nil
}
