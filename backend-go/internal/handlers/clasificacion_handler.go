package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/response"
	"github.com/PL290723/URBIA/backend-go/internal/services"
)

type ClasificacionHandler struct {
	svc *services.ClasificacionService
}

func NewClasificacionHandler(svc *services.ClasificacionService) *ClasificacionHandler {
	return &ClasificacionHandler{svc: svc}
}

// Confirmar: PUT /api/v1/reportes/:id/categoria  {"id_categoria": 3}
// El ciudadano acepta la sugerencia de la IA o la corrige (HU 06.3).
func (h *ClasificacionHandler) Confirmar(c *gin.Context) {
	sesion, ok := sesionRequerida(c)
	if !ok {
		return
	}
	idReporte, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req models.ConfirmarCategoriaRequest
	if !bindJSON(c, &req) {
		return
	}

	clasificacion, err := h.svc.Confirmar(c.Request.Context(), sesion, idReporte, req.IDCategoria)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, clasificacion)
}
