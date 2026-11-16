package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/response"
	"github.com/PL290723/URBIA/backend-go/internal/services"
)

type CercanosHandler struct {
	svc *services.CercanosService
}

func NewCercanosHandler(svc *services.CercanosService) *CercanosHandler {
	return &CercanosHandler{svc: svc}
}

// Buscar: GET /api/v1/reportes/:id/cercanos?radio_m=50&ventana_dias=30&limite=20
// Sin parámetros usa DUPLICADOS_RADIO_M y DUPLICADOS_VENTANA_DIAS.
func (h *CercanosHandler) Buscar(c *gin.Context) {
	sesion, ok := sesionRequerida(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	params, errs := dominio.ParseParametrosCercania(dominio.CamposCercania{
		RadioM:      c.Query("radio_m"),
		VentanaDias: c.Query("ventana_dias"),
		Limite:      c.Query("limite"),
	}, h.svc.Defecto())
	if len(errs) > 0 {
		response.ValidationError(c, errs)
		return
	}

	resultado, err := h.svc.Buscar(c.Request.Context(), sesion, id, params)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, resultado)
}
