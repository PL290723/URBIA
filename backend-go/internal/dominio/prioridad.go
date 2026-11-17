package dominio

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// Cálculo de prioridad explicable (HU 07.1).
//
// La prioridad es PROVISIONAL: los pesos y umbrales son una propuesta inicial que
// debe validarse con datos reales y con el área responsable antes de usarse como
// criterio definitivo. Por eso cada resultado lleva VersionFormula y Provisional.
//
// Regla principal: el sistema NO inventa valores. Un factor sin datos queda como
// "no disponible", no suma ni resta, y su peso se reparte entre los disponibles.
// Si lo disponible pesa menos que la cobertura mínima, no se asigna nivel.

const VersionFormula = "v1-provisional"

// Escala de las evaluaciones que captura el administrador.
const (
	EvaluacionMinima = 1
	EvaluacionMaxima = 5
)

// Valores por defecto (se pueden cambiar con PRIORIDAD_* en el .env).
const (
	PesoGravedadDefecto        = 0.40
	PesoAfectacionDefecto      = 0.30
	PesoRecurrenciaDefecto     = 0.20
	PesoAntiguedadDefecto      = 0.10
	AntiguedadMaxDiasDefecto   = 30   // a partir de aquí la antigüedad vale 1
	RecurrenciaMaximaDefecto   = 4    // a partir de aquí la recurrencia vale 1
	CoberturaMinimaDefecto     = 0.50 // peso mínimo con datos para asignar nivel
	JustificacionAfectacionMin = 10
	JustificacionAfectacionMax = 500
	umbralAlta                 = 67.0
	umbralMedia                = 34.0
	toleranciaSumaPesos        = 0.001
)

// FactorPrioridad identifica cada factor de la fórmula.
type FactorPrioridad string

const (
	FactorGravedad    FactorPrioridad = "gravedad_observable"
	FactorAfectacion  FactorPrioridad = "afectacion_estimada"
	FactorRecurrencia FactorPrioridad = "recurrencia_verificada"
	FactorAntiguedad  FactorPrioridad = "antiguedad"
)

// NivelPrioridad coincide con ck_reporte_prioridad y ck_calculo_prioridad_nivel.
type NivelPrioridad string

const (
	PrioridadBaja  NivelPrioridad = "baja"
	PrioridadMedia NivelPrioridad = "media"
	PrioridadAlta  NivelPrioridad = "alta"
)

// Origen de reporte.prioridad (ck_reporte_origen_prioridad).
const (
	OrigenPrioridadPorDefecto = "por_defecto"
	OrigenPrioridadCalculada  = "calculada"
)

// PesosPrioridad son los pesos de cada factor; deben sumar 1.
type PesosPrioridad struct {
	Gravedad    float64 `json:"gravedad_observable"`
	Afectacion  float64 `json:"afectacion_estimada"`
	Recurrencia float64 `json:"recurrencia_verificada"`
	Antiguedad  float64 `json:"antiguedad"`
}

// Validar comprueba que cada peso esté entre 0 y 1 y que sumen 1.
func (p PesosPrioridad) Validar() error {
	for _, w := range []float64{p.Gravedad, p.Afectacion, p.Recurrencia, p.Antiguedad} {
		if w < 0 || w > 1 {
			return errors.New("cada peso de prioridad debe estar entre 0 y 1")
		}
	}
	suma := p.Gravedad + p.Afectacion + p.Recurrencia + p.Antiguedad
	if math.Abs(suma-1) > toleranciaSumaPesos {
		return fmt.Errorf("los pesos de prioridad deben sumar 1 (suman %.3f)", suma)
	}
	return nil
}

// ConfigPrioridad es la fórmula configurable.
type ConfigPrioridad struct {
	Pesos             PesosPrioridad
	AntiguedadMaxDias int
	RecurrenciaMaxima int
	CoberturaMinima   float64
}

// ConfigPrioridadPorDefecto devuelve la fórmula v1-provisional.
func ConfigPrioridadPorDefecto() ConfigPrioridad {
	return ConfigPrioridad{
		Pesos: PesosPrioridad{
			Gravedad: PesoGravedadDefecto, Afectacion: PesoAfectacionDefecto,
			Recurrencia: PesoRecurrenciaDefecto, Antiguedad: PesoAntiguedadDefecto,
		},
		AntiguedadMaxDias: AntiguedadMaxDiasDefecto,
		RecurrenciaMaxima: RecurrenciaMaximaDefecto,
		CoberturaMinima:   CoberturaMinimaDefecto,
	}
}

// EvaluacionAdmin es lo que el administrador declara después de revisar el reporte.
// nil = no evaluado (no se inventa).
type EvaluacionAdmin struct {
	Gravedad                *int
	Afectacion              *int
	JustificacionAfectacion *string
}

