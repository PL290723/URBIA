package dominio

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Estadísticas básicas para el panel administrativo (HU 07.4).
// Solo devuelven AGREGADOS: nunca ids de usuario, nombres, correos, descripciones
// ni coordenadas exactas.

const (
	DiasEstadisticasPorDefecto = 30  // sin fechas: los últimos 30 días (hoy incluido)
	DiasEstadisticasMaximo     = 366 // rango máximo para no recorrer años de datos

	// Distribución geográfica (opcional, ?geo=true): celdas de una rejilla.
	CeldaGradosDefecto = 0.01 // ≈ 1.1 km
	CeldaGradosMinima  = 0.005
	CeldaGradosMaxima  = 0.1
	// Celdas con menos reportes se ocultan: con 1 o 2 reportes una celda puede
	// señalar la casa de quien reportó (k-anonimato básico).
	MinReportesPorCelda = 3

	// Zona con la que se agrupan los días (la misma de los filtros desde/hasta).
	ZonaHorariaEstadisticas = "America/Mexico_City"
)

// Agrupacion es el tamaño de cada periodo de la serie temporal.
type Agrupacion string

const (
	AgruparDia    Agrupacion = "dia"
	AgruparSemana Agrupacion = "semana" // semanas ISO: empiezan en lunes
	AgruparMes    Agrupacion = "mes"
)

// UnidadSQL es el argumento de date_trunc de PostgreSQL.
func (a Agrupacion) UnidadSQL() string {
	switch a {
	case AgruparSemana:
		return "week"
	case AgruparMes:
		return "month"
	default:
		return "day"
	}
}

// AgrupacionAutomatica elige un tamaño legible para la gráfica según el rango.
func AgrupacionAutomatica(dias int) Agrupacion {
	switch {
	case dias <= 31:
		return AgruparDia
	case dias <= 120:
		return AgruparSemana
	default:
		return AgruparMes
	}
}

// PuedeVerEstadisticas: el panel de estadísticas es solo para administradores.
func (r Rol) PuedeVerEstadisticas() bool { return r == RolAdministrador }

// CamposEstadisticas son los query params tal como llegan.
type CamposEstadisticas struct {
	Desde, Hasta, Agrupacion, Geo, CeldaGrados string
}

// FiltrosEstadisticas son los filtros ya validados.
type FiltrosEstadisticas struct {
	Desde       time.Time // 00:00 de "desde" en hora de Celaya (incluido)
	HastaExcl   time.Time // 00:00 del día siguiente a "hasta" (excluido)
	Dias        int
	Agrupacion  Agrupacion
	Geo         bool
	CeldaGrados float64
}

// DesdeFecha y HastaFecha son las fechas (AAAA-MM-DD) que se muestran y se usan en la serie.
func (f FiltrosEstadisticas) DesdeFecha() string { return f.Desde.Format("2006-01-02") }
func (f FiltrosEstadisticas) HastaFecha() string {
	return f.HastaExcl.AddDate(0, 0, -1).Format("2006-01-02")
}

// ParseFiltrosEstadisticas valida los filtros temporales. ahora se inyecta para probar.
//
//	sin fechas      → últimos 30 días hasta hoy
//	solo "desde"    → desde esa fecha hasta hoy
//	solo "hasta"    → los 30 días que terminan en esa fecha
func ParseFiltrosEstadisticas(c CamposEstadisticas, ahora time.Time) (FiltrosEstadisticas, []ErrorCampo) {
	var errs []ErrorCampo
	f := FiltrosEstadisticas{CeldaGrados: CeldaGradosDefecto}

	hoy := ahora.In(zonaLocal)
	hoy = time.Date(hoy.Year(), hoy.Month(), hoy.Day(), 0, 0, 0, 0, zonaLocal)

	desde, e := parseFecha("desde", c.Desde)
	errs = append(errs, e...)
	hasta, e := parseFecha("hasta", c.Hasta)
	errs = append(errs, e...)
	if len(errs) > 0 {
		return f, errs
	}

	switch {
	case desde == nil && hasta == nil:
		h := hoy
		hasta = &h
		d := hoy.AddDate(0, 0, -(DiasEstadisticasPorDefecto - 1))
		desde = &d
	case hasta == nil:
		h := hoy
		hasta = &h
	case desde == nil:
		d := hasta.AddDate(0, 0, -(DiasEstadisticasPorDefecto - 1))
		desde = &d
	}

	if desde.After(*hasta) {
		return f, []ErrorCampo{{"desde", "orden", "La fecha 'desde' no puede ser posterior a 'hasta'"}}
	}
	f.Desde = *desde
	f.HastaExcl = hasta.AddDate(0, 0, 1)
	f.Dias = int(math.Round(f.HastaExcl.Sub(f.Desde).Hours() / 24))
	if f.Dias > DiasEstadisticasMaximo {
		errs = append(errs, ErrorCampo{"hasta", "rango",
			fmt.Sprintf("El rango no puede superar %d días (pediste %d)", DiasEstadisticasMaximo, f.Dias)})
	}

	switch v := Agrupacion(strings.ToLower(strings.TrimSpace(c.Agrupacion))); v {
	case "":
		f.Agrupacion = AgrupacionAutomatica(f.Dias)
	case AgruparDia, AgruparSemana, AgruparMes:
		f.Agrupacion = v
	default:
		errs = append(errs, ErrorCampo{"agrupacion", "valor_permitido", "agrupacion debe ser dia, semana o mes"})
	}

	if v := strings.TrimSpace(c.Geo); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			errs = append(errs, ErrorCampo{"geo", "booleano", "geo debe ser true o false"})
		}
		f.Geo = b
	}

	if v := strings.TrimSpace(c.CeldaGrados); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || math.IsNaN(n) || n < CeldaGradosMinima || n > CeldaGradosMaxima {
			errs = append(errs, ErrorCampo{"celda_grados", "rango",
				fmt.Sprintf("celda_grados debe estar entre %g y %g", CeldaGradosMinima, CeldaGradosMaxima)})
		} else {
			f.CeldaGrados = n
		}
		if !f.Geo {
			errs = append(errs, ErrorCampo{"celda_grados", "requiere_geo", "celda_grados solo aplica con geo=true"})
		}
	}
	return f, errs
}
