package services

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

// Pruebas de EstadoService.Cambiar con un repositorio en memoria (HU 09.2).

// estadosEnMemoria es un doble de RepositorioEstados: guarda el estado de cada
// reporte y llama a validar() igual que el repositorio real dentro de la transacción.
type estadosEnMemoria struct {
	estados map[int64]dominio.Estado
	falla   error // error a devolver en lugar de cambiar (simula la BD)
	llamado bool
}

func (r *estadosEnMemoria) Cambiar(_ context.Context, idReporte, idUsuario int64, destino dominio.Estado,
	validar func(dominio.Estado) error) (*models.CambioEstado, error) {
	r.llamado = true
	if r.falla != nil {
		return nil, r.falla
	}
	actual, ok := r.estados[idReporte]
	if !ok {
		return nil, repositories.ErrNoEncontrado
	}
	if err := validar(actual); err != nil {
		return nil, err
	}
	r.estados[idReporte] = destino
	return &models.CambioEstado{IDReporte: idReporte, EstadoAnterior: actual, EstadoNuevo: destino,
		CambiadoPor: idUsuario, Fecha: time.Now()}, nil
}

var (
	admin     = auth.Sesion{UsuarioID: 1, Rol: dominio.RolAdministrador}
	ciudadano = auth.Sesion{UsuarioID: 2, Rol: dominio.RolCiudadano}
	logMudo   = slog.New(slog.NewTextHandler(io.Discard, nil))
)

func nuevoEstadoService(estados map[int64]dominio.Estado) (*EstadoService, *estadosEnMemoria) {
	repo := &estadosEnMemoria{estados: estados}
	return NewEstadoService(repo, logMudo), repo
}

func TestCambiarEstadoCicloCompleto(t *testing.T) {
	svc, repo := nuevoEstadoService(map[int64]dominio.Estado{10: dominio.EstadoPendiente})
	ciclo := []dominio.Estado{dominio.EstadoEnRevision, dominio.EstadoEnProceso, dominio.EstadoResuelto}
	for _, destino := range ciclo {
		c, err := svc.Cambiar(context.Background(), admin, 10, string(destino))
		if err != nil {
			t.Fatalf("→ %s: %v", destino, err)
		}
		if c.EstadoNuevo != destino || c.CambiadoPor != admin.UsuarioID {
			t.Errorf("cambio inesperado: %+v", c)
		}
		if len(c.EstadosSiguientes) != len(destino.Siguientes()) {
			t.Errorf("estados_siguientes = %v, esperado %v", c.EstadosSiguientes, destino.Siguientes())
		}
	}
	if repo.estados[10] != dominio.EstadoResuelto {
		t.Errorf("estado final = %s", repo.estados[10])
	}
}

func TestCambiarEstadoErrores(t *testing.T) {
	casos := []struct {
		nombre  string
		sesion  auth.Sesion
		id      int64
		destino string
		falla   error
		status  int
		code    string
		tocaBD  bool
	}{
		{"ciudadano", ciudadano, 10, "en_revision", nil, http.StatusForbidden, response.CodeForbidden, false},
		{"estado vacío", admin, 10, "  ", nil, http.StatusBadRequest, response.CodeValidation, false},
		{"estado inventado", admin, 10, "archivado", nil, http.StatusBadRequest, response.CodeValidation, false},
		{"reporte inexistente", admin, 99, "en_revision", nil, http.StatusNotFound, response.CodeNotFound, true},
		{"salto de estados", admin, 10, "resuelto", nil, http.StatusConflict, response.CodeConflict, true},
		{"mismo estado", admin, 10, "pendiente", nil, http.StatusConflict, response.CodeConflict, true},
		{"trigger de la BD lo rechaza", admin, 10, "en_revision", repositories.ErrCambioRechazado, http.StatusConflict, response.CodeConflict, true},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			svc, repo := nuevoEstadoService(map[int64]dominio.Estado{10: dominio.EstadoPendiente})
			repo.falla = tc.falla
			_, err := svc.Cambiar(context.Background(), tc.sesion, tc.id, tc.destino)
			esperarAppError(t, err, tc.status, tc.code)
			if repo.llamado != tc.tocaBD {
				t.Errorf("¿consultó la BD? = %v, esperado %v", repo.llamado, tc.tocaBD)
			}
			if repo.estados[10] != dominio.EstadoPendiente {
				t.Error("un cambio rechazado no debe modificar el estado")
			}
		})
	}
}

func TestCambiarEstadoErrorDeBDEsInterno(t *testing.T) {
	svc, repo := nuevoEstadoService(map[int64]dominio.Estado{10: dominio.EstadoPendiente})
	repo.falla = errors.New("deadlock detected")
	_, err := svc.Cambiar(context.Background(), admin, 10, "en_revision")
	var appErr *response.AppError
	if err == nil || errors.As(err, &appErr) {
		t.Fatalf("un error inesperado de la BD debe llegar como interno (→ 500): %v", err)
	}
}
