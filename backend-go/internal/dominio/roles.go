package dominio

import "fmt"

// Rol es el rol de un usuario. Los valores deben coincidir con
// el CHECK ck_usuario_rol de la base de datos.
type Rol string

const (
	RolCiudadano     Rol = "ciudadano"
	RolAdministrador Rol = "administrador"
)

// ParseRol convierte un texto en Rol y rechaza valores no permitidos.
func ParseRol(valor string) (Rol, error) {
	switch r := Rol(valor); r {
	case RolCiudadano, RolAdministrador:
		return r, nil
	default:
		return "", fmt.Errorf("rol no permitido: %q", valor)
	}
}

// PuedeCambiarEstado indica si el rol puede cambiar el estado de un reporte.
func (r Rol) PuedeCambiarEstado() bool {
	return r == RolAdministrador
}
