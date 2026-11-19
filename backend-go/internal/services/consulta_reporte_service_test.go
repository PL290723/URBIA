package services

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
)

func filaDePrueba() *models.FilaReporte {
	clase, conf, modelo := "basura", 0.91, "urbia-yolo26s"
	precision := 12.5
	origenCat := "sugerida"
	ahora := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	return &models.FilaReporte{
		ID: 12, IDUsuario: 5, NombreAutor: "Ana Prueba", CorreoAutor: "ana@test.mx",
		Estado: dominio.EstadoPendiente, Prioridad: "media",
		Descripcion: "Basura acumulada en la esquina",
		Categoria:   &models.Categoria{ID: 3, Nombre: "basura"}, OrigenCategoria: &origenCat,
		Latitud: 20.523512, Longitud: -100.815788, OrigenUbicacion: "gps", PrecisionM: &precision,
		FechaCreacion: ahora, FechaActualizacion: ahora,
		Foto:              &models.Archivo{ID: 30, MIME: "image/jpeg", Tamano: 1234, FechaSubida: ahora},
		Deteccion:         &models.Deteccion{ID: 9, Resultado: dominio.ResultadoDetectado, ClaseDetectada: &clase, Confianza: &conf, Modelo: &modelo},
		CategoriaSugerida: &models.Categoria{ID: 3, Nombre: "basura"},
		Historial:         []models.HistorialEstado{{EstadoNuevo: dominio.EstadoPendiente, Fecha: ahora}},
	}
}

func aJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDetalleVistaPublica(t *testing.T) {
	d := detalleDe(filaDePrueba(), dominio.VistaPublica, 99)
	if !d.Ubicacion.Aproximada || d.Ubicacion.Latitud != 20.524 || d.Ubicacion.Longitud != -100.816 {
		t.Errorf("la ubicación pública debe ser aproximada: %+v", d.Ubicacion)
	}
	if d.EsPropio {
		t.Error("no es propio")
	}
	j := aJSON(t, d)
	for _, prohibido := range []string{"ana@test.mx", "Ana Prueba", "foto", "sugerencia", "historial",
		"administrativo", "prioridad", "precision_m", "20.523512"} {
		if strings.Contains(j, prohibido) {
			t.Errorf("la vista pública no debe contener %q: %s", prohibido, j)
		}
	}
}

func TestDetalleVistaAutor(t *testing.T) {
	d := detalleDe(filaDePrueba(), dominio.VistaAutor, 5)
	if d.Ubicacion.Aproximada || d.Ubicacion.Latitud != 20.523512 {
		t.Errorf("el autor ve su ubicación exacta: %+v", d.Ubicacion)
	}
	if !d.EsPropio || d.Foto == nil || d.Foto.URL != "/api/v1/archivos/30" {
		t.Errorf("el autor ve su foto: %+v", d.Foto)
	}
	if d.Sugerencia == nil || d.Sugerencia.Estado != dominio.SugerenciaSugerida || *d.Sugerencia.Confianza != 0.91 {
		t.Errorf("el autor ve la sugerencia: %+v", d.Sugerencia)
	}
	if len(d.Historial) != 1 {
		t.Error("el autor ve el historial")
	}
	j := aJSON(t, d)
	for _, prohibido := range []string{"ana@test.mx", "administrativo", "prioridad", "urbia-yolo26s"} {
		if strings.Contains(j, prohibido) {
			t.Errorf("el autor no ve datos administrativos (%q): %s", prohibido, j)
		}
	}
}

func TestDetalleVistaAdministrador(t *testing.T) {
	d := detalleDe(filaDePrueba(), dominio.VistaAdministrador, 1)
	if d.Admin == nil || d.Admin.Autor.Correo != "ana@test.mx" || d.Admin.Prioridad != "media" {
		t.Fatalf("el administrador ve los datos administrativos: %+v", d.Admin)
	}
	if d.Admin.Deteccion == nil || *d.Admin.Deteccion.Modelo != "urbia-yolo26s" {
		t.Error("el administrador ve el detalle de la detección")
	}
	if d.Foto == nil || d.Ubicacion.Aproximada {
		t.Error("el administrador ve foto y ubicación exacta")
	}
}

func TestResumenPorVista(t *testing.T) {
	f := filaDePrueba()
	pub := resumenDe(f, dominio.VistaPublica, 99)
	if pub.IDFoto != nil || pub.Admin != nil || !pub.Ubicacion.Aproximada {
		t.Errorf("resumen público con datos de más: %+v", pub)
	}
	aut := resumenDe(f, dominio.VistaAutor, 5)
	if aut.IDFoto == nil || *aut.IDFoto != 30 || aut.Admin != nil || !aut.EsPropio {
		t.Errorf("resumen del autor incorrecto: %+v", aut)
	}
	adm := resumenDe(f, dominio.VistaAdministrador, 1)
	if adm.Admin == nil || adm.Admin.Deteccion != nil {
		t.Errorf("el resumen de admin lleva datos administrativos pero no la detección: %+v", adm.Admin)
	}
}

func TestSugerenciaSinDeteccion(t *testing.T) {
	f := filaDePrueba()
	f.Deteccion.Resultado, f.CategoriaSugerida = dominio.ResultadoError, nil
	d := detalleDe(f, dominio.VistaAutor, 5)
	if d.Sugerencia.Estado != dominio.SugerenciaNoDisponible || d.Sugerencia.Confianza != nil {
		t.Errorf("con error de IA no hay sugerencia: %+v", d.Sugerencia)
	}
	f.Deteccion = nil
	if d := detalleDe(f, dominio.VistaAutor, 5); d.Sugerencia != nil {
		t.Error("reporte sin detección: sin bloque de sugerencia")
	}
}
