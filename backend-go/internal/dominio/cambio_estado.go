package dominio

import (
	"fmt"
	"strings"
)

// Cambio de estado administrativo (HU 07.2).
// Las transiciones permitidas son las de la tabla transicion_estado (migración 00003):
// pendiente → en_revision → en_proceso → resuelto. Solo hacia adelante y de uno en uno.

// estadosValidos en el orden del ciclo de vida (para mensajes y respuestas).
var estadosValidos = []Estado{EstadoPendiente, EstadoEnRevision, EstadoEnProceso, EstadoResuelto}

// Siguientes devuelve los estados a los que puede pasar un reporte en el estado e.
// Vacío si es un estado final.
func (e Estado) Siguientes() []Estado {
	return append([]Estado{}, transiciones[e]...)
}

// EsFinal indica que el reporte ya no puede cambiar de estado.
func (e Estado) EsFinal() bool {
	_, existe := transiciones[e]
	return existe && len(transiciones[e]) == 0
}

// ParseEstadoDestino valida el estado que pide el administrador.
func ParseEstadoDestino(valor string) (Estado, []ErrorCampo) {
	v := strings.TrimSpace(valor)
	if v == "" {
		return "", []ErrorCampo{{"estado", "requerido", "Indica el estado al que quieres cambiar el reporte"}}
	}
	e, err := ParseEstado(v)
	if err != nil {
		return "", []ErrorCampo{{"estado", "valor_permitido",
			fmt.Sprintf("Estado no válido; usa uno de: %s", ListaEstados(estadosValidos))}}
	}
	return e, nil
}

// MotivoCambioInvalido explica por qué no se puede pasar de actual a destino.
// Devuelve "" si el cambio es válido.
func MotivoCambioInvalido(actual, destino Estado) string {
	switch {
	case actual == destino:
		return fmt.Sprintf("El reporte ya está en %s", actual)
	case actual.EsFinal():
		return fmt.Sprintf("El reporte está %s; es un estado final y ya no puede cambiar", actual)
	case !actual.PuedeCambiarA(destino):
		return fmt.Sprintf("No se puede pasar de %s a %s; desde %s solo se permite: %s",
			actual, destino, actual, ListaEstados(actual.Siguientes()))
	default:
		return ""
	}
}

// ListaEstados une los estados con comas, para mensajes.
func ListaEstados(es []Estado) string {
	partes := make([]string, len(es))
	for i, e := range es {
		partes[i] = string(e)
	}
	return strings.Join(partes, ", ")
}
