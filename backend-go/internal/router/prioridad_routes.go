package router

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/handlers"
	"github.com/PL290723/URBIA/backend-go/internal/middleware"
)

// La prioridad la calcula y la consulta solo el administrador (HU 07.1).
func registerPrioridadRoutes(rg *gin.RouterGroup, h *handlers.PrioridadHandler, requireAuth gin.HandlerFunc) {
	soloAdmin := middleware.RequirePermiso(dominio.Rol.PuedeCalcularPrioridad)
	rg.POST("/reportes/:id/prioridad", requireAuth, soloAdmin, h.Calcular)
	rg.GET("/reportes/:id/prioridad", requireAuth, soloAdmin, h.Consultar)
}
