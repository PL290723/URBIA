package dominio

// OrigenCategoria indica cómo se llegó a la categoría final del reporte (HU 06.3).
// Coincide con ck_reporte_origen_categoria y ck_clasificacion_origen.
type OrigenCategoria string

const (
	CategoriaSugerida  OrigenCategoria = "sugerida"  // aceptó la categoría que sugirió la IA
	CategoriaCorregida OrigenCategoria = "corregida" // eligió una distinta a la sugerida
	CategoriaManual    OrigenCategoria = "manual"    // no había sugerencia, o la eligió antes de verla
)

// OrigenDeClasificacion decide el origen comparando la categoría elegida con la
// sugerida por la IA (nil si no hubo sugerencia utilizable).
func OrigenDeClasificacion(elegida int16, sugerida *int16) OrigenCategoria {
	switch {
	case sugerida == nil:
		return CategoriaManual
	case *sugerida == elegida:
		return CategoriaSugerida
	default:
		return CategoriaCorregida
	}
}

// PuedeConfirmarCategoria: solo el ciudadano decide la categoría de SU reporte.
// La reclasificación por administradores corresponde a su propio módulo.
func (r Rol) PuedeConfirmarCategoria() bool {
	return r == RolCiudadano
}

// PermiteCambiarCategoria: el ciudadano solo puede cambiarla mientras nadie la revisa.
func (e Estado) PermiteCambiarCategoria() bool {
	return e == EstadoPendiente
}
