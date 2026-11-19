package handlers

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/middleware"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
	"github.com/PL290723/URBIA/backend-go/internal/response"
	"github.com/PL290723/URBIA/backend-go/internal/services"
)

// Pruebas HTTP de PATCH /reportes/:id/estado con la misma cadena de middlewares
// que la ruta real (HU 09.2).

func routerEstado(repo *estadosEnMemoria) *gin.Engine {
	h := NewEstadoHandler(services.NewEstadoService(repo, logMudo))
	r := gin.New()
	r.Use(middleware.LimiteCuerpo())
	r.PATCH("/reportes/:id/estado", middleware.RequireAuth(tokensPrueba),
		middleware.RequirePermiso(dominio.Rol.PuedeCambiarEstado), h.Cambiar)
	return r
}

func TestCambiarEstadoHTTP(t *testing.T) {
	repo := &estadosEnMemoria{estados: map[int64]dominio.Estado{10: dominio.EstadoPendiente}}
	r := routerEstado(repo)
	admin := tokenDe(t, 1, dominio.RolAdministrador)

	w := servir(r, peticion(http.MethodPatch, "/reportes/10/estado", `{"estado":"en_revision"}`, admin))
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	d := leer(t, w).Data
	if d["estado_anterior"] != "pendiente" || d["estado_nuevo"] != "en_revision" || d["cambiado_por"] != float64(1) {
		t.Errorf("respuesta inesperada: %v", d)
	}
	if sig, _ := d["estados_siguientes"].([]any); len(sig) != 1 || sig[0] != "en_proceso" {
		t.Errorf("estados_siguientes = %v", d["estados_siguientes"])
	}
}

func TestCambiarEstadoHTTPErrores(t *testing.T) {
	admin := tokenDe(t, 1, dominio.RolAdministrador)
	ciudadano := tokenDe(t, 2, dominio.RolCiudadano)

	casos := []struct {
		nombre, ruta, cuerpo, token string
		falla                       error
		status                      int
		code                        string
	}{
		{"sin token", "/reportes/10/estado", `{"estado":"en_revision"}`, "", nil, 401, response.CodeUnauthorized},
		{"ciudadano", "/reportes/10/estado", `{"estado":"en_revision"}`, ciudadano, nil, 403, response.CodeForbidden},
		{"id no numérico", "/reportes/abc/estado", `{"estado":"en_revision"}`, admin, nil, 400, response.CodeBadRequest},
		{"id cero", "/reportes/0/estado", `{"estado":"en_revision"}`, admin, nil, 400, response.CodeBadRequest},
		{"sin cuerpo", "/reportes/10/estado", "", admin, nil, 400, response.CodeBadRequest},
		{"sin campo estado", "/reportes/10/estado", `{}`, admin, nil, 400, response.CodeValidation},
		{"estado inválido", "/reportes/10/estado", `{"estado":"borrado"}`, admin, nil, 400, response.CodeValidation},
		{"reporte inexistente", "/reportes/99/estado", `{"estado":"en_revision"}`, admin, nil, 404, response.CodeNotFound},
		{"salto de estados", "/reportes/10/estado", `{"estado":"resuelto"}`, admin, nil, 409, response.CodeConflict},
		{"trigger rechaza", "/reportes/10/estado", `{"estado":"en_revision"}`, admin, repositories.ErrCambioRechazado, 409, response.CodeConflict},
		{"BD caída", "/reportes/10/estado", `{"estado":"en_revision"}`, admin, errors.New("conn reset"), 500, response.CodeInternal},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			repo := &estadosEnMemoria{estados: map[int64]dominio.Estado{10: dominio.EstadoPendiente}, falla: tc.falla}
			w := servir(routerEstado(repo), peticion(http.MethodPatch, tc.ruta, tc.cuerpo, tc.token))
			esperarError(t, w, tc.status, tc.code)
			if repo.estados[10] != dominio.EstadoPendiente {
				t.Error("una petición rechazada no debe cambiar el estado")
			}
		})
	}
}

// El 409 de una transición inválida explica el estado actual y los permitidos.
func TestCambiarEstadoHTTPConflictoExplicaPermitidos(t *testing.T) {
	repo := &estadosEnMemoria{estados: map[int64]dominio.Estado{10: dominio.EstadoPendiente}}
	w := servir(routerEstado(repo), peticion(http.MethodPatch, "/reportes/10/estado",
		`{"estado":"en_proceso"}`, tokenDe(t, 1, dominio.RolAdministrador)))
	res := esperarError(t, w, http.StatusConflict, response.CodeConflict)

	d, _ := res.Error.Details.(map[string]any)
	permitidos, _ := d["estados_permitidos"].([]any)
	if d["estado_actual"] != "pendiente" || len(permitidos) != 1 || permitidos[0] != "en_revision" {
		t.Errorf("details = %v", res.Error.Details)
	}
}
