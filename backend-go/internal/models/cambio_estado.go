package models

import (
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
)

// CambiarEstadoRequest es el cuerpo de PATCH /api/v1/reportes/:id/estado.
type CambiarEstadoRequest struct {
	Estado string `json:"estado" binding:"required"`
}

// CambioEstado es el resultado de un cambio de estado (HU 07.2).
type CambioEstado struct {
	IDReporte          int64            `json:"id_reporte"`
	EstadoAnterior     dominio.Estado   `json:"estado_anterior"`
	EstadoNuevo        dominio.Estado   `json:"estado_nuevo"`
	IDHistorial        int64            `json:"id_historial_estado"`
	CambiadoPor        int64            `json:"cambiado_por"` // administrador que hizo el cambio
	Fecha              time.Time        `json:"fecha"`        // fecha del registro en historial_estado
	FechaActualizacion time.Time        `json:"fecha_actualizacion"`
	EstadosSiguientes  []dominio.Estado `json:"estados_siguientes"` // vacío si ya es final
}
