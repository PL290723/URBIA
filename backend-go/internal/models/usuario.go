package models

import (
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
)

type Usuario struct {
	ID                 int64       `json:"id_usuario"`
	Nombre             string      `json:"nombre"`
	Correo             string      `json:"correo"`
	HashContrasena     string      `json:"-"` // nunca saldra en el JSON
	Rol                dominio.Rol `json:"rol"`
	FechaRegistro      time.Time   `json:"fecha_registro"`
	FechaActualizacion time.Time   `json:"fecha_actualizacion"`
}
