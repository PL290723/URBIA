package services

import (
	"strings"
	"testing"
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
)

func eventosDePrueba() []models.HistorialEstado {
	t0 := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	pendiente, revision := dominio.EstadoPendiente, dominio.EstadoEnRevision
	idAna, idJuan := int64(5), int64(6)
	ana, juan := "Ana Prueba", "Juan Admin"
	return []models.HistorialEstado{
		{ID: 1, Tipo: dominio.EventoCreacion, EstadoNuevo: pendiente, Fecha: t0,
			Actor: &models.ActorHistorial{IDUsuario: &idAna, Nombre: &ana, Rol: dominio.RolCiudadano}},
		{ID: 2, Tipo: dominio.EventoCambioEstado, EstadoAnterior: &pendiente, EstadoNuevo: revision,
			Fecha: t0.Add(time.Hour), Actor: &models.ActorHistorial{IDUsuario: &idJuan, Nombre: &juan, Rol: dominio.RolAdministrador}},
	}
}

func TestHistorialParaAdministradorVeActorCompleto(t *testing.T) {
	h := historialPara(eventosDePrueba(), dominio.VistaAdministrador)
	a := h[1].Actor
	if a == nil || a.IDUsuario == nil || *a.IDUsuario != 6 || a.Nombre == nil || *a.Nombre != "Juan Admin" {
		t.Fatalf("el administrador debe ver quién hizo el cambio: %+v", a)
	}
	if h[1].Tipo != dominio.EventoCambioEstado || *h[1].EstadoAnterior != dominio.EstadoPendiente {
		t.Errorf("debe incluir tipo y estado anterior: %+v", h[1])
	}
}

func TestHistorialParaAutorSoloVeRol(t *testing.T) {
	original := eventosDePrueba()
	h := historialPara(original, dominio.VistaAutor)
	j := aJSON(t, h)
	if strings.Contains(j, "Juan Admin") || strings.Contains(j, `"id_usuario"`) {
		t.Errorf("el autor no debe ver el nombre ni el id del administrador: %s", j)
	}
	if h[1].Actor == nil || h[1].Actor.Rol != dominio.RolAdministrador {
		t.Errorf("el autor sí ve que lo cambió un administrador: %+v", h[1].Actor)
	}
	if original[1].Actor.Nombre == nil {
		t.Error("historialPara no debe modificar los datos originales")
	}
}

func TestHistorialParaVistaPublicaSinActor(t *testing.T) {
	for _, e := range historialPara(eventosDePrueba(), dominio.VistaPublica) {
		if e.Actor != nil {
			t.Errorf("la vista pública no lleva actor: %+v", e)
		}
	}
	if historialPara(nil, dominio.VistaAdministrador) != nil {
		t.Error("sin eventos no se inventa una lista")
	}
}

func TestHistorialReporteDe(t *testing.T) {
	f := filaDePrueba()
	f.Estado = dominio.EstadoEnRevision
	r := historialReporteDe(f, eventosDePrueba(), dominio.VistaAdministrador)
	if r.IDReporte != 12 || r.EstadoActual != dominio.EstadoEnRevision || r.Total != 2 || !r.SoloLectura {
		t.Fatalf("respuesta de historial inesperada: %+v", r)
	}
	if r.Eventos[0].Tipo != dominio.EventoCreacion || r.Eventos[0].EstadoAnterior != nil {
		t.Errorf("el primer evento es la creación: %+v", r.Eventos[0])
	}
}
