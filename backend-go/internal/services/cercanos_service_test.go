package services

import (
	"testing"
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
)

func TestArmarCercanos(t *testing.T) {
	hoy := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	bache := &models.Categoria{ID: 1, Nombre: "bache"}
	basura := &models.Categoria{ID: 3, Nombre: "basura"}
	base := &models.ReporteCercanoFila{ID: 10, IDUsuario: 5, Categoria: bache, FechaCreacion: hoy}
	candidatos := []models.ReporteCercanoFila{ // vienen ordenados por distancia, como de la BD
		{ID: 11, IDUsuario: 6, Categoria: basura, FechaCreacion: hoy, DistanciaM: 3},                      // muy cerca, otra categoría
		{ID: 12, IDUsuario: 5, Categoria: bache, FechaCreacion: hoy.Add(-48 * time.Hour), DistanciaM: 10}, // mismo autor, misma categoría
		{ID: 13, IDUsuario: 7, FechaCreacion: hoy.AddDate(0, -6, 0), DistanciaM: 45},                      // sin categoría, viejo
	}
	p := dominio.ParametrosCercania{RadioM: 50, VentanaDias: 30, Limite: 2}

	r := armarCercanos(base, candidatos, p)

	if r.Total != 3 || len(r.Coincidencias) != 2 {
		t.Fatalf("se esperaban 3 candidatos y 2 tras el límite, se obtuvo %d / %d", r.Total, len(r.Coincidencias))
	}
	primera := r.Coincidencias[0]
	if primera.ID != 12 || primera.Nivel != dominio.CoincidenciaAlta || !primera.MismoAutor || primera.DiasDiferencia != -2 {
		t.Errorf("la coincidencia más fuerte debe ser el 12 (misma categoría y autor): %+v", primera)
	}
	if r.Coincidencias[1].ID != 11 || *r.Coincidencias[1].MismaCategoria {
		t.Errorf("la segunda debe ser el 11 (cerca pero otra categoría): %+v", r.Coincidencias[1])
	}
	if r.Reporte.ID != 10 || r.Parametros != p {
		t.Error("la respuesta debe incluir el reporte base y los parámetros usados")
	}
}

func TestArmarCercanosSinCandidatos(t *testing.T) {
	base := &models.ReporteCercanoFila{ID: 10}
	r := armarCercanos(base, nil, dominio.ParametrosCercania{RadioM: 50, VentanaDias: 30, Limite: 20})
	if r.Total != 0 || r.Coincidencias == nil || len(r.Coincidencias) != 0 {
		t.Errorf("sin candidatos debe devolver una lista vacía (no null): %+v", r)
	}
}
