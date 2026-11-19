package dominio

import (
	"math"
	"strconv"
	"strings"
)

// Límites de la búsqueda de reportes cercanos o duplicados (HU 06.5).
// Los valores por defecto se configuran con DUPLICADOS_RADIO_M y
// DUPLICADOS_VENTANA_DIAS; el administrador puede ajustarlos por consulta.
const (
	RadioMinimoM          = 1
	RadioMaximoM          = 1000
	RadioPorDefectoM      = 50
	VentanaMinimaDias     = 1
	VentanaMaximaDias     = 365
	VentanaPorDefectoDias = 30
	LimiteCercanosDefecto = 20
	LimiteCercanosMaximo  = 50
)

// Peso de cada señal en la puntuación (suman 1).
const (
	pesoDistancia = 0.50 // la cercanía es la señal principal
	pesoCategoria = 0.35 // misma categoría final
	pesoTiempo    = 0.15 // creados dentro de la ventana de tiempo
)

// ParametrosCercania son el radio, la ventana de tiempo y el máximo de resultados.
type ParametrosCercania struct {
	RadioM      int `json:"radio_m"`
	VentanaDias int `json:"ventana_dias"`
	Limite      int `json:"limite"`
}

// CamposCercania son los query params tal como llegan.
type CamposCercania struct {
	RadioM, VentanaDias, Limite string
}

// ParseParametrosCercania aplica los query params sobre los valores por defecto.
func ParseParametrosCercania(c CamposCercania, defecto ParametrosCercania) (ParametrosCercania, []ErrorCampo) {
	p := defecto
	var errs []ErrorCampo
	entero := func(campo, valor string, minimo, maximo int, destino *int) {
		v := strings.TrimSpace(valor)
		if v == "" {
			return
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < minimo || n > maximo {
			errs = append(errs, ErrorCampo{campo, "rango",
				campo + " debe ser un entero entre " + strconv.Itoa(minimo) + " y " + strconv.Itoa(maximo)})
			return
		}
		*destino = n
	}
	entero("radio_m", c.RadioM, RadioMinimoM, RadioMaximoM, &p.RadioM)
	entero("ventana_dias", c.VentanaDias, VentanaMinimaDias, VentanaMaximaDias, &p.VentanaDias)
	entero("limite", c.Limite, 1, LimiteCercanosMaximo, &p.Limite)
	return p, errs
}

// NivelCoincidencia resume qué tan probable es que dos reportes sean el mismo problema.
type NivelCoincidencia string

const (
	CoincidenciaAlta  NivelCoincidencia = "alta"
	CoincidenciaMedia NivelCoincidencia = "media"
	CoincidenciaBaja  NivelCoincidencia = "baja"
)

// SenalesCercania son los datos de un candidato respecto al reporte base.
type SenalesCercania struct {
	DistanciaM     float64
	MismaCategoria *bool   // nil si alguno de los dos no tiene categoría
	DiasDiferencia float64 // con signo; se compara su valor absoluto
}

// Evaluacion es el resultado de comparar un candidato con el reporte base.
type Evaluacion struct {
	DentroVentana bool
	Puntuacion    float64 // 0 a 1, dos decimales
	Nivel         NivelCoincidencia
}

// EvaluarCoincidencia combina las tres señales. La distancia es la principal; la
// categoría y la ventana de tiempo solo la complementan. Es una sugerencia: nunca
// se usa para fusionar ni modificar reportes.
func EvaluarCoincidencia(s SenalesCercania, p ParametrosCercania) Evaluacion {
	cercania := 1 - s.DistanciaM/float64(p.RadioM)
	cercania = math.Max(0, math.Min(1, cercania))

	categoria := 0.0
	if s.MismaCategoria != nil && *s.MismaCategoria {
		categoria = 1
	}

	dias := math.Abs(s.DiasDiferencia)
	dentro := dias <= float64(p.VentanaDias)
	tiempo := 0.0
	if dentro {
		tiempo = 1 - dias/float64(p.VentanaDias)
	}

	puntuacion := pesoDistancia*cercania + pesoCategoria*categoria + pesoTiempo*tiempo
	puntuacion = math.Round(puntuacion*100) / 100

	nivel := CoincidenciaBaja
	switch {
	case puntuacion >= 0.70:
		nivel = CoincidenciaAlta
	case puntuacion >= 0.40:
		nivel = CoincidenciaMedia
	}
	return Evaluacion{DentroVentana: dentro, Puntuacion: puntuacion, Nivel: nivel}
}

// MismaCategoria compara dos categorías; nil si falta alguna.
func MismaCategoria(a, b *int16) *bool {
	if a == nil || b == nil {
		return nil
	}
	igual := *a == *b
	return &igual
}

// PuedeRevisarDuplicados: solo los administradores ven las sugerencias de duplicados.
func (r Rol) PuedeRevisarDuplicados() bool {
	return r == RolAdministrador
}
