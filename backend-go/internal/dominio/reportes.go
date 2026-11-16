package dominio

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// EstadoInicialReporte: todo reporte nace pendiente (HU 06.1).
const EstadoInicialReporte = EstadoPendiente

// Límites de un reporte. Deben coincidir con los CHECK de la tabla reporte.
const (
	DescripcionMinima  = 10   // ck_reporte_descripcion
	DescripcionMaxima  = 1000 // ck_reporte_descripcion
	PrecisionMaximaGPS = 100  // metros; ck_reporte_precision
	decimalesCoord     = 6    // ~11 cm, más es ruido del GPS
)

// OrigenUbicacion indica de dónde salió la ubicación. Coincide con ck_reporte_origen_ubicacion.
type OrigenUbicacion string

const (
	OrigenGPS    OrigenUbicacion = "gps"
	OrigenManual OrigenUbicacion = "manual" // marcada en el mapa: GPS apagado, sin señal o impreciso
)

// Ubicacion es el punto del problema en WGS 84 (SRID 4326).
type Ubicacion struct {
	Latitud    float64         `json:"latitud"`
	Longitud   float64         `json:"longitud"`
	Origen     OrigenUbicacion `json:"origen"`
	PrecisionM *float64        `json:"precision_m"` // nil cuando el origen es manual
}

// CamposReporte son los valores tal como llegan del formulario (multipart).
type CamposReporte struct {
	Descripcion     string
	IDCategoria     string
	Latitud         string
	Longitud        string
	OrigenUbicacion string
	PrecisionM      string
}

// NuevoReporte son los datos ya validados y normalizados.
type NuevoReporte struct {
	Descripcion string
	IDCategoria *int16 // opcional: si no viene, la asigna después el modelo o un administrador
	Ubicacion   Ubicacion
}

// ErrorCampo describe por qué un campo no es válido. Es el formato de "details" en la respuesta.
type ErrorCampo struct {
	Campo   string `json:"campo"`
	Regla   string `json:"regla"`
	Mensaje string `json:"mensaje"`
}

// PuedeCrearReporte indica si el rol puede registrar reportes (solo ciudadanos).
func (r Rol) PuedeCrearReporte() bool {
	return r == RolCiudadano
}

// ParseNuevoReporte valida todos los campos y devuelve TODOS los errores juntos,
// para que la app pueda marcar cada campo de una vez.
func ParseNuevoReporte(c CamposReporte) (NuevoReporte, []ErrorCampo) {
	var n NuevoReporte
	var errs []ErrorCampo

	desc, e := validarDescripcion(c.Descripcion)
	n.Descripcion = desc
	errs = append(errs, e...)

	cat, e := parseCategoria(c.IDCategoria)
	n.IDCategoria = cat
	errs = append(errs, e...)

	ub, e := parseUbicacion(c.Latitud, c.Longitud, c.OrigenUbicacion, c.PrecisionM)
	n.Ubicacion = ub
	errs = append(errs, e...)

	return n, errs
}

func validarDescripcion(valor string) (string, []ErrorCampo) {
	d := strings.TrimSpace(valor)
	switch {
	case d == "":
		return "", []ErrorCampo{{"descripcion", "requerido", "Describe el problema"}}
	case !utf8.ValidString(d) || strings.ContainsRune(d, 0):
		return "", []ErrorCampo{{"descripcion", "texto_invalido", "La descripción contiene caracteres no válidos"}}
	}
	n := utf8.RuneCountInString(d) // la BD cuenta caracteres, no bytes
	if n < DescripcionMinima || n > DescripcionMaxima {
		return "", []ErrorCampo{{"descripcion", "longitud",
			"La descripción debe tener entre " + strconv.Itoa(DescripcionMinima) +
				" y " + strconv.Itoa(DescripcionMaxima) + " caracteres"}}
	}
	return d, nil
}

