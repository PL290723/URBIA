package dominio

// ResultadoDeteccion coincide con ck_deteccion_resultado de la tabla deteccion.
type ResultadoDeteccion string

const (
	ResultadoDetectado   ResultadoDeteccion = "detectado"
	ResultadoNoDetectado ResultadoDeteccion = "no_detectado"
	ResultadoError       ResultadoDeteccion = "error" // la IA no respondió: el reporte se conserva igual
)

// EstadoSugerencia es lo que ve el cliente sobre la sugerencia de la IA (HU 06.2).
type EstadoSugerencia string

const (
	SugerenciaSugerida     EstadoSugerencia = "sugerida"      // hay una categoría sugerida
	SugerenciaSinDeteccion EstadoSugerencia = "sin_deteccion" // la IA respondió pero no encontró nada concluyente
	SugerenciaNoDisponible EstadoSugerencia = "no_disponible" // la IA falló o no está configurada
)

// CodigoNoConfigurado: INFERENCE_GRPC_ADDR vacía, la API funciona sin IA.
const CodigoNoConfigurado = "NO_CONFIGURADO"

// EstadoDeSugerencia traduce el resultado guardado al estado que ve el cliente.
// Una clase detectada que no corresponde a ninguna categoría activa no se sugiere.
func EstadoDeSugerencia(r ResultadoDeteccion, categoriaEncontrada bool) EstadoSugerencia {
	switch {
	case r == ResultadoDetectado && categoriaEncontrada:
		return SugerenciaSugerida
	case r == ResultadoDetectado, r == ResultadoNoDetectado:
		return SugerenciaSinDeteccion
	default:
		return SugerenciaNoDisponible
	}
}

// CoincideConCategoria compara la categoría que eligió el ciudadano con la sugerida.
// Devuelve nil si falta alguna de las dos (no hay nada que comparar).
// Es solo informativo: la sugerencia NUNCA reemplaza la categoría del ciudadano.
func CoincideConCategoria(confirmada, sugerida *int16) *bool {
	if confirmada == nil || sugerida == nil {
		return nil
	}
	c := *confirmada == *sugerida
	return &c
}
