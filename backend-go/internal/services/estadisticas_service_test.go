package services

import (
	"strings"
	"testing"
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
)

func filtrosPrueba(t *testing.T, geo string) dominio.FiltrosEstadisticas {
	t.Helper()
	f, errs := dominio.ParseFiltrosEstadisticas(dominio.CamposEstadisticas{
		Desde: "2026-11-01", Hasta: "2026-11-16", Geo: geo,
	}, time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC))
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	return f
}

func estadoPtr(e dominio.Estado) *dominio.Estado { return &e }
func textoPtr(s string) *string                  { return &s }

func datosPrueba() *models.DatosEstadisticas {
	bache, basura, alumbrado := int16(1), int16(3), int16(2)
	return &models.DatosEstadisticas{
		PorEstado: map[dominio.Estado]int64{dominio.EstadoPendiente: 5, dominio.EstadoResuelto: 2},
		PorCategoria: []models.FilaCategoriaEstado{
			{IDCategoria: &bache, Nombre: textoPtr("bache"), Activa: true, Estado: estadoPtr(dominio.EstadoPendiente), Total: 3},
			{IDCategoria: &bache, Nombre: textoPtr("bache"), Activa: true, Estado: estadoPtr(dominio.EstadoResuelto), Total: 2},
			{IDCategoria: &basura, Nombre: textoPtr("basura"), Activa: true, Estado: estadoPtr(dominio.EstadoPendiente), Total: 1},
			{IDCategoria: &alumbrado, Nombre: textoPtr("alumbrado_publico"), Activa: true}, // activa sin reportes
			{Activa: true, Estado: estadoPtr(dominio.EstadoPendiente), Total: 1},           // sin categoría
		},
		PorPeriodo: []models.ConteoPeriodo{{Inicio: "2026-11-01", Total: 4}, {Inicio: "2026-11-02", Total: 0}},
		Celdas: []models.CeldaGeo{
			{Latitud: 20.525, Longitud: -100.815, Total: 5},
			{Latitud: 20.535, Longitud: -100.825, Total: 2},
		},
	}
}

func TestArmarEstadisticasTotalesYEstados(t *testing.T) {
	e := armarEstadisticas(filtrosPrueba(t, ""), datosPrueba())
	if e.Total != 7 {
		t.Errorf("el total es la suma por estado: %d", e.Total)
	}
	if len(e.PorEstado) != 4 || e.PorEstado[1].Estado != dominio.EstadoEnRevision || e.PorEstado[1].Total != 0 {
		t.Errorf("deben venir los 4 estados en orden, con ceros: %+v", e.PorEstado)
	}
	if e.Filtros.Desde != "2026-11-01" || e.Filtros.Hasta != "2026-11-16" || e.Filtros.Agrupacion != dominio.AgruparDia {
		t.Errorf("filtros aplicados inesperados: %+v", e.Filtros)
	}
	if e.Geografica != nil {
		t.Error("sin geo=true no hay distribución geográfica")
	}
}

func TestArmarCategorias(t *testing.T) {
	c := armarEstadisticas(filtrosPrueba(t, ""), datosPrueba()).PorCategoria
	if len(c) != 4 {
		t.Fatalf("bache, basura, alumbrado y sin_categoria: %+v", c)
	}
	if c[0].Nombre != "bache" || c[0].Total != 5 || c[0].PorEstado[dominio.EstadoResuelto] != 2 || c[0].PorEstado[dominio.EstadoEnProceso] != 0 {
		t.Errorf("bache primero con su desglose por estado: %+v", c[0])
	}
	if c[2].Nombre != "alumbrado_publico" || c[2].Total != 0 {
		t.Errorf("la categoría activa sin reportes aparece con 0: %+v", c[2])
	}
	if c[3].IDCategoria != nil || c[3].Nombre != "sin_categoria" || c[3].Total != 1 {
		t.Errorf("sin_categoria va al final: %+v", c[3])
	}
	sinFilas := armarCategorias(nil)
	if len(sinFilas) != 1 || sinFilas[0].Nombre != "sin_categoria" || len(sinFilas[0].PorEstado) != 4 {
		t.Errorf("sin datos igual llega sin_categoria con ceros: %+v", sinFilas)
	}
}

func TestFiltrarCeldasOcultaPocosReportes(t *testing.T) {
	e := armarEstadisticas(filtrosPrueba(t, "true"), datosPrueba())
	g := e.Geografica
	if g == nil || len(g.Celdas) != 1 || g.Celdas[0].Total != 5 {
		t.Fatalf("solo la celda con 5 reportes es visible: %+v", g)
	}
	if g.CeldasOcultas != 1 || g.ReportesOcultos != 2 || g.MinimoPorCelda != dominio.MinReportesPorCelda {
		t.Errorf("debe informar lo oculto: %+v", g)
	}
}

func TestEstadisticasSinDatosPersonales(t *testing.T) {
	j := aJSON(t, armarEstadisticas(filtrosPrueba(t, "true"), datosPrueba()))
	for _, prohibido := range []string{"id_usuario", "nombre_autor", "correo", "descripcion", "id_reporte", "20.535"} {
		if strings.Contains(j, prohibido) {
			t.Errorf("las estadísticas no deben incluir %q: %s", prohibido, j)
		}
	}
}