func parseCategoria(valor string) (*int16, []ErrorCampo) {
	v := strings.TrimSpace(valor)
	if v == "" {
		return nil, nil
	}
	id, err := strconv.ParseInt(v, 10, 16)
	if err != nil || id <= 0 {
		return nil, []ErrorCampo{{"id_categoria", "entero_positivo", "La categoría debe ser un identificador válido"}}
	}
	id16 := int16(id)
	return &id16, nil
}

func parseUbicacion(latStr, lonStr, origenStr, precStr string) (Ubicacion, []ErrorCampo) {
	var u Ubicacion
	var errs []ErrorCampo

	latStr, lonStr = strings.TrimSpace(latStr), strings.TrimSpace(lonStr)
	if latStr == "" && lonStr == "" {
		// Ausencia total de ubicación: la app debe pedir GPS o que se marque en el mapa.
		return u, []ErrorCampo{{"ubicacion", "requerido",
			"No se recibió la ubicación: activa el GPS o marca el punto en el mapa"}}
	}

	lat, e := parseCoordenada("latitud", latStr, 90)
	errs = append(errs, e...)
	lon, e := parseCoordenada("longitud", lonStr, 180)
	errs = append(errs, e...)
	if len(errs) == 0 && lat == 0 && lon == 0 {
		// (0, 0) está en el océano: es el valor que mandan muchos GPS cuando no tienen señal.
		errs = append(errs, ErrorCampo{"ubicacion", "sin_senal",
			"La ubicación (0, 0) no es válida: marca el punto en el mapa"})
	}
	u.Latitud, u.Longitud = redondear(lat, decimalesCoord), redondear(lon, decimalesCoord)

	switch OrigenUbicacion(strings.TrimSpace(origenStr)) {
	case OrigenManual:
		u.Origen = OrigenManual // la precisión del GPS no aplica y se ignora
	case OrigenGPS:
		u.Origen = OrigenGPS
		p, e := parsePrecision(precStr)
		u.PrecisionM = p
		errs = append(errs, e...)
	case "":
		errs = append(errs, ErrorCampo{"origen_ubicacion", "requerido", "Indica si la ubicación es 'gps' o 'manual'"})
	default:
		errs = append(errs, ErrorCampo{"origen_ubicacion", "oneof", "El origen de la ubicación debe ser 'gps' o 'manual'"})
	}
	return u, errs
}

func parseCoordenada(campo, valor string, limite float64) (float64, []ErrorCampo) {
	if valor == "" {
		return 0, []ErrorCampo{{campo, "requerido", "Falta la " + campo}}
	}
	v, err := strconv.ParseFloat(valor, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, []ErrorCampo{{campo, "numero", "La " + campo + " debe ser un número decimal"}}
	}
	if v < -limite || v > limite {
		return 0, []ErrorCampo{{campo, "rango",
			"La " + campo + " debe estar entre -" + strconv.Itoa(int(limite)) + " y " + strconv.Itoa(int(limite))}}
	}
	return v, nil
}

func parsePrecision(valor string) (*float64, []ErrorCampo) {
	v := strings.TrimSpace(valor)
	if v == "" {
		return nil, []ErrorCampo{{"precision_m", "requerido", "Con ubicación por GPS se requiere la precisión en metros"}}
	}
	p, err := strconv.ParseFloat(v, 64)
	if err != nil || math.IsNaN(p) || math.IsInf(p, 0) || p <= 0 {
		return nil, []ErrorCampo{{"precision_m", "numero_positivo", "La precisión debe ser un número mayor que 0"}}
	}
	if p > PrecisionMaximaGPS {
		return nil, []ErrorCampo{{"precision_m", "precision_insuficiente",
			"El GPS tiene una precisión de " + strconv.FormatFloat(p, 'f', 0, 64) +
				" m (máximo " + strconv.Itoa(PrecisionMaximaGPS) + " m): marca el punto manualmente en el mapa"}}
	}
	p = redondear(p, 2)
	return &p, nil
}

func redondear(v float64, decimales int) float64 {
	f := math.Pow(10, float64(decimales))
	return math.Round(v*f) / f
}
