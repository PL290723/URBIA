package dominio

import (
	"strings"
	"testing"
)

func TestParseEstadoDestino(t *testing.T) {
	if e, errs := ParseEstadoDestino(" en_revision "); len(errs) > 0 || e != EstadoEnRevision {
		t.Errorf("en_revision debe aceptarse (con espacios recortados): %v %+v", e, errs)
	}
	if _, errs := ParseEstadoDestino(""); reglas(errs)["estado"] != "requerido" {
		t.Errorf("estado vacío debe ser requerido: %+v", errs)
	}
	for _, v := range []string{"cerrado", "rechazado", "RESUELTO"} {
		if _, errs := ParseEstadoDestino(v); reglas(errs)["estado"] != "valor_permitido" {
			t.Errorf("%q debe rechazarse: %+v", v, errs)
		}
	}
}

func TestSiguientesYFinal(t *testing.T) {
	if s := EstadoPendiente.Siguientes(); len(s) != 1 || s[0] != EstadoEnRevision {
		t.Errorf("desde pendiente solo en_revision: %v", s)
	}
	if !EstadoResuelto.EsFinal() || EstadoEnProceso.EsFinal() {
		t.Error("solo resuelto es final")
	}
	if len(EstadoResuelto.Siguientes()) != 0 {
		t.Error("resuelto no tiene siguientes")
	}
	s := EstadoPendiente.Siguientes()
	s[0] = EstadoResuelto // modificar la copia no debe alterar las transiciones
	if !EstadoPendiente.PuedeCambiarA(EstadoEnRevision) || EstadoPendiente.PuedeCambiarA(EstadoResuelto) {
		t.Error("Siguientes debe devolver una copia")
	}
}

func TestMotivoCambioInvalido(t *testing.T) {
	casos := []struct {
		actual, destino Estado
		contiene        string // "" = válido
	}{
		{EstadoPendiente, EstadoEnRevision, ""},
		{EstadoEnRevision, EstadoEnProceso, ""},
		{EstadoEnProceso, EstadoResuelto, ""},
		{EstadoEnRevision, EstadoEnRevision, "ya está"},
		{EstadoPendiente, EstadoResuelto, "solo se permite: en_revision"},
		{EstadoEnProceso, EstadoPendiente, "solo se permite: resuelto"},
		{EstadoResuelto, EstadoEnProceso, "estado final"},
	}
	for _, c := range casos {
		m := MotivoCambioInvalido(c.actual, c.destino)
		if c.contiene == "" && m != "" {
			t.Errorf("%s → %s debería permitirse: %q", c.actual, c.destino, m)
		}
		if c.contiene != "" && !strings.Contains(m, c.contiene) {
			t.Errorf("%s → %s: se esperaba %q en %q", c.actual, c.destino, c.contiene, m)
		}
	}
}
