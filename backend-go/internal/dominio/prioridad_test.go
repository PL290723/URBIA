package dominio

import (
	"strings"
	"testing"
)

func entero(n int) *int      { return &n }
func texto(s string) *string { return &s }
func nivel(r ResultadoPrioridad) string {
	if r.Nivel == nil {
		return "<nil>"
	}
	return string(*r.Nivel)
}

func factor(r ResultadoPrioridad, f FactorPrioridad) Factor {
	for _, x := range r.Factores {
		if x.Factor == f {
			return x
		}
	}
	return Factor{}
}

func TestPesosPorDefectoSumanUno(t *testing.T) {
	if err := ConfigPrioridadPorDefecto().Pesos.Validar(); err != nil {
		t.Fatalf("los pesos por defecto deben ser válidos: %v", err)
	}
	if err := (PesosPrioridad{0.5, 0.3, 0.2, 0.1}).Validar(); err == nil {
		t.Error("pesos que suman 1.1 deben rechazarse")
	}
	if err := (PesosPrioridad{1.2, -0.2, 0, 0}).Validar(); err == nil {
		t.Error("pesos fuera de 0..1 deben rechazarse")
	}
}

func TestCalcularPrioridadTodosLosFactores(t *testing.T) {
	d := DatosPrioridad{
		Evaluacion: EvaluacionAdmin{Gravedad: entero(5), Afectacion: entero(3),
			JustificacionAfectacion: texto("Avenida principal con paso de transporte público")},
		Recurrencias: entero(2), IDsRecurrentes: []int64{41, 42},
		RadioM: 50, VentanaDias: 30, DiasAntiguedad: 15,
	}
	r := CalcularPrioridad(d, ConfigPrioridadPorDefecto())

	// 0.40·1 + 0.30·0.5 + 0.20·0.5 + 0.10·0.5 = 0.70 → 70
	if r.Puntuacion != 70 || nivel(r) != "alta" || r.Cobertura != 1 || !r.Suficiente {
		t.Fatalf("se esperaba 70 / alta / cobertura 1: %+v", r)
	}
	if len(r.Faltantes) != 0 || !r.Provisional || r.VersionFormula != VersionFormula {
		t.Errorf("sin faltantes y marcado como provisional: %+v", r)
	}
	suma := 0.0
	for _, f := range r.Factores {
		if f.Aporte == nil || f.Explicacion == "" {
			t.Errorf("cada factor disponible debe explicar su aporte: %+v", f)
			continue
		}
		suma += *f.Aporte
	}
	if suma != r.Puntuacion {
		t.Errorf("los aportes (%v) deben sumar la puntuación (%v)", suma, r.Puntuacion)
	}
}

func TestCalcularPrioridadNoInventaFaltantes(t *testing.T) {
	// Sin evaluación del administrador: solo hay recurrencia y antigüedad (30 %).
	d := DatosPrioridad{Recurrencias: entero(4), RadioM: 50, VentanaDias: 30, DiasAntiguedad: 60}
	r := CalcularPrioridad(d, ConfigPrioridadPorDefecto())

	if r.Suficiente || r.Nivel != nil || r.Cobertura != 0.3 {
		t.Fatalf("con 30%% de cobertura no debe asignarse nivel: %+v", r)
	}
	g := factor(r, FactorGravedad)
	if g.Disponible || g.Valor != nil || g.Aporte != nil {
		t.Errorf("la gravedad no evaluada no debe tener valor: %+v", g)
	}
	if len(r.Faltantes) != 2 || r.Faltantes[0] != FactorGravedad || r.Faltantes[1] != FactorAfectacion {
		t.Errorf("faltantes esperados gravedad y afectación: %v", r.Faltantes)
	}
	if !strings.Contains(r.Resumen, "insuficientes") {
		t.Errorf("el resumen debe decir que faltan datos: %q", r.Resumen)
	}
}

