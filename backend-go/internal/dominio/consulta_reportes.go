package dominio

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// Paginación del listado de reportes (HU 06.4).
const (
	PorPaginaPorDefecto = 20
	PorPaginaMaxima     = 100
	PaginaMaxima        = 10000 // evita OFFSET gigantes
)

// DecimalesUbicacionPublica: lo que ve un ciudadano de los reportes AJENOS.
// 3 decimales ≈ 110 m: suficiente para el mapa, sin señalar una casa exacta.
const DecimalesUbicacionPublica = 3

// zonaLocal interpreta "desde"/"hasta" (AAAA-MM-DD) en hora de Celaya.
// México no tiene horario de verano desde 2022: UTC-6 fijo.
var zonaLocal = time.FixedZone("America/Mexico_City", -6*60*60)

// BBox es el rectángulo visible del mapa, en grados WGS 84.
type BBox struct {
	MinLon, MinLat, MaxLon, MaxLat float64
}

// FiltrosReporte son los filtros ya validados del listado.
type FiltrosReporte struct {
	Pagina    int
	PorPagina int
	Estados   []Estado   // vacío = todos
	Categoria *int16     // nil = todas
	Desde     *time.Time // incluido
	Hasta     *time.Time // excluido (día siguiente a la fecha pedida, 00:00)
	BBox      *BBox
	SoloMios  bool
}

// Offset para la consulta SQL.
func (f FiltrosReporte) Offset() int { return (f.Pagina - 1) * f.PorPagina }

// CamposFiltro son los query params tal como llegan.
type CamposFiltro struct {
	Pagina, PorPagina, Estado, IDCategoria, Desde, Hasta, BBox, Mios string
}

// ParseFiltrosReporte valida los query params del listado y devuelve todos los errores juntos.
func ParseFiltrosReporte(c CamposFiltro) (FiltrosReporte, []ErrorCampo) {
	f := FiltrosReporte{Pagina: 1, PorPagina: PorPaginaPorDefecto}
	var errs []ErrorCampo

	if v := strings.TrimSpace(c.Pagina); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > PaginaMaxima {
			errs = append(errs, ErrorCampo{"pagina", "rango", "La página debe ser un entero entre 1 y " + strconv.Itoa(PaginaMaxima)})
		} else {
			f.Pagina = n
		}
	}
	if v := strings.TrimSpace(c.PorPagina); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > PorPaginaMaxima {
			errs = append(errs, ErrorCampo{"por_pagina", "rango", "por_pagina debe ser un entero entre 1 y " + strconv.Itoa(PorPaginaMaxima)})
		} else {
			f.PorPagina = n
		}
	}

	if v := strings.TrimSpace(c.Estado); v != "" {
		vistos := map[Estado]bool{}
		for _, parte := range strings.Split(v, ",") {
			e, err := ParseEstado(strings.TrimSpace(parte))
			if err != nil {
				errs = append(errs, ErrorCampo{"estado", "oneof",
					"Estado no válido: usa pendiente, en_revision, en_proceso o resuelto (separados por coma)"})
				break
			}
			if !vistos[e] {
				vistos[e] = true
				f.Estados = append(f.Estados, e)
			}
		}
	}

	if v := strings.TrimSpace(c.IDCategoria); v != "" {
		id, err := strconv.ParseInt(v, 10, 16)
		if err != nil || id <= 0 {
			errs = append(errs, ErrorCampo{"id_categoria", "entero_positivo", "La categoría debe ser un identificador válido"})
		} else {
			id16 := int16(id)
			f.Categoria = &id16
		}
	}

	desde, e := parseFecha("desde", c.Desde)
	errs = append(errs, e...)
	hasta, e := parseFecha("hasta", c.Hasta)
	errs = append(errs, e...)
	if hasta != nil {
		h := hasta.AddDate(0, 0, 1) // "hasta" incluye todo ese día
		hasta = &h
	}
	if desde != nil && hasta != nil && !desde.Before(*hasta) {
		errs = append(errs, ErrorCampo{"desde", "orden", "La fecha 'desde' no puede ser posterior a 'hasta'"})
	}
	f.Desde, f.Hasta = desde, hasta

	if v := strings.TrimSpace(c.BBox); v != "" {
		b, err := parseBBox(v)
		if err != nil {
			errs = append(errs, *err)
		} else {
			f.BBox = &b
		}
	}

	if v := strings.TrimSpace(c.Mios); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			errs = append(errs, ErrorCampo{"mios", "booleano", "mios debe ser true o false"})
		} else {
			f.SoloMios = b
		}
	}
	return f, errs
}

