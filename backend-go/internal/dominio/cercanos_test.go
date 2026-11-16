package dominio

import "testing"

var paramsPrueba = ParametrosCercania{RadioM: 50, VentanaDias: 30, Limite: 20}

func TestParseParametrosCercania(t *testing.T) {
	p, errs := ParseParametrosCercania(CamposCercania{}, paramsPrueba)
	if len(errs) > 0 || p != paramsPrueba {
		t.Errorf("sin parámetros se usan los de la configuración: %+v %+v", p, errs)
	}

	p, errs = ParseParametrosCercania(CamposCercania{RadioM: "120", VentanaDias: "7", Limite: "5"}, paramsPrueba)
	if len(errs) > 0 || p.RadioM != 120 || p.VentanaDias != 7 || p.Limite != 5 {
		t.Errorf("parámetros mal interpretados: %+v %+v", p, errs)
	}

	_, errs = ParseParametrosCercania(CamposCercania{RadioM: "0", VentanaDias: "400", Limite: "x"}, paramsPrueba)
	r := reglas(errs)
	if r["radio_m"] != "rango" || r["ventana_dias"] != "rango" || r["limite"] != "rango" {
		t.Errorf("se esperaban 3 errores de rango: %+v", errs)
	}
}

func TestEvaluarCoincidencia(t *testing.T) {
	si, no := true, false
	casos := []struct {
		nombre string
		s      SenalesCercania
		nivel  NivelCoincidencia
		dentro bool
	}{
		{"mismo punto, misma categoría, mismo día", SenalesCercania{0, &si, 0}, CoincidenciaAlta, true},
		{"a 25 m, misma categoría, 3 días", SenalesCercania{25, &si, 3}, CoincidenciaAlta, true},
		{"mismo punto, otra categoría, mismo día", SenalesCercania{0, &no, 0}, CoincidenciaMedia, true},
		{"mismo punto, sin categoría, mismo día", SenalesCercania{0, nil, 0}, CoincidenciaMedia, true},
		{"en el borde del radio, otra categoría, hace un año", SenalesCercania{50, &no, 365}, CoincidenciaBaja, false},
		{"a 40 m, misma categoría, hace 2 meses", SenalesCercania{40, &si, -60}, CoincidenciaMedia, false},
	}
	for _, c := range casos {
		e := EvaluarCoincidencia(c.s, paramsPrueba)
		if e.Nivel != c.nivel || e.DentroVentana != c.dentro {
			t.Errorf("%s: se obtuvo %+v, se esperaba nivel %s y dentro=%v", c.nombre, e, c.nivel, c.dentro)
		}
		if e.Puntuacion < 0 || e.Puntuacion > 1 {
			t.Errorf("%s: puntuación fuera de 0..1: %v", c.nombre, e.Puntuacion)
		}
	}
}

func TestPuntuacionMaxima(t *testing.T) {
	si := true
	if e := EvaluarCoincidencia(SenalesCercania{0, &si, 0}, paramsPrueba); e.Puntuacion != 1 {
		t.Errorf("todas las señales al máximo deben dar 1, se obtuvo %v", e.Puntuacion)
	}
}

func TestMismaCategoria(t *testing.T) {
	a, b := int16(1), int16(3)
	if MismaCategoria(nil, &a) != nil {
		t.Error("sin categoría no se puede comparar")
	}
	if c := MismaCategoria(&a, &a); c == nil || !*c {
		t.Error("misma categoría")
	}
	if c := MismaCategoria(&a, &b); c == nil || *c {
		t.Error("categorías distintas")
	}
}

func TestPuedeRevisarDuplicados(t *testing.T) {
	if !RolAdministrador.PuedeRevisarDuplicados() || RolCiudadano.PuedeRevisarDuplicados() {
		t.Error("solo el administrador revisa duplicados")
	}
}
