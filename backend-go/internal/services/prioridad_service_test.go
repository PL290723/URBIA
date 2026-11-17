package services

import (
	"strings"
	"testing"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
)

func ptrInt(n int) *int         { return &n }
func ptrInt16(n int16) *int16   { return &n }
func ptrTexto(s string) *string { return &s }

func TestArmarCalculoAsignaSoloConDatosSuficientes(t *testing.T) {
	cfg := dominio.ConfigPrioridadPorDefecto()

	sinEvaluar := armarCalculo(7, dominio.DatosPrioridad{
		Recurrencias: ptrInt(1), RadioM: 50, VentanaDias: 30, DiasAntiguedad: 2.04,
	}, ptrInt16(1), cfg)
	if sinEvaluar.PrioridadAsignada || sinEvaluar.Nivel != nil {
		t.Errorf("sin evaluación del administrador no debe asignarse prioridad: %+v", sinEvaluar)
	}
	if sinEvaluar.DiasAntiguedad != 2 || *sinEvaluar.Recurrencias != 1 || *sinEvaluar.IDCategoria != 1 {
		t.Errorf("deben guardarse las entradas usadas: %+v", sinEvaluar)
	}

	evaluado := armarCalculo(7, dominio.DatosPrioridad{
		Evaluacion:   dominio.EvaluacionAdmin{Gravedad: ptrInt(5)},
		Recurrencias: ptrInt(1), RadioM: 50, VentanaDias: 30, DiasAntiguedad: 2,
	}, ptrInt16(1), cfg)
	if !evaluado.PrioridadAsignada || evaluado.Nivel == nil {
		t.Errorf("con la gravedad evaluada sí debe asignarse: %+v", evaluado)
	}
}

func TestCombinarEvaluacionReutilizaLaAnterior(t *testing.T) {
	anterior := &models.CalculoPrioridad{Evaluacion: dominio.EvaluacionAdmin{
		Gravedad: ptrInt(4), Afectacion: ptrInt(3), JustificacionAfectacion: ptrTexto("Frente a una escuela"),
	}}

	e := combinarEvaluacion(dominio.EvaluacionAdmin{}, anterior)
	if *e.Gravedad != 4 || *e.Afectacion != 3 || *e.JustificacionAfectacion != "Frente a una escuela" {
		t.Errorf("un recálculo sin cuerpo debe reutilizar la evaluación anterior: %+v", e)
	}

	e = combinarEvaluacion(dominio.EvaluacionAdmin{Gravedad: ptrInt(2)}, anterior)
	if *e.Gravedad != 2 || *e.Afectacion != 3 {
		t.Errorf("lo que manda el administrador reemplaza solo ese campo: %+v", e)
	}

	if e = combinarEvaluacion(dominio.EvaluacionAdmin{}, nil); e.Gravedad != nil || e.Afectacion != nil {
		t.Errorf("sin cálculo anterior no se inventa evaluación: %+v", e)
	}
}

func TestCompararVigencia(t *testing.T) {
	cfg := dominio.ConfigPrioridadPorDefecto()
	datos := dominio.DatosPrioridad{Evaluacion: dominio.EvaluacionAdmin{Gravedad: ptrInt(3)},
		Recurrencias: ptrInt(0), RadioM: 50, VentanaDias: 30, DiasAntiguedad: 1}
	anterior := armarCalculo(7, datos, ptrInt16(1), cfg)

	if v := compararVigencia(anterior, armarCalculo(7, datos, ptrInt16(1), cfg), dominio.EstadoPendiente); v.Desactualizado {
		t.Errorf("con los mismos datos el cálculo sigue vigente: %+v", v)
	}

	datos.Recurrencias = ptrInt(4)
	v := compararVigencia(anterior, armarCalculo(7, datos, ptrInt16(2), cfg), dominio.EstadoPendiente)
	todo := strings.Join(v.Motivos, " | ")
	if !v.Desactualizado || !strings.Contains(todo, "categoría") || !strings.Contains(todo, "0 → 4") {
		t.Errorf("debe explicar el cambio de categoría y de recurrencia: %+v", v)
	}

	if v := compararVigencia(nil, anterior, dominio.EstadoPendiente); !v.Desactualizado || len(v.Motivos) != 1 {
		t.Errorf("sin cálculo previo debe pedirse calcular: %+v", v)
	}
}
