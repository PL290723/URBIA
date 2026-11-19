package dominio

import "testing"

func TestPuedeAccederArchivo(t *testing.T) {
	casos := []struct {
		nombre         string
		rol            Rol
		usuario, autor int64
		esperado       bool
	}{
		{"autor del reporte", RolCiudadano, 5, 5, true},
		{"otro ciudadano", RolCiudadano, 6, 5, false},
		{"administrador", RolAdministrador, 1, 5, true},
		{"usuario sin id", RolCiudadano, 0, 0, false},
	}
	for _, c := range casos {
		if got := PuedeAccederArchivo(c.rol, c.usuario, c.autor); got != c.esperado {
			t.Errorf("%s: se obtuvo %v, se esperaba %v", c.nombre, got, c.esperado)
		}
	}
}

func TestExtensionPorMIME(t *testing.T) {
	if ext, ok := ExtensionPorMIME("image/png"); !ok || ext != "png" {
		t.Errorf("image/png debería dar png, dio %q", ext)
	}
	for _, m := range []string{"image/gif", "image/svg+xml", "text/html", ""} {
		if _, ok := ExtensionPorMIME(m); ok {
			t.Errorf("%q no debería aceptarse", m)
		}
	}
}

func TestParseVariante(t *testing.T) {
	if _, err := ParseVariante("procesada"); err != nil {
		t.Error(err)
	}
	if _, err := ParseVariante("../otra"); err == nil {
		t.Error("una variante desconocida debería rechazarse")
	}
}
