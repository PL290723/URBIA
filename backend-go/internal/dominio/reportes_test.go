package dominio

import (
	"strings"
	"testing"
)

func camposValidos() CamposReporte {
	return CamposReporte{
		Descripcion:     "Bache profundo frente a la escuela",
		IDCategoria:     "1",
		Latitud:         "20.5235",
		Longitud:        "-100.8157",
		OrigenUbicacion: "gps",
		PrecisionM:      "12.5",
	}
}

func reglas(errs []ErrorCampo) map[string]string {
	m := map[string]string{}
	for _, e := range errs {
		m[e.Campo] = e.Regla
	}
	return m
}

func TestParseNuevoReporteValido(t *testing.T) {
	n, errs := ParseNuevoReporte(camposValidos())
	if len(errs) > 0 {
		t.Fatalf("no debería tener errores: %+v", errs)
	}
	if n.IDCategoria == nil || *n.IDCategoria != 1 {
		t.Errorf("categoría esperada 1, se obtuvo %v", n.IDCategoria)
	}
	if n.Ubicacion.Origen != OrigenGPS || n.Ubicacion.PrecisionM == nil || *n.Ubicacion.PrecisionM != 12.5 {
		t.Errorf("ubicación GPS mal interpretada: %+v", n.Ubicacion)
	}
}

func TestParseNuevoReporteCamposObligatorios(t *testing.T) {
	_, errs := ParseNuevoReporte(CamposReporte{})
	r := reglas(errs)
	if r["descripcion"] != "requerido" || r["ubicacion"] != "requerido" {
		t.Errorf("faltan errores de obligatorios: %+v", errs)
	}
	if _, ok := r["id_categoria"]; ok {
		t.Error("la categoría es opcional")
	}

	// Con coordenadas pero sin origen, se exige el origen
	c := camposValidos()
	c.OrigenUbicacion = ""
	if _, errs := ParseNuevoReporte(c); reglas(errs)["origen_ubicacion"] != "requerido" {
		t.Errorf("falta el error de origen_ubicacion: %+v", errs)
	}
}

func TestDescripcion(t *testing.T) {
	casos := map[string]string{
		"corta":           "bache",
		"solo espacios":   "            ",
		"demasiado larga": strings.Repeat("a", DescripcionMaxima+1),
		"byte nulo":       "bache grande\x00aquí",
	}
	for nombre, d := range casos {
		c := camposValidos()
		c.Descripcion = d
		if _, errs := ParseNuevoReporte(c); reglas(errs)["descripcion"] == "" {
			t.Errorf("%s: debería rechazarse", nombre)
		}
	}
	// 10 caracteres con acentos (más de 10 bytes) sí es válida
	c := camposValidos()
	c.Descripcion = "  áéíóúñáéí ü  " // 10 caracteres sin los espacios de los extremos
	n, errs := ParseNuevoReporte(c)
	if len(errs) > 0 || n.Descripcion != "áéíóúñáéí ü" {
		t.Errorf("descripción con acentos rechazada: %+v", errs)
	}
}

func TestCoordenadas(t *testing.T) {
	casos := []struct{ lat, lon, campo, regla string }{
		{"91", "-100.8", "latitud", "rango"},
		{"20.5", "-181", "longitud", "rango"},
		{"NaN", "-100.8", "latitud", "numero"},
		{"20.5", "Inf", "longitud", "numero"},
		{"abc", "-100.8", "latitud", "numero"},
		{"20.5", "", "longitud", "requerido"},
		{"0", "0", "ubicacion", "sin_senal"},
	}
	for _, cs := range casos {
		c := camposValidos()
		c.Latitud, c.Longitud = cs.lat, cs.lon
		_, errs := ParseNuevoReporte(c)
		if reglas(errs)[cs.campo] != cs.regla {
			t.Errorf("(%s, %s): se esperaba %s/%s, se obtuvo %+v", cs.lat, cs.lon, cs.campo, cs.regla, errs)
		}
	}
}

func TestCoordenadasSeRedondean(t *testing.T) {
	c := camposValidos()
	c.Latitud, c.Longitud = "20.52351234999", "-100.81571234999"
	n, _ := ParseNuevoReporte(c)
	if n.Ubicacion.Latitud != 20.523512 || n.Ubicacion.Longitud != -100.815712 {
		t.Errorf("redondeo a 6 decimales incorrecto: %+v", n.Ubicacion)
	}
}

func TestPrecisionGPS(t *testing.T) {
	casos := map[string]string{
		"":    "requerido",
		"0":   "numero_positivo",
		"-5":  "numero_positivo",
		"x":   "numero_positivo",
		"250": "precision_insuficiente",
	}
	for p, regla := range casos {
		c := camposValidos()
		c.PrecisionM = p
		if _, errs := ParseNuevoReporte(c); reglas(errs)["precision_m"] != regla {
			t.Errorf("precisión %q: se esperaba %s, se obtuvo %+v", p, regla, errs)
		}
	}
}

func TestUbicacionManual(t *testing.T) {
	c := camposValidos()
	c.OrigenUbicacion = "manual"
	c.PrecisionM = "500" // se ignora: no aplica a una ubicación marcada en el mapa
	n, errs := ParseNuevoReporte(c)
	if len(errs) > 0 {
		t.Fatalf("la ubicación manual debería aceptarse: %+v", errs)
	}
	if n.Ubicacion.Origen != OrigenManual || n.Ubicacion.PrecisionM != nil {
		t.Errorf("ubicación manual mal interpretada: %+v", n.Ubicacion)
	}

	c.OrigenUbicacion = "wifi"
	if _, errs := ParseNuevoReporte(c); reglas(errs)["origen_ubicacion"] != "oneof" {
		t.Errorf("origen desconocido debería rechazarse: %+v", errs)
	}
}

func TestCategoria(t *testing.T) {
	for _, v := range []string{"0", "-1", "abc", "40000"} {
		c := camposValidos()
		c.IDCategoria = v
		if _, errs := ParseNuevoReporte(c); reglas(errs)["id_categoria"] != "entero_positivo" {
			t.Errorf("categoría %q debería rechazarse", v)
		}
	}
	c := camposValidos()
	c.IDCategoria = ""
	if n, errs := ParseNuevoReporte(c); len(errs) > 0 || n.IDCategoria != nil {
		t.Error("sin categoría debería aceptarse con IDCategoria nil")
	}
}

func TestPuedeCrearReporte(t *testing.T) {
	if !RolCiudadano.PuedeCrearReporte() {
		t.Error("el ciudadano debe poder crear reportes")
	}
	if RolAdministrador.PuedeCrearReporte() {
		t.Error("el administrador no crea reportes")
	}
}

func TestEstadoInicial(t *testing.T) {
	if EstadoInicialReporte != EstadoPendiente {
		t.Errorf("el estado inicial debe ser pendiente, es %s", EstadoInicialReporte)
	}
}
