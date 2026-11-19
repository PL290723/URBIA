package dominio

// Historial de estados (HU 07.3).
// historial_estado es de solo inserción (migración 00009): la API solo lo consulta.

// TipoEvento distingue la creación del reporte de un cambio administrativo.
type TipoEvento string

const (
	EventoCreacion     TipoEvento = "creacion"      // estado_anterior NULL: el ciudadano creó el reporte
	EventoCambioEstado TipoEvento = "cambio_estado" // un administrador avanzó el estado (HU 07.2)
)

// TipoDeEvento se deduce del estado anterior: sin anterior es la creación.
func TipoDeEvento(anterior *Estado) TipoEvento {
	if anterior == nil {
		return EventoCreacion
	}
	return EventoCambioEstado
}

// VeActorCompleto: solo el administrador ve quién hizo cada cambio (id y nombre).
// El autor ve el historial de su reporte, pero solo el rol del actor.
func (v Vista) VeActorCompleto() bool { return v == VistaAdministrador }