func parseFecha(campo, valor string) (*time.Time, []ErrorCampo) {
	v := strings.TrimSpace(valor)
	if v == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", v, zonaLocal)
	if err != nil {
		return nil, []ErrorCampo{{campo, "fecha", "La fecha '" + campo + "' debe tener el formato AAAA-MM-DD"}}
	}
	return &t, nil
}

// parseBBox: "min_lon,min_lat,max_lon,max_lat" (el orden que usan Leaflet y OpenLayers).
func parseBBox(v string) (BBox, *ErrorCampo) {
	invalido := &ErrorCampo{"bbox", "formato",
		"bbox debe ser min_lon,min_lat,max_lon,max_lat con longitudes entre -180 y 180, latitudes entre -90 y 90 y mínimos menores que máximos"}
	partes := strings.Split(v, ",")
	if len(partes) != 4 {
		return BBox{}, invalido
	}
	var n [4]float64
	for i, p := range partes {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return BBox{}, invalido
		}
		n[i] = f
	}
	b := BBox{MinLon: n[0], MinLat: n[1], MaxLon: n[2], MaxLat: n[3]}
	if b.MinLon < -180 || b.MaxLon > 180 || b.MinLat < -90 || b.MaxLat > 90 ||
		b.MinLon >= b.MaxLon || b.MinLat >= b.MaxLat {
		return BBox{}, invalido
	}
	return b, nil
}

// ---------------------------------------------------------------------
// Visibilidad: qué parte de un reporte ve cada quién.
// ---------------------------------------------------------------------

// Vista indica cuánto de un reporte puede ver quien consulta.
type Vista string

const (
	VistaAdministrador Vista = "administrador" // todo, incluidos datos del autor y de la IA
	VistaAutor         Vista = "autor"         // su propio reporte: ubicación exacta, foto, sugerencia, historial
	VistaPublica       Vista = "publica"       // reporte de otro ciudadano: datos del problema, ubicación aproximada
)

// VistaDeReporte decide la vista de un reporte para quien consulta.
func VistaDeReporte(rol Rol, idUsuario, idAutor int64) Vista {
	switch {
	case rol == RolAdministrador:
		return VistaAdministrador
	case idUsuario > 0 && idUsuario == idAutor:
		return VistaAutor
	default:
		return VistaPublica
	}
}

// VePrivado: ubicación exacta, foto, sugerencia de la IA e historial.
// Coincide con dominio.PuedeAccederArchivo (autor o administrador).
func (v Vista) VePrivado() bool { return v == VistaAdministrador || v == VistaAutor }

// VeAdministrativo: autor (nombre, correo), prioridad, origen y precisión de la ubicación, detalle de la IA.
func (v Vista) VeAdministrativo() bool { return v == VistaAdministrador }

// UbicacionAproximada redondea a DecimalesUbicacionPublica.
func UbicacionAproximada(lat, lon float64) (float64, float64) {
	return redondear(lat, DecimalesUbicacionPublica), redondear(lon, DecimalesUbicacionPublica)
}

// AjustarBBoxPublica agranda el rectángulo a la cuadrícula de la ubicación
// aproximada. Así, moviendo el mapa en pasos muy pequeños, un ciudadano no
// puede averiguar la ubicación exacta de un reporte ajeno.
func AjustarBBoxPublica(b BBox) BBox {
	f := math.Pow(10, DecimalesUbicacionPublica)
	const eps = 1e-9 // evita que 20.523 se lea como 20.52299999
	return BBox{
		MinLon: math.Max(-180, math.Floor(b.MinLon*f+eps)/f),
		MinLat: math.Max(-90, math.Floor(b.MinLat*f+eps)/f),
		MaxLon: math.Min(180, math.Ceil(b.MaxLon*f-eps)/f),
		MaxLat: math.Min(90, math.Ceil(b.MaxLat*f-eps)/f),
	}
}
