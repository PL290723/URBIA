package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/response"
	"github.com/PL290723/URBIA/backend-go/internal/services"
)

type EstadoHandler struct {
	svc *services.EstadoService
}

func NewEstadoHandler(svc *services.EstadoService) *EstadoHandler {
	return &EstadoHandler{svc: svc}
}

// Cambiar: PATCH /api/v1/reportes/:id/estado  {"estado": "en_revision"}
// El administrador avanza el ciclo de vida del reporte (HU 07.2).
func (h *EstadoHandler) Cambiar(c *gin.Context) {
	sesion, ok := sesionRequerida(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req models.CambiarEstadoRequest
	if !bindJSON(c, &req) {
		return
	}

	cambio, err := h.svc.Cambiar(c.Request.Context(), sesion, id, req.Estado)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, cambio)
}
