package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/response"
	"github.com/PL290723/URBIA/backend-go/internal/services"
)

type EstadisticasHandler struct {
	svc *services.EstadisticasService
}

func NewEstadisticasHandler(svc *services.EstadisticasService) *EstadisticasHandler {
	return &EstadisticasHandler{svc: svc}
}

// Calcular: GET /api/v1/estadisticas (HU 07.4)
//
//	desde, hasta     AAAA-MM-DD en hora de Celaya, ambos incluidos (por defecto los últimos 30 días; máx. 366)
//	agrupacion       dia | semana | mes (por defecto según el rango)
//	geo              true = incluir distribución geográfica agregada
//	celda_grados     tamaño de celda con geo=true (0.005 a 0.1; por defecto 0.01 ≈ 1.1 km)
func (h *EstadisticasHandler) Calcular(c *gin.Context) {
	sesion, ok := sesionRequerida(c)
	if !ok {
		return
	}
	filtros, errs := dominio.ParseFiltrosEstadisticas(dominio.CamposEstadisticas{
		Desde:       c.Query("desde"),
		Hasta:       c.Query("hasta"),
		Agrupacion:  c.Query("agrupacion"),
		Geo:         c.Query("geo"),
		CeldaGrados: c.Query("celda_grados"),
	}, h.svc.Ahora())
	if len(errs) > 0 {
		response.ValidationError(c, errs)
		return
	}

	est, err := h.svc.Calcular(c.Request.Context(), sesion, filtros)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, est)
}
