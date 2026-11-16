package dominio

import "testing"

func TestEstadoDeSugerencia(t *testing.T) {
	casos := []struct {
		resultado ResultadoDeteccion
		categoria bool
		esperado  EstadoSugerencia
	}{
		{ResultadoDetectado, true, SugerenciaSugerida},
		{ResultadoDetectado, false, SugerenciaSinDeteccion}, // clase sin categoría activa
		{ResultadoNoDetectado, false, SugerenciaSinDeteccion},
		{ResultadoError, false, SugerenciaNoDisponible},
		{"", false, SugerenciaNoDisponible},
	}
	for _, c := range casos {
		if got := EstadoDeSugerencia(c.resultado, c.categoria); got != c.esperado {
			t.Errorf("(%q, %v): se obtuvo %s, se esperaba %s", c.resultado, c.categoria, got, c.esperado)
		}
	}
}

func TestCoincideConCategoria(t *testing.T) {
	uno, dos := int16(1), int16(2)
	if CoincideConCategoria(nil, &uno) != nil || CoincideConCategoria(&uno, nil) != nil {
		t.Error("sin alguna de las dos categorías debe ser nil")
	}
	if c := CoincideConCategoria(&uno, &uno); c == nil || !*c {
		t.Error("misma categoría debe coincidir")
	}
	if c := CoincideConCategoria(&uno, &dos); c == nil || *c {
		t.Error("categorías distintas no deben coincidir")
	}
}