func TestCalcularPrioridadRepartePesos(t *testing.T) {
	// Reporte sin categoría: la recurrencia no es verificable. Gravedad 4 y 3 días.
	d := DatosPrioridad{Evaluacion: EvaluacionAdmin{Gravedad: entero(4)},
		MotivoSinRecurrencia: "el reporte no tiene categoría confirmada", DiasAntiguedad: 3}
	r := CalcularPrioridad(d, ConfigPrioridadPorDefecto())

	// Cobertura 0.40 + 0.10 = 0.50 → gravedad pesa 0.8 y antigüedad 0.2
	// 100·(0.8·0.75 + 0.2·0.1) = 62 → media
	if r.Cobertura != 0.5 || r.Puntuacion != 62 || nivel(r) != "media" {
		t.Fatalf("se esperaba cobertura 0.5, 62, media: %+v", r)
	}
	rec := factor(r, FactorRecurrencia)
	if rec.Disponible || !strings.Contains(rec.Explicacion, "categoría") {
		t.Errorf("la recurrencia debe explicarse como no verificable: %+v", rec)
	}
	if pe := factor(r, FactorGravedad).PesoEfectivo; pe == nil || *pe != 0.8 {
		t.Errorf("peso efectivo de la gravedad debe ser 0.8: %v", pe)
	}
}

func TestCalcularPrioridadCeroEsUnDatoReal(t *testing.T) {
	// Recurrencia 0 sí es un dato (se verificó y no hay otros reportes).
	d := DatosPrioridad{Evaluacion: EvaluacionAdmin{Gravedad: entero(1)},
		Recurrencias: entero(0), RadioM: 50, VentanaDias: 30}
	r := CalcularPrioridad(d, ConfigPrioridadPorDefecto())
	if r.Puntuacion != 0 || nivel(r) != "baja" || !factor(r, FactorRecurrencia).Disponible {
		t.Fatalf("se esperaba 0 / baja con recurrencia disponible: %+v", r)
	}
}

func TestNivelDePuntuacion(t *testing.T) {
	casos := map[float64]NivelPrioridad{0: PrioridadBaja, 33.9: PrioridadBaja, 34: PrioridadMedia,
		66.9: PrioridadMedia, 67: PrioridadAlta, 100: PrioridadAlta}
	for p, esperado := range casos {
		if n := NivelDePuntuacion(p); n != esperado {
			t.Errorf("NivelDePuntuacion(%v) = %s, se esperaba %s", p, n, esperado)
		}
	}
}

func TestValidarEvaluacion(t *testing.T) {
	_, errs := ValidarEvaluacion(EvaluacionAdmin{Gravedad: entero(6), Afectacion: entero(0)})
	r := reglas(errs)
	if r["gravedad_observable"] != "rango" || r["afectacion_estimada"] != "rango" {
		t.Errorf("se esperaban errores de rango: %+v", errs)
	}

	_, errs = ValidarEvaluacion(EvaluacionAdmin{Afectacion: entero(4)})
	if reglas(errs)["justificacion_afectacion"] != "requerido" {
		t.Errorf("la afectación sin justificación debe rechazarse: %+v", errs)
	}

	_, errs = ValidarEvaluacion(EvaluacionAdmin{Afectacion: entero(4), JustificacionAfectacion: texto("   corta ")})
	if reglas(errs)["justificacion_afectacion"] != "longitud" {
		t.Errorf("la justificación corta debe rechazarse: %+v", errs)
	}

	e, errs := ValidarEvaluacion(EvaluacionAdmin{Gravedad: entero(3), Afectacion: entero(2),
		JustificacionAfectacion: texto("  Calle cerrada, afecta a 3 casas  ")})
	if len(errs) > 0 || *e.JustificacionAfectacion != "Calle cerrada, afecta a 3 casas" {
		t.Errorf("evaluación válida rechazada o sin recortar: %+v %+v", e, errs)
	}

	if _, errs = ValidarEvaluacion(EvaluacionAdmin{}); len(errs) > 0 {
		t.Errorf("una evaluación vacía es válida (se calcula con lo disponible): %+v", errs)
	}
}

func TestPermisosPrioridad(t *testing.T) {
	if !RolAdministrador.PuedeCalcularPrioridad() || RolCiudadano.PuedeCalcularPrioridad() {
		t.Error("solo el administrador calcula prioridad")
	}
	if EstadoResuelto.PermiteCalcularPrioridad() || !EstadoPendiente.PermiteCalcularPrioridad() {
		t.Error("un reporte resuelto no se prioriza")
	}
}
