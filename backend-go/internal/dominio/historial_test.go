package dominio

import "testing"

func TestTipoDeEvento(t *testing.T) {
	if TipoDeEvento(nil) != EventoCreacion {
		t.Error("sin estado anterior es la creación")
	}
	anterior := EstadoPendiente
	if TipoDeEvento(&anterior) != EventoCambioEstado {
		t.Error("con estado anterior es un cambio de estado")
	}
}

func TestVeActorCompleto(t *testing.T) {
	if !VistaAdministrador.VeActorCompleto() || VistaAutor.VeActorCompleto() || VistaPublica.VeActorCompleto() {
		t.Error("solo el administrador ve el actor completo")
	}
}
