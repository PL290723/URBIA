package dominio

import "testing"

func TestParseEstado(t *testing.T) {
	validos := []string{"pendiente", "en_revision", "en_proceso", "resuelto"}
	for _, v := range validos {
		if _, err := ParseEstado(v); err != nil {
			t.Errorf("ParseEstado(%q) debería aceptarse: %v", v, err)
		}
	}

	invalidos := []string{"", "rechazado", "PENDIENTE", "en revision", "cerrado"}
	for _, v := range invalidos {
		if _, err := ParseEstado(v); err == nil {
			t.Errorf("ParseEstado(%q) debería rechazarse", v)
		}
	}
}

func TestParseRol(t *testing.T) {
	for _, v := range []string{"ciudadano", "administrador"} {
		if _, err := ParseRol(v); err != nil {
			t.Errorf("ParseRol(%q) debería aceptarse: %v", v, err)
		}
	}
	for _, v := range []string{"", "admin", "superadmin", "CIUDADANO"} {
		if _, err := ParseRol(v); err == nil {
			t.Errorf("ParseRol(%q) debería rechazarse", v)
		}
	}
}

func TestValidarTransicion(t *testing.T) {
	casos := []struct {
		desde, hacia Estado
		permitida    bool
	}{
		{EstadoPendiente, EstadoEnRevision, true},
		{EstadoEnRevision, EstadoEnProceso, true},
		{EstadoEnProceso, EstadoResuelto, true},
		{EstadoPendiente, EstadoResuelto, false}, // no se puede saltar pasos
		{EstadoEnRevision, EstadoResuelto, false},
		{EstadoResuelto, EstadoPendiente, false},  // resuelto es final
		{EstadoEnProceso, EstadoPendiente, false}, // no se regresa
		{EstadoPendiente, EstadoPendiente, false}, // mismo estado
	}
	for _, c := range casos {
		err := ValidarTransicion(c.desde, c.hacia)
		if c.permitida && err != nil {
			t.Errorf("%s → %s debería permitirse: %v", c.desde, c.hacia, err)
		}
		if !c.permitida && err == nil {
			t.Errorf("%s → %s debería rechazarse", c.desde, c.hacia)
		}
	}
}

func TestSoloAdministradorCambiaEstado(t *testing.T) {
	if !RolAdministrador.PuedeCambiarEstado() {
		t.Error("el administrador debería poder cambiar estados")
	}
	if RolCiudadano.PuedeCambiarEstado() {
		t.Error("el ciudadano no debería poder cambiar estados")
	}
}
