package services

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

// EstadoService permite al administrador avanzar el ciclo de vida de un reporte (HU 07.2).
// RepositorioEstados es lo que EstadoService necesita de la base de datos.
// *repositories.EstadoRepository lo cumple; las pruebas unitarias (HU 09.2) usan un doble.
type RepositorioEstados interface {
	Cambiar(ctx context.Context, idReporte, idUsuario int64, destino dominio.Estado,
		validar func(actual dominio.Estado) error) (*models.CambioEstado, error)
}

// EstadoService permite al administrador avanzar el ciclo de vida de un reporte (HU 07.2).
type EstadoService struct {
	repo RepositorioEstados
	log  *slog.Logger
}

func NewEstadoService(repo RepositorioEstados, log *slog.Logger) *EstadoService {
	return &EstadoService{repo: repo, log: log}
}

// Cambiar valida el estado pedido, lo aplica y registra quién lo cambió.
func (s *EstadoService) Cambiar(ctx context.Context, sesion auth.Sesion, idReporte int64,
	estado string) (*models.CambioEstado, error) {
	if !sesion.Rol.PuedeCambiarEstado() { // la ruta ya lo filtra; defensa adicional
		return nil, response.Forbidden("Solo un administrador puede cambiar el estado de un reporte")
	}

	destino, errs := dominio.ParseEstadoDestino(estado)
	if len(errs) > 0 {
		return nil, response.Validation(errs)
	}

	cambio, err := s.repo.Cambiar(ctx, idReporte, sesion.UsuarioID, destino,
		func(actual dominio.Estado) error { return validarCambio(actual, destino) })
	switch {
	case errors.Is(err, repositories.ErrNoEncontrado):
		return nil, response.NotFound(msgReporteNoEncontrado)
	case errors.Is(err, repositories.ErrCambioRechazado):
		return nil, response.Conflict("La transición de estado no está permitida")
	case err != nil:
		return nil, err
	}

	cambio.EstadosSiguientes = destino.Siguientes()
	s.log.Info("estado de reporte cambiado", "id_reporte", idReporte, "id_usuario", sesion.UsuarioID,
		"estado_anterior", cambio.EstadoAnterior, "estado_nuevo", cambio.EstadoNuevo)
	return cambio, nil
}

// validarCambio devuelve un 409 que explica el estado actual y a cuáles se puede pasar.
// Función pura: se prueba sin BD.
func validarCambio(actual, destino dominio.Estado) error {
	motivo := dominio.MotivoCambioInvalido(actual, destino)
	if motivo == "" {
		return nil
	}
	return &response.AppError{
		Status:  http.StatusConflict,
		Code:    response.CodeConflict,
		Message: motivo,
		Details: map[string]any{
			"estado_actual":      actual,
			"estado_solicitado":  destino,
			"estados_permitidos": actual.Siguientes(),
		},
	}
}
