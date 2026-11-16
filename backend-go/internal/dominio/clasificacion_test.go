package dominio

import "testing"

func TestOrigenDeClasificacion(t *testing.T) {
	bache, basura := int16(1), int16(3)
	casos := []struct {
		nombre   string
		elegida  int16
		sugerida *int16
		esperado OrigenCategoria
	}{
		{"sin sugerencia", bache, nil, CategoriaManual},
		{"acepta la sugerencia", basura, &basura, CategoriaSugerida},
		{"corrige la sugerencia", bache, &basura, CategoriaCorregida},
	}
	for _, c := range casos {
		if got := OrigenDeClasificacion(c.elegida, c.sugerida); got != c.esperado {
			t.Errorf("%s: se obtuvo %s, se esperaba %s", c.nombre, got, c.esperado)
		}
	}
}

func TestPuedeConfirmarCategoria(t *testing.T) {
	if !RolCiudadano.PuedeConfirmarCategoria() {
		t.Error("el ciudadano debe poder confirmar la categoría de su reporte")
	}
	if RolAdministrador.PuedeConfirmarCategoria() {
		t.Error("el administrador no usa este flujo")
	}
}

func TestPermiteCambiarCategoria(t *testing.T) {
	if !EstadoPendiente.PermiteCambiarCategoria() {
		t.Error("en pendiente sí se puede cambiar")
	}
	for _, e := range []Estado{EstadoEnRevision, EstadoEnProceso, EstadoResuelto, "rechazado"} {
		if e.PermiteCambiarCategoria() {
			t.Errorf("en %s ya no se puede cambiar", e)
		}
	}
}