// ValidarEvaluacion revisa rangos y exige justificar la afectación: la afectación
// real (tránsito, peatones, servicios) no puede deducirse solo de la fotografía.
func ValidarEvaluacion(e EvaluacionAdmin) (EvaluacionAdmin, []ErrorCampo) {
	var errs []ErrorCampo
	rango := func(campo string, v *int) {
		if v != nil && (*v < EvaluacionMinima || *v > EvaluacionMaxima) {
			errs = append(errs, ErrorCampo{campo, "rango",
				fmt.Sprintf("%s debe ser un entero entre %d y %d", campo, EvaluacionMinima, EvaluacionMaxima)})
		}
	}
	rango(string(FactorGravedad), e.Gravedad)
	rango(string(FactorAfectacion), e.Afectacion)

	if e.JustificacionAfectacion != nil {
		j := strings.TrimSpace(*e.JustificacionAfectacion)
		e.JustificacionAfectacion = &j
		if j == "" {
			e.JustificacionAfectacion = nil
		}
	}
	if e.JustificacionAfectacion != nil {
		n := len([]rune(*e.JustificacionAfectacion))
		if n < JustificacionAfectacionMin || n > JustificacionAfectacionMax {
			errs = append(errs, ErrorCampo{"justificacion_afectacion", "longitud",
				fmt.Sprintf("La justificación debe tener entre %d y %d caracteres",
					JustificacionAfectacionMin, JustificacionAfectacionMax)})
		}
	}
	if e.Afectacion != nil && e.JustificacionAfectacion == nil {
		errs = append(errs, ErrorCampo{"justificacion_afectacion", "requerido",
			"Indica en qué te basas para estimar la afectación (no basta con la fotografía)"})
	}
	if e.Afectacion == nil && e.JustificacionAfectacion != nil {
		errs = append(errs, ErrorCampo{"afectacion_estimada", "requerido",
			"La justificación solo aplica si se estima la afectación"})
	}
	return e, errs
}

// PuedeCalcularPrioridad: solo los administradores calculan y asignan prioridad.
func (r Rol) PuedeCalcularPrioridad() bool {
	return r == RolAdministrador
}

// PermiteCalcularPrioridad: un reporte resuelto ya no necesita prioridad.
func (e Estado) PermiteCalcularPrioridad() bool {
	return e != EstadoResuelto
}

// DatosPrioridad son las entradas reales del cálculo.
type DatosPrioridad struct {
	Evaluacion EvaluacionAdmin
	// Recurrencias: reportes de OTROS ciudadanos con la misma categoría confirmada,
	// cerca y en la misma ventana de tiempo. nil si no se puede verificar.
	Recurrencias         *int
	IDsRecurrentes       []int64
	RadioM               int
	VentanaDias          int
	DiasAntiguedad       float64
	MotivoSinRecurrencia string // por qué la recurrencia no es verificable
}

// Factor es un renglón de la explicación.
type Factor struct {
	Factor       FactorPrioridad `json:"factor"`
	Peso         float64         `json:"peso"` // peso configurado
	Disponible   bool            `json:"disponible"`
	Valor        *float64        `json:"valor"`         // 0 a 1; null si no hay dato
	PesoEfectivo *float64        `json:"peso_efectivo"` // peso tras repartir los faltantes
	Aporte       *float64        `json:"aporte"`        // puntos (sobre 100) que suma
	Fuente       string          `json:"fuente"`
	Explicacion  string          `json:"explicacion"`
	Detalle      map[string]any  `json:"detalle,omitempty"`
}

// ResultadoPrioridad es el cálculo completo.
type ResultadoPrioridad struct {
	VersionFormula  string            `json:"version_formula"`
	Provisional     bool              `json:"provisional"`
	Puntuacion      float64           `json:"puntuacion"` // 0 a 100
	Nivel           *NivelPrioridad   `json:"nivel"`      // null si los datos no alcanzan
	Suficiente      bool              `json:"datos_suficientes"`
	Cobertura       float64           `json:"cobertura"`
	CoberturaMinima float64           `json:"cobertura_minima"`
	Factores        []Factor          `json:"factores"`
	Faltantes       []FactorPrioridad `json:"factores_faltantes"`
	Resumen         string            `json:"resumen"`
}

// NivelDePuntuacion convierte la puntuación (0-100) en nivel.
func NivelDePuntuacion(p float64) NivelPrioridad {
	switch {
	case p >= umbralAlta:
		return PrioridadAlta
	case p >= umbralMedia:
		return PrioridadMedia
	default:
		return PrioridadBaja
	}
}

