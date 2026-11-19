package services

import (
	"errors"
	"net/http"
	"testing"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

func TestValidarCambioPermitido(t *testing.T) {
	if err := validarCambio(dominio.EstadoPendiente, dominio.EstadoEnRevision); err != nil {
		t.Fatalf("pendiente → en_revision debe permitirse: %v", err)
	}
}

func TestValidarCambioRechazadoExplicaPermitidos(t *testing.T) {
	err := validarCambio(dominio.EstadoPendiente, dominio.EstadoResuelto)
	var appErr *response.AppError
	if !errors.As(err, &appErr) || appErr.Status != http.StatusConflict {
		t.Fatalf("saltarse estados debe dar 409: %v", err)
	}
	d := appErr.Details.(map[string]any)
	permitidos := d["estados_permitidos"].([]dominio.Estado)
	if d["estado_actual"] != dominio.EstadoPendiente || len(permitidos) != 1 || permitidos[0] != dominio.EstadoEnRevision {
		t.Errorf("el 409 debe decir el estado actual y los permitidos: %+v", d)
	}
}

func TestValidarCambioEstadoFinal(t *testing.T) {
	err := validarCambio(dominio.EstadoResuelto, dominio.EstadoEnProceso)
	var appErr *response.AppError
	if !errors.As(err, &appErr) || appErr.Status != http.StatusConflict {
		t.Fatalf("un reporte resuelto no cambia: %v", err)
	}
	if p := appErr.Details.(map[string]any)["estados_permitidos"].([]dominio.Estado); len(p) != 0 {
		t.Errorf("resuelto no tiene estados permitidos: %v", p)
	}
}
