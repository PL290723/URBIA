package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/response"
	"github.com/PL290723/URBIA/backend-go/internal/services"
)

type PrioridadHandler struct {
	svc *services.PrioridadService
}

func NewPrioridadHandler(svc *services.PrioridadService) *PrioridadHandler {
	return &PrioridadHandler{svc: svc}
}

// Calcular: POST /api/v1/reportes/:id/prioridad
// Cuerpo opcional: {"gravedad_observable": 4, "afectacion_estimada": 3, "justificacion_afectacion": "..."}
// Sin cuerpo recalcula con los datos actuales y la evaluación anterior (HU 07.1).
func (h *PrioridadHandler) Calcular(c *gin.Context) {
	sesion, ok := sesionRequerida(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req models.CalcularPrioridadRequest
	if c.Request.ContentLength != 0 && !bindJSON(c, &req) {
		return
	}

	calculo, err := h.svc.Calcular(c.Request.Context(), sesion, id, req)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.Created(c, calculo)
}

// Consultar: GET /api/v1/reportes/:id/prioridad
// Último cálculo con su explicación y si quedó desactualizado.
func (h *PrioridadHandler) Consultar(c *gin.Context) {
	sesion, ok := sesionRequerida(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	resultado, err := h.svc.Consultar(c.Request.Context(), sesion, id)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, resultado)
}
