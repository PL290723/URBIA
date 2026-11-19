package dominio

import (
	"testing"
	"time"
)

// 16 nov 2026, 23:30 en Celaya = 17 nov 05:30 UTC: "hoy" debe ser el 16 en hora local.
var ahoraPrueba = time.Date(2026, 11, 17, 5, 30, 0, 0, time.UTC)

func TestFiltrosEstadisticasPorDefecto(t *testing.T) {
	f, errs := ParseFiltrosEstadisticas(CamposEstadisticas{}, ahoraPrueba)
	if len(errs) > 0 {
		t.Fatalf("sin parámetros no hay errores: %+v", errs)
	}
	if f.DesdeFecha() != "2026-10-18" || f.HastaFecha() != "2026-11-16" || f.Dias != 30 {
		t.Errorf("se esperaban los últimos 30 días en hora local: %s a %s (%d)", f.DesdeFecha(), f.HastaFecha(), f.Dias)
	}
	if f.Agrupacion != AgruparDia || f.Geo || f.CeldaGrados != CeldaGradosDefecto {
		t.Errorf("valores por defecto inesperados: %+v", f)
	}
}

func TestFiltrosEstadisticasRangos(t *testing.T) {
	f, _ := ParseFiltrosEstadisticas(CamposEstadisticas{Desde: "2026-11-01"}, ahoraPrueba)
	if f.HastaFecha() != "2026-11-16" || f.Dias != 16 {
		t.Errorf("solo desde: hasta hoy: %s (%d)", f.HastaFecha(), f.Dias)
	}
	f, _ = ParseFiltrosEstadisticas(CamposEstadisticas{Hasta: "2026-06-30"}, ahoraPrueba)
	if f.DesdeFecha() != "2026-06-01" {
		t.Errorf("solo hasta: 30 días que terminan ahí: %s", f.DesdeFecha())
	}
	f, _ = ParseFiltrosEstadisticas(CamposEstadisticas{Desde: "2026-11-16", Hasta: "2026-11-16"}, ahoraPrueba)
	if f.Dias != 1 || !f.HastaExcl.Equal(f.Desde.AddDate(0, 0, 1)) {
		t.Errorf("un solo día debe incluir ese día completo: %+v", f)
	}
	f, _ = ParseFiltrosEstadisticas(CamposEstadisticas{Desde: "2026-01-01", Hasta: "2026-06-30"}, ahoraPrueba)
	if f.Agrupacion != AgruparMes {
		t.Errorf("seis meses se agrupan por mes: %s", f.Agrupacion)
	}
}

func TestFiltrosEstadisticasErrores(t *testing.T) {
	casos := []struct {
		c            CamposEstadisticas
		campo, regla string
	}{
		{CamposEstadisticas{Desde: "16/11/2026"}, "desde", "fecha"},
		{CamposEstadisticas{Desde: "2026-11-10", Hasta: "2026-11-01"}, "desde", "orden"},
		{CamposEstadisticas{Desde: "2024-01-01", Hasta: "2026-11-16"}, "hasta", "rango"},
		{CamposEstadisticas{Agrupacion: "anio"}, "agrupacion", "valor_permitido"},
		{CamposEstadisticas{Geo: "si"}, "geo", "booleano"},
		{CamposEstadisticas{Geo: "true", CeldaGrados: "5"}, "celda_grados", "rango"},
		{CamposEstadisticas{CeldaGrados: "0.01"}, "celda_grados", "requiere_geo"},
	}
	for _, c := range casos {
		_, errs := ParseFiltrosEstadisticas(c.c, ahoraPrueba)
		if reglas(errs)[c.campo] != c.regla {
			t.Errorf("%+v: se esperaba %s/%s, se obtuvo %+v", c.c, c.campo, c.regla, errs)
		}
	}
}

func TestAgrupacionAutomaticaYUnidad(t *testing.T) {
	if AgrupacionAutomatica(31) != AgruparDia || AgrupacionAutomatica(90) != AgruparSemana || AgrupacionAutomatica(200) != AgruparMes {
		t.Error("agrupación automática inesperada")
	}
	if AgruparDia.UnidadSQL() != "day" || AgruparSemana.UnidadSQL() != "week" || AgruparMes.UnidadSQL() != "month" {
		t.Error("unidad SQL inesperada")
	}
	if !RolAdministrador.PuedeVerEstadisticas() || RolCiudadano.PuedeVerEstadisticas() {
		t.Error("solo el administrador ve estadísticas")
	}
}
