package dominio

import (
	"testing"
	"time"
)

func TestFiltrosPorDefecto(t *testing.T) {
	f, errs := ParseFiltrosReporte(CamposFiltro{})
	if len(errs) > 0 {
		t.Fatalf("sin filtros no debe haber errores: %+v", errs)
	}
	if f.Pagina != 1 || f.PorPagina != PorPaginaPorDefecto || f.Offset() != 0 {
		t.Errorf("paginación por defecto incorrecta: %+v", f)
	}
	if f.Estados != nil || f.Categoria != nil || f.Desde != nil || f.Hasta != nil || f.BBox != nil || f.SoloMios {
		t.Errorf("sin filtros no debe filtrar nada: %+v", f)
	}
}

func TestFiltrosValidos(t *testing.T) {
	f, errs := ParseFiltrosReporte(CamposFiltro{
		Pagina: "3", PorPagina: "50", Estado: "pendiente, en_revision,pendiente",
		IDCategoria: "3", Desde: "2026-11-01", Hasta: "2026-11-16",
		BBox: "-100.85,20.50,-100.78,20.55", Mios: "true",
	})
	if len(errs) > 0 {
		t.Fatalf("no debería haber errores: %+v", errs)
	}
	if f.Offset() != 100 {
		t.Errorf("offset esperado 100, se obtuvo %d", f.Offset())
	}
	if len(f.Estados) != 2 {
		t.Errorf("los estados repetidos se deben quitar: %v", f.Estados)
	}
	if *f.Categoria != 3 || !f.SoloMios || f.BBox.MinLon != -100.85 {
		t.Errorf("filtros mal interpretados: %+v", f)
	}
	// "hasta" incluye todo el día 16: el límite es el 17 a las 00:00 (UTC-6)
	esperado := time.Date(2026, 11, 17, 6, 0, 0, 0, time.UTC)
	if !f.Hasta.Equal(esperado) {
		t.Errorf("hasta esperado %v, se obtuvo %v", esperado, f.Hasta.UTC())
	}
}

func TestFiltrosInvalidos(t *testing.T) {
	casos := []struct {
		nombre string
		campos CamposFiltro
		campo  string
	}{
		{"página 0", CamposFiltro{Pagina: "0"}, "pagina"},
		{"página texto", CamposFiltro{Pagina: "x"}, "pagina"},
		{"por_pagina grande", CamposFiltro{PorPagina: "500"}, "por_pagina"},
		{"estado inexistente", CamposFiltro{Estado: "pendiente,borrado"}, "estado"},
		{"categoría negativa", CamposFiltro{IDCategoria: "-2"}, "id_categoria"},
		{"fecha mal formada", CamposFiltro{Desde: "16/11/2026"}, "desde"},
		{"desde después de hasta", CamposFiltro{Desde: "2026-11-20", Hasta: "2026-11-10"}, "desde"},
		{"bbox incompleto", CamposFiltro{BBox: "-100.8,20.5,-100.7"}, "bbox"},
		{"bbox invertido", CamposFiltro{BBox: "-100.7,20.5,-100.8,20.6"}, "bbox"},
		{"bbox fuera de rango", CamposFiltro{BBox: "-200,20.5,-100.8,20.6"}, "bbox"},
		{"mios no booleano", CamposFiltro{Mios: "quizas"}, "mios"},
	}
	for _, c := range casos {
		_, errs := ParseFiltrosReporte(c.campos)
		if reglas(errs)[c.campo] == "" {
			t.Errorf("%s: se esperaba error en %s, se obtuvo %+v", c.nombre, c.campo, errs)
		}
	}
}

func TestMismaFechaDesdeHasta(t *testing.T) {
	// Un solo día (desde = hasta) es válido.
	if _, errs := ParseFiltrosReporte(CamposFiltro{Desde: "2026-11-16", Hasta: "2026-11-16"}); len(errs) > 0 {
		t.Errorf("un solo día debe ser válido: %+v", errs)
	}
}

func TestVistaDeReporte(t *testing.T) {
	if VistaDeReporte(RolAdministrador, 1, 2) != VistaAdministrador {
		t.Error("el administrador ve todo")
	}
	if VistaDeReporte(RolCiudadano, 5, 5) != VistaAutor {
		t.Error("el autor ve su reporte completo")
	}
	if VistaDeReporte(RolCiudadano, 5, 6) != VistaPublica {
		t.Error("otro ciudadano ve la vista pública")
	}
	if VistaDeReporte(RolCiudadano, 0, 0) != VistaPublica {
		t.Error("un id vacío nunca es autor")
	}
	if VistaPublica.VePrivado() || VistaPublica.VeAdministrativo() || VistaAutor.VeAdministrativo() {
		t.Error("permisos de vista incorrectos")
	}
	if !VistaAutor.VePrivado() || !VistaAdministrador.VeAdministrativo() {
		t.Error("autor y administrador deben ver lo privado")
	}
}

func TestUbicacionAproximada(t *testing.T) {
	lat, lon := UbicacionAproximada(20.523512, -100.815788)
	if lat != 20.524 || lon != -100.816 {
		t.Errorf("redondeo a 3 decimales incorrecto: %v, %v", lat, lon)
	}
}

func TestAjustarBBoxPublica(t *testing.T) {
	b := AjustarBBoxPublica(BBox{MinLon: -100.81571, MinLat: 20.52351, MaxLon: -100.81569, MaxLat: 20.52352})
	if b.MinLon != -100.816 || b.MinLat != 20.523 || b.MaxLon != -100.815 || b.MaxLat != 20.524 {
		t.Errorf("el bbox debe agrandarse a la cuadrícula: %+v", b)
	}
	// Un bbox que ya está en la cuadrícula no cambia.
	b = AjustarBBoxPublica(BBox{MinLon: -100.85, MinLat: 20.5, MaxLon: -100.78, MaxLat: 20.55})
	if b.MinLon != -100.85 || b.MinLat != 20.5 || b.MaxLon != -100.78 || b.MaxLat != 20.55 {
		t.Errorf("un bbox en la cuadrícula no debe cambiar: %+v", b)
	}
}