// CalcularPrioridad aplica la fórmula. Función pura: se prueba sin BD.
//
//	puntuación = 100 · Σ(peso_i · valor_i) / Σ(peso_i de los factores disponibles)
func CalcularPrioridad(d DatosPrioridad, cfg ConfigPrioridad) ResultadoPrioridad {
	factores := []Factor{
		factorEvaluado(FactorGravedad, cfg.Pesos.Gravedad, d.Evaluacion.Gravedad, nil,
			"Sin evaluación del administrador. La IA solo sugiere la categoría; no mide gravedad."),
		factorEvaluado(FactorAfectacion, cfg.Pesos.Afectacion, d.Evaluacion.Afectacion,
			d.Evaluacion.JustificacionAfectacion,
			"Sin estimación del administrador. La afectación real no se deduce de la fotografía."),
		factorRecurrencia(cfg, d),
		factorAntiguedad(cfg, d.DiasAntiguedad),
	}

	cobertura := 0.0
	for _, f := range factores {
		if f.Disponible {
			cobertura += f.Peso
		}
	}
	cobertura = redondear(cobertura, 2)

	r := ResultadoPrioridad{
		VersionFormula:  VersionFormula,
		Provisional:     true,
		Cobertura:       cobertura,
		CoberturaMinima: cfg.CoberturaMinima,
		Faltantes:       []FactorPrioridad{},
	}

	suma := 0.0
	for i := range factores {
		f := &factores[i]
		if !f.Disponible {
			r.Faltantes = append(r.Faltantes, f.Factor)
			continue
		}
		if cobertura == 0 { // solo pasa si los factores con datos pesan 0
			continue
		}
		efectivo := f.Peso / cobertura
		aporte := 100 * efectivo * *f.Valor
		suma += aporte
		pe, ap := redondear(efectivo, 3), redondear(aporte, 1)
		f.PesoEfectivo, f.Aporte = &pe, &ap
	}
	r.Factores = factores
	r.Puntuacion = redondear(suma, 1)
	r.Suficiente = cobertura+1e-9 >= cfg.CoberturaMinima && cobertura > 0

	if r.Suficiente {
		n := NivelDePuntuacion(r.Puntuacion)
		r.Nivel = &n
		r.Resumen = fmt.Sprintf("Prioridad %s (%.1f/100) con %.0f%% de la fórmula respaldada por datos.",
			n, r.Puntuacion, cobertura*100)
	} else {
		r.Resumen = fmt.Sprintf("Datos insuficientes: solo %.0f%% de la fórmula tiene datos (mínimo %.0f%%). "+
			"No se asigna prioridad; evalúa la gravedad o la afectación.", cobertura*100, cfg.CoberturaMinima*100)
	}
	return r
}

func factorEvaluado(nombre FactorPrioridad, peso float64, eval *int, justificacion *string, sinDato string) Factor {
	f := Factor{Factor: nombre, Peso: peso, Fuente: "evaluacion_administrador"}
	if eval == nil {
		f.Explicacion = sinDato
		return f
	}
	v := redondear(float64(*eval-EvaluacionMinima)/float64(EvaluacionMaxima-EvaluacionMinima), 3)
	f.Disponible, f.Valor = true, &v
	f.Detalle = map[string]any{"evaluacion": *eval, "escala": fmt.Sprintf("%d-%d", EvaluacionMinima, EvaluacionMaxima)}
	f.Explicacion = fmt.Sprintf("El administrador la evaluó en %d de %d.", *eval, EvaluacionMaxima)
	if justificacion != nil {
		f.Detalle["justificacion"] = *justificacion
		f.Explicacion += " Justificación: " + *justificacion
	}
	return f
}

func factorRecurrencia(cfg ConfigPrioridad, d DatosPrioridad) Factor {
	f := Factor{Factor: FactorRecurrencia, Peso: cfg.Pesos.Recurrencia, Fuente: "reportes_cercanos_postgis"}
	if d.Recurrencias == nil {
		f.Explicacion = "No verificable: " + d.MotivoSinRecurrencia
		return f
	}
	n := *d.Recurrencias
	v := redondear(math.Min(float64(n), float64(cfg.RecurrenciaMaxima))/float64(cfg.RecurrenciaMaxima), 3)
	f.Disponible, f.Valor = true, &v
	ids := d.IDsRecurrentes
	if ids == nil {
		ids = []int64{}
	}
	f.Detalle = map[string]any{
		"reportes": n, "ids_reportes": ids, "radio_m": d.RadioM,
		"ventana_dias": d.VentanaDias, "maximo": cfg.RecurrenciaMaxima,
	}
	f.Explicacion = fmt.Sprintf("%d reporte(s) de otros ciudadanos con la misma categoría confirmada "+
		"a ≤ %d m y ≤ %d días (con %d o más vale el máximo).", n, d.RadioM, d.VentanaDias, cfg.RecurrenciaMaxima)
	return f
}

func factorAntiguedad(cfg ConfigPrioridad, dias float64) Factor {
	dias = math.Max(0, dias)
	v := redondear(math.Min(dias, float64(cfg.AntiguedadMaxDias))/float64(cfg.AntiguedadMaxDias), 3)
	return Factor{
		Factor: FactorAntiguedad, Peso: cfg.Pesos.Antiguedad, Fuente: "fecha_creacion",
		Disponible: true, Valor: &v,
		Detalle: map[string]any{"dias": redondear(dias, 1), "maximo_dias": cfg.AntiguedadMaxDias},
		Explicacion: fmt.Sprintf("El reporte tiene %.1f día(s) abierto (con %d o más vale el máximo).",
			dias, cfg.AntiguedadMaxDias),
	}
}
