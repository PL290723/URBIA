package dominio

import "fmt"

// Estado es el estado de un reporte. Los valores deben coincidir con el
// DOMAIN estado_reporte de la base de datos (migración 00003).
type Estado string

const (
	EstadoPendiente  Estado = "pendiente"
	EstadoEnRevision Estado = "en_revision"
	EstadoEnProceso  Estado = "en_proceso"
	EstadoResuelto   Estado = "resuelto"
)

// transiciones replica la tabla transicion_estado: estado actual -> estados a los que puede pasar.
var transiciones = map[Estado][]Estado{
	EstadoPendiente:  {EstadoEnRevision},
	EstadoEnRevision: {EstadoEnProceso},
	EstadoEnProceso:  {EstadoResuelto},
	EstadoResuelto:   {},
}

// ParseEstado convierte un texto en Estado y rechaza valores no permitidos.
func ParseEstado(valor string) (Estado, error) {
	e := Estado(valor)
	if _, ok := transiciones[e]; !ok {
		return "", fmt.Errorf("estado no permitido: %q", valor)
	}
	return e, nil
}

// PuedeCambiarA indica si un reporte en el estado e puede pasar al estado destino.
func (e Estado) PuedeCambiarA(destino Estado) bool {
	for _, permitido := range transiciones[e] {
		if permitido == destino {
			return true
		}
	}
	return false
}

// ValidarTransicion devuelve un error si el cambio desde -> hacia no está permitido.
func ValidarTransicion(desde, hacia Estado) error {
	if !desde.PuedeCambiarA(hacia) {
		return fmt.Errorf("transición de estado no permitida: %s → %s", desde, hacia)
	}
	return nil
}
