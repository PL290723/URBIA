package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/response"
	"github.com/PL290723/URBIA/backend-go/internal/services"
)

type HealthHandler struct{ svc *services.HealthService }

func NewHealthHandler(svc *services.HealthService) *HealthHandler { return &HealthHandler{svc: svc} }

func (h *HealthHandler) Check(c *gin.Context) {
	st := h.svc.Check(c.Request.Context())
	if st.Database != "up" {
		response.Error(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "La base de datos no responde")
		return
	}
	response.OK(c, st)
}
