package dominio

import "fmt"

// VarianteArchivo distingue la foto original del ciudadano de la imagen
// procesada por el modelo. Debe coincidir con ck_archivo_variante (migración 00004).
type VarianteArchivo string

const (
	VarianteOriginal  VarianteArchivo = "original"
	VarianteProcesada VarianteArchivo = "procesada"
)

// TamanoMaximoArchivo coincide con ck_archivo_tamano (10 MiB).
const TamanoMaximoArchivo int64 = 10 << 20

// Límites de dimensiones de las fotografías (HU 04.2).
const (
	LadoMinimoImagen   = 200  // px; más chica no sirve para la inferencia
	LadoMaximoImagen   = 8000 // px
	MegapixelesMaximos = 25   // cubre cámaras de celular de hasta 24 MP

	// MIMEImagenNormalizada: toda foto original se re-codifica a JPEG sin metadatos.
	MIMEImagenNormalizada = "image/jpeg"
)

// extensiones: formatos aceptados y la extensión con la que se guardan.
// La extensión SIEMPRE sale de aquí, nunca del nombre que envía el usuario.
var extensiones = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

// ExtensionPorMIME devuelve la extensión para un MIME permitido.
func ExtensionPorMIME(mime string) (string, bool) {
	ext, ok := extensiones[mime]
	return ext, ok
}

// ParseVariante convierte un texto en VarianteArchivo y rechaza valores no permitidos.
func ParseVariante(valor string) (VarianteArchivo, error) {
	v := VarianteArchivo(valor)
	if v.Carpeta() == "" {
		return "", fmt.Errorf("variante no permitida: %q", valor)
	}
	return v, nil
}

// Carpeta devuelve el subdirectorio del almacenamiento ("" si la variante no es válida).
func (v VarianteArchivo) Carpeta() string {
	switch v {
	case VarianteOriginal:
		return "originales"
	case VarianteProcesada:
		return "procesadas"
	default:
		return ""
	}
}

// PuedeAccederArchivo es la política de acceso: las imágenes NO son públicas.
// Solo el autor del reporte o un administrador pueden verlas o eliminarlas.
func PuedeAccederArchivo(rol Rol, idUsuario, idAutorReporte int64) bool {
	return rol == RolAdministrador || (idUsuario > 0 && idUsuario == idAutorReporte)
}
