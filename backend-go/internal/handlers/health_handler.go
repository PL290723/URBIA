package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/response"
	"github.com/PL290723/URBIA/backend-go/internal/services"
)

type HealthHandler struct{ svc *services.HealthService }

func NewHealthHandler(svc *services.HealthService) *HealthHandler { return &HealthHandler{svc: svc} }

// Check: estado completo (readiness). 503 solo si la base de datos no responde.
func (h *HealthHandler) Check(c *gin.Context) {
	st := h.svc.Check(c.Request.Context())
	if st.Status == models.EstadoCaido {
		response.Unavailable(c, st, "La API no puede atender solicitudes: la base de datos no responde")
		return
	}
	response.OK(c, st)
}

// Live: el proceso está vivo (liveness). No consulta dependencias.
func (h *HealthHandler) Live(c *gin.Context) {
	response.OK(c, gin.H{"status": models.EstadoOK})
}
